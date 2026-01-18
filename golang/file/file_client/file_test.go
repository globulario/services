//go:build !js

package file_client

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/file/filepb"
	"github.com/globulario/services/golang/testutil"
)

// ---------- Test harness (connect to an already-running server) ----------

type testEnv struct {
	Domain string
	Client *File_Client
	Auth   *authentication_client.Authentication_Client
	Token  string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	domain := testutil.GetDomain()
	address := testutil.GetAddress()

	// IMPORTANT: the file client id must be the File service id
	client, err := NewFileService_Client(address, "file.FileService")
	if err != nil {
		t.Fatalf("file client: %v", err)
	}

	auth, err := authentication_client.NewAuthenticationService_Client(address, "authentication.AuthenticationService")
	if err != nil {
		t.Fatalf("auth client: %v", err)
	}

	// Credentials from environment or defaults.
	saUser, saPass := testutil.GetSACredentials()
	token, err := auth.Authenticate(saUser, saPass)
	if err != nil {
		t.Fatalf("authenticate sa: %v", err)
	}

	return &testEnv{
		Domain: domain,
		Client: client,
		Auth:   auth,
		Token:  token,
	}
}

func (e *testEnv) close() {
	// nothing yet
}

func mustNoErr(t *testing.T, step string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", step, err)
	}
}

// ---------- Helpers adapted to *File_Client API ----------

// createDir makes sure a full path like "/alpha/beta" exists by creating each segment.
func createDirAll(t *testing.T, c *File_Client, token, full string) {
	t.Helper()
	full = filepath.Clean(full)
	if full == "" || full == "." || full == string(os.PathSeparator) {
		return
	}
	parts := strings.Split(full, "/")
	cur := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		parent := cur
		if parent == "" {
			parent = "/"
		}
		err := c.CreateDir(token, parent, p)
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already") {
			t.Fatalf("CreateDir %s/%s: %v", parent, p, err)
		}
		if cur == "" {
			cur = "/" + p
		} else {
			cur = cur + "/" + p
		}
	}
}

// saveFileViaUpload writes data to a local tmp file then uploads it to `dest`
// using MoveFile(token, localPath, destPath), which is the upload path your client exposes.
func saveFileViaUpload(t *testing.T, c *File_Client, token, dest string, data []byte) {
	t.Helper()

	// Ensure parent dirs exist on server
	parent := filepath.Dir(dest)
	if parent == "." {
		parent = "/"
	}
	createDirAll(t, c, token, parent)

	tmp, err := os.CreateTemp("", "fileclient_upload_*")
	mustNoErr(t, "create temp", err)
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	mustNoErr(t, "write temp", err)
	_ = tmp.Close()

	err = c.MoveFile(token, tmp.Name(), dest)
	mustNoErr(t, "MoveFile(upload)", err)
}


// ---------- End-to-end happy paths using the wrapper API ----------

func TestFileClient_SaveReadDelete(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()
	c := env.Client

	path := "/tests/docs/hello.txt"
	want := []byte("hello globular\n")

	// Save (upload) using MoveFile wrapper
	saveFileViaUpload(t, c, env.Token, path, want)

	// ReadFile wrapper returns []byte directly
	got, err := c.ReadFile(env.Token, path)
	mustNoErr(t, "ReadFile", err)
	if !bytes.Equal(got, want) {
		t.Fatalf("content mismatch\ngot:  %q\nwant: %q", got, want)
	}

	// GetFileInfo (ensure it's a file)
	gi, err := c.GetFileInfo(env.Token, path, false, 0, 0)
	mustNoErr(t, "GetFileInfo(file)", err)
	if gi == nil || gi.GetIsDir() {
		t.Fatalf("expected a file, got: %#v", gi)
	}

	// DeleteFile via wrapper, then GetFileInfo should fail
	err = c.DeleteFile(env.Token, path)
	mustNoErr(t, "DeleteFile", err)

	_, err = c.GetFileInfo(env.Token, path, false, 0, 0)
	if err == nil {
		t.Fatalf("expected GetFileInfo error on deleted file")
	}
}

func TestFileClient_ReadDirAndThumbnails(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()
	c := env.Client

	// Create structure via API:
	// /media/a.txt
	// /media/img/pic.png (fake bytes)
	createDirAll(t, c, env.Token, "/media")
	createDirAll(t, c, env.Token, "/media/img")
	saveFileViaUpload(t, c, env.Token, "/media/a.txt", []byte("A"))
	saveFileViaUpload(t, c, env.Token, "/media/img/pic.png", []byte{0x89, 0x50, 0x4E, 0x47})

	// ReadDir (non-recursive) -> slice
	list, err := c.ReadDir("/media", false, 32, 32)
	mustNoErr(t, "ReadDir", err)
	if len(list) == 0 {
		t.Fatalf("expected at least one entry, got 0")
	}
	for _, fi := range list {
		if fi.GetPath() == "" {
			t.Fatalf("missing path in info: %#v", fi)
		}
	}

	// GetThumbnails (recursive) -> JSON string
	js, err := c.GetThumbnails("/media", true, 32, 32)
	mustNoErr(t, "GetThumbnails", err)

	var anyJSON any
	if err := json.Unmarshal([]byte(js), &anyJSON); err != nil {
		t.Fatalf("thumbnails json: %v\nraw: %s", err, js)
	}
}

func TestFileClient_HtmlToPdf(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()
	c := env.Client

	pdf, err := c.HtmlToPdf("<html><body><h1>Hi</h1></body></html>")
	mustNoErr(t, "HtmlToPdf", err)
	if len(pdf) == 0 || !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("expected PDF bytes, got len=%d", len(pdf))
	}
}

// ---------- Focused error cases ----------

func TestFileClient_Errors(t *testing.T) {
	env := newTestEnv(t)
	defer env.close()
	c := env.Client

	// ReadDir on missing path
	if _, err := c.ReadDir("/nope", false, 0, 0); err == nil {
		t.Fatalf("expected error for missing dir")
	}

	// ReadFile on missing file
	if _, err := c.ReadFile(env.Token, "/missing.txt"); err == nil {
		t.Fatalf("expected error for missing file")
	}
}

// (Optional) compile-time interface check to make sure we’re still using the right proto types.
var _ = filepb.FileInfo{}
