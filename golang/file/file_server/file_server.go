package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/StalkR/imdb"

	"io"
	"io/ioutil"
	"log"
	"math"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	wkhtml "github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/davecourtois/Utility"
	"github.com/dhowden/tag"
	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/file/file_client"
	"github.com/globulario/services/golang/file/filepb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/title_client"
	"github.com/globulario/services/golang/title/titlepb"
	"github.com/golang/protobuf/jsonpb"
	"github.com/jasonlvhit/gocron"

	"github.com/tealeg/xlsx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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

	// The default domain.
	domain string = "localhost"

	// Client to validate and change file and directory permission.
	rbac_client_ *rbac_client.Rbac_Client

	// The event client.
	event_client_ *event_client.Event_Client

	// The title client.
	title_client_ *title_client.Title_Client

	// the authentication client
	authentication_client_ *authentication_client.Authentication_Client

	// Here I will keep files info in memory...
	cache *storage_store.BigCache_store // todo use cache instead of memory...
)

const (
	MAX_FFMPEG_INSTANCE = 3
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

	// If true ffmeg will use information to convert the video.
	AutomaticVideoConversion bool

	// If true video will be convert to stream
	AutomaticStreamConversion bool

	// The conversion will start at that given hour...
	StartVideoConversionHour string

	// Maximum conversion time. Conversion will not continue over this delay.
	MaximumVideoConversionDelay string

	// Public contain a list of paths reachable by the file server.
	Public []string

	// This map will contain video conversion error so the server will not try
	// to convert the same file again and again.
	videoConversionErrors *sync.Map

	// This map will contain video convertion logs.
	videoConversionLogs *sync.Map

	// The task scheduler.
	scheduler *gocron.Scheduler

	// processing video conversion (mp4, m3u8 etc...)
	isProcessing bool

	// Generate playlist and titles for audio
	isProcessingAudio bool

	// Generate playlist and titles for video and movies (series and episode.)
	isProcessingVideo bool
}

// The http address where the configuration can be found /config
func (svr *server) GetAddress() string {
	return svr.Address
}

func (svr *server) SetAddress(address string) {
	svr.Address = address
}

func (svr *server) GetProcess() int {
	return svr.Process
}

func (svr *server) SetProcess(pid int) {
	svr.Process = pid
}

func (svr *server) GetProxyProcess() int {
	return svr.ProxyProcess
}

func (svr *server) SetProxyProcess(pid int) {
	svr.ProxyProcess = pid
}

// The path of the configuration.
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
}

// The current service state
func (svr *server) GetState() string {
	return svr.State
}

func (svr *server) SetState(state string) {
	svr.State = state
}

// The last error
func (svr *server) GetLastError() string {
	return svr.LastError
}

func (svr *server) SetLastError(err string) {
	svr.LastError = err
}

// The modeTime
func (svr *server) SetModTime(modtime int64) {
	svr.ModTime = modtime
}
func (svr *server) GetModTime() int64 {
	return svr.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (file_server *server) GetId() string {
	return file_server.Id
}
func (file_server *server) SetId(id string) {
	file_server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (file_server *server) GetName() string {
	return file_server.Name
}
func (file_server *server) SetName(name string) {
	file_server.Name = name
}

// The description of the service
func (file_server *server) GetDescription() string {
	return file_server.Description
}
func (file_server *server) SetDescription(description string) {
	file_server.Description = description
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The list of keywords of the services.
func (file_server *server) GetKeywords() []string {
	return file_server.Keywords
}
func (file_server *server) SetKeywords(keywords []string) {
	file_server.Keywords = keywords
}

func (file_server *server) GetRepositories() []string {
	return file_server.Repositories
}
func (file_server *server) SetRepositories(repositories []string) {
	file_server.Repositories = repositories
}

func (file_server *server) GetDiscoveries() []string {
	return file_server.Discoveries
}
func (file_server *server) SetDiscoveries(discoveries []string) {
	file_server.Discoveries = discoveries
}

// Dist
func (file_server *server) Dist(path string) (string, error) {

	return globular.Dist(path, file_server)
}

func (server *server) GetDependencies() []string {

	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	return server.Dependencies
}

func (server *server) SetDependency(dependency string) {
	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	// Append the depency to the list.
	if !Utility.Contains(server.Dependencies, dependency) {
		server.Dependencies = append(server.Dependencies, dependency)
	}
}

func (file_server *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (file_server *server) GetPath() string {
	return file_server.Path
}
func (file_server *server) SetPath(path string) {
	file_server.Path = path
}

// The path of the .proto file.
func (file_server *server) GetProto() string {
	return file_server.Proto
}
func (file_server *server) SetProto(proto string) {
	file_server.Proto = proto
}

// The gRpc port.
func (file_server *server) GetPort() int {
	return file_server.Port
}
func (file_server *server) SetPort(port int) {
	file_server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (file_server *server) GetProxy() int {
	return file_server.Proxy
}
func (file_server *server) SetProxy(proxy int) {
	file_server.Proxy = proxy
}

// Can be one of http/https/tls
func (file_server *server) GetProtocol() string {
	return file_server.Protocol
}
func (file_server *server) SetProtocol(protocol string) {
	file_server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (file_server *server) GetAllowAllOrigins() bool {
	return file_server.AllowAllOrigins
}
func (file_server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	file_server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (file_server *server) GetAllowedOrigins() string {
	return file_server.AllowedOrigins
}

func (file_server *server) SetAllowedOrigins(allowedOrigins string) {
	file_server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (file_server *server) GetDomain() string {
	return file_server.Domain
}
func (file_server *server) SetDomain(domain string) {
	file_server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (file_server *server) GetTls() bool {
	return file_server.TLS
}
func (file_server *server) SetTls(hasTls bool) {
	file_server.TLS = hasTls
}

// The certificate authority file
func (file_server *server) GetCertAuthorityTrust() string {
	return file_server.CertAuthorityTrust
}
func (file_server *server) SetCertAuthorityTrust(ca string) {
	file_server.CertAuthorityTrust = ca
}

// The certificate file.
func (file_server *server) GetCertFile() string {
	return file_server.CertFile
}
func (file_server *server) SetCertFile(certFile string) {
	file_server.CertFile = certFile
}

// The key file.
func (file_server *server) GetKeyFile() string {
	return file_server.KeyFile
}
func (file_server *server) SetKeyFile(keyFile string) {
	file_server.KeyFile = keyFile
}

// The service version
func (file_server *server) GetVersion() string {
	return file_server.Version
}
func (file_server *server) SetVersion(version string) {
	file_server.Version = version
}

// The publisher id.
func (file_server *server) GetPublisherId() string {
	return file_server.PublisherId
}
func (file_server *server) SetPublisherId(publisherId string) {
	file_server.PublisherId = publisherId
}

func (file_server *server) GetKeepUpToDate() bool {
	return file_server.KeepUpToDate
}
func (file_server *server) SetKeepUptoDate(val bool) {
	file_server.KeepUpToDate = val
}

func (file_server *server) GetKeepAlive() bool {
	return file_server.KeepAlive
}
func (file_server *server) SetKeepAlive(val bool) {
	file_server.KeepAlive = val
}

func (file_server *server) GetPermissions() []interface{} {
	return file_server.Permissions
}
func (file_server *server) SetPermissions(permissions []interface{}) {
	file_server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (file_server *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewFileService_Client", file_client.NewFileService_Client)

	err := globular.InitService(file_server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	file_server.grpcServer, err = globular.InitGrpcServer(file_server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (file_server *server) Save() error {
	// Create the file...
	return globular.SaveService(file_server)
}

func (file_server *server) StartService() error {
	return globular.StartService(file_server, file_server.grpcServer)
}

func (file_server *server) StopService() error {
	return globular.StopService(file_server, file_server.grpcServer)
}

func (file_server *server) Stop(context.Context, *filepb.StopRequest) (*filepb.StopResponse, error) {
	return &filepb.StopResponse{}, file_server.StopService()
}

type fileInfo struct {
	Name    string      // base name of the file
	Size    int64       // length in bytes for regular files; system-dependent for others
	Mode    os.FileMode // file mode bits
	ModTime time.Time   // modification time
	IsDir   bool        // abbreviation for Mode().IsDir()
	Path    string      // The path on the server.

	Mime      string
	Thumbnail string
	Files     []*fileInfo
}

func (s *server) getThumbnail(path string, h, w int) (string, error) {

	id := path + "_" + Utility.ToString(h) + "x" + Utility.ToString(w)

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

func getFileInfo(s *server, path string, thumbnailMaxHeight, thumbnailMaxWidth int) (*fileInfo, error) {

	info := new(fileInfo)
	info.Path = path

	path = strings.ReplaceAll(path, "\\", "/")
	fileStat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Cut the root part of the path if it start with the root path.
	if len(s.Root) > 0 {
		startindex := strings.Index(info.Path, s.Root)
		if startindex == 0 {
			info.Path = info.Path[len(s.Root):]
			info.Path = strings.Replace(info.Path, "\\", "/", -1) // Set the slash instead of back slash.
		}
	}

	if strings.Contains(path, "/.hidden") {
		if strings.HasSuffix(path, "__preview__") || strings.HasSuffix(path, "__timeline__") || strings.HasSuffix(path, "__thumbnail__") {
			info.Mime = "inode/directory"
			info.IsDir = true
			return info, nil
		}
	}

	// Here I will try to get the info from the cache...
	data, err_ := cache.GetItem(path)
	if err_ == nil {
		json.Unmarshal(data, &info)
		return info, nil
	}

	info.IsDir = fileStat.IsDir()
	if info.IsDir {
		info.Mime = "inode/directory"
	}

	info.Size = fileStat.Size()
	info.Name = fileStat.Name()
	info.ModTime = fileStat.ModTime()

	// Now the section depending of the mime type...
	if !strings.Contains(path, "/.hidden") {

		// the file
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
				defer f_.Close()
			}

			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			path_, err := os.Getwd()
			if err == nil {
				path_ = strings.ReplaceAll(path_, "\\", "/")
				path_ = path_ + "/mimetypes/unknown.png"
				info.Thumbnail, _ = s.getThumbnail(path_, 80, 80)
			}

			if err == nil {
				// in case of image...
				if strings.HasPrefix(info.Mime, "image/") {
					if thumbnailMaxHeight > 0 && thumbnailMaxWidth > 0 {
						info.Thumbnail, _ = s.getThumbnail(path, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
					} else {
						info.Thumbnail, _ = s.getThumbnail(path, 80, 80)
					}
				} else if strings.HasPrefix(info.Mime, "video/") {

					// If hidden folder exist for it...
					path_ := path[0:strings.LastIndex(path, "/")]
					fileName := path[strings.LastIndex(path, "/")+1:]
					if strings.Contains(fileName, ".") {
						fileName = fileName[0:strings.LastIndex(fileName, ".")]
					}
					hiddenFolder := path_ + "/.hidden/" + fileName
					if Utility.Exists(hiddenFolder) {

						// Here I will auto generate preview if it not already exist...
						if !Utility.Exists(hiddenFolder + "/__preview__/preview_00001.jpg") {
							// generate the preview...
							os.RemoveAll(hiddenFolder + "/__preview__") // be sure it will
							go createVideoPreview(s, info.Path, 20, 128, true)

							os.RemoveAll(hiddenFolder + "/__timeline__")     // be sure it will
							go createVideoTimeLine(info.Path, 180, .2, true) // 1 frame per 5 seconds.
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
						if err == nil {

							path_ = strings.ReplaceAll(path_, "\\", "/")
							path_ = path_ + "/mimetypes/video-x-generic.png"

							info.Thumbnail, _ = s.getThumbnail(path_, 80, 80)
						}
					}

				} else if strings.HasPrefix(info.Mime, "audio/") || strings.HasSuffix(path, ".flac") || strings.HasSuffix(path, ".mp3") {
					// duration := Utility.ToInt(getVideoDuration(path) + 0.5)
					metadata, err := readMetadata(s, path, int(thumbnailMaxHeight), int(thumbnailMaxWidth))

					if err == nil {
						info.Thumbnail = metadata["ImageUrl"].(string)
					}

				} else if strings.Contains(info.Mime, "/") {
					// In that case I will get read image from png file and create a
					// thumbnail with it...
					path_, err := os.Getwd()
					if err == nil {
						path_ = strings.ReplaceAll(path_, "\\", "/")
						path_ = path_ + "/mimetypes/" + strings.ReplaceAll(strings.Split(info.Mime, ";")[0], "/", "-") + ".png"
						info.Thumbnail, _ = s.getThumbnail(path_, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
					}
				}
			}
		} else {
			/*path_, err := os.Getwd()
			if err == nil {
				path_ = strings.ReplaceAll(path_, "\\", "/")
				path_ = path_ + "/mimetypes/inode-directory.png"
				info.Thumbnail, _ = s.getThumbnail(path_, 80, 80)
			}*/

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
						info.Thumbnail, _ = s.getThumbnail(path_, 80, 80)
					}
				}
			}
		}
	}

	data, err = json.Marshal(info)
	if err == nil {
		cache.SetItem(path, data)
	}

	return info, nil
}

func getThumbnails(info *fileInfo) []interface{} {
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

func toBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func fileNameWithoutExtension(fileName string) string {
	return strings.TrimSuffix(fileName, filepath.Ext(fileName))
}

func readMetadata(s *server, path string, thumnailHeight, thumbnailWidth int) (map[string]interface{}, error) {

	path = strings.ReplaceAll(path, "\\", "/")
	f_, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f_.Close()

	m, err := tag.ReadFrom(f_)
	var metadata map[string]interface{}

	if err == nil {
		metadata = make(map[string]interface{})
		metadata["Album"] = m.Album()
		metadata["AlbumArtist"] = m.AlbumArtist()
		metadata["Artist"] = m.Artist()
		metadata["Comment"] = m.Comment()
		metadata["Composer"] = m.Composer()
		metadata["FileType"] = m.FileType()
		metadata["Format"] = m.Format()
		metadata["Genre"] = m.Genre()
		metadata["Lyrics"] = m.Lyrics()
		metadata["Picture"] = m.Picture()
		metadata["Raw"] = m.Raw()
		metadata["Title"] = m.Title()
		if len(m.Title()) == 0 {
			metadata["Title"] = fileNameWithoutExtension(filepath.Base(path))
		}
		metadata["Year"] = m.Year()

		metadata["DisckNumber"], _ = m.Disc()
		_, metadata["DiscTotal"] = m.Disc()

		metadata["TrackNumber"], _ = m.Track()
		_, metadata["TrackTotal"] = m.Track()

		if m.Picture() != nil {

			// Determine the content type of the image file
			mimeType := m.Picture().MIMEType

			// Prepend the appropriate URI scheme header depending
			fileName := Utility.RandomUUID()

			// on the MIME type
			switch mimeType {
			case "image/jpg":
				fileName += ".jpg"
			case "image/jpeg":
				fileName += ".jpg"
			case "image/png":
				fileName += ".png"
			}

			imagePath := os.TempDir() + "/" + fileName
			defer os.Remove(imagePath)

			os.WriteFile(imagePath, m.Picture().Data, 0664)

			if Utility.Exists(imagePath) {
				metadata["ImageUrl"], _ = s.getThumbnail(imagePath, thumnailHeight, thumbnailWidth)
			}

		} else {

			imagePath := path[:strings.LastIndex(path, "/")]

			// Try to find the cover image...
			if Utility.Exists(imagePath + "/cover.jpg") {
				imagePath += "/cover.jpg"
			} else if Utility.Exists(imagePath + "/Cover.jpg") {
				imagePath += "/Cover.jpg"
			} else if Utility.Exists(imagePath + "/folder.jpg") {
				imagePath += "/folder.jpg"
			} else if Utility.Exists(imagePath + "/Folder.jpg") {
				imagePath += "/Folder.jpg"
			} else if Utility.Exists(imagePath + "/AlbumArt.jpg") {
				imagePath += "/AlbumArt.jpg"
			} else if Utility.Exists(imagePath + "/Front.jpg") {
				imagePath += "/Front.jpg"
			} else if Utility.Exists(imagePath + "/front.jpg") {
				imagePath += "/front.jpg"
			} else if Utility.Exists(imagePath + "/thumb.jpg") {
				imagePath += "/thumb.jpg"
			} else if Utility.Exists(imagePath + "/Thumbnail.jpg") {
				imagePath += "/Thumbnail.jpg"
			} else {
				// take the first found image it that case...
				images := Utility.GetFilePathsByExtension(imagePath, ".jpg")
				if len(images) > 0 {
					imagePath = images[0]
					for i := 0; i < len(images); i++ {
						imagePath_ := images[i]
						if strings.Index(strings.ToLower(imagePath_), "front") != -1 || strings.Index(strings.ToLower(imagePath_), "folder") != -1 || strings.Index(strings.ToLower(imagePath_), "cover") != -1 {

							imagePath = imagePath_
							if strings.HasSuffix(strings.ToLower(imagePath_), "front.jpg") || strings.HasSuffix(strings.ToLower(imagePath_), "cover.jpg") {
								break
							}
						}
					}
				} else {
					images := Utility.GetFilePathsByExtension(imagePath[0:strings.LastIndex(imagePath, "/")], ".jpg")
					if len(images) > 0 {
						imagePath = images[0]
						for i := 0; i < len(images); i++ {
							imagePath_ := images[i]
							if strings.Index(strings.ToLower(imagePath_), "front") != -1 || strings.Index(strings.ToLower(imagePath_), "folder") != -1 || strings.Index(strings.ToLower(imagePath_), "cover") != -1 {
								imagePath = imagePath_
								if strings.HasSuffix(strings.ToLower(imagePath_), "front.jpg") || strings.HasSuffix(strings.ToLower(imagePath_), "cover.jpg") {
									break
								}
							}
						}
					}

				}
			}

			if Utility.Exists(imagePath) {
				metadata["ImageUrl"], _ = s.getThumbnail(imagePath, 300, 300)
			}
		}
	} else {
		return nil, err
	}
	return metadata, nil
}

/**
 * Read the directory and return the file info.
 */
func readDir(s *server, path string, recursive bool, thumbnailMaxWidth int32, thumbnailMaxHeight int32, readFiles bool, token string) (*fileInfo, error) {

	// get the file info
	info, err := getFileInfo(s, path, int(thumbnailMaxWidth), int(thumbnailMaxWidth))
	if err != nil {
		return nil, err
	}

	if !info.IsDir {
		return nil, errors.New(path + " is not a directory")
	}

	// read list of files...
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, f := range files {

		if f.IsDir() {
			// Here if the dir contain the file playlist.m3u8 it means it content must not be read as a file but as stream,
			// so I will not read it content...
			dirPath := path + "/" + f.Name()

			// Test if a file named playlist.m3u8 exist...
			isHls := Utility.Exists(dirPath + "/playlist.m3u8")

			if recursive && !isHls {
				info_, err := readDir(s, dirPath, recursive, thumbnailMaxWidth, thumbnailMaxHeight, true, token)
				if err != nil {
					return nil, err
				}
				info.Files = append(info.Files, info_)
			} else if f.Name() != ".hidden" { // I will not read sub-dir hidden files...
				info_, err := readDir(s, dirPath, recursive, thumbnailMaxWidth, thumbnailMaxHeight, false, token)
				if err != nil {
					return nil, err
				}
				if isHls {
					info_.Mime = "video/hls-stream"
				}

				info.Files = append(info.Files, info_)
			}

		} else if readFiles {

			info_, err := getFileInfo(s, path+"/"+f.Name(), int(thumbnailMaxHeight), int(thumbnailMaxWidth))
			if err != nil {
				return nil, err
			}

			if !info_.IsDir && readFiles {
				f_, err := os.Open(path + "/" + f.Name())
				if err != nil {
					return nil, err
				}

				defer f_.Close()

				if strings.Contains(f.Name(), ".") {
					fileExtension := f.Name()[strings.LastIndex(f.Name(), "."):]
					info_.Mime = mime.TypeByExtension(fileExtension)
				} else {
					info_.Mime, _ = Utility.GetFileContentType(f_)
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

							info_.Thumbnail, err = s.getThumbnail(path_, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
							if err != nil {
								fmt.Println("fail to create thumbnail with error ", err)
							}

						}
					} else {
						path_, err := os.Getwd()

						if err == nil {
							path_ = strings.ReplaceAll(path_, "\\", "/")
							path_ = strings.ReplaceAll(path_, "\\", "/")
							path_ = path_ + "/mimetypes/unknown.png"
							info_.Thumbnail, _ = s.getThumbnail(path_, 80, 80)
						}
					}
				}

				info.Files = append(info.Files, info_)
			}
		}

	}
	return info, err
}

func (file_server *server) formatPath(path string) string {
	path, _ = url.PathUnescape(path)
	path = strings.ReplaceAll(path, "\\", "/")
	if strings.HasPrefix(path, "/") {
		if len(path) > 1 {
			if strings.HasPrefix(path, "/") {
				if !file_server.isPublic(path) {
					// Must be in the root path if it's not in public path.
					if Utility.Exists(file_server.Root + path) {
						path = file_server.Root + path
					} else if Utility.Exists(config.GetWebRootDir() + path) {
						path = config.GetWebRootDir() + path

					} else if strings.HasPrefix(path, "/users/") || strings.HasPrefix(path, "/applications/") {
						path = config.GetDataDir() + "/files" + path
					}
				}
			} else {
				path = file_server.Root + "/" + path
			}
		} else {
			// '/' represent the root path
			path = file_server.Root
		}
	}

	return path
}

// //////////////////////////////////////////////////////////////////////////////
// Directory operations
// //////////////////////////////////////////////////////////////////////////////
// Append public dir to the list of dir...
func (file_server *server) AddPublicDir(ctx context.Context, rqst *filepb.AddPublicDirRequest) (*filepb.AddPublicDirResponse, error) {
	path := strings.ReplaceAll(rqst.Path, "\\", "/")
	if !Utility.Exists(path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("file with path "+rqst.Path+"dosen't exist")))
	}

	// So here I will test if the path is already in the public path...
	if Utility.Contains(file_server.Public, path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Path "+path+" already exist in Pulbic paths")))
	}

	// Append the path in the list...
	file_server.Public = append(file_server.Public, path)

	// save it in the configuration...
	err := file_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.AddPublicDirResponse{}, nil
}

// Append public dir to the list of dir...
func (file_server *server) RemovePublicDir(ctx context.Context, rqst *filepb.RemovePublicDirRequest) (*filepb.RemovePublicDirResponse, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("file with path "+rqst.Path+"dosen't exist")))
	}

	// So here I will test if the path is already in the public path...
	if !Utility.Contains(file_server.Public, rqst.Path) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Path "+rqst.Path+" dosen exist in Pulbic paths")))
	}

	// Append the path in the list...
	file_server.Public = Utility.RemoveString(file_server.Public, rqst.Path)

	// save it in the configuration...
	err := file_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.RemovePublicDirResponse{}, nil
}

// Return the list of public path from a given file server...
func (file_server *server) GetPublicDirs(context.Context, *filepb.GetPublicDirsRequest) (*filepb.GetPublicDirsResponse, error) {
	return &filepb.GetPublicDirsResponse{Dirs: file_server.Public}, nil
}

func (file_server *server) ReadDir(rqst *filepb.ReadDirRequest, stream filepb.FileService_ReadDirServer) error {

	var err error
	var token string
	ctx := stream.Context()
	if ctx != nil {
		// Now I will index the conversation to be retreivable for it creator...
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			token = strings.Join(md["token"], "")
			if len(token) > 0 {
				_, err := security.ValidateToken(token)
				if err != nil {
					return err
				}
			} else {
				errors.New("no token was given for path " + rqst.Path)
			}
		}
	} else {
		return errors.New("no valid context found")
	}

	path := file_server.formatPath(rqst.Path)

	info, err := readDir(file_server, path, rqst.GetRecursive(), rqst.GetThumnailWidth(), rqst.GetThumnailHeight(), true, token)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I will serialyse the data into JSON.
	jsonStr, err := json.Marshal(info)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		err = stream.Send(&filepb.ReadDirResponse{
			Data: data,
		})
		if err != nil {
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return nil
}

// Create a new directory
func (file_server *server) CreateDir(ctx context.Context, rqst *filepb.CreateDirRequest) (*filepb.CreateDirResponse, error) {
	path := file_server.formatPath(rqst.GetPath())
	err := Utility.CreateDirIfNotExist(path + "/" + rqst.GetName())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var token string
	if ctx != nil {
		// Now I will index the conversation to be retreivable for it creator...
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			token = strings.Join(md["token"], "")
			if len(token) > 0 {
				_, err := security.ValidateToken(token)
				if err != nil {
					return nil, err
				}
			} else {
				errors.New("no token was given for path " + rqst.Path)
			}
		}
	} else {
		return nil, errors.New("no valid context found")
	}

	file_server.createPermission(token, rqst.GetPath()+"/"+rqst.GetName())
	// The directory was successfuly created.
	return &filepb.CreateDirResponse{
		Result: true,
	}, nil
}

// Return true if the file is found in the public path...
func (file_server *server) isPublic(path string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	if Utility.Exists(path) {
		for i := 0; i < len(file_server.Public); i++ {
			if strings.HasPrefix(path, file_server.Public[i]) {
				return true
			}
		}
	}
	return false
}

// Create an archive from a given dir and set it with name.
func (file_server *server) CreateAchive(ctx context.Context, rqst *filepb.CreateArchiveRequest) (*filepb.CreateArchiveResponse, error) {

	var user string
	var domain string
	var err error
	var token string

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			user = claims.Id
			domain = claims.Domain
		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("CreateAchive no token was given")))
		}
	}

	// Here I will create the directory...
	tmp := os.TempDir() + "/" + rqst.GetName()
	createTempDir := true

	// If there only one file no temps dir is required...
	if len(rqst.Paths) == 1 {
		path := rqst.Paths[0]
		if !file_server.isPublic(path) {
			// if the path is not in the Public list it must be in the path...
			path = file_server.Root + path
		}

		// be sure the file exist.
		if !Utility.Exists(path) {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file exist for path "+path)))
		}

		info, _ := os.Stat(path)
		if info.IsDir() {
			tmp = file_server.Root + path
			createTempDir = false
		}
	}

	// This will create a temporary directory...
	if createTempDir {
		Utility.CreateDirIfNotExist(tmp)
		defer os.RemoveAll(tmp)
		for i := 0; i < len(rqst.Paths); i++ {
			// The file or directory must be in the path.
			if Utility.Exists(file_server.Root+rqst.Paths[i]) || file_server.isPublic(rqst.Paths[i]) {
				path := rqst.Paths[i]
				if !file_server.isPublic(rqst.Paths[i]) {
					path = file_server.Root + path
				}
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
	dest := "/users/" + user + "@" + domain + "/" + rqst.GetName() + ".tgz"

	// Set user as owner.
	file_server.createPermission(token, dest)

	// Now I will save the file to the destination.
	err = ioutil.WriteFile(file_server.Root+dest, buf.Bytes(), 0644)
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

func (file_server *server) createPermission(token, path string) error {
	var clientId string

	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil {
			return err
		}
		clientId = claims.Id + "@" + claims.UserDomain
	} else {
		err := errors.New("CreateBlogPost no token was given")
		return err
	}

	var err error
	// Now I will set it in the rbac as resource owner...
	permissions := &rbacpb.Permissions{
		Allowed: []*rbacpb.Permission{},
		Denied:  []*rbacpb.Permission{},
		Owners: &rbacpb.Permission{
			Name:          "owner", // The name is informative in that particular case.
			Applications:  []string{},
			Accounts:      []string{clientId},
			Groups:        []string{},
			Peers:         []string{},
			Organizations: []string{},
		},
	}

	// Set the owner of the conversation.
	rbac_client_, err = getRbacClient()
	if err != nil {
		return err
	}

	err = rbac_client_.SetResourcePermissions(token, path, "file", permissions)

	if err != nil {
		return err
	}

	return nil
}

// Rename a file or a directory.
func (file_server *server) Rename(ctx context.Context, rqst *filepb.RenameRequest) (*filepb.RenameResponse, error) {

	var token string
	if ctx != nil {
		// Now I will index the conversation to be retreivable for it creator...
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			token = strings.Join(md["token"], "")
			if len(token) == 0 {
				return nil, errors.New("No token given")
			}
		}
	} else {
		return nil, errors.New("no valid context found")
	}

	path := file_server.formatPath(rqst.GetPath())

	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	titles := make(map[string][]*titlepb.Title, 0)
	file_server.getFileTitlesAssociation(client, rqst.GetPath()+"/"+rqst.OldName, titles)

	videos := make(map[string][]*titlepb.Video, 0)
	file_server.getFileVideosAssociation(client, rqst.GetPath()+"/"+rqst.OldName, videos)

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
	info, _ := os.Stat(file_server.formatPath(from))

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
func (file_server *server) DeleteDir(ctx context.Context, rqst *filepb.DeleteDirRequest) (*filepb.DeleteDirResponse, error) {
	path := file_server.formatPath(rqst.GetPath())
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
	file_server.getFileTitlesAssociation(client, rqst.GetPath(), titles)

	videos := make(map[string][]*titlepb.Video, 0)
	file_server.getFileVideosAssociation(client, rqst.GetPath(), videos)

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

	// Remove the file itself.
	err = os.RemoveAll(path)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
		log.Println(err)
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
func (file_server *server) GetFileInfo(ctx context.Context, rqst *filepb.GetFileInfoRequest) (*filepb.GetFileInfoResponse, error) {
	path := file_server.formatPath(rqst.GetPath())

	info, err := getFileInfo(file_server, path, int(rqst.GetThumnailHeight()), int(rqst.GetThumnailWidth()))
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var jsonStr string
	jsonStr, err = Utility.ToJson(info)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.GetFileInfoResponse{
		Data: jsonStr,
	}, nil
}

// Read file, can be use for small to medium file...
func (file_server *server) ReadFile(rqst *filepb.ReadFileRequest, stream filepb.FileService_ReadFileServer) error {
	path := file_server.formatPath(rqst.GetPath())

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
func (file_server *server) SaveFile(stream filepb.FileService_SaveFileServer) error {
	// Here I will receive the file
	data := make([]byte, 0)
	var path string
	for {
		rqst, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				// Here all data is read...
				err := ioutil.WriteFile(path, data, 0644)

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
			// The root will be the Root specefied by the server.
			path = file_server.formatPath(msg.Path)

		case *filepb.SaveFileRequest_Data:
			data = append(data, msg.Data...)
		}
	}
}

// Delete file
func (file_server *server) DeleteFile(ctx context.Context, rqst *filepb.DeleteFileRequest) (*filepb.DeleteFileResponse, error) {

	path := file_server.formatPath(rqst.GetPath())

	// Here I will remove the whole
	cache.RemoveItem(path)
	cache.RemoveItem(filepath.Dir(path))

	err := os.Remove(path)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rbac_client_, err = getRbacClient()
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
	dissociateFileWithTitle(rqst.GetPath())

	return &filepb.DeleteFileResponse{
		Result: true,
	}, nil

}

// Convert html to pdf.
func (file_server *server) HtmlToPdf(ctx context.Context, rqst *filepb.HtmlToPdfRqst) (*filepb.HtmlToPdfResponse, error) {
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

	data, err := ioutil.ReadFile(path)
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
	var err error
	if event_client_ == nil {
		address, _ := config.GetAddress()
		event_client_, err = event_client.NewEventService_Client(address, "event.EventService")
		if err != nil {
			return nil, err
		}

	}
	return event_client_, nil
}

/**
 * Return an instance of the title client.
 */
func getTitleClient() (*title_client.Title_Client, error) {
	var err error
	if title_client_ == nil {
		address, _ := config.GetAddress()
		title_client_, err = title_client.NewTitleService_Client(address, "title.TitleService")
		if err != nil {
			return nil, err
		}

	}
	return title_client_, nil
}

func getRbacClient() (*rbac_client.Rbac_Client, error) {
	var err error
	if rbac_client_ == nil {
		address, _ := config.GetAddress()
		rbac_client_, err = rbac_client.NewRbacService_Client(address, "rbac.RbacService")
		if err != nil {
			return nil, err
		}

	}
	return rbac_client_, nil
}

func getAuticationClient() (*authentication_client.Authentication_Client, error) {
	var err error
	if authentication_client_ == nil {
		address, _ := config.GetAddress()
		authentication_client_, err = authentication_client.NewAuthenticationService_Client(address, "authentication.AuthenticationService")
		if err != nil {
			return nil, err
		}

	}
	return authentication_client_, nil
}

func (server *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := getRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
}

func (file_server *server) updateAudioInformation(client *title_client.Title_Client, path string, metadata map[string]interface{}) error {

	return nil
}

// Recursively get all titles for a given path...
func (file_server *server) getFileTitlesAssociation(client *title_client.Title_Client, path string, titles map[string][]*titlepb.Title) error {

	path_ := file_server.formatPath(path)

	info, err := os.Stat(path_)
	if err != nil {
		return err
	}

	if info.IsDir() && !Utility.Exists(path_+"/playlist.m3u8") {
		files, err := ioutil.ReadDir(path_)
		if err == nil {
			for _, f := range files {
				path_ := path + "/" + f.Name()
				if !strings.Contains(path_, ".hidden/") {
					file_server.getFileTitlesAssociation(client, path_, titles)
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

/**
 * return the audios and file associations.
 */
func (file_server *server) getFileAudiosAssociation(client *title_client.Title_Client, path string, audios map[string][]*titlepb.Audio) error {
	path_ := file_server.formatPath(path)
	audios_, err := client.GetFileAudios(config.GetDataDir()+"/search/audios", path_)
	if err == nil {
		audios[path] = audios_
	}

	return err
}

/**
 * Return the list of videos description and file association
 */
func (file_server *server) getFileVideosAssociation(client *title_client.Title_Client, path string, videos map[string][]*titlepb.Video) error {
	path_ := file_server.formatPath(path)
	info, err := os.Stat(path_)
	if err != nil {
		return err
	}

	if info.IsDir() && !Utility.Exists(path_+"/playlist.m3u8") {
		files, err := ioutil.ReadDir(path_)
		if err == nil {
			for _, f := range files {
				path_ := path + "/" + f.Name()
				if !strings.Contains(path_, ".hidden/") {
					file_server.getFileVideosAssociation(client, path_, videos)
				}
			}
		}
	} else {
		videos_, err := getFileVideos(path_)
		if err == nil {
			videos[path] = videos_
		}
	}

	return nil
}

// Move a file/directory
func (file_server *server) Move(ctx context.Context, rqst *filepb.MoveRequest) (*filepb.MoveResponse, error) {

	var token string
	if ctx != nil {
		// Now I will index the conversation to be retreivable for it creator...
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			token = strings.Join(md["token"], "")
			if len(token) == 0 {
				return nil, errors.New("No token given")
			}
		}
	} else {
		return nil, errors.New("no valid context found")
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
		from := file_server.formatPath(rqst.Files[i])
		dest := file_server.formatPath(rqst.Path)
		info, _ := os.Stat(from)

		file_permissions, _ := rbac_client_.GetResourcePermissionsByResourceType("file")
		permissions, _ := rbac_client_.GetResourcePermissions(rqst.Files[i])

		if Utility.Exists(from) {

			titles := make(map[string][]*titlepb.Title, 0)
			file_server.getFileTitlesAssociation(client, rqst.Files[i], titles)

			videos := make(map[string][]*titlepb.Video, 0)
			file_server.getFileVideosAssociation(client, rqst.Files[i], videos)

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

			// Move the file...
			// TODO test if rqst.Path is in the root path...
			err := Utility.Move(from, dest)

			if err == nil {
				// Associate titles...
				for f, titles_ := range titles {
					for _, title := range titles_ {
						var f_ string
						if !info.IsDir() {
							f_ = rqst.Path + "/" + f[strings.LastIndex(f, "/")+1:]
						} else {
							values := strings.Split(rqst.Files[i], "/")
							dest := rqst.Path + "/" + values[len(values)-1]
							f_ = strings.ReplaceAll(f, rqst.Files[i], dest)
						}
						client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, f_)
						if err != nil {
							fmt.Println("fail to asscociate file ", err)
						}
					}
				}

				// Associate titles...
				for f, title_ := range titles {
					for _, t := range title_ {
						var f_ string
						if !info.IsDir() {
							f_ = rqst.Path + "/" + f[strings.LastIndex(f, "/")+1:]
						} else {
							values := strings.Split(rqst.Files[i], "/")
							dest := rqst.Path + "/" + values[len(values)-1]
							f_ = strings.ReplaceAll(f, rqst.Files[i], dest)
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
							f_ = rqst.Path + "/" + f[strings.LastIndex(f, "/")+1:]
						} else {
							values := strings.Split(rqst.Files[i], "/")
							dest := rqst.Path + "/" + values[len(values)-1]
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
							values := strings.Split(rqst.Files[i], "/")
							dest := rqst.Path + "/" + values[len(values)-1]
							p.Path = strings.ReplaceAll(p.Path, rqst.Files[i], dest)
							err := rbac_client_.SetResourcePermissions(token, p.Path, p.ResourceType, p)
							if err != nil {
								fmt.Println("fail to update the permission: ", err)
							}
						}
					}

				} else {
					rbac_client_.DeleteResourcePermissions(rqst.Files[i])
					permissions.Path = rqst.Path + "/" + permissions.Path[strings.LastIndex(permissions.Path, "/")+1:]
					err := rbac_client_.SetResourcePermissions(token, permissions.Path, permissions.ResourceType, permissions)
					if err != nil {
						fmt.Println("fail to update the permission: ", err)
					}
				}

				// If hidden folder exist for it...
				path_ := from[0:strings.LastIndex(from, "/")]
				fileName := from[strings.LastIndex(from, "/")+1:]
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
			}
		}
	}

	return &filepb.MoveResponse{Result: true}, nil
}

// Copy a file/directory
func (file_server *server) Copy(ctx context.Context, rqst *filepb.CopyRequest) (*filepb.CopyResponse, error) {
	var token string
	if ctx != nil {
		// Now I will index the conversation to be retreivable for it creator...
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			token = strings.Join(md["token"], "")
			if len(token) == 0 {
				return nil, errors.New("No token given")
			}
		}
	} else {
		return nil, errors.New("no valid context found")
	}

	// get the rbac client.
	rbac_client_, err := getRbacClient()
	if err != nil {
		return nil, err
	}

	// So here I will call the function mv at repetition for each path...
	for i := 0; i < len(rqst.Files); i++ {
		f := file_server.Root + rqst.Files[i]

		// So here I will try to retreive indexation for the file...
		client, err := getTitleClient()
		if err != nil {
			return nil, err
		}

		file_permissions, _ := rbac_client_.GetResourcePermissionsByResourceType("file")
		permissions, _ := rbac_client_.GetResourcePermissions(rqst.Files[i])

		titles := make(map[string][]*titlepb.Title, 0)
		file_server.getFileTitlesAssociation(client, rqst.Files[i], titles)

		videos := make(map[string][]*titlepb.Video, 0)
		file_server.getFileVideosAssociation(client, rqst.Files[i], videos)

		if Utility.Exists(f) {
			info, err := os.Stat(f)
			if err == nil {
				if info.IsDir() {
					// Copy the directory
					Utility.CopyDir(f, file_server.Root+rqst.Path)

					// Associate titles...
					for f, title_ := range titles {
						for _, t := range title_ {
							var f_ string
							values := strings.Split(rqst.Files[i], "/")
							dest := rqst.Path + "/" + values[len(values)-1]
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
							values := strings.Split(rqst.Files[i], "/")
							dest := rqst.Path + "/" + values[len(values)-1]
							f_ = strings.ReplaceAll(f, rqst.Files[i], dest)
							err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f_)
							if err != nil {
								fmt.Println("fail to asscociate file ", err)
							}
						}
					}

					// So here I will get the list of all file permission and change the one with
					// the old file prefix...
					for j := 0; j < len(file_permissions); j++ {
						p := file_permissions[j]
						if strings.HasPrefix(p.Path, rqst.Files[i]) {
							values := strings.Split(rqst.Files[i], "/")
							dest := rqst.Path + "/" + values[len(values)-1]
							p.Path = strings.ReplaceAll(p.Path, rqst.Files[i], dest)
							err := rbac_client_.SetResourcePermissions(token, p.Path, p.ResourceType, p)
							if err != nil {
								fmt.Println("fail to update the permission: ", err)
							}
						}
					}

				} else {
					// Copy the file
					Utility.CopyFile(f, file_server.Root+rqst.Path)

					// Associate titles...
					for f, title_ := range titles {
						for _, t := range title_ {
							var f_ string
							f_ = rqst.Path + "/" + f[strings.LastIndex(f, "/")+1:]
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
							f_ = rqst.Path + "/" + f[strings.LastIndex(f, "/")+1:]
							err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, f_)
							if err != nil {
								fmt.Println("fail to asscociate file ", err)
							}
						}
					}

					permissions.Path = rqst.Path + "/" + permissions.Path[strings.LastIndex(permissions.Path, "/")+1:]
					err := rbac_client_.SetResourcePermissions(token, permissions.Path, permissions.ResourceType, permissions)
					if err != nil {
						fmt.Println("fail to update the permission: ", err)
					}

					// If hidden folder exist for it...
					path_ := f[0:strings.LastIndex(f, "/")]
					fileName := f[strings.LastIndex(f, "/")+1:]
					if strings.Contains(fileName, ".") {
						fileName = fileName[0:strings.LastIndex(fileName, ".")]
					}
					hiddenFolder := path_ + "/.hidden/" + fileName

					if Utility.Exists(hiddenFolder) {
						err := Utility.CopyDir(hiddenFolder, file_server.Root+rqst.Path+"/.hidden")
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
func (file_server *server) GetThumbnails(rqst *filepb.GetThumbnailsRequest, stream filepb.FileService_GetThumbnailsServer) error {
	var token string
	ctx := stream.Context()
	if ctx != nil {
		// Now I will index the conversation to be retreivable for it creator...
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			token = strings.Join(md["token"], "")
			if len(token) == 0 {
				return errors.New("No token given")
			}
		}
	} else {
		return errors.New("no valid context found")
	}

	path := rqst.GetPath()

	// The root will be the Root specefied by the server.
	if strings.HasPrefix(path, "/") {
		path = file_server.Root + path
		// Set the path separator...
		path = strings.Replace(path, "\\", "/", -1)
	}

	info, err := readDir(file_server, path, rqst.GetRecursive(), rqst.GetThumnailHeight(), rqst.GetThumnailWidth(), true, token)
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

func (file_server *server) WriteExcelFile(ctx context.Context, rqst *filepb.WriteExcelFileRequest) (*filepb.WriteExcelFileResponse, error) {
	path := rqst.GetPath()

	// The root will be the Root specefied by the server.
	if strings.HasPrefix(path, "/") {
		path = file_server.Root + path
		// Set the path separator...
		path = strings.Replace(path, "\\", "/", -1)
	}

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

	err = file_server.writeExcelFile(path, sheets)

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
func (file_server *server) writeExcelFile(path string, sheets map[string]interface{}) error {

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

// /////////////////////////////////////////////////////////////////////////////////////////////////////////
// ffmpeg and video conversion stuff...
// /////////////////////////////////////////////////////////////////////////////////////////////////////////
func (file_server *server) getStartTime() time.Time {
	values := strings.Split(file_server.StartVideoConversionHour, ":")
	var startTime time.Time
	now := time.Now()
	if len(values) == 2 {
		startTime = time.Date(now.Year(), now.Month(), now.Day(), Utility.ToInt(values[0]), Utility.ToInt(values[1]), 0, 0, now.Location())
	}

	return startTime
}

func (file_server *server) isExpired() bool {
	values := strings.Split(file_server.MaximumVideoConversionDelay, ":")
	if len(values) == 2 {
		delay := time.Duration(Utility.ToInt(values[0]))*time.Hour + time.Duration(Utility.ToInt(values[1]))*time.Minute
		if delay == 0 {
			return false
		}

		startTime := file_server.getStartTime()
		endTime := startTime.Add(delay)
		now := time.Now()
		//fmt.Println("no new conversion will be started after: ", endTime)
		return now.After(endTime)

	}
	return false
}

func (file_server *server) startProcessAudios() {
	// Start feeding the time series...
	ticker := time.NewTicker(4 * time.Hour)
	go func() {
		for {
			select {

			case <-ticker.C:
				processAudios(file_server)
			}
		}
	}()
}

func (file_server *server) startProcessVideos() {
	// Start feeding the time series...
	ticker := time.NewTicker(4 * time.Hour)
	go func() {
		for {
			select {
			case <-ticker.C:
				processVideos(file_server)
			}
		}
	}()

	processVideos(file_server)
}

/**
 * Create playlist and search informations.
 */
func processAudios(file_server *server) {

	if file_server.isProcessingAudio {
		return
	}

	file_server.isProcessingAudio = true
	// Process audio files...
	audios := getAudioPaths()

	for _, audio := range audios {
		dir := filepath.Dir(audio)
		if !Utility.Exists(dir + "/audio.m3u") {
			file_server.generatePlaylist(dir, "")
		}
	}
	file_server.isProcessingAudio = false
}

func restoreVideoInfos(video_path string) error {
	// get video info from metadata
	infos, err := getVideoInfos(video_path)
	if err != nil {
		fmt.Println("fail to get video info for file at path: ", video_path)
		return err
	}

	// Get the title
	// so here I will make sure the title exist...
	client, err := getTitleClient()
	if err != nil {
		fmt.Println("fail to connect to local title client ", err)
		return err
	}

	fmt.Println("try to restore file ", video_path)

	if err == nil && infos != nil {
		if infos["format"] != nil {
			if infos["format"].(map[string]interface{})["tags"] != nil {
				tags := infos["format"].(map[string]interface{})["tags"].(map[string]interface{})
				if tags["comment"] != nil {
					comment := tags["comment"].(string)
					if len(comment) > 0 {
						jsonStr, err := base64.StdEncoding.DecodeString(comment)
						if err != nil {
							jsonStr = []byte(comment)
						}
						title := new(titlepb.Title)
						err = jsonpb.UnmarshalString(string(jsonStr), title)
						if err == nil {
							t, paths, err := client.GetTitleById(config.GetDataDir()+"/search/titles", title.ID)
							if err != nil {
								// the title was no found...
								if t == nil {
									client_ := http.DefaultClient
									title__, err := imdb.NewTitle(client_, title.ID)
									if err == nil {
										title.Poster.URL = title__.Poster.ContentURL
										title.Poster.ContentUrl = title__.Poster.ContentURL

										// The hidden folder path...
										lastIndex := -1
										if strings.Contains(video_path, ".mp4") {
											lastIndex = strings.LastIndex(video_path, ".")
										}

										// The hidden folder path...
										path_ := video_path[0:strings.LastIndex(video_path, "/")]

										name_ := video_path[strings.LastIndex(video_path, "/")+1:]
										if lastIndex != -1 {
											name_ = video_path[strings.LastIndex(video_path, "/")+1 : lastIndex]
										}

										thumbnail_path := path_ + "/.hidden/" + name_ + "/__thumbnail__"
										Utility.CreateIfNotExists(thumbnail_path, 0644)
										err = Utility.DownloadFile(title.Poster.URL, thumbnail_path+"/"+title.Poster.URL[strings.LastIndex(title.Poster.URL, "/")+1:])
										if err == nil {

											thumbnail, err := Utility.CreateThumbnail(thumbnail_path+"/"+title.Poster.URL[strings.LastIndex(title.Poster.URL, "/")+1:], 300, 180)
											if err == nil {
												os.WriteFile(thumbnail_path+"/"+"data_url.txt", []byte(thumbnail), 0664)
												title.Poster.ContentUrl = thumbnail
											}

										}

										title.Rating = float32(Utility.ToNumeric(title__.Rating))
										title.RatingCount = int32(title__.RatingCount)
									}

									err = client.CreateTitle("", config.GetDataDir()+"/search/titles", title)
									if err == nil {
										// now I will associate the path.
										path := strings.Replace(video_path, config.GetDataDir()+"/files", "", -1)
										path = strings.ReplaceAll(video_path, "/playlist.m3u8", "")

										client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, path)

									} else {
										fmt.Println("fail to create title ", title.ID, " with error ", err)
									}
								}

							} else {
								path := strings.Replace(video_path, config.GetDataDir()+"/files", "", -1)
								path = strings.Replace(video_path, "/playlist.m3u8", "", -1)
								if !Utility.Contains(paths, path) {
									// associate the path.
									client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", t.ID, path)
								}
							}

						} else {
							video := new(titlepb.Video)
							err := jsonpb.UnmarshalString(string(jsonStr), video)
							if err == nil && video != nil {

								// so here I will make sure the title exist...
								client, err := getTitleClient()
								if err == nil {

									v, paths, _ := client.GetVideoById(config.GetDataDir()+"/search/videos", video.ID)
									if v == nil {
										if video.Poster == nil {
											video.Poster = new(titlepb.Poster)
											video.Poster.ID = video.ID
										}

										video.Poster.ContentUrl, _ = downloadThumbnail(video.ID, video.URL, video_path)
										// the title was no found...
										err := client.CreateVideo("", config.GetDataDir()+"/search/videos", video)
										if err == nil {

											// now I will associate the path.
											path := strings.Replace(video_path, config.GetDataDir()+"/files", "", -1)
											path = strings.Replace(video_path, "/playlist.m3u8", "", -1)
											client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", title.ID, path)

										}

									} else {
										path := strings.Replace(video_path, config.GetDataDir()+"/files", "", -1)
										path = strings.Replace(video_path, "/playlist.m3u8", "", -1)
										if !Utility.Contains(paths, path) {
											// associate the path.
											client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", v.ID, path)
										}
									}
								}
							}
						}

					}
				}
			}
		}
	}

	if err != nil {
		fmt.Println("fail to restore info for file ", video_path, err)
	}

	return err

}

func processVideos(file_server *server) {

	if file_server.isProcessing {
		return
	}

	file_server.isProcessing = true

	// This will be execute in case the server was stop when it process file.
	// Step 1 convert .info.json to video and audio info and move downloaded media file from hidden to the final destination...
	infos := getVideoInfoPaths()
	for i := 0; i < len(infos); i++ {
		info_path := infos[i]
		media_info := make(map[string]interface{})
		data, err := os.ReadFile(info_path)
		if err == nil {
			err = json.Unmarshal(data, &media_info)
			if err == nil {
				if media_info["ext"] != nil {
					path_ := filepath.Dir(info_path)
					path_ = strings.ReplaceAll(path_, "\\", "/")

					fileName_ := filepath.Base(info_path)
					ext := media_info["ext"].(string)

					// this is the actual path on the disk...
					media_path := path_ + "/" + fileName_[0:strings.Index(fileName_, ".")] + "." + ext
					media_path = strings.ReplaceAll(media_path, "\\", "/")

					mac, _ := Utility.MyMacAddr(Utility.MyLocalIP())
					token, _ := security.GetLocalToken(mac)

					// create the file permission...
					dest := strings.ReplaceAll(path_, config.GetDataDir()+"/files/", "/")
					dest = strings.ReplaceAll(dest, "\\", "/")
					dest = strings.ReplaceAll(dest, "/.hidden", "")

					if ext == "mp4" {
						if Utility.Exists(media_path) {
							err = file_server.createVideoInfo(token, dest, path_, info_path)
							go func() {
								fileName_ := strings.ReplaceAll(media_path, "/.hidden/", "/")
								createVideoPreview(file_server, fileName_, 20, 128, false)
								generateVideoPreview(fileName_, 10, 320, 30, true)
								createVideoTimeLine(fileName_, 180, .2, false) // 1 frame per 5 seconds.
							}()
						}
					} else if ext == "mp3" {
						if Utility.Exists(media_path) {
							dir := filepath.Dir(media_path)
							if !Utility.Exists(dir + "/audio.m3u") {
								os.Remove(dir + "/audio.m3u")
							}

							// regenerate the playlist and also save the audio info...
							err = file_server.generatePlaylist(dir, "")
							if err != nil {
								fmt.Println("fail to generate playlist with error ", err)
							}
						}

					}

					if err == nil {
						err = file_server.createPermission(token, dest+"/"+filepath.Base(media_path))
					}

					err := os.Remove(info_path)
					if err != nil {
						fmt.Println("fail to remove file ", info_path, err)
					}

				}
			}
		}

	}

	// Step 2 generate previews, gif
	videos := getVideoPaths()

	// Restore information as needed...
	for i := 0; i < len(videos); i++ {
		err := restoreVideoInfos(videos[i])
		if err != nil {
			fmt.Println("fail to restore video infos with error: ", err)
		}
	}

	for _, video := range videos {
		fmt.Println("-----------------------> 2755: ", video)
		// Create preview and timeline...
		createVideoPreviewLog := new(filepb.VideoConversionLog)
		createVideoPreviewLog.LogTime = time.Now().Unix()
		createVideoPreviewLog.Msg = "Create video preview"
		createVideoPreviewLog.Path = strings.ReplaceAll(video, config.GetDataDir()+"/files", "")
		createVideoPreviewLog.Status = "running"
		file_server.videoConversionLogs.Store(createVideoPreviewLog.LogTime, createVideoPreviewLog)
		file_server.publishConvertionLogEvent(createVideoPreviewLog)

		err := createVideoPreview(file_server, video, 20, 128, false)
		if err != nil {
			fmt.Println("-----------------------> 2767: ", video, err)
			createVideoPreviewLog.Status = "fail"
			file_server.publishConvertionLogEvent(createVideoPreviewLog)
			file_server.publishConvertionLogError(createVideoPreviewLog.Path, err)
			err = nil
		} else {
			createVideoPreviewLog.Status = "done"
			file_server.publishConvertionLogEvent(createVideoPreviewLog)
		}

		generateVideoPreviewLog := new(filepb.VideoConversionLog)
		generateVideoPreviewLog.LogTime = time.Now().Unix()
		generateVideoPreviewLog.Msg = "Generate video Gif image"
		generateVideoPreviewLog.Path = strings.ReplaceAll(video, config.GetDataDir()+"/files", "")
		generateVideoPreviewLog.Status = "running"
		file_server.videoConversionLogs.Store(generateVideoPreviewLog.LogTime, generateVideoPreviewLog)
		file_server.publishConvertionLogEvent(generateVideoPreviewLog)

		err = generateVideoPreview(video, 10, 320, 30, false)
		if err != nil {
			generateVideoPreviewLog.Status = "fail"
			file_server.publishConvertionLogEvent(generateVideoPreviewLog)
			file_server.publishConvertionLogError(generateVideoPreviewLog.Path, err)
			err = nil
		} else {
			generateVideoPreviewLog.Status = "done"
			file_server.publishConvertionLogEvent(generateVideoPreviewLog)
		}

		createVideoTimeLineLog := new(filepb.VideoConversionLog)
		createVideoTimeLineLog.LogTime = time.Now().Unix()
		createVideoTimeLineLog.Msg = "Generate video time line"
		createVideoTimeLineLog.Path = strings.ReplaceAll(video, config.GetDataDir()+"/files", "")
		createVideoTimeLineLog.Status = "running"
		file_server.videoConversionLogs.Store(createVideoTimeLineLog.LogTime, createVideoTimeLineLog)
		file_server.publishConvertionLogEvent(createVideoTimeLineLog)

		err = createVideoTimeLine(video, 180, .2, false) // 1 frame per 5 seconds.
		if err != nil {
			createVideoTimeLineLog.Status = "fail"
			file_server.publishConvertionLogEvent(createVideoTimeLineLog)
			file_server.publishConvertionLogError(createVideoTimeLineLog.Path, err)
			err = nil
		} else {
			createVideoTimeLineLog.Status = "done"
			file_server.publishConvertionLogEvent(createVideoTimeLineLog)
		}
	}

	// Step 3 Convert .mp4 to stream...
	for _, video := range videos {

		// all video mp4 must
		if !strings.HasSuffix(video, ".m3u8") && strings.Contains(video, ".") {
			dir := video[0:strings.LastIndex(video, ".")]
			if !Utility.Exists(dir+"/playlist.m3u8") && Utility.Exists(video) {
				var err error
				_, hasAlreadyFail := file_server.videoConversionErrors.Load(video)

				// TODO test if delay was busted...

				if !hasAlreadyFail {
					if strings.HasSuffix(video, ".mkv") || strings.HasPrefix(video, ".MKV") || strings.HasSuffix(video, ".avi") || strings.HasPrefix(video, ".AVI") || getCodec(video) == "hevc" {

						createVideoMpeg4H264Log := new(filepb.VideoConversionLog)
						createVideoMpeg4H264Log.LogTime = time.Now().Unix()
						createVideoMpeg4H264Log.Msg = "Convert video to mp4 h.264"
						createVideoMpeg4H264Log.Path = strings.ReplaceAll(video, config.GetDataDir()+"/files", "")
						createVideoMpeg4H264Log.Status = "running"
						file_server.videoConversionLogs.Store(createVideoMpeg4H264Log.LogTime, createVideoMpeg4H264Log)
						file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)

						video_, err := createVideoMpeg4H264(video)
						if err != nil {
							if err != nil {
								createVideoMpeg4H264Log.Status = "fail"
								file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)
								fmt.Println("fail with error", err.Error())

								file_server.publishConvertionLogError(video_, err)
							}
						} else {
							video = video_
							createVideoMpeg4H264Log.Status = "done"
							file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)
						}
					}

					// Convert to stream...
					if err == nil && file_server.AutomaticStreamConversion {
						createHlsStreamFromMpeg4H264Log := new(filepb.VideoConversionLog)
						createHlsStreamFromMpeg4H264Log.LogTime = time.Now().Unix()
						createHlsStreamFromMpeg4H264Log.Msg = "Convert video to mp4"
						createHlsStreamFromMpeg4H264Log.Path = strings.ReplaceAll(video, config.GetDataDir()+"/files", "")
						createHlsStreamFromMpeg4H264Log.Status = "running"
						file_server.videoConversionLogs.Store(createHlsStreamFromMpeg4H264Log.LogTime, createHlsStreamFromMpeg4H264Log)
						file_server.publishConvertionLogEvent(createHlsStreamFromMpeg4H264Log)
						err := createHlsStreamFromMpeg4H264(video)
						if err != nil {
							fmt.Println("fail with error", err.Error())
							createHlsStreamFromMpeg4H264Log.Status = "fail"
							file_server.publishConvertionLogEvent(createHlsStreamFromMpeg4H264Log)
							file_server.publishConvertionLogError(video, err)

						} else {
							createHlsStreamFromMpeg4H264Log.Status = "done"
							file_server.publishConvertionLogEvent(createHlsStreamFromMpeg4H264Log)
						}
					}
				}

			} else {
				os.Remove(video)
			}
		}

		// exit if the server was stop or the time is expired...
		if !file_server.isProcessing || file_server.isExpired() {
			break // exit
		}

	}

	file_server.isProcessing = false

}

func getAudioPaths() []string {
	// Here I will use at most one concurrent ffmeg...
	medias := make([]string, 0)
	dirs := make([]string, 0)
	dirs = append(dirs, config.GetPublicDirs()...)
	dirs = append(dirs, config.GetDataDir()+"/files")

	for _, dir := range dirs {
		filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if info == nil {
					return errors.New("fail to get info for path " + path)
				}

				if info.IsDir() {
					isEmpty, err := Utility.IsEmpty(path + "/" + info.Name())
					if err == nil && isEmpty {
						// remove empty dir...
						os.RemoveAll(path + "/" + info.Name())
					}
				}
				if err != nil {
					return err
				}

				path_ := strings.ReplaceAll(path, "\\", "/")
				if !strings.Contains(path_, ".hidden") &&  !strings.Contains(path_, ".temp") && strings.HasSuffix(path_, ".mp3") || strings.HasSuffix(path_, ".wav") || strings.HasSuffix(path_, ".flac") || strings.HasSuffix(path_, ".flc") || strings.HasSuffix(path_, ".acc") || strings.HasSuffix(path_, ".ogg") {
					medias = append(medias, path_)
				}
				return nil
			})
	}

	// Return the list of file to be process...
	return medias
}

// Recursively convert all video that are not in the correct
// format.
func getVideoPaths() []string {

	// Here I will use at most one concurrent ffmeg...
	medias := make([]string, 0)
	dirs := make([]string, 0)
	dirs = append(dirs, config.GetPublicDirs()...)
	dirs = append(dirs, config.GetDataDir()+"/files/users")
	dirs = append(dirs, config.GetDataDir()+"/files/applications")

	for _, dir := range dirs {
		fmt.Println("try to find video in directory ", dir)
		filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {

				if err != nil {
					return err
				}

				if info == nil {
					return errors.New("fail to get info for path " + path)
				}

				if info.IsDir() {
					isEmpty, err := Utility.IsEmpty(path + "/" + info.Name())

					if err == nil && isEmpty {
						// remove empty dir...
						os.RemoveAll(path + "/" + info.Name())
					}
				} else {
					path_ := strings.ReplaceAll(path, "\\", "/")
					if !strings.Contains(path_, ".hidden") && !strings.Contains(path_, ".temp") {
						if strings.HasSuffix(path_, "playlist.m3u8") || strings.HasSuffix(path_, ".mp4") || strings.HasSuffix(path_, ".mkv") || strings.HasSuffix(path_, ".avi") || strings.HasSuffix(path_, ".mov") || strings.HasSuffix(path_, ".wmv") {
							medias = append(medias, path_)
						}
					}
				}
				return nil
			})
	}

	// Return the list of file to be process...
	return medias
}

func getVideoInfoPaths() []string {

	// Here I will use at most one concurrent ffmeg...
	medias := make([]string, 0)
	dirs := make([]string, 0)
	dirs = append(dirs, config.GetPublicDirs()...)
	dirs = append(dirs, config.GetDataDir()+"/files")

	for _, dir := range dirs {
		filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {

				if err != nil {
					return err
				}

				if info == nil {
					return errors.New("fail to get info for path " + path)
				}

				if info.IsDir() {
					isEmpty, err := Utility.IsEmpty(path + "/" + info.Name())
					if err == nil && isEmpty {
						// remove empty dir...
						os.RemoveAll(path + "/" + info.Name())
					}
				}
				if err != nil {
					return err
				}

				path_ := strings.ReplaceAll(path, "\\", "/")
				if strings.HasSuffix(path_, ".info.json") {
					medias = append(medias, path_)
				}

				return nil
			})
	}

	// Return the list of file to be process...
	return medias
}

func getStreamInfos(path string) (map[string]interface{}, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	cmd := exec.Command("ffprobe", "-v", "error", "-show_format", "-show_streams", "-print_format", "json", path)
	data, _ := cmd.CombinedOutput()
	infos := make(map[string]interface{})
	err := json.Unmarshal(data, &infos)
	if err != nil {
		if strings.Contains(err.Error(), "moov atom not found") {
			os.Remove(path) // remove the corrupt of errornous media file.
		}
		return nil, err
	}
	return infos, nil
}

// Get the key frame interval
func getStreamFrameRateInterval(path string) (int, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v", "-of", "default=noprint_wrappers=1:nokey=1", "-show_entries", "stream=r_frame_rate", path)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return -1, err
	}
	values := strings.Split(string(data), "/")
	fps := Utility.ToNumeric(strings.TrimSpace(values[0])) / Utility.ToNumeric(strings.TrimSpace(values[1]))
	return int(fps + .5), nil
}

/**
 * Convert all kind of video to mp4 h64 container so all browser will be able to read it.
 */
func createVideoMpeg4H264(path string) (string, error) {

	process, _ := Utility.GetProcessIdsByName("ffmpeg")
	if len(process) > MAX_FFMPEG_INSTANCE {
		return "", errors.New("number of ffmeg instance has been reach, try it latter")
	}

	if !strings.Contains(path, ".") {
		return "", errors.New(path + " does not has extension")
	}

	path = strings.ReplaceAll(path, "\\", "/")
	path_ := path[0:strings.LastIndex(path, "/")]
	name_ := path[strings.LastIndex(path, "/"):strings.LastIndex(path, ".")]
	output := path_ + "/" + name_ + ".mp4"

	if !strings.HasSuffix(path, ".mp4") {
		if Utility.Exists(output) {
			os.Remove(output)
		}
	} else {
		path = path_ + "/" + name_ + ".hevc"
		if Utility.Exists(path) {
			return "", errors.New("currently processing video " + output)
		}
		Utility.MoveFile(output, path)
	}

	var cmd *exec.Cmd

	streamInfos, err := getStreamInfos(path)

	if err != nil {

		return "", err
	}

	// Here I will test if the encoding is valid
	encoding := ""
	for _, stream := range streamInfos["streams"].([]interface{}) {
		if stream.(map[string]interface{})["codec_type"].(string) == "video" {
			encoding = stream.(map[string]interface{})["codec_long_name"].(string)
		}
	}

	//  https://docs.nvidia.com/video-technologies/video-codec-sdk/ffmpeg-with-nvidia-gpu/
	if hasEnableCudaNvcc() {
		if strings.HasPrefix(encoding, "H.264") || strings.HasPrefix(encoding, "MPEG-4 part 2") {
			cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "h264_nvenc", "-c:a", "aac", output)
		} else if strings.HasPrefix(encoding, "H.265") || strings.HasPrefix(encoding, "Motion JPEG") {
			// in future when all browser will support H.265 I will compile it with this line instead.
			//cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "libx265", "-c:a", "aac", output)
			cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "h264_nvenc", "-c:a", "aac", "-pix_fmt", "yuv420p", output)

		} else {
			err := errors.New("no encoding command foud for " + encoding)
			return "", err
		}

	} else {
		// ffmpeg -i input.mkv -c:v libx264 -c:a aac output.mp4
		if strings.HasPrefix(encoding, "H.264") || strings.HasPrefix(encoding, "MPEG-4 part 2") {
			cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "libx264", "-c:a", "aac", output)
		} else if strings.HasPrefix(encoding, "H.265") || strings.HasPrefix(encoding, "Motion JPEG") {
			// in future when all browser will support H.265 I will compile it with this line instead.
			//cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "libx265", "-c:a", "aac", output)
			cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "libx264", "-c:a", "aac", "-pix_fmt", "yuv420p", output)
		} else {
			err := errors.New("no encoding command foud for " + encoding)

			return "", err
		}
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return "", errors.New(cmd.String() + " " + err.Error())
	}

	// Here I will remove the input file...
	os.Remove(path)

	return output, nil
}

// Dissociate file, if the if is deleted...
func dissociateFileWithTitle(path string) error {
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
	videos, err := getFileVideos(path)
	if err == nil {
		// Here I will asscociate the path
		for _, video := range videos {
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, path)
		}
	}
	return nil
}

func getFileVideos(path string) ([]*titlepb.Video, error) {

	id := path + ":videos"

	data, err := cache.GetItem(id)
	videos := new(titlepb.Videos)

	if err == nil && data != nil {
		err = jsonpb.UnmarshalString(string(data), videos)
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

	// get from the title server.
	videos.Videos, err = client.GetFileVideos(config.GetDataDir()+"/search/videos", path)
	if err != nil {
		return nil, err
	}

	// keep to cache...
	var marshaler jsonpb.Marshaler
	str, _ := marshaler.MarshalToString(videos)
	cache.SetItem(id, []byte(str))

	return videos.Videos, nil

}

func getFileTitles(path string) ([]*titlepb.Title, error) {

	id := path + ":titles"

	data, err := cache.GetItem(id)
	titles := new(titlepb.Titles)

	if err == nil && data != nil {
		err = jsonpb.UnmarshalString(string(data), titles)
		if err == nil {
			return titles.Titles, err
		}
		cache.RemoveItem(id)
	}

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	titles.Titles, err = client.GetFileTitles(config.GetDataDir()+"/search/titles", path)
	if err != nil {
		return nil, err
	}
	// keep to cache...
	var marshaler jsonpb.Marshaler
	str, _ := marshaler.MarshalToString(titles)
	cache.SetItem(id, []byte(str))

	return titles.Titles, nil
}

// Reassociate a path when it name was change...
func reassociatePath(path, new_path string) error {
	path = strings.ReplaceAll(path, "\\", "/")

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return err
	}

	// Now I will asscociate the title.
	titles, err := getFileTitles(path)
	if err == nil {
		// Here I will asscociate the path
		for _, title := range titles {
			client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, new_path)
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, path)
		}
	}

	// Look for videos
	videos, err := getFileVideos(path)

	if err == nil {
		// Here I will asscociate the path
		for _, video := range videos {
			err_0 := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, new_path)
			if err_0 != nil {
				fmt.Println("fail to associte file ", err)
			}
			err_1 := client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, path)
			if err_1 != nil {
				fmt.Println("fail to dissocite file ", err_1)
			}
		}
	} else {
		fmt.Println("no videos found for ", config.GetDataDir()+"/search/videos", " ", path, err)
	}

	return nil
}

func hasEnableCudaNvcc() bool {
	getVersion := exec.Command("ffmpeg", "-encoders")
	encoders, _ := getVersion.CombinedOutput()

	if strings.Index(string(encoders), "hevc_nvenc") > -1 {
		return true
	}
	return false
}

func getCodec(path string) string {

	// ffprobe -v error -select_streams v:0 -show_entries stream=codec_name -of default=noprint_wrappers=1:nokey=1 video.mkv
	getVersion := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", path)
	codec, _ := getVersion.CombinedOutput()
	return strings.TrimSpace(string(codec))
}

// Create the streams...
// segment_target_duration  	try to create a new segment every X seconds
// max_bitrate_ratio 			maximum accepted bitrate fluctuations
// rate_monitor_buffer_ratio	maximum buffer size between bitrate conformance checks
func createHlsStream(src, dest string, segment_target_duration int, max_bitrate_ratio, rate_monitor_buffer_ratio float32) error {

	process, _ := Utility.GetProcessIdsByName("ffmpeg")
	if len(process) > MAX_FFMPEG_INSTANCE {
		return errors.New("number of ffmeg instance has been reach, try it latter")
	}

	src = strings.ReplaceAll(src, "\\", "/")
	dest = strings.ReplaceAll(dest, "\\", "/")
	streamInfos, err := getStreamInfos(src)
	if err != nil {

		return err
	}

	key_frames_interval, err := getStreamFrameRateInterval(src)

	args := make([]string, 0)
	// Here I will test if the encoding is valid
	encoding := ""
	for _, stream := range streamInfos["streams"].([]interface{}) {
		if stream.(map[string]interface{})["codec_type"].(string) == "video" {
			encoding = stream.(map[string]interface{})["codec_long_name"].(string)
		}
	}

	//  https://docs.nvidia.com/video-technologies/video-codec-sdk/ffmpeg-with-nvidia-gpu/
	if hasEnableCudaNvcc() {
		if strings.HasPrefix(encoding, "H.264") || strings.HasPrefix(encoding, "MPEG-4 part 2") {
			args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "h264_nvenc", "-c:a", "aac"}
		} else if strings.HasPrefix(encoding, "H.265") || strings.HasPrefix(encoding, "Motion JPEG") {

			args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "h264_nvenc", "-c:a", "aac", "-pix_fmt", "yuv420p"}
			//args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "hevc_nvenc", "-c:a", "aac"}

		} else {
			err := errors.New("no encoding command foud for " + encoding)
			return err
		}

	} else {
		// ffmpeg -i input.mkv -c:v libx264 -c:a aac output.mp4
		if strings.HasPrefix(encoding, "H.264") || strings.HasPrefix(encoding, "MPEG-4 part 2") {
			args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "libx264", "-c:a", "aac"}
		} else if strings.HasPrefix(encoding, "H.265") || strings.HasPrefix(encoding, "Motion JPEG") {
			// in future when all browser will support H.265 I will compile it with this line instead.
			//cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "libx265", "-c:a", "aac", output)
			args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "libx264", "-c:a", "aac", "-pix_fmt", "yuv420p"}
			//args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "libx265", "-c:a", "aac"}
		} else {
			err := errors.New("no encoding command foud for " + encoding)
			fmt.Println(err.Error())
			return err
		}
	}

	// resolution  bitrate  audio-rate
	renditions := make([]map[string]interface{}, 0)

	w, _ := getVideoResolution(src)
	if w >= 426 {
		renditions = append(renditions, map[string]interface{}{"resolution": "426x240", "bitrate": "400k", "audio-rate": "64k"})
	}
	if w >= 640 {
		renditions = append(renditions, map[string]interface{}{"resolution": "640x360", "bitrate": "400k", "audio-rate": "64k"})
	}

	if w >= 842 {
		renditions = append(renditions, map[string]interface{}{"resolution": "842x480", "bitrate": "1400k", "audio-rate": "128k"})
	}

	if w >= 1280 {
		renditions = append(renditions, map[string]interface{}{"resolution": "1280x720", "bitrate": "2800k", "audio-rate": "128k"})
	}

	if w >= 1920 {
		renditions = append(renditions, map[string]interface{}{"resolution": "1920x1080", "bitrate": "5000k", "audio-rate": "192k"})
	}

	if w >= 3840 {
		renditions = append(renditions, map[string]interface{}{"resolution": "3840x2160", "bitrate": "5000k", "audio-rate": "192k"})
	}

	master_playlist := `#EXTM3U
#EXT-X-VERSION:3
`

	// List of static parameters...
	var static_params []string
	static_params = append(static_params, []string{"-profile:v", "main", "-sc_threshold", "0"}...)
	static_params = append(static_params, []string{"-g", Utility.ToString(key_frames_interval), "-keyint_min", Utility.ToString(key_frames_interval), "-hls_time", Utility.ToString(segment_target_duration)}...)
	static_params = []string{"-hls_playlist_type", "vod"}

	// Now I will append the the renditions parameters to the list...
	for _, rendition := range renditions {
		resolution := rendition["resolution"].(string)
		width := strings.Split(resolution, "x")[0]
		height := strings.Split(resolution, "x")[1]
		bitrate := rendition["bitrate"].(string)
		audiorate := rendition["audio-rate"].(string)
		maxrate := int(float32(Utility.ToNumeric(bitrate[0:len(bitrate)-1])) * max_bitrate_ratio)
		bufsize := int(float32(Utility.ToNumeric(bitrate[0:len(bitrate)-1])) * rate_monitor_buffer_ratio)
		bandwidth := Utility.ToInt(bitrate[0:len(bitrate)-1]) * 1000
		name := height + "p"
		args = append(args, static_params...)
		args = append(args, []string{"-vf", "scale=-2:min(" + width + "\\,if(mod(ih\\,2)\\,ih-1\\,ih))"}...)
		args = append(args, []string{"-b:v", Utility.ToString(bitrate), "-maxrate", Utility.ToString(maxrate) + "k", "-bufsize", Utility.ToString(bufsize) + "k", "-b:a", audiorate}...)
		args = append(args, []string{"-hls_segment_filename", dest + "/" + name + `_%04d.ts`, dest + "/" + name + ".m3u8"}...)

		// static_params = append(static_params, )
		master_playlist += `#EXT-X-STREAM-INF:BANDWIDTH=` + Utility.ToString(bandwidth) + `,RESOLUTION=` + resolution + `
` + name + `.m3u8
`
	}

	cmd := exec.Command("ffmpeg", args...)

	cmd_str_ := "ffmpeg"
	for i := 0; i < len(args); i++ {
		cmd_str_ += " " + args[i]
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		return err
	}
	os.WriteFile(dest+"/playlist.m3u8", []byte(master_playlist), 0644)

	return nil
}

// Create a stream from a vide file, mkv, mpeg4, avi etc...
func createHlsStreamFromMpeg4H264(path string) error {

	process, _ := Utility.GetProcessIdsByName("ffmpeg")
	if len(process) > MAX_FFMPEG_INSTANCE {
		return errors.New("number of ffmeg instance has been reach, try it latter")
	}


	if !strings.Contains(path, ".") {
		return errors.New(path + " does not has extension")
	}

	path = strings.ReplaceAll(path, "\\", "/")
	ext := path[strings.LastIndex(path, ".")+1:]

	// Test if it's already exist.
	output_path := path[0:strings.LastIndex(path, ".")]

	// Here I will remove the existing folder...
	os.RemoveAll(output_path)

	fileName := Utility.GenerateUUID(path[strings.LastIndex(path, "/")+1:])
	Utility.CopyFile(path, os.TempDir()+"/"+fileName+"."+ext)

	// Create the output path...
	os.Remove(os.TempDir() + "/" + fileName)
	Utility.CreateDirIfNotExist(os.TempDir() + "/" + fileName)

	// remove the renamed file and the temp output if te command did not finish......
	defer os.Remove(os.TempDir() + "/" + fileName + "." + ext)
	defer os.Remove(os.TempDir() + "/" + fileName)

	// Create the stream...
	err := createHlsStream(os.TempDir()+"/"+fileName+"."+ext, os.TempDir()+"/"+fileName, 4, 1.07, 1.5)
	if err != nil {
		fmt.Println("fail to generate stream for ", path, "output to", os.TempDir()+"/"+fileName, err)
		return err
	}

	// Move to the correct location...
	err = os.Rename(os.TempDir()+"/"+fileName, os.TempDir()+"/"+output_path[strings.LastIndex(output_path, "/"):])
	if err != nil {
		fmt.Println("fail to rename dir ", os.TempDir()+"/"+fileName, " to ", output_path, err)
		return err
	}

	err = Utility.Move(os.TempDir()+"/"+output_path[strings.LastIndex(output_path, "/"):], output_path[0:strings.LastIndex(output_path, "/")])
	if err != nil {
		fmt.Println("fail to move dir ", os.TempDir()+"/"+fileName, " to ", output_path, err)
		return err
	}

	// remove the mp4 file...
	if Utility.Exists(output_path + "/playlist.m3u8") {

		// reassociate the title here...
		path_ := strings.ReplaceAll(path, config.GetDataDir()+"/files", "")
		reassociatePath(path_, path_[0:strings.LastIndex(path_, ".")])

		// remove the original file.
		os.Remove(path) // remove the orignal file.
	}

	return nil
}

// Format
func formatDuration(duration time.Duration) string {

	var str string
	d_ := duration.Milliseconds()

	// Hours
	h_ := d_ / (1000 * 60 * 60) // The number of hours

	if h_ < 10 {
		str = "0" + Utility.ToString(h_)
	} else {
		Utility.ToString(h_)
	}
	str += ":"

	d_ -= h_ * (1000 * 60 * 60)

	// Minutes
	m_ := d_ / (1000 * 60)
	if m_ < 10 {
		str += "0" + Utility.ToString(m_)
	} else {
		str += Utility.ToString(m_)
	}
	str += ":"

	d_ -= m_ * (1000 * 60)

	// Second
	s_ := d_ / 1000

	if s_ < 10 {
		str += "0" + Utility.ToString(s_)
	} else {
		str += Utility.ToString(s_)
	}

	// set milisecond to 0
	str += ".000"

	return str
}

// Create the video preview...
func generateVideoPreview(path string, fps, scale, duration int, force bool) error {

	process, _ := Utility.GetProcessIdsByName("ffmpeg")
	if len(process) > MAX_FFMPEG_INSTANCE {
		return errors.New("number of ffmeg instance has been reach, try it latter")
	}

	if strings.Contains(path, ".hidden") || strings.Contains(path, ".temp") {
		return nil
	}
	duration_total := getVideoDuration(path)
	if duration == 0 {
		return errors.New("the video lenght is 0 sec")
	}

	if Utility.Exists(path+"/playlist.m3u8") && !strings.HasSuffix(path, "playlist.m3u8") {
		path += "/playlist.m3u8"
	}

	if !strings.Contains(path, ".") {
		return errors.New(path + " does not has extension")
	}

	path_ := path[0:strings.LastIndex(path, "/")]
	name_ := ""

	if strings.HasSuffix(path, "playlist.m3u8") {
		name_ = path_[strings.LastIndex(path_, "/")+1:]
		path_ = path_[0:strings.LastIndex(path_, "/")]
	} else {
		name_ = path[strings.LastIndex(path, "/")+1 : strings.LastIndex(path, ".")]
	}

	output := path_ + "/.hidden/" + name_
	if Utility.Exists(output+"/preview.gif") && Utility.Exists(output+"/preview.mp4") {
		if !force {
			return nil
		}
		os.Remove(output + "/preview.gif")
		os.Remove(output + "/preview.mp4")
	}

	Utility.CreateDirIfNotExist(output)

	if !Utility.Exists(output + "/preview.gif") {
		cmd := exec.Command("ffmpeg", "-ss", Utility.ToString(duration_total*.1), "-t", Utility.ToString(duration), "-i", path, "-vf", "fps="+Utility.ToString(fps)+",scale="+Utility.ToString(scale)+":-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=32[p];[s1][p]paletteuse=dither=bayer", `-loop`, `0`, `preview.gif`)
		cmd.Dir = output // the output directory...
		err := cmd.Run()
		if err != nil {
			os.Remove(output + "/preview.gif")
			return err
		}
	}

	//ffmpeg -y -i /mnt/synology_disk_01/porn/ph5b4d49c0180fb.mp4 -ss 00:00:10 -t 30 -movflags +faststart -filter_complex "[0:v]select='lt(mod(t,1/10),1)',setpts=N/(FRAME_RATE*TB),scale=320:-2" -an outputfile.mp4
	if !Utility.Exists(output + "/preview.mp4") {
		cmd := exec.Command("ffmpeg", "-y", "-i", path, "-ss", Utility.ToString(duration_total*.1), "-t", Utility.ToString(duration), "-filter_complex", `[0:v]select='lt(mod(t,1/10),1)',setpts=N/(FRAME_RATE*TB),scale=`+Utility.ToString(scale)+`:-2`, "-an", "preview.mp4")
		cmd.Dir = output // the output directory...
		err := cmd.Run()
		if err != nil {
			os.Remove(output + "/preview.mp4")
			return err
		}
	}

	

	return nil
}

func createVttFile(output string, fps float32) error {
	// Now I will generate the WEBVTT file with the infos...
	output = strings.ReplaceAll(output, "\\", "/")
	webvtt := "WEBVTT\n\n"

	// So here I will read the file (each file represent is valid for 1/fps second...)
	delay := int(1 / fps)

	thumbnails, err := Utility.ReadDir(output)
	if err != nil {
		return err
	}

	time_ := 0
	index := 1
	address, _ := config.GetAddress()
	localConfig, _ := config.GetLocalConfig(true)

	for _, thumbnail := range thumbnails {
		if strings.HasSuffix(thumbnail.Name(), ".jpg") {
			webvtt += Utility.ToString(index) + "\n"
			start_ := time.Duration(time_ * int(time.Second))
			time_ += delay
			end_ := time.Duration(time_ * int(time.Second))

			webvtt += formatDuration(start_) + " --> " + formatDuration(end_) + "\n"
			webvtt += localConfig["Protocol"].(string) + "://" + address + "/" + strings.ReplaceAll(output, config.GetDataDir()+"/files/", "") + "/" + thumbnail.Name() + "\n\n"
			index++
		}

	}

	// delete previous file...
	os.Remove(output + "/thumbnails.vtt")

	// Now  I will write the file...
	return os.WriteFile(output+"/thumbnails.vtt", []byte(webvtt), 777)
}

// Here I will create the small viedeo video
func createVideoTimeLine(path string, width int, fps float32, force bool) error {

	process, _ := Utility.GetProcessIdsByName("ffmpeg")
	if len(process) > MAX_FFMPEG_INSTANCE {
		return errors.New("number of ffmeg instance has been reach, try it latter")
	}


	path = strings.ReplaceAll(path, "\\", "/")
	// One frame at each 5 seconds...
	if fps == 0 {
		fps = 0.2
	}

	if width == 0 {
		width = 180 // px
	}

	if Utility.Exists(path+"/playlist.m3u8") && !strings.HasSuffix(path, "playlist.m3u8") {
		path += "/playlist.m3u8"
	}

	if !strings.Contains(path, ".") {
		return errors.New(path + " does not has extension")
	}

	path_ := path[0:strings.LastIndex(path, "/")]
	name_ := ""

	if strings.HasSuffix(path, "playlist.m3u8") {
		name_ = path_[strings.LastIndex(path_, "/")+1:]
		path_ = path_[0:strings.LastIndex(path_, "/")]
	} else {
		name_ = path[strings.LastIndex(path, "/")+1 : strings.LastIndex(path, ".")]
	}

	output := path_ + "/.hidden/" + name_ + "/__timeline__"

	if Utility.Exists(output) {
		if !force {
			return createVttFile(output, fps)
		}
		os.Remove(output)
	}

	Utility.CreateDirIfNotExist(output)

	duration := getVideoDuration(path)
	if duration == 0 {
		return errors.New("the video lenght is 0 sec for video at path " + path)
	}

	// ffmpeg -i bob_ross_img-0-Animated.mp4 -ss 15 -t 16 -f image2 preview_%05d.jpg
	cmd := exec.Command("ffmpeg", "-i", path, "-ss", "0", "-t", Utility.ToString(duration), "-vf", "scale=-1:"+Utility.ToString(width)+",fps="+Utility.ToString(fps), "thumbnail_%05d.jpg")
	cmd.Dir = output // the output directory...

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("fail to create time line with error: ", err)
		return err
	}

	return createVttFile(output, fps)
}

// Here I will create the small viedeo video
func createVideoPreview(s *server, path string, nb int, height int, force bool) error {

	process, _ := Utility.GetProcessIdsByName("ffmpeg")
	if len(process) > MAX_FFMPEG_INSTANCE {
		return  errors.New("number of ffmeg instance has been reach, try it latter")
	}


	path = strings.ReplaceAll(path, "\\", "/")

	if Utility.Exists(path+"/playlist.m3u8") && !strings.HasSuffix(path, "playlist.m3u8") {
		path += "/playlist.m3u8"
	}

	if !strings.Contains(path, ".") {
		fmt.Print("fail to create dir ", path, " has no file extension")
		return errors.New(path + " does not has extension")
	}

	// This is the parent path.
	path_ := path[0:strings.LastIndex(path, "/")]
	name_ := ""

	if strings.HasSuffix(path, "playlist.m3u8") {
		name_ = path_[strings.LastIndex(path_, "/")+1:]
		path_ = path_[0:strings.LastIndex(path_, "/")]
	} else {
		name_ = path[strings.LastIndex(path, "/")+1 : strings.LastIndex(path, ".")]
	}

	output := path_ + "/.hidden/" + name_ + "/__preview__"

	if Utility.Exists(output) {
		if !force {
			//fmt.Print("fail to create dir ", output, " already exist")
			return nil
		}
		os.Remove(output)
	}

	// remove it from the cache.
	cache.RemoveItem(path)
	cache.RemoveItem(output)

	// wait for the file to be accessible...
	duration := getVideoDuration(path)
	for nbTry := 60 * 5; duration == 0 && nbTry > 0; nbTry-- {
		time.Sleep(1 * time.Second)
		duration = getVideoDuration(path)
	}

	if duration == 0 {
		fmt.Println("fail to get video duration for", path)
		return errors.New("the video lenght is 0 sec")
	}

	// ffmpeg -i bob_ross_img-0-Animated.mp4 -ss 15 -t 16 -f image2 preview_%05d.jpg

	start := .1 * duration
	laps := 120 // 1 minutes
	var err error
	for nbTry := 60 * 5; nbTry > 0; nbTry-- {
		// Create dir fail for no reason in windows so I will try repeat it until it succed... give im time...
		Utility.CreateDirIfNotExist(output)

		cmd := exec.Command("ffmpeg", "-i", path, "-ss", Utility.ToString(start), "-t", Utility.ToString(laps), "-vf", "scale="+Utility.ToString(height)+":-1,fps=.250", "preview_%05d.jpg")
		cmd.Dir = output // the output directory...

		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr

		err = cmd.Run()
		if err == nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	if err != nil {
		return err
	}

	path_ = strings.ReplaceAll(path, config.GetDataDir()+"/files", "")
	path_ = path_[0:strings.LastIndex(path_, "/")]

	client, err := getEventClient()
	if err == nil {
		dir := filepath.Dir(path)
		dir = strings.ReplaceAll(dir, "\\", "/")
		client.Publish("reload_dir_event", []byte(dir))
	}

	return err
}

func getVideoResolution(path string) (int, int) {
	path = strings.ReplaceAll(path, "\\", "/")

	// original command...
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "default=nw=1", path)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return -1, -1
	}

	w := out.String()[strings.Index(out.String(), "=")+1 : strings.Index(out.String(), "\n")]
	h := out.String()[strings.LastIndex(out.String(), "=")+1:]
	return Utility.ToInt(strings.TrimSpace(w)), Utility.ToInt(strings.TrimSpace(h))
}

// Return the information store in a video file.
func getVideoInfos(path string) (map[string]interface{}, error) {

	path = strings.ReplaceAll(path, "\\", "/")

	if strings.Contains(path, ".hidden") == true {
		return nil, errors.New("no info found for hidden file at path " + path)
	}

	if strings.HasSuffix(path, "playlist.m3u8") {

		// get video info from file.
		path_ := path[0:strings.LastIndex(path, "/")]
		if Utility.Exists(path_ + "/infos.json") {
			data, err := os.ReadFile(path_ + "/infos.json")
			title := make(map[string]interface{})
			err = json.Unmarshal(data, &title)
			if err != nil {
				return nil, err
			}

			// Convert the videos info to json string
			data_, err := json.Marshal(title)
			if err != nil {
				return nil, err
			}

			// encode the data to base 64
			str := base64.StdEncoding.EncodeToString(data_)

			// set the infos in a map... map->format->tags->comment
			infos := make(map[string]interface{})
			infos["format"] = make(map[string]interface{})
			infos["format"].(map[string]interface{})["tags"] = make(map[string]interface{})
			infos["format"].(map[string]interface{})["tags"].(map[string]interface{})["comment"] = str

			return infos, nil

		} else {
			client, err := getTitleClient()
			if err != nil {
				return nil, err
			}

			// Test for videos
			videos, err := getFileVideos(path_)
			if err == nil && videos != nil {
				if len(videos) > 0 {
					// Convert the videos info to json string
					data, err := json.Marshal(videos[0])
					if err != nil {
						return nil, err
					}

					// encode the data to base 64
					str := base64.StdEncoding.EncodeToString(data)

					// set the infos in a map... map->format->tags->comment
					infos := make(map[string]interface{})
					infos["format"] = make(map[string]interface{})
					infos["format"].(map[string]interface{})["tags"] = make(map[string]interface{})
					infos["format"].(map[string]interface{})["tags"].(map[string]interface{})["comment"] = str

					err = os.WriteFile(path_+"/infos.json", data, 0664)
					if err != nil {
						return nil, err
					}

					// return the infos...
					return infos, nil
				}
			}

			// Test for movies
			titles, err := client.GetFileTitles(config.GetDataDir()+"/search/titles", path_)
			if err == nil && titles != nil {
				if len(titles) > 0 {
					// Convert the videos info to json string
					data, err := json.Marshal(titles[0])
					if err != nil {
						return nil, err
					}

					// encode the data to base 64
					str := base64.StdEncoding.EncodeToString(data)

					// set the infos in a map... map->format->tags->comment
					infos := make(map[string]interface{})
					infos["format"] = make(map[string]interface{})
					infos["format"].(map[string]interface{})["tags"] = make(map[string]interface{})
					infos["format"].(map[string]interface{})["tags"].(map[string]interface{})["comment"] = str

					err = os.WriteFile(path_+"/infos.json", data, 0664)
					if err != nil {
						return nil, err
					}

					// return the infos...
					return infos, nil
				}
			}

			return nil, errors.New("no inforamtion was available for file at path " + path)
		}

	} else {

		// original command
		// ffprobe -hide_banner -loglevel fatal  -show_format -print_format -show_private_data json -i 9EcjWd-O4jI.mp4
		cmd := exec.Command(`ffprobe`, `-hide_banner`, `-loglevel`, `fatal`, `-show_format`, `-print_format`, `json`, `-i`, path)
		cmd.Dir = os.TempDir()

		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err := cmd.Run()

		if err != nil {
			return nil, err
		}

		infos := make(map[string]interface{})
		err = json.Unmarshal(out.Bytes(), &infos)
		if err != nil {
			return nil, err
		}

		return infos, nil
	}

}

func getVideoDuration(path string) float64 {
	path = strings.ReplaceAll(path, "\\", "/")
	// original command...
	// ffprobe -v quiet -print_format compact=print_section=0:nokey=1:escape=csv -show_entries format=duration bob_ross_img-0-Animated.mp4
	cmd := exec.Command("ffprobe", `-v`, `quiet`, `-print_format`, `compact=print_section=0:nokey=1:escape=csv`, `-show_entries`, `format=duration`, path)
	cmd.Dir = filepath.Dir(path)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return 0.0
	}

	duration, _ := strconv.ParseFloat(strings.TrimSpace(out.String()), 64)

	return duration
}

func (file_server *server) publishConvertionLogError(path string, err error) {
	file_server.videoConversionErrors.Store(path, err.Error())
	client, err := getEventClient()
	if err != nil {
		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(&filepb.VideoConversionError{Path: path, Error: err.Error()})
		if err != nil {
			client.Publish("conversion_error_event", []byte(jsonStr))
		}
	}
}

func (file_server *server) publishConvertionLogEvent(convertionLog *filepb.VideoConversionLog) {
	client, err := getEventClient()
	if err != nil {
		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(convertionLog)
		if err != nil {
			client.Publish("conversion_log_event", []byte(jsonStr))
		}
	}
}

// Create a viedeo Preview...
func (file_server *server) CreateVideoPreview(ctx context.Context, rqst *filepb.CreateVideoPreviewRequest) (*filepb.CreateVideoPreviewResponse, error) {

	path := file_server.formatPath(rqst.Path)

	if !Utility.Exists(path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	createVideoPreviewLog := new(filepb.VideoConversionLog)
	createVideoPreviewLog.LogTime = time.Now().Unix()
	createVideoPreviewLog.Msg = "Create Video Preview"
	createVideoPreviewLog.Path = rqst.Path
	createVideoPreviewLog.Status = "running"

	// Store the conversion log...
	file_server.videoConversionLogs.Store(createVideoPreviewLog.LogTime, createVideoPreviewLog)
	file_server.publishConvertionLogEvent(createVideoPreviewLog)

	err := createVideoPreview(file_server, path, int(rqst.Nb), int(rqst.Height), true)
	if err != nil {
		createVideoPreviewLog.Status = "fail"
		file_server.publishConvertionLogEvent(createVideoPreviewLog)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	createVideoPreviewLog.Status = "done"
	file_server.publishConvertionLogEvent(createVideoPreviewLog)

	generateVideoGifLog := new(filepb.VideoConversionLog)
	generateVideoGifLog.LogTime = time.Now().Unix()
	generateVideoGifLog.Msg = "Create Video Gif image"
	generateVideoGifLog.Path = path
	generateVideoGifLog.Status = "running"

	// Store the conversion log...
	file_server.videoConversionLogs.Store(generateVideoGifLog.LogTime, generateVideoGifLog)
	file_server.publishConvertionLogEvent(generateVideoGifLog)
	err = generateVideoPreview(path, 10, 320, 30, true)
	if err != nil {
		generateVideoGifLog.Status = "fail"
		file_server.publishConvertionLogEvent(generateVideoGifLog)
		file_server.publishConvertionLogError(rqst.Path, err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	generateVideoGifLog.Status = "done"
	file_server.publishConvertionLogEvent(generateVideoGifLog)

	return &filepb.CreateVideoPreviewResponse{}, nil

}

func (file_server *server) createAudio(client *title_client.Title_Client, path string, duration int, metadata map[string]interface{}) error {
	// here I will create the info in the title server...
	audios := make(map[string][]*titlepb.Audio, 0)
	err := file_server.getFileAudiosAssociation(client, path, audios)
	if err != nil {
		if err.Error() == "no audios found" {
			// so here I will create the information from the metadata...
			track := new(titlepb.Audio)
			track.ID = Utility.GenerateUUID(metadata["Album"].(string) + ":" + metadata["Title"].(string) + ":" + metadata["AlbumArtist"].(string))
			track.Album = metadata["Album"].(string)
			track.AlbumArtist = metadata["AlbumArtist"].(string)
			track.Artist = metadata["Artist"].(string)
			track.Comment = metadata["Comment"].(string)
			track.Composer = metadata["Composer"].(string)
			track.Genres = strings.Split(metadata["Genre"].(string), " / ")
			track.Lyrics = metadata["Lyrics"].(string)
			track.Title = metadata["Title"].(string)
			track.Year = int32(Utility.ToInt(metadata["Year"]))
			track.DiscNumber = int32(Utility.ToInt(metadata["DiscNumber"]))
			track.DiscTotal = int32(Utility.ToInt(metadata["DiscTotal"]))
			track.TrackNumber = int32(Utility.ToInt(metadata["TrackNumber"]))
			track.TrackTotal = int32(Utility.ToInt(metadata["TrackTotal"]))
			track.Duration = int32(duration)
			imageUrl := ""
			if metadata["ImageUrl"] != nil {
				imageUrl = metadata["ImageUrl"].(string)
			}

			track.Poster = &titlepb.Poster{ID: track.ID, URL: "", TitleId: track.ID, ContentUrl: imageUrl}

			err := client.CreateAudio("", config.GetDataDir()+"/search/audios", track)
			if err == nil {
				err := client.AssociateFileWithTitle(config.GetDataDir()+"/search/audios", track.ID, path)
				if err != nil {
					fmt.Println("fail to asscociate file ", err)
				}
			} else {
				fmt.Println("fail to create audio info with error: ", err)
			}
		}
	}

	return nil
}

// Generate an audio playlist
func (file_server *server) generateAudioPlaylist(path, token string, paths []string) error {

	if len(paths) == 0 {
		return errors.New("no paths was given")
	}

	client, err := getTitleClient()
	if err != nil {
		return err
	}

	playlist := "#EXTM3U\n\n"
	playlist += "#PLAYLIST: " + strings.ReplaceAll(path, config.GetDataDir()+"/files/", "/") + "\n\n"

	for i := 0; i < len(paths); i++ {
		metadata, err := readMetadata(file_server, paths[i], 300, 300)
		duration := Utility.ToInt(getVideoDuration(paths[i]) + 0.5)
		if duration > 0 && err == nil {

			id := Utility.GenerateUUID(metadata["Album"].(string) + ":" + metadata["Title"].(string) + ":" + metadata["AlbumArtist"].(string))
			playlist += "#EXTINF:" + Utility.ToString(duration) + ","
			playlist += metadata["Title"].(string) + `, tvg-id="` + id + `"` + ` tvg-url=""`
			playlist += "\n"

			// now I will generate the url...
			localConfig, _ := config.GetLocalConfig(true)
			domain, _ := config.GetDomain()
			url_ := localConfig["Protocol"].(string) + "://" + domain + ":"
			if localConfig["Protocol"] == "https" {
				url_ += Utility.ToString(localConfig["PortHttps"])
			} else {
				url_ += Utility.ToString(localConfig["PortHttp"])
			}

			path_ := strings.ReplaceAll(paths[i], "\\", "/")
			path_ = strings.ReplaceAll(path_, config.GetDataDir()+"/files/", "/")

			if path_[0] != '/' {
				path_ = "/" + path_
			}

			values := strings.Split(path_, "/")
			path_ = ""
			for j := 0; j < len(values); j++ {

				path_ += url.PathEscape(values[j])
				if j < len(values)-1 {
					path_ += "/"
				}
			}

			url_ += path_

			if !file_server.isPublic(path) && len(token) > 0 {
				url_ += "?token=" + token
			}

			playlist += url_ + "\n\n"

			file_server.createAudio(client, paths[i], duration, metadata)

		}
	}

	// Here I will save the file...
	Utility.WriteStringToFile(path+"/audio.m3u", playlist)

	return nil
}

// Generate an audio playlist
func (file_server *server) generateVideoPlaylist(path, token string, paths []string) error {
	if len(paths) == 0 {
		return errors.New("no paths was given")
	}
	client, err := getTitleClient()
	if err != nil {
		return err
	}

	playlist := "#EXTM3U\n\n"
	playlist += "#PLAYLIST: " + strings.ReplaceAll(path, config.GetDataDir()+"/files/", "/") + "\n\n"

	for i := 0; i < len(paths); i++ {

		videos := make(map[string][]*titlepb.Video, 0)
		file_server.getFileVideosAssociation(client, paths[i], videos)
		duration := getVideoDuration(paths[i])

		if duration > 0 {

			playlist += "#EXTINF:" + Utility.ToString(duration)

			if len(videos[paths[i]]) > 0 {
				videoInfo := videos[paths[i]][0]
				playlist += ` tvg-id="` + videoInfo.ID + `"` + ` tvg-url="` + videoInfo.URL + `"` + "," + videoInfo.Description
			}

			playlist += "\n"

			// now I will generate the url...
			localConfig, _ := config.GetLocalConfig(true)
			domain, _ := config.GetDomain()
			url := localConfig["Protocol"].(string) + "://" + domain + ":"
			if localConfig["Protocol"] == "https" {
				url += Utility.ToString(localConfig["PortHttps"])
			} else {
				url += Utility.ToString(localConfig["PortHttp"])
			}

			url += strings.ReplaceAll(paths[i], config.GetDataDir()+"/files/", "/")
			if !file_server.isPublic(path) {
				url += "?token=" + token
			}
			playlist += url + "\n\n"
		}
	}

	// Here I will save the file...
	Utility.WriteStringToFile(path+"/video.m3u", playlist)

	return nil
}

// Generate video and audio playlist for a given directory.
func (file_server *server) generatePlaylist(path, token string) error {

	// first of all I will retreive media files from the folder...
	infos, err := Utility.ReadDir(path) // getFileInfo(file_server, path)

	if err != nil {
		return err
	}

	videos := make([]string, 0)
	audios := make([]string, 0)

	for i := 0; i < len(infos); i++ {
		filename := filepath.Join(path, infos[i].Name())
		if !strings.HasSuffix(filename, ".m3u") {
			info, err := getFileInfo(file_server, filename, -1, -1)
			if err == nil {
				if strings.HasPrefix(info.Mime, "audio/") {
					audios = append(audios, filename)
				} else if strings.HasPrefix(info.Mime, "video/") {
					videos = append(videos, filename)
				}
			}
		}
	}

	// here I will generate the audio playlist
	if len(audios) > 0 {
		file_server.generateAudioPlaylist(path, token, audios)
	}

	// Here I will generate video playlist.
	if len(videos) > 0 {
		file_server.generateVideoPlaylist(path, token, videos)
	}

	// tell client that something new append!!!
	file_server.publishReloadDirEvent(path)

	return nil
}

// Generate the playlists for a directory...
func (file_server *server) GeneratePlaylist(ctx context.Context, rqst *filepb.GeneratePlaylistRequest) (*filepb.GeneratePlaylistResponse, error) {
	token := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")
		if len(token) > 0 {
			_, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("GeneratePlaylist no token was given")))
		}
	}

	// retreive the path...
	path := file_server.formatPath(rqst.Dir)
	if !Utility.Exists(path) {
		return nil, errors.New("no file found at path " + rqst.Dir)
	}

	// remove the previous playlist...
	os.Remove(path + "/audio.mu3")
	os.Remove(path + "/video.mu3")

	err := file_server.generatePlaylist(path, token)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.GeneratePlaylistResponse{}, nil
}

// Create video time line
func (file_server *server) CreateVideoTimeLine(ctx context.Context, rqst *filepb.CreateVideoTimeLineRequest) (*filepb.CreateVideoTimeLineResponse, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	createVideoTimeLineLog := new(filepb.VideoConversionLog)
	createVideoTimeLineLog.LogTime = time.Now().Unix()
	createVideoTimeLineLog.Msg = "Create Video time line"
	createVideoTimeLineLog.Path = rqst.Path
	createVideoTimeLineLog.Status = "running"

	file_server.videoConversionLogs.Store(createVideoTimeLineLog.LogTime, createVideoTimeLineLog)
	file_server.publishConvertionLogEvent(createVideoTimeLineLog)

	err := createVideoTimeLine(rqst.Path, int(rqst.Width), rqst.Fps, true)
	if err != nil {
		createVideoTimeLineLog.Status = "fail"
		file_server.publishConvertionLogEvent(createVideoTimeLineLog)
		file_server.publishConvertionLogError(rqst.Path, err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	createVideoTimeLineLog.Status = "done"
	file_server.publishConvertionLogEvent(createVideoTimeLineLog)

	return &filepb.CreateVideoTimeLineResponse{}, nil
}

// Convert a file from mkv, avi or other format to MPEG-4 AVC
func (file_server *server) ConvertVideoToMpeg4H264(ctx context.Context, rqst *filepb.ConvertVideoToMpeg4H264Request) (*filepb.ConvertVideoToMpeg4H264Response, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	createVideoMpeg4H264Log := new(filepb.VideoConversionLog)
	createVideoMpeg4H264Log.LogTime = time.Now().Unix()
	createVideoMpeg4H264Log.Msg = "Convert video to mp4"
	createVideoMpeg4H264Log.Path = rqst.Path
	createVideoMpeg4H264Log.Status = "running"

	file_server.videoConversionLogs.Store(createVideoMpeg4H264Log.LogTime, createVideoMpeg4H264Log)
	file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)

	_, err := createVideoMpeg4H264(rqst.Path)
	if err != nil {
		file_server.publishConvertionLogError(rqst.Path, err)
		createVideoMpeg4H264Log.Status = "fail"
		file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	createVideoMpeg4H264Log.Status = "done"
	file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)

	return &filepb.ConvertVideoToMpeg4H264Response{}, nil
}

// Convert a video file (must be  MPEG-4 H264) to HLS stream... That will automatically generate the
// the streams for various resolutions. (see script create-vod-hls.sh for more info)
func (file_server *server) ConvertVideoToHls(ctx context.Context, rqst *filepb.ConvertVideoToHlsRequest) (*filepb.ConvertVideoToHlsResponse, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	// in case of a mkv Need conversion before...
	if strings.HasSuffix(rqst.Path, ".avi") || strings.HasPrefix(rqst.Path, ".AVI") || strings.HasSuffix(rqst.Path, ".mkv") || strings.HasPrefix(rqst.Path, ".MKV") || getCodec(rqst.Path) == "hevc" {
		var err error
		createVideoMpeg4H264Log := new(filepb.VideoConversionLog)
		createVideoMpeg4H264Log.LogTime = time.Now().Unix()
		createVideoMpeg4H264Log.Msg = "Convert video to mp4"
		createVideoMpeg4H264Log.Path = rqst.Path
		createVideoMpeg4H264Log.Status = "running"

		file_server.videoConversionLogs.Store(createVideoMpeg4H264Log.LogTime, createVideoMpeg4H264Log)
		file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)
		rqst.Path, err = createVideoMpeg4H264(rqst.Path)
		if err != nil {
			file_server.publishConvertionLogError(rqst.Path, err)
			createVideoMpeg4H264Log.Status = "fail"
			file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)

			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
		createVideoMpeg4H264Log.Status = "done"
		file_server.publishConvertionLogEvent(createVideoMpeg4H264Log)
	}

	// Create the hls stream from MPEG-4 H264 file.
	createHlsStreamFromMpeg4H264Log := new(filepb.VideoConversionLog)
	createHlsStreamFromMpeg4H264Log.LogTime = time.Now().Unix()
	createHlsStreamFromMpeg4H264Log.Msg = "Convert video to stream"
	createHlsStreamFromMpeg4H264Log.Path = rqst.Path
	createHlsStreamFromMpeg4H264Log.Status = "running"
	file_server.videoConversionLogs.Store(createHlsStreamFromMpeg4H264Log.LogTime, createHlsStreamFromMpeg4H264Log)
	file_server.publishConvertionLogEvent(createHlsStreamFromMpeg4H264Log)

	err := createHlsStreamFromMpeg4H264(rqst.Path)
	if err != nil {
		file_server.publishConvertionLogError(rqst.Path, err)
		createHlsStreamFromMpeg4H264Log.Status = "fail"
		file_server.publishConvertionLogEvent(createHlsStreamFromMpeg4H264Log)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	createHlsStreamFromMpeg4H264Log.Status = "done"
	file_server.publishConvertionLogEvent(createHlsStreamFromMpeg4H264Log)

	return &filepb.ConvertVideoToHlsResponse{}, nil
}

func shortDur(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

func (srv *server) publishReloadDirEvent(path string) {
	client, err := getEventClient()
	path = strings.ReplaceAll(path, "\\", "/")
	if err == nil {
		client.Publish("reload_dir_event", []byte(path))
	}
}

func (file_server *server) createVideoInfo(token, path, absolute_path, info_path string) error {
	if strings.Contains(path, ".hidden") {
		return nil
	}

	data, err := ioutil.ReadFile(info_path)
	if err == nil {
		info := make(map[string]interface{})
		err = json.Unmarshal(data, &info)
		if err == nil {
			// So here I will
			//  indexPornhubVideo(token, video_url, index_path, video_path, file_path  string)
			// Scrapper...
			video_url := info["webpage_url"].(string)
			var video *titlepb.Video
			var video_id = info["id"].(string)
			var file_path = absolute_path + "/" + video_id + ".mp4"
			var video_path = path + "/" + video_id + ".mp4"
			var index_path = config.GetDataDir() + "/search/videos"
			if strings.Contains(video_url, "pornhub") {
				video, err = indexPornhubVideo(token, video_id, video_url, index_path, video_path, strings.ReplaceAll(file_path, "/.hidden/", ""))
			} else if strings.Contains(video_url, "xnxx") {
				video, err = indexXnxxVideo(token, video_id, video_url, index_path, video_path, strings.ReplaceAll(file_path, "/.hidden/", ""))
			} else if strings.Contains(video_url, "xvideo") {
				video, err = indexXvideosVideo(token, video_id, video_url, index_path, video_path, strings.ReplaceAll(file_path, "/.hidden/", ""))
			} else if strings.Contains(video_url, "xhamster") {
				video, err = indexXhamsterVideo(token, video_id, video_url, index_path, video_path, strings.ReplaceAll(file_path, "/.hidden/", ""))
			} else if strings.Contains(video_url, "youtube") {
				video, err = indexYoutubeVideo(token, video_id, video_url, index_path, video_path, strings.ReplaceAll(file_path, "/.hidden/", ""))
				if info["thumbnails"] != nil {
					if len(info["thumbnails"].([]interface{})) > 0 {
						video.Poster.URL = info["thumbnails"].([]interface{})[0].(map[string]interface{})["url"].(string)
					}
				}
			}
			if err == nil && video != nil {
				// set info from the json file...
				if info["fulltitle"] != nil {
					video.Description = info["fulltitle"].(string)
					if info["thumbnail"] != nil {
						video.Poster.URL = info["thumbnail"].(string)
					}
				}

				// set genre (categories)
				if info["categories"] != nil {
					categories := info["categories"].([]interface{})
					for i := 0; i < len(categories); i++ {
						video.Genres = append(video.Genres, categories[i].(string))
					}
				}

				if info["tags"] != nil {
					// set tags
					tags := info["tags"].([]interface{})
					for i := 0; i < len(tags); i++ {
						video.Tags = append(video.Tags, tags[i].(string))
					}
				}

				if info["like_count"] != nil {
					video.Likes = int64(Utility.ToInt(info["like_count"]))
					video.Count = int64(Utility.ToInt(info["view_count"]))
					if info["dislike_count"] != nil {
						video.Rating = float32(info["like_count"].(float64)/(info["like_count"].(float64)+info["dislike_count"].(float64))) * 10
					}
				}

				if info["duration"] != nil {
					video.Duration = shortDur(time.Duration(Utility.ToInt(info["duration"])))
				}

				title_client_, err := getTitleClient()
				if err != nil {
					return err
				}

				err = title_client_.CreateVideo(token, index_path, video)
				if err == nil {
					err := title_client_.AssociateFileWithTitle(index_path, video.ID, video_path)
					if err != nil {
						fmt.Println("fail to associate file with video information ", err)
						return err
					}
				} else {
					fmt.Println("fail to associate file with video information ", err)
					return err
				}

			} else {
				fmt.Println("fail to index video with error ", err)
				return err
			}
		}
	}

	return err
}

// Upload a video from a given url, it use youtube-dl.
func (file_server *server) UploadVideo(rqst *filepb.UploadVideoRequest, stream filepb.FileService_UploadVideoServer) error {
	var err error

	path := file_server.formatPath(rqst.Dest)
	pid := -1

	if !Utility.Exists(path) {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no folder found with path "+path)))
	}

	// Now I will set the path to the hidden folder...
	//path += "/.hidden"
	path = strings.ReplaceAll(path, "\\", "/")

	Utility.CreateDirIfNotExist(path)
	done := make(chan bool)

	baseCmd := "yt-dlp"
	var cmdArgs []string

	if rqst.Format == "mp3" {
		cmdArgs = append(cmdArgs, []string{"-f", "bestaudio", "--extract-audio", "--audio-format", "mp3", "--audio-quality", "0", "--embed-thumbnail", "--embed-metadata", "--write-info-json", "-o", `%(id)s.%(ext)s`, rqst.Url}...)
	} else {
		cmdArgs = append(cmdArgs, []string{"-f", "mp4", "--write-info-json", "--embed-thumbnail", "-o", `%(id)s.%(ext)s`, rqst.Url}...)
	}

	cmd := exec.Command(baseCmd, cmdArgs...)
	cmd.Dir = path

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	output := make(chan string)

	// Process message util the command is done.
	go func() {
		for {
			select {
			case <-done:
				break

			case result := <-output:
				if cmd.Process != nil {
					pid = cmd.Process.Pid
				}

				stream.Send(
					&filepb.UploadVideoResponse{
						Pid:    int32(pid),
						Result: result,
					},
				)
			}
		}
	}()

	// Start reading the output
	go Utility.ReadOutput(output, stdout)
	err = cmd.Run()
	if err != nil {
		fmt.Println("fail to run command ", err)
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	cmd.Wait()

	// Close the output.
	stdout.Close()
	done <- true

	fmt.Println("-----------------> 4519: ", path)

	// Done with upload now I will porcess videos
	ctx := stream.Context()
	var token string
	var domain string

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return err
			}
			domain = claims.UserDomain
		}
	}

	// Process videos...
	videos := Utility.GetFilePathsByExtension(path, ".mp4")
	for i := 0; i < len(videos); i++ {

		// Replace file name...
		fileName := videos[i]
		info_path := strings.ReplaceAll(fileName, ".mp4", ".info.json")
		if Utility.Exists(info_path) {

			// here I will validate the token...
			_, err = security.ValidateToken(token)
			if err != nil {
				client, err := authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
				if err != nil {
					return err
				}

				// Try to refresh the token...
				token, err = client.RefreshToken(token)
				if err != nil {
					return err
				}
			}

			stream.Send(
				&filepb.UploadVideoResponse{
					Pid:    int32(pid),
					Result: "create video info for " + fileName,
				},
			)

			err = file_server.createVideoInfo(token, rqst.Dest, path, info_path)

			if err != nil {
				fmt.Println("fail to create video info with error ", err)
				stream.Send(
					&filepb.UploadVideoResponse{
						Pid:    int32(pid),
						Result: "fail to create video info with error " + err.Error(),
					},
				)
			}

			// create the file permission...
			err = file_server.createPermission(token, rqst.Dest+"/"+filepath.Base(fileName))
			stream.Send(
				&filepb.UploadVideoResponse{
					Pid:    int32(pid),
					Result: "create permission " + fileName,
				},
			)
			if err != nil {
				fmt.Println("fail to create video permission with error ", err)
				stream.Send(
					&filepb.UploadVideoResponse{
						Pid:    int32(pid),
						Result: "fail to create video permission with error " + err.Error(),
					},
				)
			}

			// remove the files...
			stream.Send(
				&filepb.UploadVideoResponse{
					Pid:    int32(pid),
					Result: "remove file " + info_path,
				},
			)
			err := os.Remove(info_path)
			if err != nil {
				fmt.Println("fail to remove file ", info_path, err)
				stream.Send(
					&filepb.UploadVideoResponse{
						Pid:    int32(pid),
						Result: "fail to remove file " + err.Error(),
					},
				)
			}

			if !Utility.Exists(path + "/video.m3u") {
				os.Remove(path + "/video.m3u")
			}

			// regenerate the playlist and also save the audio info...
			err = file_server.generatePlaylist(path, "")
			if err != nil {
				fmt.Println("fail to generate playlist with error ", err)
			}

			// call videos processing and return...
			go func() {
				fileName_ := strings.ReplaceAll(fileName, "/.hidden/", "/")
				createVideoPreview(file_server, fileName_, 20, 128, false)
				generateVideoPreview(fileName_, 10, 320, 30, true)
				createVideoTimeLine(fileName_, 180, .2, false) // 1 frame per 5 seconds.

			}()
		}
	}

	// Process audio
	audios := Utility.GetFilePathsByExtension(path, ".mp3")
	needRefresh := false
	for i := 0; i < len(audios); i++ {
		fileName := audios[i]
		info_path := strings.ReplaceAll(fileName, ".mp3", ".info.json")
		if Utility.Exists(info_path) {
			needRefresh = true
			// create the file permission...
			err = file_server.createPermission(token, rqst.Dest+"/"+filepath.Base(fileName))
			if err != nil {
				fmt.Println("fail to create video permission with error ", err)
			}

			err := os.Remove(info_path)
			if err != nil {
				fmt.Println("fail to remove file ", info_path, err)
			}
		}

	}
	if needRefresh {

		if !Utility.Exists(path + "/audio.m3u") {
			os.Remove(path + "/audio.m3u")
		}

		// regenerate the playlist and also save the audio info...
		err = file_server.generatePlaylist(path, "")
		if err != nil {
			fmt.Println("fail to generate playlist with error ", err)
		}
	}

	stream.Send(
		&filepb.UploadVideoResponse{
			Pid:    int32(pid),
			Result: "done",
		},
	)

	// So now I have the file uploaded...
	return err
}

// Start process video on the server.
func (file_server *server) StartProcessVideo(ctx context.Context, rqst *filepb.StartProcessVideoRequest) (*filepb.StartProcessVideoResponse, error) {
	// Convert video file, set permissions...
	if file_server.isProcessing {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("video conversion is already runnig")))
	}

	// start conversion.
	go func() {
		processVideos(file_server) // Process files...
	}()

	return &filepb.StartProcessVideoResponse{}, nil
}

// Return true if video processing is running.
func (file_server *server) IsProcessVideo(ctx context.Context, rqst *filepb.IsProcessVideoRequest) (*filepb.IsProcessVideoResponse, error) {
	return &filepb.IsProcessVideoResponse{IsProcessVideo: file_server.isProcessing}, nil
}

// Stop process video on the server.
func (file_server *server) StopProcessVideo(ctx context.Context, rqst *filepb.StopProcessVideoRequest) (*filepb.StopProcessVideoResponse, error) {

	file_server.isProcessing = false

	// kill current procession...
	err := Utility.KillProcessByName("ffmpeg")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.StopProcessVideoResponse{}, nil
}

// Set video processing.
func (file_server *server) SetVideoConversion(ctx context.Context, rqst *filepb.SetVideoConversionRequest) (*filepb.SetVideoConversionResponse, error) {

	file_server.AutomaticVideoConversion = rqst.Value
	// remove process video...
	file_server.scheduler.Remove(processVideos)

	if file_server.AutomaticVideoConversion {
		file_server.scheduler.Every(1).Day().At(file_server.StartVideoConversionHour).Do(processVideos, file_server)
		file_server.scheduler.Start()
	}

	err := file_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &filepb.SetVideoConversionResponse{}, nil
}

// Set video stream conversion.
func (file_server *server) SetVideoStreamConversion(ctx context.Context, rqst *filepb.SetVideoStreamConversionRequest) (*filepb.SetVideoStreamConversionResponse, error) {
	file_server.AutomaticStreamConversion = rqst.Value
	err := file_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.SetVideoStreamConversionResponse{}, nil
}

// Set the hour when the video conversion must start.
func (file_server *server) SetStartVideoConversionHour(ctx context.Context, rqst *filepb.SetStartVideoConversionHourRequest) (*filepb.SetStartVideoConversionHourResponse, error) {
	file_server.StartVideoConversionHour = rqst.Value

	// remove actual process video...
	file_server.scheduler.Remove(processVideos)

	if file_server.AutomaticVideoConversion {
		file_server.scheduler.Every(1).Day().At(file_server.StartVideoConversionHour).Do(processVideos, file_server)
		file_server.scheduler.Start()
	}

	err := file_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &filepb.SetStartVideoConversionHourResponse{}, nil
}

// Set the maximum delay when conversion can run, it will finish actual conversion but it will not begin new conversion past this delay.
func (file_server *server) SetMaximumVideoConversionDelay(ctx context.Context, rqst *filepb.SetMaximumVideoConversionDelayRequest) (*filepb.SetMaximumVideoConversionDelayResponse, error) {
	file_server.MaximumVideoConversionDelay = rqst.Value
	err := file_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &filepb.SetMaximumVideoConversionDelayResponse{}, nil
}

// Return the list of failed video conversion.
func (file_server *server) GetVideoConversionErrors(ctx context.Context, rqst *filepb.GetVideoConversionErrorsRequest) (*filepb.GetVideoConversionErrorsResponse, error) {
	video_conversion_errors := make([]*filepb.VideoConversionError, 0)

	file_server.videoConversionErrors.Range(func(key, value interface{}) bool {
		video_conversion_errors = append(video_conversion_errors, &filepb.VideoConversionError{Path: key.(string), Error: value.(string)})
		return true
	})

	return &filepb.GetVideoConversionErrorsResponse{Errors: video_conversion_errors}, nil
}

// Clear the video conversion errors
func (file_server *server) ClearVideoConversionErrors(ctx context.Context, rqst *filepb.ClearVideoConversionErrorsRequest) (*filepb.ClearVideoConversionErrorsResponse, error) {
	file_server.videoConversionErrors.Range(func(key, value interface{}) bool {
		file_server.videoConversionErrors.Delete(key)
		return true
	})

	return &filepb.ClearVideoConversionErrorsResponse{}, nil
}

// Clear a specific video conversion error
func (file_server *server) ClearVideoConversionError(ctx context.Context, rqst *filepb.ClearVideoConversionErrorRequest) (*filepb.ClearVideoConversionErrorResponse, error) {
	file_server.videoConversionErrors.Delete(rqst.Path)
	return &filepb.ClearVideoConversionErrorResponse{}, nil
}

// Clear a specific video conversion log
func (file_server *server) ClearVideoConversionLogs(ctx context.Context, rqst *filepb.ClearVideoConversionLogsRequest) (*filepb.ClearVideoConversionLogsResponse, error) {

	file_server.videoConversionLogs.Range(func(key, value interface{}) bool {
		file_server.videoConversionLogs.Delete(key)
		return true
	})

	return &filepb.ClearVideoConversionLogsResponse{}, nil
}

// Return the list of log messages
func (file_server *server) GetVideoConversionLogs(ctx context.Context, rqst *filepb.GetVideoConversionLogsRequest) (*filepb.GetVideoConversionLogsResponse, error) {
	logs := make([]*filepb.VideoConversionLog, 0)

	file_server.videoConversionLogs.Range(func(key, value interface{}) bool {
		logs = append(logs, value.(*filepb.VideoConversionLog))
		return true
	})

	return &filepb.GetVideoConversionLogsResponse{Logs: logs}, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// The actual server implementation.
	s_impl := new(server)

	// The name must the same as the grpc service name.
	s_impl.Name = string(filepb.File_file_proto.Services().Get(0).FullName())
	s_impl.Proto = filepb.File_file_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "globulario"
	s_impl.Permissions = make([]interface{}, 12)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"rbac.RbacService"}
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.Public = make([]string, 0) // The list of public directory where files can be read...

	cache = storage_store.NewBigCache_store()
	cache.Open("")

	// Video conversion retalted configuration.
	s_impl.scheduler = gocron.NewScheduler()
	s_impl.videoConversionErrors = new(sync.Map)
	s_impl.videoConversionLogs = new(sync.Map)
	s_impl.AutomaticStreamConversion = false
	s_impl.AutomaticVideoConversion = false
	s_impl.MaximumVideoConversionDelay = "00:00" // convert for 8 hours...
	s_impl.StartVideoConversionHour = "00:00"    // start conversion at midnight, when every one sleep

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
	s_impl.Permissions[10] = map[string]interface{}{"action": "/file.FileService/CreateAchive", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[11] = map[string]interface{}{"action": "/file.FileService/FileUploadHandler", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}

	// Set the root path if is pass as argument.
	if len(s_impl.Root) == 0 {
		s_impl.Root = os.TempDir()
	}

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

	if len(s_impl.MaximumVideoConversionDelay) == 0 {
		s_impl.StartVideoConversionHour = "00:00"

	}

	if len(s_impl.StartVideoConversionHour) == 0 {
		s_impl.StartVideoConversionHour = "00:00"
	}

	// Register the echo services
	filepb.RegisterFileServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Now the event client service.
	go func() {

		client, err := getEventClient()
		if err == nil {

			channel_0 := make(chan string)
			channel_1 := make(chan string)
			channel_2 := make(chan string)

			// Process request received...
			go func() {
				for {
					select {
					case path := <-channel_0:
						// Now I will create the ownership...
						fmt.Println("generate_video_preview_event received for path ", path)
						if strings.HasPrefix(path, "/users/") {
							values := strings.Split(path, "/")
							if len(values) > 1 {
								owner := values[2]

								// Now I will set it in the rbac as resource owner...
								permissions := &rbacpb.Permissions{
									Allowed: []*rbacpb.Permission{},
									Denied:  []*rbacpb.Permission{},
									Owners: &rbacpb.Permission{
										Name:          "owner", // The name is informative in that particular case.
										Applications:  []string{},
										Accounts:      []string{owner},
										Groups:        []string{},
										Peers:         []string{},
										Organizations: []string{},
									},
								}

								// Set the owner of the conversation.
								rbac_client_, err = getRbacClient()
								if err == nil {
									domain, _ := config.GetDomain()
									token, err := os.ReadFile(config.GetConfigDir() + "/tokens/" + domain + "_token")
									if err == nil {
										err = rbac_client_.SetResourcePermissions(string(token), path, "file", permissions)
										if err != nil {
											fmt.Println("fail to set file owner with error ", err)
										}
									} else {
										fmt.Println("fail to get local token with error: ", err)
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
						restoreVideoInfos(path_)
						createVideoPreview(s_impl, path_, 20, 128, false)
						dir := string(path)[0:strings.LastIndex(string(path), "/")]

						// remove it from the cache.
						cache.RemoveItem(path_)

						client.Publish("reload_dir_event", []byte(dir))
						go func() {
							channel_2 <- path
						}()

					case path := <-channel_2:
						path_ := s_impl.formatPath(path)
						generateVideoPreview(path_, 10, 320, 30, false)
						createVideoTimeLine(path_, 180, .2, false) // 1 frame per 5 seconds.
					}
				}
			}()

			// refresh dir event
			err := client.Subscribe("generate_video_preview_event", Utility.RandomUUID(), func(evt *eventpb.Event) {

				channel_0 <- string(evt.Data)
			})
			if err != nil {
				fmt.Println("Fail to connect to event channel generate_video_preview_event")
			}
		}

	}()

	// Here I will sync the permission to be sure everything is inline...

	// Process video at every day at the given hour...
	s_impl.scheduler.Every(1).Day().At(s_impl.StartVideoConversionHour).Do(processVideos, s_impl)
	if s_impl.AutomaticVideoConversion {
		// Start the scheduler
		fmt.Println("start scheduler!")
		s_impl.scheduler.Start()
	}

	// Now i will be sure that users are owner of every file in their user dir.
	go processAudios(s_impl)
	s_impl.startProcessAudios()

	// use the scheduler instead, this is for development 
	//go processVideos(s_impl)

	// Start the service.
	s_impl.StartService()

}
