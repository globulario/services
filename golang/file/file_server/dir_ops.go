// --- dir_ops.go ---
package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/file/filepb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/title/title_client"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// printDownloadPercent periodically sends progress updates to the stream.
func printDownloadPercent(done chan int64, path string, total int64, stream filepb.FileService_UploadFileServer) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			f, err := os.Open(path)
			if err != nil {
				slog.Error("upload progress open failed", "path", path, "err", err)
				return
			}
			st, err := f.Stat()
			_ = f.Close()
			if err != nil {
				slog.Error("upload progress stat failed", "path", path, "err", err)
				return
			}
			sz := st.Size()
			if sz == 0 {
				sz = 1
			}
			pct := float64(sz) / float64(total) * 100
			if err := stream.Send(&filepb.UploadFileResponse{
				Uploaded: sz, Total: total, Info: fmt.Sprintf("%.2f%%", pct),
			}); err != nil {
				slog.Warn("upload progress send failed", "path", path, "err", err)
				return
			}
		}
	}
}

func (srv *server) uploadFile(token, urlStr, dest, name string, stream filepb.FileService_UploadFileServer) error {
	path := srv.formatPath(dest)
	if !Utility.Exists(path) {
		return fmt.Errorf("no folder found with path %s", path)
	}
	if err := Utility.CreateDirIfNotExist(path); err != nil {
		return err
	}

	outPath := filepath.Join(path, name)
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	headResp, err := http.Head(urlStr)
	if err != nil {
		return err
	}
	defer headResp.Body.Close()

	size, err := strconv.Atoi(headResp.Header.Get("Content-Length"))
	if err != nil {
		return err
	}
	done := make(chan int64)
	go printDownloadPercent(done, outPath, int64(size), stream)

	slog.Info("upload: downloading", "url", urlStr, "dest", outPath)
	resp, err := http.Get(urlStr)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	done <- n

	if err := stream.Send(&filepb.UploadFileResponse{Uploaded: 100, Total: 100, Info: "Download completed"}); err != nil {
		slog.Warn("upload: completion send failed", "dest", outPath, "err", err)
	}

	if err := srv.setOwner(token, dest+"/"+name); err != nil {
		slog.Warn("upload: set owner failed", "path", dest+"/"+name, "err", err)
	}

	if info, err := getFileInfo(srv, outPath, -1, -1); err == nil {
		switch {
		case strings.HasPrefix(info.Mime, "video/"):
			_ = stream.Send(&filepb.UploadFileResponse{Uploaded: 100, Total: 100, Info: "Process video information..."})
			processVideos(srv, token, []string{path})
		case strings.HasSuffix(info.Name, ".pdf"):
			_ = stream.Send(&filepb.UploadFileResponse{Uploaded: 100, Total: 100, Info: "Index text information..."})
			if err := srv.indexPdfFile(outPath, info); err != nil {
				slog.Warn("upload: pdf indexing failed", "path", outPath, "err", err)
			}
		}
	} else {
		slog.Warn("upload: get file info failed", "path", outPath, "err", err)
	}

	if err := stream.Send(&filepb.UploadFileResponse{Uploaded: 100, Total: 100, Info: "Done"}); err != nil {
		slog.Warn("upload: final send failed", "dest", outPath, "err", err)
	}
	return nil
}

// UploadFile streams or imports a file/dir from a URL.
func (srv *server) UploadFile(rqst *filepb.UploadFileRequest, stream filepb.FileService_UploadFileServer) error {
	_, token, err := security.GetClientId(stream.Context())
	if err != nil {
		return err
	}

	if rqst.IsDir {
		fileClient, err := srv.GetFileClient(rqst.Domain)
		if err != nil {
			return err
		}
		u, err := url.Parse(rqst.Url)
		if err != nil {
			return err
		}

		_ = stream.Send(&filepb.UploadFileResponse{Uploaded: 100, Total: 100, Info: "Create archive for " + rqst.Name + " ..."})
		tmpName := Utility.RandomUUID()
		archivePath, err := fileClient.CreateArchive(token, []string{u.Path}, tmpName)
		if err != nil {
			return err
		}

		archiveURL := u.Scheme + "://" + u.Host + archivePath + "?token=" + token
		if err := srv.uploadFile(token, archiveURL, rqst.Dest, tmpName+".tar.gz", stream); err != nil {
			return err
		}

		if err := fileClient.DeleteFile(token, archivePath); err != nil {
			slog.Warn("upload dir: cleanup remote archive failed", "remotePath", archivePath, "err", err)
		}

		path := srv.formatPath(rqst.Dest)
		_ = stream.Send(&filepb.UploadFileResponse{Uploaded: 100, Total: 100, Info: "Unpack archive for " + rqst.Name + " ..."})
		defer os.RemoveAll(filepath.Join(path, tmpName+".tar.gz"))

		f, err := os.Open(filepath.Join(path, tmpName+".tar.gz"))
		if err != nil {
			return err
		}
		r := bufio.NewReader(f)
		extracted, err := Utility.ExtractTarGz(r)
		_ = f.Close()
		if err != nil {
			return err
		}

		if err := Utility.Move(extracted, path); err != nil {
			return err
		}
		if err := os.Rename(filepath.Join(path, filepath.Base(extracted)), filepath.Join(path, rqst.Name)); err != nil {
			return err
		}

		if Utility.Exists(filepath.Join(path, rqst.Name, "playlist.m3u8")) {
			_ = stream.Send(&filepb.UploadFileResponse{Uploaded: 100, Total: 100, Info: "Process video information..."})
			processVideos(srv, token, []string{filepath.Join(path, rqst.Name)})
		}

		if err := srv.setOwner(token, rqst.Dest+"/"+rqst.Name); err != nil {
			slog.Warn("upload dir: set owner failed", "path", rqst.Dest+"/"+rqst.Name, "err", err)
		}

		srv.publishReloadDirEvent(path)
		return nil
	}

	// Single file
	if err := srv.uploadFile(token, rqst.Url, rqst.Dest, rqst.Name, stream); err != nil {
		_ = stream.Send(&filepb.UploadFileResponse{Info: "fail to upload file " + rqst.Name + " with error " + err.Error()})
		if strings.Contains(err.Error(), "signal: killed") {
			return fmt.Errorf("fail to upload file %s with error %s", rqst.Name, err.Error())
		}
	} else {
		srv.publishReloadDirEvent(rqst.Dest)
	}
	return nil
}


// ReadDir streams FileInfo structures for a directory.
func (srv *server) ReadDir(rqst *filepb.ReadDirRequest, stream filepb.FileService_ReadDirServer) error {
	if len(rqst.Path) == 0 {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("path is empty")))
	}
	p := srv.formatPath(rqst.Path)
	ctx := stream.Context()
	storage := srv.storageForPath(p)
	info, err := storage.Stat(ctx, p)
	fmt.Println("-------------> ", info)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
			return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("no file found with path %s", p)))
		}
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if !info.IsDir() {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("path %s is not a directory", p)))
	}

	filesInfoChan := make(chan *filepb.FileInfo)
	errChan := make(chan error)
	go func() {
		defer close(filesInfoChan)
		defer close(errChan)
		_, _ = readDir(ctx, srv, p, rqst.GetRecursive(), rqst.ThumbnailWidth, rqst.ThumbnailHeight, true, filesInfoChan, errChan)
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

// readDir recursively reads directory entries and emits FileInfo structures.
func readDir(ctx context.Context, s *server, path string, recursive bool, thumbnailMaxWidth int32, thumbnailMaxHeight int32, readFiles bool, fileInfosChan chan *filepb.FileInfo, errChan chan error) (*filepb.FileInfo, error) {
	info, err := getFileInfo(s, path, int(thumbnailMaxWidth), int(thumbnailMaxWidth))
	if err != nil {
		if errChan != nil {
			errChan <- err
		}
		return nil, err
	}
	if !info.IsDir {
		err := fmt.Errorf("path %s is not a directory", path)
		if errChan != nil {
			errChan <- err
		}
		return nil, err
	}

	entries, err := s.storageForPath(path).ReadDir(ctx, path)
	if err != nil {
		if errChan != nil {
			errChan <- err
		}
		return nil, err
	}
	if fileInfosChan != nil {
		fileInfosChan <- info
	}

	for _, e := range entries {
		p := filepath.Join(path, e.Name())
		if e.IsDir() {
			isHls := s.pathExists(ctx, filepath.Join(p, "playlist.m3u8"))
			if recursive && !isHls && e.Name() != ".hidden" {
				child, err := readDir(ctx, s, p, recursive, thumbnailMaxWidth, thumbnailMaxHeight, true, fileInfosChan, errChan)
				if err != nil {
					if errChan != nil {
						errChan <- err
					}
					return nil, err
				}
				if fileInfosChan != nil {
					fileInfosChan <- child
				} else {
					info.Files = append(info.Files, child)
				}
			} else if e.Name() != ".hidden" {
				child, err := readDir(ctx, s, p, recursive, thumbnailMaxWidth, thumbnailMaxHeight, false, fileInfosChan, errChan)
				if err != nil {
					if errChan != nil {
						errChan <- err
					}
					return nil, err
				}
				if isHls {
					child.Mime = "video/hls-stream"
				}
				if fileInfosChan != nil {
					fileInfosChan <- child
				} else {
					info.Files = append(info.Files, child)
				}
			}
		} else if readFiles {
			fi, err := getFileInfo(s, p, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
			if err != nil {
				if errChan != nil {
					errChan <- err
				}
				return nil, nil
			}
			if !fi.IsDir {
				if dot := strings.LastIndex(e.Name(), "."); dot != -1 {
					fi.Mime = mime.TypeByExtension(e.Name()[dot:])
				} else if f, err := s.storageForPath(p).Open(ctx, p); err == nil {
					if file, ok := f.(*os.File); ok {
						fi.Mime, _ = Utility.GetFileContentType(file)
					}
					_ = f.Close()
				}
				if !strings.Contains(path, ".hidden") && len(fi.Thumbnail) == 0 {
					if strings.HasPrefix(fi.Mime, "image/") && thumbnailMaxHeight > 0 && thumbnailMaxWidth > 0 {
						fi.Thumbnail, _ = s.getThumbnail(p, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
					} else if strings.Contains(fi.Mime, "/") {
						if cwd, err := os.Getwd(); err == nil {
							icon := s.formatPath(filepath.Join(cwd, "mimetypes", strings.ReplaceAll(strings.Split(fi.Mime, ";")[0], "/", "-")+".png"))
							fi.Thumbnail, _ = s.getMimeTypesUrl(icon)
						}
					} else if cwd, err := os.Getwd(); err == nil {
						icon := s.formatPath(filepath.Join(cwd, "mimetypes", "unknown.png"))
						fi.Thumbnail, _ = s.getMimeTypesUrl(icon)
					}
				}
				if fileInfosChan != nil {
					fileInfosChan <- fi
				} else {
					info.Files = append(info.Files, fi)
				}
			}
		}
	}
	return info, nil
}

// publishReloadDirEvent notifies clients to refresh a directory.
func (srv *server) publishReloadDirEvent(path string) {
	client, err := getEventClient()
	p := srv.formatPath(path)
	if err != nil {
		slog.Warn("publish reload dir event: no event client", "path", p, "err", err)
		return
	}
	if err := client.Publish("reload_dir_event", []byte(p)); err != nil {
		slog.Warn("publish reload dir event failed", "path", p, "err", err)
	}
}

// AddPublicDir registers a folder as public.
func (srv *server) AddPublicDir(ctx context.Context, rqst *filepb.AddPublicDirRequest) (*filepb.AddPublicDirResponse, error) {
	p := rqst.Path
	if p == "" || !Utility.Exists(p) {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("file with path %s doesn't exist", rqst.Path)))
	}
	if Utility.Contains(srv.Public, p) {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("path %s already exist in public paths", p)))
	}
	srv.Public = append(srv.Public, p)
	if err := srv.Save(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	slog.Info("public dir added", "path", p)
	return &filepb.AddPublicDirResponse{}, nil
}

// RemovePublicDir unregisters a public folder.
func (srv *server) RemovePublicDir(ctx context.Context, rqst *filepb.RemovePublicDirRequest) (*filepb.RemovePublicDirResponse, error) {
	p := rqst.Path
	if p == "" || !Utility.Exists(p) {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("file with path %s doesn't exist", rqst.Path)))
	}
	if !Utility.Contains(srv.Public, p) {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("path %s doesn't exist in public paths", p)))
	}
	srv.Public = Utility.RemoveString(srv.Public, p)
	if err := srv.Save(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	slog.Info("public dir removed", "path", p)
	return &filepb.RemovePublicDirResponse{}, nil
}

// GetPublicDirs returns configured public directories.
func (srv *server) GetPublicDirs(context.Context, *filepb.GetPublicDirsRequest) (*filepb.GetPublicDirsResponse, error) {
	return &filepb.GetPublicDirsResponse{Dirs: srv.Public}, nil
}

// isPublic returns true if a concrete filesystem path is inside a public root.
func (srv *server) isPublic(path string) bool {
	p := srv.formatPath(filepath.Clean(path))
	sep := "/"
	if p == "" {
		return false
	}
	for _, root := range srv.Public {
		cleanRoot := srv.formatPath(filepath.Clean(root))
		if cleanRoot == "" {
			continue
		}
		if strings.HasPrefix(p+sep, cleanRoot+sep) || p == cleanRoot {
			return true
		}
	}
	return false
}

func (srv *server) getFileVideosAssociation(client *title_client.Title_Client, path string, videos map[string][]*titlepb.Video) error {
	path_ := srv.formatPath(path)
	info, err := os.Stat(path_)
	if err != nil {
		return err
	}

	// If it's a directory and not an HLS stream, recurse into children
	if info.IsDir() && !Utility.Exists(path_+"/playlist.m3u8") {
		files, err := os.ReadDir(path_)
		if err == nil {
			for _, f := range files {
				childPath := path + "/" + f.Name()
				if !strings.Contains(childPath, ".hidden/") {
					srv.getFileVideosAssociation(client, childPath, videos)
				}
			}
		}
	} else {
		videos_, err := getFileVideos(path_, srv.Domain)
		if err == nil {
			videos[path] = videos_
		}
	}
	return nil
}

// Recursively get all titles for a given path.
func (srv *server) getFileTitlesAssociation(client *title_client.Title_Client, path string, titles map[string][]*titlepb.Title) error {
	path_ := srv.formatPath(path)
	info, err := os.Stat(path_)
	if err != nil {
		return err
	}

	if info.IsDir() && !Utility.Exists(path_+"/playlist.m3u8") {
		files, err := os.ReadDir(path_)
		if err == nil {
			for _, f := range files {
				child := path + "/" + f.Name()
				if !strings.Contains(child, ".hidden/") {
					srv.getFileTitlesAssociation(client, child, titles)
				}
			}
		}
	} else {
		titles_, err := client.GetFileTitles(config.GetDataDir()+"/search/titles", path)
		if err == nil {
			titles[path] = titles_
		}
	}
	return nil
}

// Copy a file/directory
func (srv *server) Copy(ctx context.Context, rqst *filepb.CopyRequest) (*filepb.CopyResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// get the rbac client.
	rbacClient, err := getRbacClient()
	if err != nil {
		return nil, err
	}

	// format the path to make it absolute on the server.
	destPath := srv.formatPath(rqst.Path)

	// For each file in the request, copy and update associations/permissions.
	for i := 0; i < len(rqst.Files); i++ {
		srcFile := srv.formatPath(rqst.Files[i])

		// Retrieve indexation for the file.
		titleClient, err := getTitleClient()
		if err != nil {
			return nil, err
		}

		filePermissions, _ := rbacClient.GetResourcePermissionsByResourceType("file")
		permissions, _ := rbacClient.GetResourcePermissions(rqst.Files[i])

		titles := make(map[string][]*titlepb.Title)
		_ = srv.getFileTitlesAssociation(titleClient, rqst.Files[i], titles)

		videos := make(map[string][]*titlepb.Video)
		_ = srv.getFileVideosAssociation(titleClient, rqst.Files[i], videos)

		if Utility.Exists(srcFile) {
			info, err := os.Stat(srcFile)
			if err == nil {
				if info.IsDir() {
					// Copy the directory
					if err := Utility.CopyDir(srcFile, destPath); err != nil {
						slog.Error("copy dir failed", "src", srcFile, "dest", destPath, "err", err)
						return nil, err
					}

					// Associate titles
					for f, titleList := range titles {
						for _, t := range titleList {
							filename := filepath.Base(rqst.Files[i])
							dest := rqst.Path + "/" + filename
							f_ := strings.ReplaceAll(f, rqst.Files[i], dest)
							if err := titleClient.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f_); err != nil {
								slog.Warn("associate title after copy failed", "titleID", t.ID, "file", f_, "err", err)
							}
						}
					}

					// Associate videos
					for f, videoList := range videos {
						for _, v := range videoList {
							filename := filepath.Base(rqst.Files[i])
							dest := rqst.Path + "/" + filename
							f_ := strings.ReplaceAll(f, rqst.Files[i], dest)
							if err := titleClient.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", v.ID, f_); err != nil {
								slog.Warn("associate video after copy failed", "videoID", v.ID, "file", f_, "err", err)
							}
						}
					}

					// Update permissions
					for j := 0; j < len(filePermissions); j++ {
						p := filePermissions[j]
						if strings.HasPrefix(p.Path, rqst.Files[i]) {
							filename := filepath.Base(rqst.Files[i])
							dest := rqst.Path + "/" + filename
							p.Path = strings.ReplaceAll(p.Path, rqst.Files[i], dest)
							if err := rbacClient.SetResourcePermissions(token, p.Path, p.ResourceType, p); err != nil {
								slog.Warn("update permission after copy failed", "path", p.Path, "err", err)
							}
						}
					}
				} else {
					// Copy the file
					if err := Utility.CopyFile(srcFile, destPath); err != nil {
						slog.Error("copy file failed", "src", srcFile, "dest", destPath, "err", err)
						return nil, err
					}

					// Associate titles
					for f, titleList := range titles {
						for _, t := range titleList {
							f_ := rqst.Path + "/" + filepath.Base(f)
							if err := titleClient.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f_); err != nil {
								slog.Warn("associate title after copy failed", "titleID", t.ID, "file", f_, "err", err)
							}
						}
					}

					// Associate videos
					for f, videoList := range videos {
						for _, v := range videoList {
							f_ := rqst.Path + "/" + filepath.Base(f)
							if err := titleClient.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", v.ID, f_); err != nil {
								slog.Warn("associate video after copy failed", "videoID", v.ID, "file", f_, "err", err)
							}
						}
					}

					// Update permissions
					if permissions != nil {
						permissions.Path = rqst.Path + "/" + filepath.Base(permissions.Path)
						if err := rbacClient.SetResourcePermissions(token, permissions.Path, permissions.ResourceType, permissions); err != nil {
							slog.Warn("update permission after copy failed", "path", permissions.Path, "err", err)
						}
					}

					// Copy hidden folder if exists
					baseDir := filepath.Dir(srcFile)
					fileName := filepath.Base(srcFile)
					if dot := strings.LastIndex(fileName, "."); dot != -1 {
						fileName = fileName[:dot]
					}
					hiddenFolder := filepath.Join(baseDir, ".hidden", fileName)
					if Utility.Exists(hiddenFolder) {
						if err := Utility.CopyDir(hiddenFolder, filepath.Join(destPath, ".hidden")); err != nil {
							slog.Warn("copy hidden folder failed", "src", hiddenFolder, "dest", filepath.Join(destPath, ".hidden"), "err", err)
						}
					}
				}
			}
		}
	}

	return &filepb.CopyResponse{Result: true}, nil
}

// CreateArchive creates a .tar.gz from the given paths and stores it in the user's area.
func (srv *server) CreateArchive(ctx context.Context, rqst *filepb.CreateArchiveRequest) (*filepb.CreateArchiveResponse, error) {
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	tmp := filepath.Join(os.TempDir(), rqst.GetName())
	createTempDir := true

	// If only one path and it's a directory, we can archive it directly.
	if len(rqst.Paths) == 1 {
		p := rqst.Paths[0]
		if !srv.isPublic(p) {
			p = srv.formatPath(p)
		}
		if !Utility.Exists(p) {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("no file exist for path %s", p)))
		}
		if info, _ := os.Stat(p); info.IsDir() {
			tmp = p
			createTempDir = false
		}
	}

	if createTempDir {
		if err := Utility.CreateDirIfNotExist(tmp); err != nil {
			return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		defer os.RemoveAll(tmp)

		for _, rp := range rqst.Paths {
			if Utility.Exists(srv.Root+rp) || srv.isPublic(rp) {
				p := srv.formatPath(rp)
				info, _ := os.Stat(p)
				fileName := p[strings.LastIndex(p, "/"):]
				if info.IsDir() {
					if err := Utility.CopyDir(p, filepath.Join(tmp, fileName)); err != nil {
						return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
					}
				} else {
					if err := Utility.CopyFile(p, filepath.Join(tmp, fileName)); err != nil {
						return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
					}
				}
			}
		}
	}

	var buf bytes.Buffer
	if _, err := Utility.CompressDir(tmp, &buf); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	dest := "/users/" + clientId + "/" + rqst.GetName() + ".tar.gz"

	if err := srv.setOwner(token, dest); err != nil {
		slog.Warn("create archive: set owner failed", "path", dest, "err", err)
	}

	if err := os.WriteFile(srv.Root+dest, buf.Bytes(), 0644); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	slog.Info("archive created", "dest", dest)
	return &filepb.CreateArchiveResponse{Result: dest}, nil
}

func (srv *server) CreateDir(ctx context.Context, rqst *filepb.CreateDirRequest) (*filepb.CreateDirResponse, error) {
	path := srv.formatPath(rqst.GetPath())
	if err := Utility.CreateDirIfNotExist(filepath.Join(path, rqst.GetName())); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	if err := srv.setOwner(token, rqst.GetPath()+"/"+rqst.GetName()); err != nil {
		return nil, err
	}

	slog.Info("dir created", "path", filepath.Join(path, rqst.GetName()))
	return &filepb.CreateDirResponse{Result: true}, nil
}

func (srv *server) DeleteDir(ctx context.Context, rqst *filepb.DeleteDirRequest) (*filepb.DeleteDirResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	path := srv.formatPath(rqst.GetPath())
	if !Utility.Exists(path) {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("no directory with path %s was found", path)))
	}

	cache.RemoveItem(path)

	// Remove file associations contained by files in that directory
	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	titles := make(map[string][]*titlepb.Title)
	_ = srv.getFileTitlesAssociation(client, rqst.GetPath(), titles)

	videos := make(map[string][]*titlepb.Video)
	_ = srv.getFileVideosAssociation(client, rqst.GetPath(), videos)

	// Dissociate titles
	for f, list := range titles {
		for _, t := range list {
			if err := client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f); err != nil {
				slog.Warn("dissociate title failed", "file", f, "titleID", t.ID, "err", err)
			}
		}
	}
	// Dissociate videos
	for f, list := range videos {
		for _, v := range list {
			if err := client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", v.ID, f); err != nil {
				slog.Warn("dissociate video failed", "file", f, "videoID", v.ID, "err", err)
			}
		}
	}

	// Delete resource permissions
	rbacClient, err := getRbacClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Recursively remove all sub-dir and file permissions
	if permissions, err := rbacClient.GetResourcePermissionsByResourceType("file"); err == nil {
		for _, p := range permissions {
			if strings.HasPrefix(p.Path, path) {
				if err := rbacClient.DeleteResourcePermissions(token, p.GetPath()); err != nil {
					slog.Warn("delete sub-permission failed", "path", p.GetPath(), "err", err)
				}
			}
		}
	}

	// Remove the directory permission entry itself
	if err := rbacClient.DeleteResourcePermissions(token, rqst.GetPath()); err != nil {
		slog.Warn("delete dir permission failed", "path", rqst.GetPath(), "err", err)
	}

	// Remove the directory itself
	if err := os.RemoveAll(path); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	slog.Info("dir deleted", "path", path)
	return &filepb.DeleteDirResponse{Result: true}, nil
}

// Move a file/directory
func (srv *server) Move(ctx context.Context, rqst *filepb.MoveRequest) (*filepb.MoveResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}
	rbacClient, err := getRbacClient()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(rqst.Files); i++ {
		from := srv.formatPath(rqst.Files[i])
		dest := srv.formatPath(rqst.Path)

		// Test if from has same parent as dest
		if filepath.Dir(from) == dest || from == dest {
			// no need to move
			continue
		}

		info, _ := os.Stat(from)
		filePerms, _ := rbacClient.GetResourcePermissionsByResourceType("file")

		if Utility.Exists(from) {
			titles := make(map[string][]*titlepb.Title)
			_ = srv.getFileTitlesAssociation(client, rqst.Files[i], titles)
			videos := make(map[string][]*titlepb.Video)
			_ = srv.getFileVideosAssociation(client, rqst.Files[i], videos)

			// Dissociate titles/videos at old path root
			for f, list := range titles {
				if f == rqst.Files[i] {
					for _, t := range list {
						if err := client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f); err != nil {
							slog.Warn("move: dissociate title failed", "file", f, "titleID", t.ID, "err", err)
						}
					}
				}
			}
			for f, list := range videos {
				for _, v := range list {
					if f == rqst.Files[i] {
						if err := client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", v.ID, f); err != nil {
							slog.Warn("move: dissociate video failed", "file", f, "videoID", v.ID, "err", err)
						}
					}
				}
			}

			// Move the file/dir
			if err := Utility.Move(from, dest); err != nil {
				slog.Error("move failed", "from", from, "dest", dest, "err", err)
				continue
			}

			// purge cache of old path
			cache.RemoveItem(from)

			// Re-associate at new path(s)
			for f, list := range titles {
				for _, t := range list {
					var f_ string
					if !info.IsDir() {
						f_ = rqst.Path + "/" + filepath.Base(f)
					} else {
						d := rqst.Path + "/" + filepath.Base(rqst.Files[i])
						f_ = strings.ReplaceAll(f, rqst.Files[i], d)
					}
					if err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f_); err != nil {
						slog.Warn("move: associate title failed", "titleID", t.ID, "file", f_, "err", err)
					}
				}
			}
			for f, list := range videos {
				for _, v := range list {
					var f_ string
					if !info.IsDir() {
						f_ = rqst.Path + "/" + filepath.Base(f)
					} else {
						d := rqst.Path + "/" + filepath.Base(rqst.Files[i])
						f_ = strings.ReplaceAll(f, rqst.Files[i], d)
					}
					if err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", v.ID, f_); err != nil {
						slog.Warn("move: associate video failed", "videoID", v.ID, "file", f_, "err", err)
					}
				}
			}

			// Update permissions
			if info.IsDir() {
				for j := 0; j < len(filePerms); j++ {
					p := filePerms[j]
					if strings.HasPrefix(p.Path, rqst.Files[i]) {
						if err := rbacClient.DeleteResourcePermissions(token, p.Path); err != nil {
							slog.Warn("move: delete old permission failed", "path", p.Path, "err", err)
						}
						d := rqst.Path + "/" + filepath.Base(rqst.Files[i])
						p.Path = strings.ReplaceAll(p.Path, rqst.Files[i], d)
						if err := rbacClient.SetResourcePermissions(token, p.Path, p.ResourceType, p); err != nil {
							slog.Warn("move: set new permission failed", "path", p.Path, "err", err)
						}
					}
				}
			} else {
				if perm, err := rbacClient.GetResourcePermissions(rqst.Files[i]); err == nil && perm != nil {
					if err := rbacClient.DeleteResourcePermissions(token, rqst.Files[i]); err != nil {
						slog.Warn("move: delete file permission failed", "path", rqst.Files[i], "err", err)
					}
					perm.Path = rqst.Path + "/" + filepath.Base(perm.Path)
					if err := rbacClient.SetResourcePermissions(token, perm.Path, perm.ResourceType, perm); err != nil {
						slog.Warn("move: set file permission failed", "path", perm.Path, "err", err)
					}
				}
			}

			// Move hidden folder if exists
			baseDir := filepath.Dir(from)
			fileName := filepath.Base(from)
			if dot := strings.LastIndex(fileName, "."); dot != -1 {
				fileName = fileName[:dot]
			}
			hiddenFolder := filepath.Join(baseDir, ".hidden", fileName)
			if Utility.Exists(hiddenFolder) {
				if err := Utility.CreateDirIfNotExist(filepath.Join(dest, ".hidden")); err != nil {
					slog.Warn("move: ensure .hidden failed", "dest", dest, "err", err)
				}
				if err := Utility.Move(hiddenFolder, filepath.Join(dest, ".hidden")); err != nil {
					slog.Warn("move: move hidden failed", "src", hiddenFolder, "dest", filepath.Join(dest, ".hidden"), "err", err)
				} else {
					output := filepath.Join(dest, ".hidden", fileName, "__timeline__")
					if err := createVttFile(output, 0.2); err != nil {
						slog.Warn("move: create VTT failed", "output", output, "err", err)
					}
				}
			}
		}
	}
	return &filepb.MoveResponse{Result: true}, nil
}

func (srv *server) Rename(ctx context.Context, rqst *filepb.RenameRequest) (*filepb.RenameResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	path := srv.formatPath(rqst.GetPath())
	if Utility.Exists(filepath.Join(path, rqst.NewName)) {
		return nil, fmt.Errorf("file with name %q already exists at path %q", rqst.NewName, path)
	}

	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	titles := make(map[string][]*titlepb.Title)
	_ = srv.getFileTitlesAssociation(client, rqst.GetPath()+"/"+rqst.OldName, titles)

	videos := make(map[string][]*titlepb.Video)
	_ = srv.getFileVideosAssociation(client, rqst.GetPath()+"/"+rqst.OldName, videos)

	// Dissociate titles
	for f, list := range titles {
		for _, t := range list {
			if err := client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f); err != nil {
				slog.Warn("rename: dissociate title failed", "file", f, "titleID", t.ID, "err", err)
			}
		}
	}
	// Dissociate videos
	for f, list := range videos {
		for _, v := range list {
			if err := client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", v.ID, f); err != nil {
				slog.Warn("rename: dissociate video failed", "file", f, "videoID", v.ID, "err", err)
			}
		}
	}

	from := rqst.GetPath() + "/" + rqst.OldName
	dest := rqst.GetPath() + "/" + rqst.NewName
	info, _ := os.Stat(srv.formatPath(from))

	rbacClient, err := getRbacClient()
	if err != nil {
		return nil, err
	}
	filePerms, _ := rbacClient.GetResourcePermissionsByResourceType("file")
	perm, _ := rbacClient.GetResourcePermissions(from)

	cache.RemoveItem(filepath.Join(path, rqst.OldName))
	cache.RemoveItem(path)

	if err := os.Rename(filepath.Join(path, rqst.OldName), filepath.Join(path, rqst.NewName)); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associate titles
	for f, list := range titles {
		for _, t := range list {
			var f_ string
			if !info.IsDir() {
				f_ = dest
			} else {
				f_ = strings.ReplaceAll(f, from, dest)
			}
			if err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f_); err != nil {
				slog.Warn("rename: associate title failed", "titleID", t.ID, "file", f_, "err", err)
			}
		}
	}
	// Associate videos
	for f, list := range videos {
		for _, v := range list {
			var f_ string
			if !info.IsDir() {
				f_ = dest
			} else {
				f_ = strings.ReplaceAll(f, from, dest)
			}
			if err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", v.ID, f_); err != nil {
				slog.Warn("rename: associate video failed", "videoID", v.ID, "file", f_, "err", err)
			}
		}
	}

	// Update permissions
	if info.IsDir() {
		for i := 0; i < len(filePerms); i++ {
			p := filePerms[i]
			if strings.HasPrefix(p.Path, from) {
				if err := rbacClient.DeleteResourcePermissions(token, p.Path); err != nil {
					slog.Warn("rename: delete old permission failed", "path", p.Path, "err", err)
				}
				p.Path = strings.ReplaceAll(p.Path, from, dest)
				if err := rbacClient.SetResourcePermissions(token, p.Path, p.ResourceType, p); err != nil {
					slog.Warn("rename: set new permission failed", "path", p.Path, "err", err)
				}
			}
		}
	} else if perm != nil {
		if err := rbacClient.DeleteResourcePermissions(token, from); err != nil {
			slog.Warn("rename: delete file permission failed", "path", from, "err", err)
		}
		perm.Path = dest
		if err := rbacClient.SetResourcePermissions(token, dest, perm.ResourceType, perm); err != nil {
			slog.Warn("rename: set file permission failed", "path", dest, "err", err)
		}
	}

	// Rename .hidden folder if exists
	oldBase := rqst.GetOldName()
	if idx := strings.LastIndex(oldBase, "/"); idx != -1 {
		oldBase = oldBase[idx+1:]
	}
	if dot := strings.LastIndex(oldBase, "."); dot != -1 {
		oldBase = oldBase[:dot]
	}

	newBase := rqst.GetNewName()
	if idx := strings.LastIndex(newBase, "/"); idx != -1 {
		newBase = newBase[idx+1:]
	}
	if dot := strings.LastIndex(newBase, "."); dot != -1 {
		newBase = newBase[:dot]
	}

	hiddenFrom := filepath.Join(path, ".hidden", oldBase)
	hiddenTo := filepath.Join(path, ".hidden", newBase)
	if Utility.Exists(hiddenFrom) {
		if err := os.Rename(hiddenFrom, hiddenTo); err != nil {
			slog.Warn("rename: rename hidden folder failed", "from", hiddenFrom, "to", hiddenTo, "err", err)
		}
	}

	slog.Info("rename completed", "path", path, "old", rqst.OldName, "new", rqst.NewName)
	return &filepb.RenameResponse{Result: true}, nil
}
