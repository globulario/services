// --- file_ops.go ---
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/file/file_client"
	"github.com/globulario/services/golang/file/filepb"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"github.com/tealeg/xlsx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

// return the a list of all info
func getFileInfos(srv *server, info *filepb.FileInfo, infos []*filepb.FileInfo) []*filepb.FileInfo {
	infos = append(infos, info)
	for i := 0; i < len(info.Files); i++ {
		path_ := srv.formatPath(info.Files[i].Path)
		if Utility.Exists(path_) {
			// do not send Thumbnail...
			if info.Files[i].IsDir {
				if !Utility.Exists(path_ + "/playlist.m3u8") {
					info.Files[i].Thumbnail = "" // remove the icon for dir
				}
			}
			infos = getFileInfos(srv, info.Files[i], infos)
		} else {
			cache.RemoveItem(info.Files[i].Path)
		}
	}
	// empty the arrays...
	if info.IsDir {
		path_ := srv.formatPath(info.Path)
		if !Utility.Exists(path_ + "/playlist.m3u8") {
			info.Files = make([]*filepb.FileInfo, 0)
		}
	}
	return infos
}

func getFileVideos(path string, domain string) ([]*titlepb.Video, error) {

	id := path + "@" + domain + ":videos"
	data, err := cache.GetItem(id)
	videos := new(titlepb.Videos)

	if err == nil && data != nil {
		err = protojson.Unmarshal(data, videos)
		if err == nil {
			return videos.Videos, err
		}
		cache.RemoveItem(id)
	}

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	// get from the title srv.
	videos.Videos, err = client.GetFileVideos(config.GetDataDir()+"/search/videos", path)
	if err != nil {
		return nil, err
	}

	// keep to cache...
	str, _ := protojson.Marshal(videos)
	cache.SetItem(id, str)

	return videos.Videos, nil

}

func dissociateFileWithTitle(path string, domain string) error {

	path = strings.ReplaceAll(path, "\\", "/")

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return err
	}

	titles, err := client.GetFileTitles(config.GetDataDir()+"/search/titles", path)
	if err == nil {
		// Here I will asscociate the path
		for _, title := range titles {
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, path)
		}
	}

	// Look for videos
	videos, err := getFileVideos(path, domain)
	if err == nil {
		// Here I will asscociate the path
		for _, video := range videos {
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, path)
		}
	}

	return nil
}

// GetFileClient returns a file service client for a given address.
func (srv *server) GetFileClient(address string) (*file_client.File_Client, error) {
	Utility.RegisterFunction("NewFileService_Client", file_client.NewFileService_Client)
	c, err := globular_client.GetClient(address, "file.FileService", "NewFileService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*file_client.File_Client), nil
}

// GetFileInfo returns a single FileInfo and (internally) flattens children for caching.
func (srv *server) GetFileInfo(ctx context.Context, rqst *filepb.GetFileInfoRequest) (*filepb.GetFileInfoResponse, error) {
	p := srv.formatPath(rqst.GetPath())
	fi, err := getFileInfo(srv, p, int(rqst.ThumbnailHeight), int(rqst.ThumbnailWidth))
	if err != nil {
		return nil, err
	}
	infos := make([]*filepb.FileInfo, 0)
	infos = getFileInfos(srv, fi, infos)
	return &filepb.GetFileInfoResponse{Info: infos[0]}, nil
}

// ReadDir streams FileInfo structures for a directory.
func (srv *server) ReadDir(rqst *filepb.ReadDirRequest, stream filepb.FileService_ReadDirServer) error {
	if len(rqst.Path) == 0 {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("path is empty")))
	}
	p := srv.formatPath(rqst.Path)
	if !Utility.Exists(p) {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("no file found with path %s", p)))
	}
	if info, err := getFileInfo(srv, p, 64, 64); err != nil || !info.IsDir {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("path %s is not a directory", p)))
	}

	filesInfoChan := make(chan *filepb.FileInfo)
	errChan := make(chan error)
	go func() {
		defer close(filesInfoChan)
		defer close(errChan)
		_, _ = readDir(srv, p, rqst.GetRecursive(), rqst.ThumbnailWidth, rqst.ThumbnailHeight, true, filesInfoChan, errChan)
	}()
	for {
		select {
		case fi, ok := <-filesInfoChan:
			if !ok {
				if err := <-errChan; err != nil {
					return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}
				return nil
			}
			if err := stream.Send(&filepb.ReadDirResponse{Info: fi}); err != nil {
				if strings.Contains(err.Error(), "context canceled") {
					return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}
			}
		case err := <-errChan:
			if err != nil {
				return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			return nil
		}
	}
}

// ReadFile streams file bytes in chunks.
func (srv *server) ReadFile(rqst *filepb.ReadFileRequest, stream filepb.FileService_ReadFileServer) error {
	p := srv.formatPath(rqst.GetPath())
	f, err := os.Open(p)
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	defer f.Close()
	buf := make([]byte, 5*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			_ = stream.Send(&filepb.ReadFileResponse{Data: buf[:n]})
		}
		if err != nil {
			if err != io.EOF {
				return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			break
		}
	}
	return nil
}

// SaveFile writes an incoming stream to disk.
func (srv *server) SaveFile(stream filepb.FileService_SaveFileServer) error {
	var data []byte
	var path string
	for {
		rqst, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				if err := os.WriteFile(path, data, 0644); err != nil {
					return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}
				if err := stream.SendAndClose(&filepb.SaveFileResponse{Result: true}); err != nil {
					slog.Error("save send/close failed", "err", err)
					return err
				}
				return nil
			}
			return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		switch msg := rqst.File.(type) {
		case *filepb.SaveFileRequest_Path:
			path = srv.formatPath(msg.Path)
		case *filepb.SaveFileRequest_Data:
			data = append(data, msg.Data...)
		}
	}
}

// DeleteFile removes a single file and updates related state.
func (srv *server) DeleteFile(ctx context.Context, rqst *filepb.DeleteFileRequest) (*filepb.DeleteFileResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	p := srv.formatPath(rqst.GetPath())
	cache.RemoveItem(p)
	cache.RemoveItem(filepath.Dir(p))
	rbac, err := getRbacClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = rbac.DeleteResourcePermissions(token, rqst.GetPath())

	dir := filepath.Dir(p)
	name := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	hidden := filepath.Join(dir, ".hidden", name)
	if Utility.Exists(hidden) {
		_ = os.RemoveAll(hidden)
	}
	_ = dissociateFileWithTitle(rqst.GetPath(), srv.Domain)

	if Utility.Exists(filepath.Join(dir, "audio.m3u")) {
		cache.RemoveItem(filepath.Join(dir, "audio.m3u"))
		_ = os.Remove(filepath.Join(dir, "audio.m3u"))
		_ = srv.generatePlaylist(dir, token)
	}
	if Utility.Exists(filepath.Join(dir, "video.m3u")) {
		cache.RemoveItem(filepath.Join(dir, "video.m3u"))
		_ = os.Remove(filepath.Join(dir, "video.m3u"))
		_ = srv.generatePlaylist(dir, token)
	}
	if err := os.Remove(p); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &filepb.DeleteFileResponse{Result: true}, nil
}

// HtmlToPdf converts raw HTML to PDF and returns the bytes.
func (srv *server) HtmlToPdf(ctx context.Context, rqst *filepb.HtmlToPdfRqst) (*filepb.HtmlToPdfResponse, error) {
	pdfg, err := wkhtmltopdf.NewPDFGenerator()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	pdfg.AddPage(wkhtmltopdf.NewPageReader(strings.NewReader(rqst.Html)))
	if err := pdfg.Create(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	path := filepath.Join(os.TempDir(), Utility.RandomUUID())
	defer os.Remove(path)
	if err := pdfg.WriteFile(path); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &filepb.HtmlToPdfResponse{Pdf: b}, nil
}

func getThumbnails(info *filepb.FileInfo) []interface{} {
	// The array of thumbnail
	thumbnails := make([]interface{}, 0)
	// Now from the info i will extract the thumbnail
	for i := 0; i < len(info.Files); i++ {
		if !info.Files[i].IsDir {
			thumbnail := make(map[string]string)
			thumbnail["path"] = info.Files[i].Path
			thumbnail["thumbnail"] = info.Files[i].Thumbnail
			thumbnails = append(thumbnails, thumbnail)
		} else {
			thumbnails = append(thumbnails, getThumbnails(info.Files[i])...)
		}
	}
	return thumbnails
}

// GetThumbnails returns a JSON stream of thumbnails for files under a directory.
func (srv *server) GetThumbnails(rqst *filepb.GetThumbnailsRequest, stream filepb.FileService_GetThumbnailsServer) error {
	p := rqst.GetPath()
	if strings.HasPrefix(p, "/") {
		p = toSlash(filepath.Join(srv.Root, p))
	}
	info, err := readDir(srv, p, rqst.GetRecursive(), rqst.ThumbnailHeight, rqst.ThumbnailWidth, true, nil, nil)
	if err != nil {
		return err
	}
	thumbs := getThumbnails(info)
	jsonStr, err := json.Marshal(thumbs)
	if err != nil {
		return err
	}
	const max = 5 * 1024
	for i := 0; i < int(math.Ceil(float64(len(jsonStr))/float64(max))); i++ {
		start, end := i*max, (i+1)*max
		if end > len(jsonStr) {
			end = len(jsonStr)
		}
		_ = stream.Send(&filepb.GetThumbnailsResponse{Data: jsonStr[start:end]})
	}
	return nil
}

// CreateLnk writes a link file into a directory and assigns ownership.
func (srv *server) CreateLnk(ctx context.Context, rqst *filepb.CreateLnkRequest) (*filepb.CreateLnkResponse, error) {
	p := srv.formatPath(rqst.Path)
	if !Utility.Exists(p) {
		return nil, status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("no directory found at path %s", p)))
	}
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(p, rqst.Name), []byte(rqst.Lnk), 0644); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	_ = srv.setOwner(token, rqst.Path+"/"+rqst.Name)
	return &filepb.CreateLnkResponse{}, nil
}

// WriteExcelFile writes an .xlsx file from JSON data.
func (srv *server) WriteExcelFile(ctx context.Context, rqst *filepb.WriteExcelFileRequest) (*filepb.WriteExcelFileResponse, error) {
	p := srv.formatPath(rqst.Path)
	if Utility.Exists(p) {
		if err := os.Remove(p); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	sheets := map[string]interface{}{}
	if err := json.Unmarshal([]byte(rqst.Data), &sheets); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if err := srv.writeExcelFile(p, sheets); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &filepb.WriteExcelFileResponse{Result: true}, nil
}

// writeExcelFile writes sheets to disk (helper, unexported).
func (srv *server) writeExcelFile(path string, sheets map[string]interface{}) error {
	xlFile, err := xlsx.OpenFile(path)
	var xlSheet *xlsx.Sheet
	if err != nil {
		xlFile = xlsx.NewFile()
	}
	for name, data := range sheets {
		xlSheet, _ = xlFile.AddSheet(name)
		values := data.([]interface{})
		for i := 0; i < len(values); i++ {
			row := xlSheet.AddRow()
			cols := values[i].([]interface{})
			for j := 0; j < len(cols); j++ {
				if cols[j] == nil {
					continue
				}
				cell := row.AddCell()
				if reflect.TypeOf(cols[j]).String() == "string" {
					str := cols[j].(string)
					if dt, err := Utility.DateTimeFromString(str, "2006-01-02 15:04:05"); err == nil {
						cell.SetDateTime(dt)
					} else {
						cell.SetString(str)
					}
				} else {
					cell.SetValue(cols[j])
				}
			}
		}
	}
	if err := xlFile.Save(path); err != nil {
		return err
	}
	return nil
}

// GetFileMetadata returns structured file metadata extracted by ExifTool.
func (srv *server) GetFileMetadata(ctx context.Context, rqst *filepb.GetFileMetadataRequest) (*filepb.GetFileMetadataResponse, error) {
	p := srv.formatPath(rqst.Path)
	md, err := ExtractMetada(p)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	obj, err := structpb.NewStruct(md)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &filepb.GetFileMetadataResponse{Result: obj}, nil
}
