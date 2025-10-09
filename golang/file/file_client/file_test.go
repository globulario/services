//go:build !js

package file_client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/globulario/services/golang/file/filepb"
)

// startTestFileService spins up a gRPC server hosting the file service bound to a temp root.
// It returns the client, temp root, a stop func, and any error.
func startTestFileService(t *testing.T) (filepb.FileServiceClient, string, func(), error) {
	t.Helper()

	// Create temp root
	root := t.TempDir()

	// Ensure predictable separators
	root = strings.ReplaceAll(root, "\\", "/")

	// Instantiate service server (from package main)
	s := &server{
		Root:   root,
		Domain: "localhost",
	}
	// Some tests expect thumbnails / playlist generation not to panic
	srv := grpc.NewServer()
	filepb.RegisterFileServiceServer(srv, s)

	// Listen on a random loopback port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", nil, err
	}
	go func() {
		_ = srv.Serve(lis)
	}()

	// Create client
	cc, err := grpc.Dial(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, "", nil, err
	}
	client := filepb.NewFileServiceClient(cc)

	stop := func() {
		cc.Close()
		srv.Stop()
		_ = os.RemoveAll(root)
	}

	return client, root, stop, nil
}

func writeFile(t *testing.T, p string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readAllStream(t *testing.T, stream any) []byte {
	t.Helper()
	var out bytes.Buffer
	switch st := stream.(type) {
	case filepb.FileService_ReadFileClient:
		for {
			chunk, err := st.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("read stream: %v", err)
			}
			out.Write(chunk.GetData())
		}
	case filepb.FileService_GetThumbnailsClient:
		for {
			chunk, err := st.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("read stream: %v", err)
			}
			out.Write(chunk.GetData())
		}
	default:
		t.Fatalf("unsupported stream type %T", stream)
	}
	return out.Bytes()
}

func TestFileService_SaveReadDelete(t *testing.T) {
	c, root, stop, err := startTestFileService(t)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	path := "/docs/hello.txt"
	want := []byte("hello globular\n")

	// --- SaveFile (client-streaming) ---
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	up, err := c.SaveFile(ctx)
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	if err := up.Send(&filepb.SaveFileRequest{File: &filepb.SaveFileRequest_Path{Path: path}}); err != nil {
		t.Fatalf("send path: %v", err)
	}
	if err := up.Send(&filepb.SaveFileRequest{File: &filepb.SaveFileRequest_Data{Data: want[:6]}}); err != nil {
		t.Fatalf("send data1: %v", err)
	}
	if err := up.Send(&filepb.SaveFileRequest{File: &filepb.SaveFileRequest_Data{Data: want[6:]}}); err != nil {
		t.Fatalf("send data2: %v", err)
	}
	_, err = up.CloseAndRecv()
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	// --- ReadFile (server-streaming) ---
	rctx, rcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer rcancel()
	st, err := c.ReadFile(rctx, &filepb.ReadFileRequest{Path: path})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := readAllStream(t, st)
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch\ngot:  %q\nwant: %q", got, want)
	}

	// --- GetFileInfo (unary) ---
	gi, err := c.GetFileInfo(context.Background(), &filepb.GetFileInfoRequest{Path: path})
	if err != nil {
		t.Fatalf("GetFileInfo: %v", err)
	}
	if gi.GetInfo() == nil || gi.GetInfo().GetIsDir() {
		t.Fatalf("expected a file, got: %#v", gi.GetInfo())
	}

	// --- DeleteFile (unary) ---
	if _, err := c.DeleteFile(context.Background(), &filepb.DeleteFileRequest{Path: path}); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, stat err=%v", err)
	}
}

func TestFileService_ReadDirAndThumbnails(t *testing.T) {
	c, root, stop, err := startTestFileService(t)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	// Create layout:
	// /media/a.txt
	// /media/img/pic.png  (fake bytes)
	writeFile(t, filepath.Join(root, "media", "a.txt"), []byte("A"))
	writeFile(t, filepath.Join(root, "media", "img", "pic.png"), []byte{0x89, 0x50, 0x4E, 0x47})

	// --- ReadDir (non-recursive) ---
	rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rd, err := c.ReadDir(rctx, &filepb.ReadDirRequest{Path: "/media", Recursive: false, ThumbnailHeight: 32, ThumbnailWidth: 32})
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	count := 0
	for {
		msg, err := rd.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadDir recv: %v", err)
		}
		if msg.GetInfo().GetPath() == "" {
			t.Fatalf("missing path in info: %#v", msg.GetInfo())
		}
		count++
	}
	if count == 0 {
		t.Fatalf("expected at least one entry, got 0")
	}

	// --- GetThumbnails (recursive) ---
	tctx, tcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer tcancel()
	th, err := c.GetThumbnails(tctx, &filepb.GetThumbnailsRequest{Path: "/media", Recursive: true, ThumbnailHeight: 32, ThumbnailWidth: 32})
	if err != nil {
		t.Fatalf("GetThumbnails: %v", err)
	}
	raw := readAllStream(t, th)

	// Thumbnails API streams JSON array chunks; ensure valid JSON overall when concatenated.
	var js any
	if err := json.Unmarshal(raw, &js); err != nil {
		t.Fatalf("thumbnails json: %v\nraw: %s", err, string(raw))
	}
}

func TestFileService_CreateDir_Move_Copy_Rename(t *testing.T) {
	c, root, stop, err := startTestFileService(t)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	// CreateDir
	if _, err := c.CreateDir(context.Background(), &filepb.CreateDirRequest{Path: "/alpha/beta"}); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	if st, err := os.Stat(filepath.Join(root, "alpha", "beta")); err != nil || !st.IsDir() {
		t.Fatalf("dir not created, err=%v", err)
	}

	// Save a file we will operate on
	src := "/alpha/beta/file.txt"
	data := []byte("XYZ")
	up, _ := c.SaveFile(context.Background())
	_ = up.Send(&filepb.SaveFileRequest{File: &filepb.SaveFileRequest_Path{Path: src}})
	_ = up.Send(&filepb.SaveFileRequest{File: &filepb.SaveFileRequest_Data{Data: data}})
	_, _ = up.CloseAndRecv()

	// Move
	dst := "/alpha/file.txt"
	if _, err := c.Move(context.Background(), &filepb.MoveRequest{Oldpath: src, Newpath: dst}); err != nil {
		t.Fatalf("Move: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, strings.TrimPrefix(src, "/"))); !os.IsNotExist(err) {
		t.Fatalf("move: source still exists")
	}
	if _, err := os.Stat(filepath.Join(root, strings.TrimPrefix(dst, "/"))); err != nil {
		t.Fatalf("move: dest missing: %v", err)
	}

	// Copy
	cp := "/alpha/file_copy.txt"
	if _, err := c.Copy(context.Background(), &filepb.CopyRequest{Src: dst, Dst: cp}); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, strings.TrimPrefix(cp, "/"))); err != nil {
		t.Fatalf("copy: dest missing: %v", err)
	}

	// Rename
	rn := "/alpha/renamed.txt"
	if _, err := c.Rename(context.Background(), &filepb.RenameRequest{Oldpath: dst, Newpath: rn}); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, strings.TrimPrefix(rn, "/"))); err != nil {
		t.Fatalf("rename: dest missing: %v", err)
	}
}

func TestFileService_CreateLnk_HtmlToPdf_WriteExcelFile(t *testing.T) {
	c, root, stop, err := startTestFileService(t)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	// CreateLnk
	if _, err := c.CreateDir(context.Background(), &filepb.CreateDirRequest{Path: "/links"}); err != nil {
		t.Fatalf("CreateDir: %v", err)
	}
	lnkReq := &filepb.CreateLnkRequest{Path: "/links", Name: "example.lnk", Lnk: "https://globular.io"}
	if _, err := c.CreateLnk(context.Background(), lnkReq); err != nil {
		t.Fatalf("CreateLnk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "links", "example.lnk")); err != nil {
		t.Fatalf("link not created: %v", err)
	}

	// HtmlToPdf
	pdf, err := c.HtmlToPdf(context.Background(), &filepb.HtmlToPdfRqst{Html: "<html><body><h1>Hi</h1></body></html>"})
	if err != nil {
		t.Fatalf("HtmlToPdf: %v", err)
	}
	if len(pdf.GetPdf()) == 0 || !bytes.HasPrefix(pdf.GetPdf(), []byte("%PDF")) {
		t.Fatalf("expected PDF bytes, got len=%d", len(pdf.GetPdf()))
	}

	// WriteExcelFile
	xlsPath := "/report.xlsx"
	jsonData := `[{"name":"Alice","age":30},{"name":"Bob","age":40}]`
	if _, err := c.WriteExcelFile(context.Background(), &filepb.WriteExcelFileRequest{Path: xlsPath, Data: jsonData}); err != nil {
		t.Fatalf("WriteExcelFile: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(root, "report.xlsx")); err != nil || fi.Size() == 0 {
		t.Fatalf("xlsx not created properly: err=%v size=%v", err, fi.Size())
	}
}

func TestFileService_Errors(t *testing.T) {
	c, _, stop, err := startTestFileService(t)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	// ReadDir on missing path
	_, err = c.ReadDir(context.Background(), &filepb.ReadDirRequest{Path: "/nope"})
	if err == nil {
		t.Fatalf("expected error for missing dir")
	}

	// ReadFile on missing file
	_, err = c.ReadFile(context.Background(), &filepb.ReadFileRequest{Path: "/missing.txt"})
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}