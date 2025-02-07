package main

import (
	"bufio"
	"bytes"
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/barasher/go-exiftool"
	"github.com/karmdip-mi/go-fitz"

	wkhtml "github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/davecourtois/Utility"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/file/file_client"
	"github.com/globulario/services/golang/file/filepb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/media/media_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/search/search_engine"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/title_client"
	"github.com/globulario/services/golang/title/titlepb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/tealeg/xlsx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10043
	defaultProxy = 10044

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// Client to validate and change file and directory permission.
	rbac_client_ *rbac_client.Rbac_Client

	// Here I will keep files info in cache...
	cache storage_store.Store
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id                 string
	Mac                string
	Name               string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	AllowAllOrigins    bool
	AllowedOrigins     string // comma separated string.
	Protocol           string
	Domain             string
	Address            string
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	CertFile           string
	CertAuthorityTrust string
	KeyFile            string
	TLS                bool
	Version            string
	PublisherId        string
	KeepUpToDate       bool
	Plaform            string
	Checksum           string
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	State              string

	// The grpc server.
	grpcServer *grpc.Server

	// The root contain applications and users data folder.
	Root string

	// Define the backend to use as cache it can be scylla, badger or leveldb the default is bigcache a memory cache.
	CacheType string

	// Define the cache address in case is not local.
	CacheAddress string

	// the number of replication for the cache.
	CacheReplicationFactor int

	// Public contain a list of paths reachable by the file srv.
	Public []string
}

// The path of the configuration.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
}

// The http address where the configuration can be found /config
func (srv *server) GetAddress() string {
	return srv.Address
}

func (srv *server) SetAddress(address string) {
	srv.Address = address
}

func (srv *server) GetProcess() int {
	return srv.Process
}

func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		if cache != nil {
			cache.Close()
		}
	}
	srv.Process = pid
}

func (srv *server) GetProxyProcess() int {
	return srv.ProxyProcess
}

func (srv *server) SetProxyProcess(pid int) {
	srv.ProxyProcess = pid
}

// The current service state
func (srv *server) GetState() string {
	return srv.State
}

func (srv *server) SetState(state string) {
	srv.State = state
}

// The last error
func (srv *server) GetLastError() string {
	return srv.LastError
}

func (srv *server) SetLastError(err string) {
	srv.LastError = err
}

// The modeTime
func (srv *server) SetModTime(modtime int64) {
	srv.ModTime = modtime
}
func (srv *server) GetModTime() int64 {
	return srv.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (srv *server) GetId() string {
	return srv.Id
}
func (srv *server) SetId(id string) {
	srv.Id = id
}

// The name of a service, must be the gRpc Service name.
func (srv *server) GetName() string {
	return srv.Name
}
func (srv *server) SetName(name string) {
	srv.Name = name
}

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}
func (srv *server) SetDescription(description string) {
	srv.Description = description
}

func (srv *server) GetMac() string {
	return srv.Mac
}

func (srv *server) SetMac(mac string) {
	srv.Mac = mac
}

// The list of keywords of the services.
func (srv *server) GetKeywords() []string {
	return srv.Keywords
}
func (srv *server) SetKeywords(keywords []string) {
	srv.Keywords = keywords
}

func (srv *server) GetRepositories() []string {
	return srv.Repositories
}
func (srv *server) SetRepositories(repositories []string) {
	srv.Repositories = repositories
}

func (srv *server) GetDiscoveries() []string {
	return srv.Discoveries
}
func (srv *server) SetDiscoveries(discoveries []string) {
	srv.Discoveries = discoveries
}

// Dist
func (srv *server) Dist(path string) (string, error) {

	return globular.Dist(path, srv)
}

func (srv *server) GetDependencies() []string {

	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}

	return srv.Dependencies
}

func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}

	// Append the depency to the list.
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

func (srv *server) GetChecksum() string {

	return srv.Checksum
}

func (srv *server) SetChecksum(checksum string) {
	srv.Checksum = checksum
}

func (srv *server) GetPlatform() string {
	return srv.Plaform
}

func (srv *server) SetPlatform(platform string) {
	srv.Plaform = platform
}

// The path of the executable.
func (srv *server) GetPath() string {
	return srv.Path
}
func (srv *server) SetPath(path string) {
	srv.Path = path
}

// The path of the .proto file.
func (srv *server) GetProto() string {
	return srv.Proto
}
func (srv *server) SetProto(proto string) {
	srv.Proto = proto
}

// The gRpc port.
func (srv *server) GetPort() int {
	return srv.Port
}
func (srv *server) SetPort(port int) {
	srv.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (srv *server) GetProxy() int {
	return srv.Proxy
}
func (srv *server) SetProxy(proxy int) {
	srv.Proxy = proxy
}

// Can be one of http/https/tls
func (srv *server) GetProtocol() string {
	return srv.Protocol
}
func (srv *server) SetProtocol(protocol string) {
	srv.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (srv *server) GetAllowAllOrigins() bool {
	return srv.AllowAllOrigins
}
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) {
	srv.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (srv *server) GetAllowedOrigins() string {
	return srv.AllowedOrigins
}

func (srv *server) SetAllowedOrigins(allowedOrigins string) {
	srv.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (srv *server) GetDomain() string {
	return srv.Domain
}
func (srv *server) SetDomain(domain string) {
	srv.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (srv *server) GetTls() bool {
	return srv.TLS
}
func (srv *server) SetTls(hasTls bool) {
	srv.TLS = hasTls
}

// The certificate authority file
func (srv *server) GetCertAuthorityTrust() string {
	return srv.CertAuthorityTrust
}
func (srv *server) SetCertAuthorityTrust(ca string) {
	srv.CertAuthorityTrust = ca
}

// The certificate file.
func (srv *server) GetCertFile() string {
	return srv.CertFile
}
func (srv *server) SetCertFile(certFile string) {
	srv.CertFile = certFile
}

// The key file.
func (srv *server) GetKeyFile() string {
	return srv.KeyFile
}
func (srv *server) SetKeyFile(keyFile string) {
	srv.KeyFile = keyFile
}

// The service version
func (srv *server) GetVersion() string {
	return srv.Version
}
func (srv *server) SetVersion(version string) {
	srv.Version = version
}

// The publisher id.
func (srv *server) GetPublisherId() string {
	return srv.PublisherId
}
func (srv *server) SetPublisherId(publisherId string) {
	srv.PublisherId = publisherId
}

func (srv *server) GetKeepUpToDate() bool {
	return srv.KeepUpToDate
}
func (srv *server) SetKeepUptoDate(val bool) {
	srv.KeepUpToDate = val
}

func (srv *server) GetKeepAlive() bool {
	return srv.KeepAlive
}
func (srv *server) SetKeepAlive(val bool) {
	srv.KeepAlive = val
}

func (srv *server) GetPermissions() []interface{} {
	return srv.Permissions
}
func (srv *server) SetPermissions(permissions []interface{}) {
	srv.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (srv *server) Init() error {

	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (srv *server) Save() error {
	// Create the file...
	return globular.SaveService(srv)
}

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

func (srv *server) Stop(context.Context, *filepb.StopRequest) (*filepb.StopResponse, error) {
	return &filepb.StopResponse{}, srv.StopService()
}

func (s *server) getThumbnail(path string, h, w int) (string, error) {

	id := path + "_" + Utility.ToString(h) + "x" + Utility.ToString(w) + "@" + s.Domain

	data, err := cache.GetItem(id)
	if err == nil {
		return string(data), nil
	}

	t, err := Utility.CreateThumbnail(path, h, w)
	if err != nil {
		return "", err
	}

	cache.SetItem(id, []byte(t))

	return t, nil
}

func getFileInfo(s *server, path string, thumbnailMaxHeight, thumbnailMaxWidth int) (*filepb.FileInfo, error) {

	path = strings.ReplaceAll(path, "\\", "/")
	info := new(filepb.FileInfo)
	info.Path = path

	fileStat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Cut the root part of the path if it start with the root path.
	if strings.HasPrefix(info.Path, config.GetDataDir()+"/files") {
		info.Path = info.Path[len(config.GetDataDir()+"/files"):]
	}

	if strings.Contains(path, "/.hidden") {
		if strings.HasSuffix(path, "__preview__") || strings.HasSuffix(path, "__timeline__") || strings.HasSuffix(path, "__thumbnail__") {
			info.Mime = "inode/directory"
			info.IsDir = true
			return info, nil
		}
	}

	// Here I will try to get the info from the cache...
	data, err_ := cache.GetItem(path + "@" + s.Domain)
	if err_ == nil {
		err := protojson.Unmarshal(data, info)
		if err == nil {
			// change the mime type to stream if the dir contain a playlist.
			if info.IsDir {
				if Utility.Exists(path + "/playlist.m3u8") {
					info.Mime = "video/hls-stream"
				}
			}

			return info, nil
		}

		// remove it from the cache...
		cache.RemoveItem(path)
	}

	info.IsDir = fileStat.IsDir()
	if info.IsDir {
		info.Mime = "inode/directory"
		path_, err := os.Getwd()
		if err == nil {
			path_ = strings.ReplaceAll(path_, "\\", "/")
			path_ = path_ + "/mimetypes/inode-directory.png"
			info.Thumbnail, _ = s.getMimeTypesUrl(path_)
		}
	} else {
		info.Checksum = Utility.CreateFileChecksum(path)
	}

	info.Size = fileStat.Size()
	info.Name = fileStat.Name()
	info.ModeTime = fileStat.ModTime().Unix()

	// Now the section depending of the mime type...
	if !info.IsDir {
		if strings.Contains(fileStat.Name(), ".") {
			fileExtension := fileStat.Name()[strings.LastIndex(fileStat.Name(), "."):]
			info.Mime = mime.TypeByExtension(fileExtension)
		} else {
			f_, err := os.Open(path)
			if err != nil {
				return nil, err
			}

			info.Mime, err = Utility.GetFileContentType(f_)
			if err != nil {
				return nil, err
			}

			defer f_.Close()
		}

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		path_, err := os.Getwd()
		if err == nil {

			// Here I will read the metadata and set it if it
			// exist...
			if !strings.Contains(path, "/.hidden/") {
				path_ = strings.ReplaceAll(path_, "\\", "/")
				path_ = path_ + "/mimetypes/unknown.png"
				info.Thumbnail, _ = s.getMimeTypesUrl(path_)
			}

			// If hidden folder exist for it...
			path_ := filepath.Dir(path)
			fileName := filepath.Base(path)
			if strings.Contains(fileName, ".") {
				fileName = fileName[0:strings.LastIndex(fileName, ".")]
			}

			hiddenFolder := path_ + "/.hidden/" + fileName

			// in case of image...
			if strings.HasPrefix(info.Mime, "image/") {
				if thumbnailMaxHeight > 0 && thumbnailMaxWidth > 0 {
					info.Thumbnail, _ = s.getThumbnail(path, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
				} else {
					info.Thumbnail, _ = s.getThumbnail(path, 80, 80)
				}
			} else if strings.HasPrefix(info.Mime, "video/") {

				if Utility.Exists(hiddenFolder) {
					// Here I will auto generate preview if it not already exist...
					if !Utility.Exists(hiddenFolder + "/__preview__/preview_00001.jpg") {
						// generate the preview...
						os.RemoveAll(hiddenFolder + "/__preview__") // be sure it will
						go s.createVideoPreview(info.Path, 20, 128)

						os.RemoveAll(hiddenFolder + "/__timeline__") // be sure it will
						go s.createVideoTimeLine(info.Path, 180, .2) // 1 frame per 5 seconds.
					}

					if Utility.Exists(hiddenFolder + "/__preview__/preview_00001.jpg") {
						// So here if the mime type is a video I will get thumbnail from it preview images.
						info.Thumbnail, err = s.getThumbnail(hiddenFolder+"/__preview__/preview_00001.jpg", -1, -1)
						if err != nil {
							fmt.Println("fail to create thumbnail with error: ", err)
						}

					} else if Utility.Exists(hiddenFolder + "/__thumbnail__/data_url.txt") {
						thumbnail, err := os.ReadFile(hiddenFolder + "/__thumbnail__/data_url.txt")
						if err == nil {
							info.Thumbnail = string(thumbnail)
						}

					} else if Utility.Exists(hiddenFolder + "/__thumbnail__") {
						// Here I will try to read data from the thumbnail dir...

						files, err := Utility.ReadDir(hiddenFolder + "/__thumbnail__")
						if err == nil {
							for i := 0; i < len(files); i++ {
								f := files[i]

								info.Thumbnail, err = s.getThumbnail(hiddenFolder+"/__thumbnail__"+"/"+f.Name(), 72, 128)
								if err == nil {
									os.WriteFile(hiddenFolder+"/__thumbnail__/data_url.txt", []byte(info.Thumbnail), 0644)
									break
								}
							}
						}
					}

				} else {
					path_, err := os.Getwd()
					path_ = strings.ReplaceAll(path_, "\\", "/")
					if err == nil {
						path_ = path_ + "/mimetypes/video-x-generic.png"
						info.Thumbnail, _ = s.getMimeTypesUrl(path_)
					}
				}

			} else if strings.HasPrefix(info.Mime, "audio/") || strings.HasSuffix(path, ".flac") || strings.HasSuffix(path, ".mp3") {
				// duration := Utility.GetVideoDuration(path)
				metadata, err := Utility.ReadAudioMetadata(path, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
				if err == nil {
					info.Thumbnail = metadata["ImageUrl"].(string)
				}

			} else if Utility.Exists(hiddenFolder + "/__thumbnail__/data_url.txt") {
				thumbnail, err := os.ReadFile(hiddenFolder + "/__thumbnail__/data_url.txt")
				if err == nil {
					info.Thumbnail = string(thumbnail)
				}
			} else if strings.Contains(info.Mime, "/") {

				// In that case I will get read image from png file and create a
				// thumbnail with it...
				path_, err := os.Getwd()
				if err == nil {
					path_ = strings.ReplaceAll(path_, "\\", "/")
					path_ = path_ + "/mimetypes/" + strings.ReplaceAll(strings.Split(info.Mime, ";")[0], "/", "-") + ".png"
					info.Thumbnail, _ = s.getMimeTypesUrl(path_)
				}
			}
		}
	} else {
		if Utility.Exists(path + "/playlist.m3u8") {
			path_ := path[0:strings.LastIndex(path, "/")]
			fileName := path[strings.LastIndex(path, "/")+1:]
			hiddenFolder := path_ + "/.hidden/" + fileName
			if Utility.Exists(hiddenFolder) {
				previewImage := hiddenFolder + "/__preview__/preview_00001.jpg"
				if Utility.Exists(previewImage) {
					// So here if the mime type is a video I will get thumbnail from it preview images.

					info.Thumbnail, err = s.getThumbnail(previewImage, -1, -1)
					if err != nil {
						fmt.Println("fail to create thumbnail with error: ", err)
					}

				}
			} else {
				path_, err := os.Getwd()
				if err == nil {
					path_ = strings.ReplaceAll(path_, "\\", "/")
					path_ = path_ + "/mimetypes/video-x-generic.png"
					info.Thumbnail, _ = s.getMimeTypesUrl(path_)
				}
			}
		}
	}

	data_, err := protojson.Marshal(info)
	if err == nil {
		cache.SetItem(path, data_)
	}

	return info, nil
}

func printDownloadPercent(done chan int64, path string, total int64, stream filepb.FileService_UploadFileServer) {

	var stop bool = false

	for {
		select {
		case <-done:
			stop = true
		default:

			file, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}

			fi, err := file.Stat()
			if err != nil {
				log.Fatal(err)
			}

			size := fi.Size()

			if size == 0 {
				size = 1
			}

			var percent float64 = float64(size) / float64(total) * 100
			progress := fmt.Sprintf("%.2f", percent)

			fmt.Println(progress)

			stream.Send(
				&filepb.UploadFileResponse{
					Uploaded: size,
					Total:    total,
					Info:     progress + "%",
				},
			)
		}

		if stop {
			break
		}

		time.Sleep(time.Second)
	}
}

func (srv *server) uploadFile(token, url, dest, name string, stream filepb.FileService_UploadFileServer) error {
	var err error

	path := srv.formatPath(dest)
	if !Utility.Exists(path) {
		return errors.New("no folder found with path " + path)
	}

	// Now I will set the path to the hidden folder...
	Utility.CreateDirIfNotExist(path)

	start := time.Now()
	out, err := os.Create(path + "/" + name)
	if err != nil {
		return err
	}

	defer out.Close()
	headResp, err := http.Head(url)

	if err != nil {
		return err
	}

	defer headResp.Body.Close()
	size, err := strconv.Atoi(headResp.Header.Get("Content-Length"))

	if err != nil {
		return err
	}

	done := make(chan int64)
	go printDownloadPercent(done, path+"/"+name, int64(size), stream)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	n, err := io.Copy(out, resp.Body)

	if err != nil {
		return err
	}

	done <- n

	elapsed := time.Since(start)
	log.Printf("Download completed in %s", elapsed)

	stream.Send(
		&filepb.UploadFileResponse{
			Uploaded: 100,
			Total:    100,
			Info:     fmt.Sprintf("Download completed in %.2fs", elapsed.Seconds()),
		},
	)

	// Now I will set the file permission.
	srv.setOwner(token, dest+"/"+name)

	info, err := getFileInfo(srv, path+"/"+name, -1, -1)
	if err == nil {
		if strings.HasPrefix(info.Mime, "video/") {
			// Here I will resfresh generate video related files...
			stream.Send(
				&filepb.UploadFileResponse{
					Uploaded: 100,
					Total:    100,
					Info:     "Process video information...",
				},
			)
			processVideos(srv, token, []string{path})

		} else if strings.HasSuffix(info.Name, ".pdf") {
			stream.Send(
				&filepb.UploadFileResponse{
					Uploaded: 100,
					Total:    100,
					Info:     "Index text information...",
				},
			)
			srv.indexPdfFile(path+"/"+name, info)
		}
	}

	stream.Send(
		&filepb.UploadFileResponse{
			Uploaded: 100,
			Total:    100,
			Info:     "Done",
		},
	)

	return err
}

/**
 * Get a file client for a given domain.
 */
func (srv *server) GetFileClient(address string) (*file_client.File_Client, error) {
	// validate the port has not change...
	Utility.RegisterFunction("NewFileService_Client", file_client.NewFileService_Client)
	client, err := globular_client.GetClient(address, "file.FileService", "NewFileService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*file_client.File_Client), nil
}

// Upload a video from a given url, it use youtube-dl.
func (srv *server) UploadFile(rqst *filepb.UploadFileRequest, stream filepb.FileService_UploadFileServer) error {

	_, token, err := security.GetClientId(stream.Context())
	if err != nil {
		return err
	}

	if rqst.IsDir {

		file_client_, err := srv.GetFileClient(rqst.Domain)
		if err != nil {
			return err
		}

		u, err := url.Parse(rqst.Url)
		if err != nil {
			return err
		}

		stream.Send(
			&filepb.UploadFileResponse{
				Uploaded: 100,
				Total:    100,
				Info:     "Create archive for " + rqst.Name + " ...",
			},
		)

		// create temporary archive on the remote srv.
		__name__ := Utility.RandomUUID()
		archive_path_, err := file_client_.CreateArchive(token, []string{u.Path}, __name__)
		if err != nil {
			return err
		}

		archive_url_ := u.Scheme + "://" + u.Host + archive_path_ + "?token=" + token

		err = srv.uploadFile(token, archive_url_, rqst.Dest, __name__+".tar.gz", stream)
		if err != nil {
			return err
		}

		// I can now remove the created archived file...
		file_client_.DeleteFile(token, archive_path_)

		path := srv.formatPath(rqst.Dest)

		stream.Send(
			&filepb.UploadFileResponse{
				Uploaded: 100,
				Total:    100,
				Info:     "Unpack archive for " + rqst.Name + " ...",
			},
		)

		defer os.RemoveAll(path + "/" + __name__ + ".tar.gz")

		// Now I will unpack the archive in it destination.
		file, err := os.Open(path + "/" + __name__ + ".tar.gz")

		if err != nil {
			return err
		}

		defer file.Close()

		r := bufio.NewReader(file)
		_extracted_path_, err := Utility.ExtractTarGz(r)
		if err != nil {
			return err
		}

		err = Utility.Move(_extracted_path_, path)
		if err != nil {
			return err
		}

		err = os.Rename(path+"/"+filepath.Base(_extracted_path_), path+"/"+rqst.Name)
		if err != nil {
			return err
		}

		if Utility.Exists(path + "/" + rqst.Name + "/playlist.m3u8") {
			stream.Send(
				&filepb.UploadFileResponse{
					Uploaded: 100,
					Total:    100,
					Info:     "Process video information...",
				},
			)
			processVideos(srv, token, []string{path + "/" + rqst.Name})

		}

		// Set the file owner.
		srv.setOwner(token, rqst.Dest+"/"+rqst.Name)

		srv.publishReloadDirEvent(path)
	} else {

		// Start upload video...
		err = srv.uploadFile(token, rqst.Url, rqst.Dest, rqst.Name, stream)

		// display the error...
		if err != nil {
			stream.Send(
				&filepb.UploadFileResponse{
					Info: "fail to upload file " + rqst.Name + " with error " + err.Error(),
				},
			)
			if strings.Contains(err.Error(), "signal: killed") {
				return errors.New("fail to upload file " + rqst.Name + " with error " + err.Error())

			}
		} else {
			// Reload the
			srv.publishReloadDirEvent(rqst.Dest)
		}
	}

	return err
}

func ExtractMetada(path string) (map[string]interface{}, error) {
	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, err
	}

	defer et.Close()

	fileInfos := et.ExtractMetadata(path)
	if len(fileInfos) > 0 {
		return fileInfos[0].Fields, nil
	}

	return nil, errors.New("no metadata found for " + path)
}


// indexPdfFile indexes the content of a PDF file into a search engine.
func (srv *server) indexPdfFile(path string, fileInfos *filepb.FileInfo) error {
	log.Println("Indexing PDF file:", path)

	// Validate MIME type
	if fileInfos.Mime != "application/pdf" {
		return fmt.Errorf("file is not a PDF: %s", path)
	}

	// Determine base paths
	dir := filepath.Dir(path)
	baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	hiddenFolder := filepath.Join(dir, ".hidden", baseName)

	thumbnailPath := filepath.Join(hiddenFolder, "__thumbnail__")
	indexationPath := filepath.Join(hiddenFolder, "__index_db__")

	// Create necessary directories
	Utility.CreateIfNotExists(thumbnailPath, 0755)
	Utility.CreateIfNotExists(indexationPath, 0755)

	// Check if indexing is already done
	if Utility.Exists(filepath.Join(thumbnailPath, "data_url.txt")) {
		return errors.New("indexing info already exists")
	}

	// Open PDF document
	doc, err := fitz.New(path)
	if err != nil {
		return fmt.Errorf("failed to open PDF file: %w", err)
	}
	defer doc.Close()

	// Extract metadata
	metadata, _ := ExtractMetada(path)
	metadataJSON, _ := Utility.ToJson(metadata)
	docId :=   Utility.GenerateUUID(path)
	metadata["DocId"] = docId

	// Initialize search engine
	searchEngine := new(search_engine.BleveSearchEngine)
	err = searchEngine.IndexJsonObject(indexationPath, metadataJSON, "english", "SourceFile", []string{"FileName", "Author", "Producer", "Title"}, "")

	if err != nil {
		log.Println("Metadata indexing failed:", err)
	}

	// Process pages
	for i := 0; i < doc.NumPage(); i++ {
		pageMap := map[string]interface{}{
			"Id":     fmt.Sprintf("%s_page_%d", docId, i),
			"Number": i,
			"Path":   path,
			"DocId":  docId,
		}

		// Process first page image as thumbnail
		if i == 0 {
			if err := processThumbnail(doc, thumbnailPath); err != nil {
				log.Println("Failed to create thumbnail:", err)
			}
		}

		// Extract text from the page
		text, err := doc.Text(i)
		if err != nil || len(text) == 0 {
			text, err = extractTextFromImage(doc, i)
			if err != nil {
				log.Println("Failed to extract text from image on page", i, ":", err)
			}
		}

		pageMap["Text"] = Utility.CleanText(text)
		pageJSON, err := Utility.ToJson(pageMap)
		if err == nil {
			err = searchEngine.IndexJsonObject(indexationPath, pageJSON, "english", "Id", []string{"Text"}, "")
			if err != nil {
				log.Println("Failed to index page", i, ":", err)
			}else{
				fmt.Println(pageJSON)
			}
		}
	}

	return nil
}

// processThumbnail generates a thumbnail from the first page of a PDF.
func processThumbnail(doc *fitz.Document, thumbnailPath string) error {
	img, err := doc.Image(0)
	if err != nil || img == nil {
		return fmt.Errorf("no image found on the first page")
	}

	tmpFile := filepath.Join(os.TempDir(), Utility.RandomUUID()+".jpg")
	defer os.Remove(tmpFile)

	f, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: jpeg.DefaultQuality}); err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	dataURL, err := Utility.CreateThumbnail(tmpFile, 256, 256)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail: %w", err)
	}

	return os.WriteFile(filepath.Join(thumbnailPath, "data_url.txt"), []byte(dataURL), 0755)
}

// extractTextFromImage runs OCR on a page image if text extraction fails.
func extractTextFromImage(doc *fitz.Document, pageIndex int) (string, error) {
	img, err := doc.Image(pageIndex)
	if err != nil || img == nil {
		return "", fmt.Errorf("no image found on page %d", pageIndex)
	}

	tmpFile := filepath.Join(os.TempDir(), Utility.RandomUUID()+".jpg")
	defer os.Remove(tmpFile)

	f, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: jpeg.DefaultQuality}); err != nil {
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	return Utility.ExtractTextFromJpeg(tmpFile)
}
// Index text contain in a pdf file
func (srv *server) indexTextFile(path string, fileInfos *filepb.FileInfo) error {

	if fileInfos.Mime != "text/plain" {
		fmt.Println("file is not a text file")
		return errors.New("file is not a text file")
	}

	// test if the file exist...
	if !Utility.Exists(path) {
		return errors.New("file not found")
	}

	// The hidden folder path...
	path_ := path[0:strings.LastIndex(path, "/")]
	lastIndex := -1
	if strings.Contains(path, ".pdf") {
		lastIndex = strings.LastIndex(path, ".")
	}

	name_ := path[strings.LastIndex(path, "/")+1:]
	if lastIndex != -1 {
		name_ = path[strings.LastIndex(path, "/")+1 : lastIndex]
	}

	hidden_folder := path_ + "/.hidden/" + name_

	thumbnail_path := hidden_folder + "/__thumbnail__"
	Utility.CreateIfNotExists(thumbnail_path, 0755)

	indexation_path := hidden_folder + "/__index_db__"
	Utility.CreateIfNotExists(indexation_path, 0755)

	if Utility.Exists(thumbnail_path + "/data_url.txt") {
		return errors.New("info already exist")
	}


	metadata_, _ := ExtractMetada(path)
	metadata_str, _ := Utility.ToJson(metadata_)

	search_engine := new(search_engine.BleveSearchEngine)

	err := search_engine.IndexJsonObject(indexation_path, metadata_str, "english", "SourceFile", []string{"FileName", "Author", "Producer", "Title"}, "")
	if err != nil {
		log.Println(err)
	}

	doc_ := make(map[string]interface{})

	doc_["Metadata"] = metadata_

	// Now the text...
	text, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	doc_["Text"] = text

	doc_str, err := Utility.ToJson(doc_)

	if err == nil {
		err = search_engine.IndexJsonObject(indexation_path, doc_str, "english", "SourceFile", []string{"Text"}, "")
		if err != nil {
			fmt.Println("fail to text file: ", err)
		}
	}

	return err
}

// That function is use to index file at given path so the user will be able to
// search
func (srv *server) indexFile(path string) error {

	// from the mime type I will choose how the document must be indexed.
	fileInfos, err := getFileInfo(srv, path, -1, -1)

	if err != nil {
		return err
	}

	if fileInfos.Mime == "application/pdf" {
		return srv.indexPdfFile(path, fileInfos)
	} else if strings.HasPrefix(fileInfos.Mime, "text") {
		return srv.indexTextFile(path, fileInfos)
	}

	return errors.New("no indexation exist for file type " + fileInfos.Mime)
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

/**
 * Read the directory and return the file info.
 */
func readDir(s *server, path string, recursive bool, thumbnailMaxWidth int32, thumbnailMaxHeight int32, readFiles bool, /*token string,*/ fileInfos_chan chan *filepb.FileInfo, err_chan chan error) (*filepb.FileInfo, error) {

	// get the file info
	info, err := getFileInfo(s, path, int(thumbnailMaxWidth), int(thumbnailMaxWidth))
	if err != nil {
		if err_chan != nil {
			err_chan <- err
		}

		return nil, err
	}

	if !info.IsDir {
		// make the server panic...
		if err_chan != nil {
			err_chan <- errors.New("path " + path + " is not a directory")
		}
		return nil, err
	}

	// read list of files...
	files, err := os.ReadDir(path)
	if err != nil {
		if err_chan != nil {
			err_chan <- err
		}
		return nil, err
	}

	if fileInfos_chan != nil {
		fileInfos_chan <- info
	}

	for _, f := range files {

		if f.IsDir() {
			// Here if the dir contain the file playlist.m3u8 it means it content must not be read as a file but as stream,
			// so I will not read it content...
			dirPath := path + "/" + f.Name()

			// Test if a file named playlist.m3u8 exist...
			isHls := Utility.Exists(dirPath + "/playlist.m3u8")

			if recursive && !isHls && f.Name() != ".hidden" {

				info_, err := readDir(s, dirPath, recursive, thumbnailMaxWidth, thumbnailMaxHeight, true, fileInfos_chan, err_chan)
				if err != nil {
					fmt.Println("fail to read dir ", dirPath, " with error ", err)
					if err_chan != nil {
						err_chan <- err
					}
					return nil, err
				}

				if fileInfos_chan != nil {
					fileInfos_chan <- info_
				} else {
					info.Files = append(info.Files, info_)
				}

			} else if f.Name() != ".hidden" { // I will not read sub-dir hidden files...
				info_, err := readDir(s, dirPath, recursive, thumbnailMaxWidth, thumbnailMaxHeight, false, fileInfos_chan, err_chan)
				if err != nil {
					if err_chan != nil {
						err_chan <- err
					}
					return nil, err
				}
				if isHls {
					info_.Mime = "video/hls-stream"
				}

				if fileInfos_chan != nil {
					fileInfos_chan <- info_
				} else {
					info.Files = append(info.Files, info_)
				}
			}

		} else if readFiles {

			info_, err := getFileInfo(s, path+"/"+f.Name(), int(thumbnailMaxHeight), int(thumbnailMaxWidth))
			if err != nil {
				if err_chan != nil {
					err_chan <- err
				}
				return nil, nil
			}

			
			if !info_.IsDir && readFiles {
				if strings.Contains(f.Name(), ".") {
					fileExtension := f.Name()[strings.LastIndex(f.Name(), "."):]
					info_.Mime = mime.TypeByExtension(fileExtension)
				} else {
					f_, err := os.Open(path + "/" + f.Name())
					if err != nil {
						if err_chan != nil {
							err_chan <- err
						}
						return nil, err
					}
					info_.Mime, _ = Utility.GetFileContentType(f_)
					f_.Close()
				}

				// Create thumbnail if the path is not in hidden file...
				if !strings.Contains(path, ".hidden") && len(info_.Thumbnail) == 0 {

					if strings.HasPrefix(info_.Mime, "image/") {
						if thumbnailMaxHeight > 0 && thumbnailMaxWidth > 0 {
							info_.Thumbnail, _ = s.getThumbnail(path+"/"+f.Name(), int(thumbnailMaxHeight), int(thumbnailMaxWidth))
						}
					} else if strings.Contains(info_.Mime, "/") {
						// In that case I will get read image from png file and create a
						// thumbnail with it...
						path_, err := os.Getwd()

						if err == nil {
							path_ = strings.ReplaceAll(path_, "\\", "/")
							path_ = path_ + "/mimetypes/" + strings.ReplaceAll(strings.Split(info_.Mime, ";")[0], "/", "-") + ".png"
							info_.Thumbnail, _ = s.getMimeTypesUrl(path_)
							if err != nil {
								fmt.Println("fail to create thumbnail with error ", err)
							}

						}
					} else {
						path_, err := os.Getwd()

						if err == nil {
							path_ = strings.ReplaceAll(path_, "\\", "/")
							path_ = path_ + "/mimetypes/unknown.png"
							info_.Thumbnail, _ = s.getMimeTypesUrl(path_)
						}
					}
				}

				if fileInfos_chan != nil {
					fileInfos_chan <- info_
				} else {
					
					info.Files = append(info.Files, info_)
				}
			}
		}

	}
	return info, nil
}

// return the icon address...
func (srv *server) getMimeTypesUrl(path string) (string, error) {

	// http link...

	// data url...
	image_url, err := srv.getThumbnail(path, int(80), int(80))
	return image_url, err
}

func (srv *server) formatPath(path string) string {
	path, _ = url.PathUnescape(path)
	path = strings.ReplaceAll(path, "\\", "/")

	if strings.HasPrefix(path, "/") {
		if len(path) > 1 {
			if strings.HasPrefix(path, "/") {
				if !srv.isPublic(path) {
					// Must be in the root path if it's not in public path.
					if Utility.Exists(srv.Root + path) {
						path = srv.Root + path
					} else if Utility.Exists(config.GetWebRootDir() + path){
						path = config.GetWebRootDir() + path
					}else if strings.HasPrefix(path, "/users/") || strings.HasPrefix(path, "/applications/") {
						path = config.GetDataDir() + "/files" + path
					} else if Utility.Exists("/" + path) { // network path...
						path = "/" + path
					} else {
						path = srv.Root + "/" + path
					}
				}
			} else {
				path = srv.Root + "/" + path
			}
		} else {
			// '/' represent the root path
			path = srv.Root
		}
	}

	// remove the double slash...
	path = strings.ReplaceAll(path, "//", "/")

	return path
}

// //////////////////////////////////////////////////////////////////////////////
// Directory operations
// //////////////////////////////////////////////////////////////////////////////

func (srv *server) publishReloadDirEvent(path string) {
	client, err := getEventClient()
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.ReplaceAll(path, config.GetDataDir()+"/files", "")
	if err == nil {
		client.Publish("reload_dir_event", []byte(path))
	}
}

// Append public dir to the list of dir...
func (srv *server) AddPublicDir(ctx context.Context, rqst *filepb.AddPublicDirRequest) (*filepb.AddPublicDirResponse, error) {

	path := strings.ReplaceAll(rqst.Path, "\\", "/")

	if !Utility.Exists(path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("file with path " + rqst.Path + " doesn't exist")))
	}

	// So here I will test if the path is already in the public path...
	if Utility.Contains(srv.Public, path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), fmt.Errorf("Path %s already exist in Public paths", path)))
	}

	// Append the path in the list...
	srv.Public = append(srv.Public, path)

	// save it in the configuration...
	err := srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.AddPublicDirResponse{}, nil
}

// Append public dir to the list of dir...
func (srv *server) RemovePublicDir(ctx context.Context, rqst *filepb.RemovePublicDirRequest) (*filepb.RemovePublicDirResponse, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("file with path "+rqst.Path+"dosen't exist")))
	}

	// So here I will test if the path is already in the public path...
	if !Utility.Contains(srv.Public, rqst.Path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Path "+rqst.Path+" dosen exist in Pulbic paths")))
	}

	// Append the path in the list...
	srv.Public = Utility.RemoveString(srv.Public, rqst.Path)

	// save it in the configuration...
	err := srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.RemovePublicDirResponse{}, nil
}

// Return the list of public path from a given file srv...
func (srv *server) GetPublicDirs(context.Context, *filepb.GetPublicDirsRequest) (*filepb.GetPublicDirsResponse, error) {
	return &filepb.GetPublicDirsResponse{Dirs: srv.Public}, nil
}

// return the a list of all info
func getFileInfos(srv *server, info *filepb.FileInfo, infos []*filepb.FileInfo) []*filepb.FileInfo {

	infos = append(infos, info)
	for i := 0; i < len(info.Files); i++ {
		path_ := srv.formatPath(info.Files[i].Path)
		if Utility.Exists(path_) {
			// do not send Thumbnail...
			if info.Files[i].IsDir {
				if !Utility.Exists(path_ + "/playlist.m3u8") {
					info.Files[i].Thumbnail = "" // remove the icon  for dir
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

func (srv *server) ReadDir(rqst *filepb.ReadDirRequest, stream filepb.FileService_ReadDirServer) error {
	if len(rqst.Path) == 0 {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("path is empty")))
	}


	path := srv.formatPath(rqst.Path)
	filesInfoChan := make(chan *filepb.FileInfo)
	errChan := make(chan error)

	// I will test if the path exist...
	if !Utility.Exists(path) {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+path)))
	}

	// Now i test if the path is a directory...
	info, err := getFileInfo(srv, path, 64, 64)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if !info.IsDir {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("path " + path + " is not a directory")))
	}

	// Start reading the directory in a goroutine
	go func() {
		defer close(filesInfoChan) // Close the channel when the goroutine exits
		defer close(errChan)       // Ensure the error channel is closed
		readDir(srv, path, rqst.GetRecursive(), rqst.ThumbnailWidth, rqst.ThumbnailHeight, true, filesInfoChan, errChan)
	}()

	// Use select to handle both file info and errors
	for {
		select {
		case fileInfo, ok := <-filesInfoChan:
			if !ok { // Check if the channel is closed
				// If fileInfosChan is closed, return the error if it exists
				err := <-errChan // Receive any error from the error channel
				if err != nil {
					return status.Errorf(
						codes.Internal,
						Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}
				return nil // Successfully completed
			}

			// Send the file info to the stream
			if err := stream.Send(&filepb.ReadDirResponse{Info: fileInfo}); err != nil {
				fmt.Println("Failed to send file info ", fileInfo.Path+"/"+fileInfo.Name)
				fmt.Println("Error: ", err)
				if err.Error() == "rpc error: code = Canceled desc = context canceled" {
					return status.Errorf(
						codes.Internal,
						Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}
			}

		case err := <-errChan:

			if err != nil {
				// If there's an error from the readDir goroutine
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}else{
				return nil
			}
		}
	}

}

// Create a new directory
func (srv *server) CreateDir(ctx context.Context, rqst *filepb.CreateDirRequest) (*filepb.CreateDirResponse, error) {
	path := srv.formatPath(rqst.GetPath())
	err := Utility.CreateDirIfNotExist(path + "/" + rqst.GetName())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	err = srv.setOwner(token, rqst.GetPath()+"/"+rqst.GetName())
	if err != nil {
		return nil, err
	}

	// The directory was successfuly created.
	return &filepb.CreateDirResponse{
		Result: true,
	}, nil
}

// Return true if the file is found in the public path...
func (srv *server) isPublic(path string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	if Utility.Exists(path) {
		for i := 0; i < len(srv.Public); i++ {
			if strings.HasPrefix(path, srv.Public[i]) {
				return true
			}
		}
	}
	return false
}

// Create an archive from a given dir and set it with name.
func (srv *server) CreateArchive(ctx context.Context, rqst *filepb.CreateArchiveRequest) (*filepb.CreateArchiveResponse, error) {

	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Here I will create the directory...
	tmp := os.TempDir() + "/" + rqst.GetName()
	createTempDir := true

	// If there only one file no temps dir is required...
	if len(rqst.Paths) == 1 {
		path := rqst.Paths[0]
		if !srv.isPublic(path) {
			// if the path is not in the Public list it must be in the path...
			path = srv.formatPath(path)
		}

		// be sure the file exist.
		if !Utility.Exists(path) {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file exist for path "+path)))
		}

		info, _ := os.Stat(path)
		if info.IsDir() {
			tmp = path
			createTempDir = false
		}
	}

	// This will create a temporary directory...
	if createTempDir {
		Utility.CreateDirIfNotExist(tmp)
		defer os.RemoveAll(tmp)
		for i := 0; i < len(rqst.Paths); i++ {
			// The file or directory must be in the path.
			if Utility.Exists(srv.Root+rqst.Paths[i]) || srv.isPublic(rqst.Paths[i]) {
				path := rqst.Paths[i]
				path = srv.formatPath(path)

				info, _ := os.Stat(path)
				fileName := path[strings.LastIndex(path, "/"):]
				if info.IsDir() {
					Utility.CopyDir(path, tmp+"/"+fileName)
				} else {
					Utility.CopyFile(path, tmp+"/"+fileName)
				}
			}
		}
	}

	var buf bytes.Buffer
	Utility.CompressDir(tmp, &buf)
	dest := "/users/" + clientId + "/" + rqst.GetName() + ".tar.gz"

	// Set user as owner.
	srv.setOwner(token, dest)

	// Now I will save the file to the destination.
	err = os.WriteFile(srv.Root+dest, buf.Bytes(), 0644)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Return the link where the archive was created.
	return &filepb.CreateArchiveResponse{
		Result: dest,
	}, nil

}

func (srv *server) setOwner(token, path string) error {
	var clientId string

	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil {
			return err
		}

		if len(claims.UserDomain) == 0 {
			return errors.New("no user domain was found in the token")
		}

		clientId = claims.Id + "@" + claims.UserDomain
	} else {
		err := errors.New("CreateBlogPost no token was given")
		return err
	}

	// Set the owner of the conversation.
	rbac_client_, err := getRbacClient()
	if err != nil {
		return err
	}

	// if path was absolute I will make it relative data path.
	if strings.Contains(path, "/files/users/") {
		path = path[strings.Index(path, "/users/"):]
	}

	// So here I will need the local token.
	err = rbac_client_.AddResourceOwner(path, "file", clientId, rbacpb.SubjectType_ACCOUNT)

	if err != nil {
		return err
	}

	return nil
}

// Rename a file or a directory.
func (srv *server) Rename(ctx context.Context, rqst *filepb.RenameRequest) (*filepb.RenameResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	path := srv.formatPath(rqst.GetPath())

	if Utility.Exists(path + "/" + rqst.NewName) {
		return nil, errors.New("file with name '" + rqst.NewName + "' already exist at path '" + path + "'")
	}

	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	titles := make(map[string][]*titlepb.Title, 0)
	srv.getFileTitlesAssociation(client, rqst.GetPath()+"/"+rqst.OldName, titles)

	videos := make(map[string][]*titlepb.Video, 0)
	srv.getFileVideosAssociation(client, rqst.GetPath()+"/"+rqst.OldName, videos)

	// Dissociates titles...
	for f, titles_ := range titles {
		for _, title := range titles_ {
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, f)
		}
	}

	// Dissociates videos...
	for f, video_ := range videos {
		for _, video := range video_ {
			err := client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f)
			if err != nil {
				fmt.Println("fail to dissocite file ", err)
			}
		}
	}

	// Associate titles...
	from := rqst.GetPath() + "/" + rqst.OldName
	dest := rqst.GetPath() + "/" + rqst.NewName
	info, _ := os.Stat(srv.formatPath(from))

	// So here I will get the list of all file permission and change the one with
	// the old file prefix...
	rbac_client_, err = getRbacClient()
	if err != nil {
		return nil, err
	}

	file_permissions, _ := rbac_client_.GetResourcePermissionsByResourceType("file")
	permissions, _ := rbac_client_.GetResourcePermissions(from)

	cache.RemoveItem(path + "/" + rqst.OldName)
	cache.RemoveItem(path)

	err = os.Rename(path+"/"+rqst.OldName, path+"/"+rqst.NewName)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for f, title_ := range titles {
		for _, t := range title_ {
			var f_ string
			if !info.IsDir() {
				f_ = dest
			} else {
				f_ = strings.ReplaceAll(f, from, dest)
			}

			err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f_)
			if err != nil {
				fmt.Println("fail to asscociate file ", err)
			}
		}
	}

	// Associate videos...
	for f, video_ := range videos {
		for _, video := range video_ {
			var f_ string
			if !info.IsDir() {
				f_ = dest
			} else {
				f_ = strings.ReplaceAll(f, from, dest)
			}
			err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f_)
			if err != nil {
				fmt.Println("fail to asscociate file ", err)
			}
		}
	}

	rbac_client_, err = getRbacClient()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// if the info is a dir...

	// if the info is a dir...
	if info.IsDir() {
		// So here I will get the list of all file permission and change the one with
		// the old file prefix...
		for i := 0; i < len(file_permissions); i++ {
			p := file_permissions[i]
			if strings.HasPrefix(p.Path, from) {
				rbac_client_.DeleteResourcePermissions(p.Path)
				p.Path = strings.ReplaceAll(p.Path, from, dest)
				err := rbac_client_.SetResourcePermissions(token, p.Path, p.ResourceType, p)
				if err != nil {
					fmt.Println("fail to update the permission: ", err)
				}
			}
		}
	} else if permissions != nil {
		// change it path to the new path.
		rbac_client_.DeleteResourcePermissions(from)

		permissions.Path = dest
		err := rbac_client_.SetResourcePermissions(token, dest, permissions.ResourceType, permissions)
		if err != nil {
			fmt.Println("fail to update the permission: ", err)
		}
	}

	startIndex := strings.LastIndex(rqst.GetOldName(), "/")
	if startIndex != -1 {
		startIndex++
	} else {
		startIndex = 0
	}

	endIndex := strings.LastIndex(rqst.GetOldName(), ".")
	if endIndex == -1 {
		endIndex = len(rqst.GetOldName())
	}

	// Rename it .hidden file.
	hiddenFolderFrom := path + "/.hidden/" + rqst.GetOldName()[startIndex:endIndex]

	startIndex = strings.LastIndex(rqst.GetNewName(), "/")
	if startIndex != -1 {
		startIndex++
	} else {
		startIndex = 0
	}

	endIndex = strings.LastIndex(rqst.GetNewName(), ".")
	if endIndex == -1 {
		endIndex = len(rqst.GetNewName())
	}

	hiddenFolderTo := path + "/.hidden/" + rqst.GetNewName()[startIndex:endIndex]

	if Utility.Exists(hiddenFolderFrom) {
		err := os.Rename(hiddenFolderFrom, hiddenFolderTo)
		if err != nil {
			fmt.Println(err)
		}
	}

	return &filepb.RenameResponse{
		Result: true,
	}, nil
}

// Delete a directory
func (srv *server) DeleteDir(ctx context.Context, rqst *filepb.DeleteDirRequest) (*filepb.DeleteDirResponse, error) {
	path := srv.formatPath(rqst.GetPath())
	if !Utility.Exists(path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No directory with path "+path+" was found!")))
	}

	cache.RemoveItem(path)

	// Remove file asscociation contain by file in that directry
	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	titles := make(map[string][]*titlepb.Title, 0)
	srv.getFileTitlesAssociation(client, rqst.GetPath(), titles)

	videos := make(map[string][]*titlepb.Video, 0)
	srv.getFileVideosAssociation(client, rqst.GetPath(), videos)

	// Dissociates titles...
	for f, titles_ := range titles {
		for _, title := range titles_ {
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, f)
		}
	}

	// Dissociates videos...
	for f, video_ := range videos {
		for _, video := range video_ {
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f)
		}
	}

	// Delete resource permission.
	rbac_client_, err = getRbacClient()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// recursively remove all sub-dir and file permissions.

	// So here I will get the list of all file permission and change the one with
	// the old file prefix...
	permissions, err := rbac_client_.GetResourcePermissionsByResourceType("file")
	if err == nil {
		for i := 0; i < len(permissions); i++ {
			p := permissions[i]
			if strings.HasPrefix(p.Path, path) {
				rbac_client_.DeleteResourcePermissions(p.GetPath())
			}
		}
	}

	// Remove the permission itself...
	rbac_client_.DeleteResourcePermissions(rqst.GetPath())
	if err != nil {
		fmt.Println(err)
	}

	// Remove the file itself.
	err = os.RemoveAll(path)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.DeleteDirResponse{
		Result: true,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
// File Operation
////////////////////////////////////////////////////////////////////////////////

// Get file info, can be use to get file thumbnail or knowing that a file exist
// or not.
func (srv *server) GetFileInfo(ctx context.Context, rqst *filepb.GetFileInfoRequest) (*filepb.GetFileInfoResponse, error) {
	path := srv.formatPath(rqst.GetPath())

	info, err := getFileInfo(srv, path, int(rqst.ThumbnailHeight), int(rqst.ThumbnailWidth))
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	infos := make([]*filepb.FileInfo, 0)
	infos = getFileInfos(srv, info, infos)

	return &filepb.GetFileInfoResponse{
		Info: infos[0],
	}, nil
}

// Read file, can be use for small to medium file...
func (srv *server) ReadFile(rqst *filepb.ReadFileRequest, stream filepb.FileService_ReadFileServer) error {
	path := srv.formatPath(rqst.GetPath())

	file, err := os.Open(path)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// close the file when done.
	defer file.Close()
	const BufferSize = 1024 * 5 // the chunck size.
	buffer := make([]byte, BufferSize)

	for {
		bytesread, err := file.Read(buffer)
		if bytesread > 0 {
			stream.Send(&filepb.ReadFileResponse{
				Data: buffer[:bytesread],
			})
		}
		if err != nil {
			if err != io.EOF {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			break
		}
	}
	return nil
}

// Save a file on the server...
func (srv *server) SaveFile(stream filepb.FileService_SaveFileServer) error {
	// Here I will receive the file
	data := make([]byte, 0)
	var path string
	for {
		rqst, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// Here all data is read...
				err := os.WriteFile(path, data, 0644)

				if err != nil {
					return status.Errorf(
						codes.Internal,
						Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}

				// Close the stream...
				err_ := stream.SendAndClose(&filepb.SaveFileResponse{
					Result: true,
				})

				if err_ != nil {
					fmt.Println("fail send response and close stream with error ", err_)
					return err_
				}
			} else {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}

		// Receive message informations.
		switch msg := rqst.File.(type) {
		case *filepb.SaveFileRequest_Path:
			// The root will be the Root specefied by the srv.
			path = srv.formatPath(msg.Path)

		case *filepb.SaveFileRequest_Data:
			data = append(data, msg.Data...)
		}
	}
}

// Dissociate file, if the if is deleted...
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

// Delete file
func (srv *server) DeleteFile(ctx context.Context, rqst *filepb.DeleteFileRequest) (*filepb.DeleteFileResponse, error) {

	// return nil, errors.New("test phase...")
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	path := srv.formatPath(rqst.GetPath())

	// Here I will remove the whole
	cache.RemoveItem(path)
	cache.RemoveItem(filepath.Dir(path)) // force reload dir...

	rbac_client_, err := getRbacClient()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// I will remove the permission from the db.
	rbac_client_.DeleteResourcePermissions(rqst.GetPath())

	// Also delete informations from .hidden
	path_ := path[0:strings.LastIndex(path, "/")]

	fileName := path[strings.LastIndex(path, "/")+1:]
	if strings.Contains(fileName, ".") {
		fileName = fileName[0:strings.LastIndex(fileName, ".")]
	}

	hiddenFolder := path_ + "/.hidden/" + fileName
	if Utility.Exists(hiddenFolder) {
		err := os.RemoveAll(hiddenFolder)
		if err != nil {
			fmt.Println(err)
		}
	}

	// Now I will disscociate the file.
	dissociateFileWithTitle(rqst.GetPath(), srv.Domain)

	// Refresh playlist...
	dir := strings.ReplaceAll(filepath.Dir(path), "\\", "/")

	if Utility.Exists(dir + "/audio.m3u") {
		cache.RemoveItem(dir + "/audio.m3u")
		os.Remove(dir + "/audio.m3u")
		srv.generatePlaylist(dir, token)
	}

	if Utility.Exists(dir + "/video.m3u") {
		cache.RemoveItem(dir + "/video.m3u")
		os.Remove(dir + "/video.m3u")
		srv.generatePlaylist(dir, token)
	}

	err = os.Remove(path)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.DeleteFileResponse{
		Result: true,
	}, nil

}

// Convert html to pdf.
func (srv *server) HtmlToPdf(ctx context.Context, rqst *filepb.HtmlToPdfRqst) (*filepb.HtmlToPdfResponse, error) {
	pdfg, err := wkhtml.NewPDFGenerator()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	pdfg.AddPage(wkhtml.NewPageReader(strings.NewReader(rqst.Html)))

	// Create PDF document in internal buffer
	err = pdfg.Create()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	//Your Pdf Name
	path := os.TempDir() + "/" + Utility.RandomUUID()
	defer os.Remove(path)

	err = pdfg.WriteFile(path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Return utf8 file data.
	return &filepb.HtmlToPdfResponse{
		Pdf: data,
	}, nil
}

/**
 * Return the event service.
 */
func getEventClient() (*event_client.Event_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		fmt.Println("fail to connect to event client with error: ", err)
		return nil, err
	}

	return client.(*event_client.Event_Client), nil
}

/**
 * Return an instance of the title client.
 */
func getTitleClient() (*title_client.Title_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewTitleService_Client", title_client.NewTitleService_Client)
	client, err := globular_client.GetClient(address, "title.TitleService", "NewTitleService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*title_client.Title_Client), nil
}

func getRbacClient() (*rbac_client.Rbac_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func getAuticationClient(address string) (*authentication_client.Authentication_Client, error) {
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
	client, err := globular_client.GetClient(address, "authentication.AuthenticationService", "NewAuthenticationService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*authentication_client.Authentication_Client), nil
}

func getMediaClient() (*media_client.Media_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewMediaService_Client", media_client.NewMediaService_Client)
	client, err := globular_client.GetClient(address, "media.MediaService", "NewMediaService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*media_client.Media_Client), nil
}

func (s *server) createVideoPreview(path string, nb int, height int) error {

	// Get the client...
	client, err := getMediaClient()
	if err != nil {
		return err
	}

	// Create the preview...
	err = client.CreateVideoPreview(path, int32(height), int32(nb))
	if err != nil {
		return err
	}

	return nil
}

func (srv *server) generatePlaylist(path, token string) error {

	// Get the client...
	client, err := getMediaClient()
	if err != nil {
		return err
	}

	// Create the preview...
	err = client.GeneratePlaylist(path, token)
	if err != nil {
		return err
	}

	return nil
}

func createVttFile(output string, fps float32) error {

	// Get the client...
	client, err := getMediaClient()
	if err != nil {
		return err
	}

	// Create the preview...
	err = client.CreateVttFile(output, fps)
	if err != nil {
		return err
	}

	return nil
}

func (s *server) createVideoTimeLine(path string, width int, fps float32) error {

	// Get the client...
	client, err := getMediaClient()
	if err != nil {
		return err
	}

	// Create the preview...
	err = client.CreateVideoTimeLine(path, int32(width), fps)
	if err != nil {
		return err
	}

	return nil
}

func processVideos(srv *server, token string, dirs []string) {
	// Get the client...
	client, err := getMediaClient()
	if err != nil {
		fmt.Println("fail to get media client with error: ", err)
		return
	}

	for i := 0; i < len(dirs); i++ {
		path := dirs[i]
		path = srv.formatPath(path)
		if Utility.Exists(path) {
			// Here I will get the list of all file permission and change the one with
			// the old file prefix...
			file_permissions, _ := rbac_client_.GetResourcePermissionsByResourceType("file")
			permissions, _ := rbac_client_.GetResourcePermissions(path)

			// if the info is a dir...
			info, _ := os.Stat(path)
			if info.IsDir() {
				// So here I will get the list of all file permission and change the one with
				// the old file prefix...
				for i := 0; i < len(file_permissions); i++ {
					p := file_permissions[i]
					if strings.HasPrefix(p.Path, path) {
						rbac_client_.DeleteResourcePermissions(p.Path)
						p.Path = strings.ReplaceAll(p.Path, path, path)
						err := rbac_client_.SetResourcePermissions(token, p.Path, p.ResourceType, p)
						if err != nil {
							fmt.Println("fail to update the permission: ", err)
						}
					}
				}
			} else if permissions != nil {
				// change it path to the new path.
				rbac_client_.DeleteResourcePermissions(path)

				permissions.Path = path
				err := rbac_client_.SetResourcePermissions(token, path, permissions.ResourceType, permissions)
				if err != nil {
					fmt.Println("fail to update the permission: ", err)
				}
			}

			// Create the preview...
			err = client.CreateVideoPreview(path, 360, 5)
			if err != nil {
				fmt.Println("fail to create video preview with error: ", err)
			}

			// Create the preview...
			err = client.CreateVideoTimeLine(path, 360, 5)
			if err != nil {
				fmt.Println("fail to create video preview with error: ", err)
			}

			// Generate playlist...
			err = client.GeneratePlaylist(path, token)
			if err != nil {
				fmt.Println("fail to generate playlist with error: ", err)
			}
		}
	}
}

// Recursively get all titles for a given path...
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
				path_ := path + "/" + f.Name()
				if !strings.Contains(path_, ".hidden/") {
					srv.getFileTitlesAssociation(client, path_, titles)
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

/**
 * Return the list of videos description and file association
 */
func (srv *server) getFileVideosAssociation(client *title_client.Title_Client, path string, videos map[string][]*titlepb.Video) error {
	path_ := srv.formatPath(path)
	info, err := os.Stat(path_)
	if err != nil {
		return err
	}

	if info.IsDir() && !Utility.Exists(path_+"/playlist.m3u8") {
		files, err := os.ReadDir(path_)
		if err == nil {
			for _, f := range files {
				path_ := path + "/" + f.Name()
				if !strings.Contains(path_, ".hidden/") {
					srv.getFileVideosAssociation(client, path_, videos)
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

// Move a file/directory
func (srv *server) Move(ctx context.Context, rqst *filepb.MoveRequest) (*filepb.MoveResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	rbac_client_, err := getRbacClient()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(rqst.Files); i++ {
		// TODO test if rqst.Path is in the root path...
		from := srv.formatPath(rqst.Files[i])
		dest := srv.formatPath(rqst.Path)
		info, _ := os.Stat(from)

		file_permissions, _ := rbac_client_.GetResourcePermissionsByResourceType("file")

		if Utility.Exists(from) {

			titles := make(map[string][]*titlepb.Title, 0)
			srv.getFileTitlesAssociation(client, rqst.Files[i], titles)

			videos := make(map[string][]*titlepb.Video, 0)
			srv.getFileVideosAssociation(client, rqst.Files[i], videos)

			// Dissociates titles...
			for f, titles_ := range titles {
				if f == rqst.Files[i] {
					for _, title := range titles_ {
						client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, f)
					}
				}
			}

			// Dissociates videos...
			for f, video_ := range videos {
				for _, video := range video_ {
					if f == rqst.Files[i] {
						err := client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f)
						if err != nil {
							fmt.Println("fail to dissocite file ", err)
						}
					}
				}
			}

			// Move the file...
			// TODO test if rqst.Path is in the root path...
			err := Utility.Move(from, dest)

			if err == nil {
				// remove it from the cache.
				cache.RemoveItem(from)

				// Associate titles...
				for f, titles_ := range titles {
					for _, title := range titles_ {
						var f_ string
						if !info.IsDir() {
							f_ = rqst.Path + "/" + filepath.Base(f)
						} else {
							dest := rqst.Path + "/" + filepath.Base(rqst.Files[i])
							f_ = strings.ReplaceAll(f, rqst.Files[i], dest)
						}
						client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, f_)
						if err != nil {
							fmt.Println("fail to asscociate file ", err)
						}
					}
				}

				// Associate videos...
				for f, video_ := range videos {
					for _, video := range video_ {
						var f_ string
						if !info.IsDir() {
							f_ = rqst.Path + "/" + filepath.Base(f)
						} else {
							dest := rqst.Path + "/" + filepath.Base(rqst.Files[i])
							f_ = strings.ReplaceAll(f, rqst.Files[i], dest)
						}

						err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f_)
						if err != nil {
							fmt.Println("fail to asscociate file ", err)
						}
					}
				}

				// if the info is a dir...
				if info.IsDir() {
					// So here I will get the list of all file permission and change the one with
					// the old file prefix...

					for j := 0; j < len(file_permissions); j++ {
						p := file_permissions[j]
						if strings.HasPrefix(p.Path, rqst.Files[i]) {
							rbac_client_.DeleteResourcePermissions(p.Path)
							dest := rqst.Path + "/" + filepath.Base(rqst.Files[i])
							p.Path = strings.ReplaceAll(p.Path, rqst.Files[i], dest)
							err := rbac_client_.SetResourcePermissions(token, p.Path, p.ResourceType, p)
							if err != nil {
								fmt.Println("fail to update the permission: ", err)
							}
						}
					}

				} else {
					permissions, err := rbac_client_.GetResourcePermissions(rqst.Files[i])
					if err == nil && permissions != nil {
						rbac_client_.DeleteResourcePermissions(rqst.Files[i])
						permissions.Path = rqst.Path + "/" + filepath.Base(permissions.Path)
						err := rbac_client_.SetResourcePermissions(token, permissions.Path, permissions.ResourceType, permissions)
						if err != nil {
							fmt.Println("fail to update the permission: ", err)
						}
					}
				}

				// If hidden folder exist for it...
				path_ := filepath.Dir(from)
				fileName := filepath.Base(from)
				if strings.Contains(fileName, ".") {
					fileName = fileName[0:strings.LastIndex(fileName, ".")]
				}
				hiddenFolder := path_ + "/.hidden/" + fileName

				if Utility.Exists(hiddenFolder) {
					Utility.CreateDirIfNotExist(dest + "/.hidden")
					err := Utility.Move(hiddenFolder, dest+"/.hidden")
					if err != nil {
						fmt.Println(err)
					}

					output := dest + "/.hidden/" + fileName + "/__timeline__"
					createVttFile(output, 0.2)
				}
			} else {
				fmt.Println("fail to move file: ", from, "to", dest, "with error", err)
			}
		}
	}

	return &filepb.MoveResponse{Result: true}, nil
}

// Copy a file/directory
func (srv *server) Copy(ctx context.Context, rqst *filepb.CopyRequest) (*filepb.CopyResponse, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// get the rbac client.
	rbac_client_, err := getRbacClient()
	if err != nil {
		return nil, err
	}

	// format the path to make it absolute on the server.
	path := srv.formatPath(rqst.Path)

	// So here I will call the function mv at repetition for each path...
	for i := 0; i < len(rqst.Files); i++ {
		f := srv.formatPath(rqst.Files[i])

		// So here I will try to retreive indexation for the file...
		client, err := getTitleClient()
		if err != nil {
			return nil, err
		}

		file_permissions, _ := rbac_client_.GetResourcePermissionsByResourceType("file")
		permissions, _ := rbac_client_.GetResourcePermissions(rqst.Files[i])

		titles := make(map[string][]*titlepb.Title, 0)
		srv.getFileTitlesAssociation(client, rqst.Files[i], titles)

		videos := make(map[string][]*titlepb.Video, 0)
		srv.getFileVideosAssociation(client, rqst.Files[i], videos)

		if Utility.Exists(f) {
			info, err := os.Stat(f)
			if err == nil {
				if info.IsDir() {
					// Copy the directory
					Utility.CopyDir(f, path)

					// Associate titles...
					for f, title_ := range titles {
						for _, t := range title_ {
							var f_ string
							filename := filepath.Base(rqst.Files[i])
							dest := rqst.Path + "/" + filename
							f_ = strings.ReplaceAll(f, rqst.Files[i], dest)

							err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f_)
							if err != nil {
								fmt.Println("fail to asscociate file ", err)
							}
						}
					}

					// Associate videos...
					for f, video_ := range videos {
						for _, video := range video_ {
							var f_ string
							filename := filepath.Base(rqst.Files[i])
							dest := rqst.Path + "/" + filename
							f_ = strings.ReplaceAll(f, rqst.Files[i], dest)
							err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f_)
							if err != nil {
								fmt.Println("fail to asscociate file ", err)
							}
						}
					}

					// So here I will get the list of all file permission and change the one with
					// the old file prefix...
					if file_permissions != nil {
						for j := 0; j < len(file_permissions); j++ {
							p := file_permissions[j]
							if strings.HasPrefix(p.Path, rqst.Files[i]) {
								filename := filepath.Base(rqst.Files[i])
								dest := rqst.Path + "/" + filename
								p.Path = strings.ReplaceAll(p.Path, rqst.Files[i], dest)
								err := rbac_client_.SetResourcePermissions(token, p.Path, p.ResourceType, p)
								if err != nil {
									fmt.Println("fail to update the permission: ", err)
								}
							}
						}
					}

				} else {
					// Copy the file
					Utility.CopyFile(f, path)

					// Associate titles...
					for f, title_ := range titles {
						for _, t := range title_ {

							f_ := rqst.Path + "/" + filepath.Base(f)
							err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, f_)
							if err != nil {
								fmt.Println("fail to asscociate file ", err)
							}
						}
					}

					// Associate videos...
					for f, video_ := range videos {
						for _, video := range video_ {

							f_ := rqst.Path + "/" + filepath.Base(f)
							err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f_)
							if err != nil {
								fmt.Println("fail to asscociate file ", err)
							}
						}
					}

					if permissions != nil {
						permissions.Path = rqst.Path + "/" + filepath.Base(permissions.Path)
						err := rbac_client_.SetResourcePermissions(token, permissions.Path, permissions.ResourceType, permissions)
						if err != nil {
							fmt.Println("fail to update the permission: ", err)
						}
					}

					// If hidden folder exist for it...
					path_ := filepath.Dir(f)
					fileName := filepath.Base(f)
					if strings.Contains(fileName, ".") {
						fileName = fileName[0:strings.LastIndex(fileName, ".")]
					}
					hiddenFolder := path_ + "/.hidden/" + fileName

					if Utility.Exists(hiddenFolder) {
						err := Utility.CopyDir(hiddenFolder, path+"/.hidden")
						if err != nil {
							fmt.Println(err)
						}
					}
				}
			}
		}
	}

	return &filepb.CopyResponse{Result: true}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Utility functions
////////////////////////////////////////////////////////////////////////////////

// Return the list of thumbnail for a given directory...
func (srv *server) GetThumbnails(rqst *filepb.GetThumbnailsRequest, stream filepb.FileService_GetThumbnailsServer) error {

	/*_, token, err := security.GetClientId(stream.Context())
	if err != nil {
		return err
	}*/

	path := rqst.GetPath()

	// The root will be the Root specefied by the srv.
	if strings.HasPrefix(path, "/") {
		path = srv.Root + path
		// Set the path separator...
		path = strings.Replace(path, "\\", "/", -1)
	}

	info, err := readDir(srv, path, rqst.GetRecursive(), rqst.ThumbnailHeight, rqst.ThumbnailWidth, true, nil, nil)
	if err != nil {
		return err
	}

	thumbnails := getThumbnails(info)

	// Here I will serialyse the data into JSON.
	jsonStr, err := json.Marshal(thumbnails)
	if err != nil {
		return err
	}

	maxSize := 1024 * 5
	size := int(math.Ceil(float64(len(jsonStr)) / float64(maxSize)))
	for i := 0; i < size; i++ {
		start := i * maxSize
		end := start + maxSize
		var data []byte
		if end > len(jsonStr) {
			data = jsonStr[start:]
		} else {
			data = jsonStr[start:end]
		}
		stream.Send(&filepb.GetThumbnailsResponse{
			Data: data,
		})
	}

	return nil
}

// Create a link file
func (srv *server) CreateLnk(ctx context.Context, rqst *filepb.CreateLnkRequest) (*filepb.CreateLnkResponse, error) {
	path := srv.formatPath(rqst.Path)

	// The root will be the Root specefied by the srv.
	if !Utility.Exists(path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no directory found at path "+path)))
	}

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	err = os.WriteFile(path+"/"+rqst.Name, []byte(rqst.Lnk), 0644)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setOwner(token, rqst.Path+"/"+rqst.Name)

	return &filepb.CreateLnkResponse{}, nil
}

func (srv *server) WriteExcelFile(ctx context.Context, rqst *filepb.WriteExcelFileRequest) (*filepb.WriteExcelFileResponse, error) {
	path := srv.formatPath(rqst.Path)

	if Utility.Exists(path) {
		err := os.Remove(path)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

	}

	sheets := make(map[string]interface{})

	err := json.Unmarshal([]byte(rqst.Data), &sheets)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.writeExcelFile(path, sheets)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.WriteExcelFileResponse{
		Result: true,
	}, nil
}

/**
 * Save excel file to a given destination.
 * The sheets must contain a with values map[pageName] [[], [], []] // 2D array.
 */
func (srv *server) writeExcelFile(path string, sheets map[string]interface{}) error {

	xlFile, err_ := xlsx.OpenFile(path)
	var xlSheet *xlsx.Sheet
	if err_ != nil {
		xlFile = xlsx.NewFile()
	}

	for name, data := range sheets {
		xlSheet, _ = xlFile.AddSheet(name)
		values := data.([]interface{})
		// So here I got the xl file open and sheet ready to write into.
		for i := 0; i < len(values); i++ {
			row := xlSheet.AddRow()
			for j := 0; j < len(values[i].([]interface{})); j++ {
				if values[i].([]interface{})[j] != nil {
					cell := row.AddCell()
					if reflect.TypeOf(values[i].([]interface{})[j]).String() == "string" {
						str := values[i].([]interface{})[j].(string)
						// here I will try to format the date time if it can be...
						dateTime, err := Utility.DateTimeFromString(str, "2006-01-02 15:04:05")
						if err != nil {
							cell.SetString(str)
						} else {
							cell.SetDateTime(dateTime)
						}
					} else {
						if values[i].([]interface{})[j] != nil {
							cell.SetValue(values[i].([]interface{})[j])
						}
					}
				}
			}
		}

	}

	// Here I will save the file at the given path...
	err := xlFile.Save(path)

	if err != nil {
		return nil
	}

	return nil
}

// Return the file metadata, more specific infos store in the file itself.
func (srv *server) GetFileMetadata(ctx context.Context, rqst *filepb.GetFileMetadataRequest) (*filepb.GetFileMetadataResponse, error) {
	path := srv.formatPath(rqst.Path)
	metadata, err := ExtractMetada(path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	obj, err := structpb.NewStruct(metadata)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.GetFileMetadataResponse{Result: obj}, nil
}

func removeTempFiles(rootDir string) error {
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		// Check if it's a regular file and its name ends with ".temp.mp4"
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".temp.mp4") {
			fmt.Printf("Removing file: %s\n", path)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("error removing file %s: %v", path, err)
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking directory: %v", err)
	}

	return nil
}

// Start the process of removing temp files...
func (srv *server) startRemoveTempFiles() {

	go func() {
		// The dir to scan...
		dirs := make([]string, 0)
		dirs = append(dirs, config.GetPublicDirs()...)
		dirs = append(dirs, config.GetDataDir()+"/files/users")
		dirs = append(dirs, config.GetDataDir()+"/files/applications")
		for _, dir := range dirs {
			removeTempFiles(dir)
		}
	}()
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// The actual server implementation.
	s_impl := new(server)

	// The name must the same as the grpc service name.
	s_impl.Name = string(filepb.File_file_proto.Services().Get(0).FullName())
	s_impl.Proto = filepb.File_file_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.CacheAddress, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "localhost"
	s_impl.Permissions = make([]interface{}, 14)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"rbac.RbacService"}
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	s_impl.Public = make([]string, 0) // The list of public directory where files can be read...
	s_impl.CacheReplicationFactor = 3

	// register new client creator.
	Utility.RegisterFunction("NewFileService_Client", file_client.NewFileService_Client)

	// So here I will set the default permissions for services actions.
	// Permission are use in conjonctions of resource.
	s_impl.Permissions[0] = map[string]interface{}{"action": "/file.FileService/ReadDir", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}}
	s_impl.Permissions[1] = map[string]interface{}{"action": "/file.FileService/CreateDir", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[2] = map[string]interface{}{"action": "/file.FileService/DeleteDir", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[3] = map[string]interface{}{"action": "/file.FileService/Rename", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[4] = map[string]interface{}{"action": "/file.FileService/GetFileInfo", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}}
	s_impl.Permissions[5] = map[string]interface{}{"action": "/file.FileService/ReadFile", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}}
	s_impl.Permissions[6] = map[string]interface{}{"action": "/file.FileService/SaveFile", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[7] = map[string]interface{}{"action": "/file.FileService/DeleteFile", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[8] = map[string]interface{}{"action": "/file.FileService/GetThumbnails", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}}
	s_impl.Permissions[9] = map[string]interface{}{"action": "/file.FileService/WriteExcelFile", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[10] = map[string]interface{}{"action": "/file.FileService/CreateArchive", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}}
	s_impl.Permissions[11] = map[string]interface{}{"action": "/file.FileService/FileUploadHandler", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[12] = map[string]interface{}{"action": "/file.FileService/UploadFile", "resources": []interface{}{map[string]interface{}{"index": 1, "permission": "write"}}}
	s_impl.Permissions[13] = map[string]interface{}{"action": "/file.FileService/CreateLnk", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}

	// Set the root path if is pass as argument.
	s_impl.Root = config.GetDataDir() + "/files"

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	if s_impl.CacheType == "BADGER" {
		cache = storage_store.NewBadger_store()
	} else if s_impl.CacheType == "SCYLLA" {
		// set the default storage.
		cache = storage_store.NewScylla_store(s_impl.CacheAddress, "files", s_impl.CacheReplicationFactor)
	} else if s_impl.CacheType == "LEVELDB" {
		// set the default storage.
		cache = storage_store.NewLevelDB_store()
	} else {
		// set in memory store
		cache = storage_store.NewBigCache_store()
	}

	// Register the echo services
	filepb.RegisterFileServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)
	Utility.CreateDirIfNotExist(s_impl.Root + "/cache")

	err = cache.Open(`{"path":"` + s_impl.Root + `", "name":"files"}`)
	if err != nil {
		fmt.Println("fail to open cache with error:", err)
	}

	// Now the event client service.
	go func() {

		event_client, err := getEventClient()
		if err == nil {

			channel_0 := make(chan string)
			channel_1 := make(chan string)

			// Process request received...
			go func() {
				for {
					select {
					case path := <-channel_0:
						// Now I will create the ownership...
						if strings.HasPrefix(path, "/users/") {
							values := strings.Split(path, "/")
							if len(values) > 1 {
								owner := values[2]

								// Set the owner of the conversation.
								rbac_client_, err = getRbacClient()
								if err == nil {
									err = rbac_client_.AddResourceOwner(path, "file", owner, rbacpb.SubjectType_ACCOUNT)
									if err != nil {
										fmt.Println("fail to set file owner with error ", err)
									}
								}
							}
						}
						// send to the other channel but dont wait...
						go func() {
							channel_1 <- path
						}()

					case path := <-channel_1:
						path_ := s_impl.formatPath(path)
						err := s_impl.indexFile(path_)
						if err != nil {
							fmt.Println("fail to index file with error: ", err)
						}
					}
				}
			}()

			// index file event
			err = event_client.Subscribe("index_file_event", Utility.RandomUUID(), func(evt *eventpb.Event) {
				channel_1 <- string(evt.Data)
			})

			if err != nil {
				fmt.Println("Fail to connect to event channel index_file_event")
			}

		}

	}()

	// Clean temp files...
	s_impl.startRemoveTempFiles()

	// Start the service.
	s_impl.StartService()

}
