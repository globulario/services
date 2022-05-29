package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	wkhtml "github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/davecourtois/Utility"
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
	"github.com/globulario/services/golang/title/title_client"
	"github.com/nfnt/resize"
	"github.com/polds/imgbase64"
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

	// Public contain a list of paths reachable by the file server.
	Public []string
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

/**
 * Create a thumbnail...
 */
func createThumbnail(path string, file *os.File, thumbnailMaxHeight int, thumbnailMaxWidth int) string {

	// Here if thumbnail already exist in hiden files I will use it...
	_fileName := strings.ReplaceAll(file.Name(), "\\", "/")
	endIndex := strings.LastIndex(_fileName, ".")
	if endIndex == -1 {
		endIndex = len(_fileName)
	}
	_fileName = _fileName[strings.LastIndex(_fileName, "/")+1 : endIndex]

	_path := path + "/.hidden/" + _fileName + "/_thumbnail_"
	if Utility.Exists(path + "/.hidden/" + _fileName) {
		thumbnail, err := ioutil.ReadFile(_path)
		if err == nil {
			return string(thumbnail)
		}
	}

	Utility.CreateDirIfNotExist(path + "/.hidden/" + _fileName)

	// Set the buffer pointer back to the begening of the file...
	file.Seek(0, 0)
	var originalImg image.Image
	var err error

	if strings.HasSuffix(file.Name(), ".png") || strings.HasSuffix(file.Name(), ".PNG") {
		originalImg, err = png.Decode(file)
	} else if strings.HasSuffix(file.Name(), ".jpeg") || strings.HasSuffix(file.Name(), ".jpg") || strings.HasSuffix(file.Name(), ".JPEG") || strings.HasSuffix(file.Name(), ".JPG") {
		originalImg, err = jpeg.Decode(file)
	} else if strings.HasSuffix(file.Name(), ".gif") || strings.HasSuffix(file.Name(), ".GIF") {
		originalImg, err = gif.Decode(file)
	} else {
		return ""
	}

	if err != nil {
		return ""
	}

	// I will get the ratio for the new image size to respect the scale.
	hRatio := thumbnailMaxHeight / originalImg.Bounds().Size().Y
	wRatio := thumbnailMaxWidth / originalImg.Bounds().Size().X

	var h int
	var w int

	// First I will try with the height
	if hRatio*originalImg.Bounds().Size().Y < thumbnailMaxWidth {
		h = thumbnailMaxHeight
		w = hRatio * originalImg.Bounds().Size().Y
	} else {
		// So here i will use it width
		h = wRatio * thumbnailMaxHeight
		w = thumbnailMaxWidth
	}

	// do not zoom...
	if hRatio > 1 {
		h = originalImg.Bounds().Size().Y
	}

	if wRatio > 1 {
		w = originalImg.Bounds().Size().X
	}

	// Now I will calculate the image size...
	img := resize.Resize(uint(h), uint(w), originalImg, resize.Lanczos3)

	var buf bytes.Buffer
	if strings.HasSuffix(file.Name(), ".png") || strings.HasSuffix(file.Name(), ".PNG") {
		err = png.Encode(&buf, img)
	} else {
		err = jpeg.Encode(&buf, img, &jpeg.Options{jpeg.DefaultQuality})
	}

	if err != nil {
		return ""
	}

	// Now I will save the buffer containt to the thumbnail...
	thumbnail := imgbase64.FromBuffer(buf)
	file.Seek(0, 0) // Set the reader back to the begenin of the file...

	// Save the thumbnail into a file to not having to recreate-it each time...
	ioutil.WriteFile(_path, []byte(thumbnail), 0644)

	return thumbnail
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

func getFileInfo(s *server, path string) (*fileInfo, error) {

	fileStat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	info := new(fileInfo)

	info.IsDir = fileStat.IsDir()
	info.Size = fileStat.Size()
	info.Name = fileStat.Name()
	info.ModTime = fileStat.ModTime()
	info.Path = path

	// Cut the root part of the path if it start with the root path.
	if len(s.Root) > 0 {
		startindex := strings.Index(info.Path, s.Root)
		if startindex == 0 {
			info.Path = info.Path[len(s.Root):]
			info.Path = strings.Replace(info.Path, "\\", "/", -1) // Set the slash instead of back slash.
		}
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

/**
 * Read the directory and return the file info.
 */
func readDir(s *server, path string, recursive bool, thumbnailMaxWidth int32, thumbnailMaxHeight int32, readFiles bool) (*fileInfo, error) {

	// get the file info
	info, err := getFileInfo(s, path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir {
		return nil, errors.New(path + " is a directory")
	}

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
				info_, err := readDir(s, dirPath, recursive, thumbnailMaxWidth, thumbnailMaxHeight, true)
				if err != nil {
					return nil, err
				}
				info.Files = append(info.Files, info_)
			} else {
				info_, err := readDir(s, dirPath, recursive, thumbnailMaxWidth, thumbnailMaxHeight, false)
				if err != nil {
					return nil, err
				}
				if isHls {
					info_.Mime = "video/hls-stream"
				}

				info.Files = append(info.Files, info_)
			}

		} else if readFiles {

			info_, err := getFileInfo(s, path+"/"+f.Name())
			if err != nil {
				return nil, err
			}

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
			if !strings.Contains(path, ".hidden") {
				if strings.HasPrefix(info_.Mime, "image/") {
					if thumbnailMaxHeight > 0 && thumbnailMaxWidth > 0 {
						info_.Thumbnail = createThumbnail(path, f_, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
					}
				} else if strings.HasPrefix(info_.Mime, "video/") {
					path, err := os.Getwd()
					if err == nil {
						path = path + "/mimetypes/video-x-generic.png"
						icon, err := os.Open(path)
						if err == nil {
							info_.Thumbnail = createThumbnail(path, icon, 80, 80)
							icon.Close()
						} else {
							fmt.Println(err)
						}
					}

				} else if strings.Contains(info_.Mime, "/") {

					// In that case I will get read image from png file and create a
					// thumbnail with it...
					path, err := os.Getwd()
					if err == nil {
						path = path + "/mimetypes/" + strings.ReplaceAll(strings.Split(info_.Mime, ";")[0], "/", "-") + ".png"
						icon, err := os.Open(path)
						if err == nil {
							info_.Thumbnail = createThumbnail(path, icon, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
							icon.Close()
						}
					}
				} else {
					path, err := os.Getwd()
					if err == nil {
						path = path + "/mimetypes/unknown.png"
						icon, err := os.Open(path)
						if err == nil {
							info_.Thumbnail = createThumbnail(path, icon, 80, 80)
							icon.Close()
						} else {
							fmt.Println(err)
						}
					}
				}
			}

			info.Files = append(info.Files, info_)
		}

	}
	return info, err
}

func (file_server *server) formatPath(path string) string {

	path = strings.ReplaceAll(path, "\\", "//")
	if strings.HasPrefix(path, "/") {
		if len(path) > 1 {
			if strings.HasPrefix(path, "/") {
				if !file_server.isPublic(path) {
					// Must be in the root path if it's not in public path.
					if !strings.HasPrefix(path, file_server.Root) {
						path = file_server.Root + path
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

////////////////////////////////////////////////////////////////////////////////
// Directory operations
////////////////////////////////////////////////////////////////////////////////
func (file_server *server) ReadDir(rqst *filepb.ReadDirRequest, stream filepb.FileService_ReadDirServer) error {

	path := file_server.formatPath(rqst.GetPath())

	info, err := readDir(file_server, path, rqst.GetRecursive(), rqst.GetThumnailWidth(), rqst.GetThumnailHeight(), true)
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

	file_server.createPermission(ctx, rqst.GetPath()+"/"+rqst.GetName())
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
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			user = claims.Id
		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no token was given")))
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
	dest := "/users/" + user + "/" + rqst.GetName() + ".tgz"

	// Set user as owner.
	file_server.createPermission(ctx, dest)

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

func (file_server *server) createPermission(ctx context.Context, path string) error {
	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return err
			}
			clientId = claims.Id
		} else {
			errors.New("no token was given")
		}
	}

	// Now I will set it in the rbac as ressource owner...
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

	err = rbac_client_.SetResourcePermissions(path, permissions)

	if err != nil {
		return err
	}

	return nil
}

// Rename a file or a directory.
func (file_server *server) Rename(ctx context.Context, rqst *filepb.RenameRequest) (*filepb.RenameResponse, error) {
	path := file_server.formatPath(rqst.GetPath())
	err := os.Rename(path+"/"+rqst.OldName, path+"/"+rqst.NewName)
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

	// Remove the permission for the previous path.
	rbac_client_.DeleteResourcePermissions(rqst.GetPath() + "/" + rqst.GetOldName())
	file_server.createPermission(ctx, rqst.GetPath()+"/"+rqst.GetNewName())

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
	err := os.RemoveAll(path)

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

	rbac_client_.DeleteResourcePermissions(rqst.GetPath())

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

	info, err := getFileInfo(file_server, path)
	if err != nil {
		return nil, err
	}

	// the file
	f_, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f_.Close()

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	info.Mime, err = Utility.GetFileContentType(f_)
	thumbnailMaxHeight := rqst.GetThumnailHeight()
	thumbnailMaxWidth := rqst.GetThumnailWidth()

	// in case of image...
	if strings.HasPrefix(info.Mime, "image/") {
		if thumbnailMaxHeight > 0 && thumbnailMaxWidth > 0 {
			info.Thumbnail = createThumbnail(path, f_, int(thumbnailMaxHeight), int(thumbnailMaxWidth))
		} else {
			info.Thumbnail = createThumbnail(path, f_, 80, 80)
		}
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

func (server *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := getRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
}

func generateVideoPreviewListener(evt *eventpb.Event) {
	path := string(evt.Data)

	if !Utility.Exists(path) {
		if Utility.Exists(config.GetDataDir() + "/files" + path) {
			path = config.GetDataDir() + "/files" + path
		}
	}

	createVideoPreview(path, 20, 128)
	go func() {
		generateVideoGifPreview(path, 10, 320, 30)
		createVideoTimeLine(path, 180, .2) // 1 frame per 5 seconds.
	}()

	client, err := getEventClient()
	if err == nil {
		dir := []byte(string(evt.Data)[0:strings.LastIndex(string(evt.Data), "/")])
		client.Publish("reload_dir_event", dir)
	}
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

	// Register the echo services
	filepb.RegisterFileServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Convert video file, set permissions...
	go func() {
		processVideos() // Process files...
	}()

	// Now the event client service.
	go func() {
		client, err := getEventClient()
		if err == nil {

			// refresh dir event
			client.Subscribe("generate_video_preview_event", Utility.RandomUUID(), generateVideoPreviewListener)
		} else {
		}

	}()

	// Start the service.
	s_impl.StartService()

}

// Move a file/directory
func (file_server *server) Move(ctx context.Context, rqst *filepb.MoveRequest) (*filepb.MoveResponse, error) {

	// So here I will call the function mv at repetition for each path...
	for i := 0; i < len(rqst.Files); i++ {
		path := file_server.Root + rqst.Files[i]
		if Utility.Exists(path) {
			err := Utility.Move(path, file_server.Root+rqst.Path)
			if err != nil {
				fmt.Println(err)
			}

			// If hidden folder exist for it...
			path_ := path[0:strings.LastIndex(path, "/")]
			fileName := path[strings.LastIndex(path, "/")+1:]
			if strings.Contains(fileName, ".") {
				fileName = fileName[0:strings.LastIndex(fileName, ".")]
			}
			hiddenFolder := path_ + "/.hidden/" + fileName

			if Utility.Exists(hiddenFolder) {
				Utility.CreateDirIfNotExist(file_server.Root + rqst.Path + "/.hidden")
				err := Utility.Move(hiddenFolder, file_server.Root+rqst.Path+"/.hidden")
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}

	return &filepb.MoveResponse{Result: true}, nil
}

// Copy a file/directory
func (file_server *server) Copy(ctx context.Context, rqst *filepb.CopyRequest) (*filepb.CopyResponse, error) {
	// So here I will call the function mv at repetition for each path...
	for i := 0; i < len(rqst.Files); i++ {
		f := file_server.Root + rqst.Files[i]
		if Utility.Exists(f) {
			info, err := os.Stat(f)
			if err == nil {
				if info.IsDir() {
					// Copy the directory
					Utility.CopyDir(f, file_server.Root+rqst.Path)
				} else {
					// Copy the file
					Utility.CopyFile(f, file_server.Root+rqst.Path)

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
		} else {
			fmt.Println("file " + f + " dosen't exist!")
		}
	}

	return &filepb.CopyResponse{Result: true}, nil
}

////////////////////////////////////////////////////////////////////////////////
// Utility functions
////////////////////////////////////////////////////////////////////////////////

// Return the list of thumbnail for a given directory...
func (file_server *server) GetThumbnails(rqst *filepb.GetThumbnailsRequest, stream filepb.FileService_GetThumbnailsServer) error {
	path := rqst.GetPath()

	// The root will be the Root specefied by the server.
	if strings.HasPrefix(path, "/") {
		path = file_server.Root + path
		// Set the path separator...
		path = strings.Replace(path, "\\", "/", -1)
	}

	info, err := readDir(file_server, path, rqst.GetRecursive(), rqst.GetThumnailHeight(), rqst.GetThumnailWidth(), true)
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

///////////////////////////////////////////////////////////////////////////////////////////////////////////
// ffmpeg and video conversion stuff...
///////////////////////////////////////////////////////////////////////////////////////////////////////////
func processVideos() {

	videos := getVideoPaths()

	for _, video := range videos {
		// Create preview and timeline...
		createVideoPreview(video, 20, 128)
		generateVideoGifPreview(video, 10, 320, 30)
		createVideoTimeLine(video, 180, .2) // 1 frame per 5 seconds.
	}

	// Step 2 Convert .mp4 to stream...
	for _, video := range videos {

		// all video mp4 must
		if !strings.HasSuffix(video, ".m3u8") {
			dir := video[0:strings.LastIndex(video, ".")]
			if !Utility.Exists(dir+"/playlist.m3u8") && Utility.Exists(video) {
				var err error
				if strings.HasSuffix(video, ".mkv") || strings.HasPrefix(video, ".MKV") {
					video, err = createVideoMpeg4H264(video)
					if err != nil {
						fmt.Println("fail to convert mkv to mp4 with error: ", err)
					}
				}

				// Convert to stream...
				if err == nil {
					createHlsStreamFromMpeg4H264(video)
				}

			} else {
				os.Remove(video)
			}
		}
	}

	// sleep a minute...
	time.Sleep(1 * time.Minute) // once hours I will refresh all the file.
	processVideos()
}

// Recursively convert all video that are not in the correct
// format.
func getVideoPaths() []string {

	// Here I will use at most one concurrent ffmeg...
	medias := make([]string, 0)
	dirs := make([]string, 0)
	dirs = append(dirs, config.GetPublicDirs()...)
	dirs = append(dirs, config.GetDataDir()+"/files")

	for _, dir := range dirs {
		filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {

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

				path_ := strings.ToLower(path)
				if !strings.Contains(path, ".hidden") && strings.HasSuffix(path_, "playlist.m3u8") || strings.HasSuffix(path_, ".mp4") || strings.HasSuffix(path_, ".mkv") || strings.HasSuffix(path_, ".avi") || strings.HasSuffix(path_, ".mov") || strings.HasSuffix(path_, ".wmv") {
					medias = append(medias, path)
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
		if strings.Contains(err.Error(), "moov atom not found"){
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
	path = strings.ReplaceAll(path, "\\", "/")
	path_ := path[0:strings.LastIndex(path, "/")]
	name_ := path[strings.LastIndex(path, "/"):strings.LastIndex(path, ".")]
	output := path_ + "/" + name_ + ".mp4"

	if Utility.Exists(output) {
		return output, nil
	}

	// Test if cuda is available.
	getVersion := exec.Command("ffmpeg", "-version")
	version, _ := getVersion.CombinedOutput()

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
	if strings.Index(string(version), "--enable-cuda-nvcc") > -1 {
		if strings.HasPrefix(encoding, "H.264") || strings.HasPrefix(encoding, "MPEG-4 part 2") {
			cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "h264_nvenc", "-c:a", "aac", output)
		} else if strings.HasPrefix(encoding, "H.265") || strings.HasPrefix(encoding, "Motion JPEG") {
			// in future when all browser will support H.265 I will compile it with this line instead.
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
			// cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "libx265", "-c:a", "aac", output)
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
		return "", err
	}
	// Here I will remove the input file...
	os.Remove(path)

	return output, nil
}

func associatePath(path string) error {
	path = strings.ReplaceAll(path, "\\", "/")
	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return err
	}

	// Now I will asscociate the title.
	output_path := path[0:strings.LastIndex(path, ".")]

	titles, err := client.GetFileTitles(config.GetDataDir()+"/search/titles", strings.ReplaceAll(path, config.GetDataDir()+"/files", ""))
	if err == nil {
		// Here I will asscociate the path
		for _, title := range titles {
			client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, strings.ReplaceAll(output_path, config.GetDataDir()+"/files", ""))
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, strings.ReplaceAll(path, config.GetDataDir()+"/files", ""))
		}
	}

	// Look for videos
	videos, err := client.GetFileVideos(config.GetDataDir()+"/search/videos", strings.ReplaceAll(path, config.GetDataDir()+"/files", ""))
	if err == nil {
		// Here I will asscociate the path
		for _, video := range videos {
			client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, strings.ReplaceAll(output_path, config.GetDataDir()+"/files", ""))
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, strings.ReplaceAll(path, config.GetDataDir()+"/files", ""))
		}
	}

	return nil
}

// Create the streams...
// segment_target_duration  	try to create a new segment every X seconds
// max_bitrate_ratio 			maximum accepted bitrate fluctuations
// rate_monitor_buffer_ratio	maximum buffer size between bitrate conformance checks
func createHlsStream(src, dest string, segment_target_duration int, max_bitrate_ratio, rate_monitor_buffer_ratio float32) error {
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

	getVersion := exec.Command("ffmpeg", "-version")
	version, _ := getVersion.CombinedOutput()

	//  https://docs.nvidia.com/video-technologies/video-codec-sdk/ffmpeg-with-nvidia-gpu/
	if strings.Index(string(version), "--enable-cuda-nvcc") > -1 {
		if strings.HasPrefix(encoding, "H.264") || strings.HasPrefix(encoding, "MPEG-4 part 2") {
			args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "h264_nvenc", "-c:a", "aac"}
		} else if strings.HasPrefix(encoding, "H.265") || strings.HasPrefix(encoding, "Motion JPEG") {
			// in future when all browser will support H.265 I will compile it with this line instead.
			//cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "hevc_nvenc",  "-c:a", "aac", output)
			args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "h264_nvenc", "-c:a", "aac", "-pix_fmt", "yuv420p"}

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
			// cmd = exec.Command("ffmpeg", "-i", path, "-c:v", "libx265", "-c:a", "aac", output)
			args = []string{"-hide_banner", "-y", "-i", src, "-c:v", "libx264", "-c:a", "aac", "-pix_fmt", "yuv420p"}
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
	for i:=0; i < len(args); i++ {
		cmd_str_ += " " + args[i]
	}

	fmt.Println(cmd_str_)

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
		// Try to associate the path...
		associatePath(path)

		// remove the original file.
		os.Remove(path)
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
func generateVideoGifPreview(path string, fps, scale, duration int) error {
	path = strings.ReplaceAll(path, "\\", "/")
	duration_total := getVideoDuration(path)
	if duration == 0 {
		return errors.New("the video lenght is 0 sec")
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
	if Utility.Exists(output + "/preview.gif") {
		//os.Remove(output + "/preview.gif")
		return nil
	}

	Utility.CreateDirIfNotExist(output)
	cmd := exec.Command("ffmpeg", "-ss", Utility.ToString(duration_total*.1), "-t", Utility.ToString(duration), "-i", path, "-vf", "fps="+Utility.ToString(fps)+",scale="+Utility.ToString(scale)+":-1:flags=lanczos,split[s0][s1];[s0]palettegen[p];[s1][p]paletteuse", `-loop`, `0`, `preview.gif`)
	cmd.Dir = output // the output directory...
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func createVttFile(output string, fps float32) error{
	// Now I will generate the WEBVTT file with the infos...
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
			webvtt += localConfig["Protocol"].(string) + "://" + address + "/"+  strings.ReplaceAll(output, config.GetDataDir()+"/files", "") + "/" + thumbnail.Name() + "\n\n"	
			index++
		}
		
	}

	// delete previous file...
	os.Remove(output+"/thumbnails.vtt")

	// Now  I will write the file...
	return os.WriteFile(output+"/thumbnails.vtt", []byte(webvtt), 777)
}

// Here I will create the small viedeo video
func createVideoTimeLine(path string, width int, fps float32) error {
	path = strings.ReplaceAll(path, "\\", "/")
	// One frame at each 5 seconds...
	if fps == 0 {
		fps = 0.2
	}

	if width == 0 {
		width = 180 // px
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
		return createVttFile(output, fps)
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
		return err
	}

	return createVttFile(output, fps)
}

// Here I will create the small viedeo video
func createVideoPreview(path string, nb int, height int) error {
	path = strings.ReplaceAll(path, "\\", "/")
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
		return nil
	}

	Utility.CreateDirIfNotExist(output)

	duration := getVideoDuration(path)
	if duration == 0 {
		return errors.New("the video lenght is 0 sec")
	}

	// ffmpeg -i bob_ross_img-0-Animated.mp4 -ss 15 -t 16 -f image2 preview_%05d.jpg
	start := .1 * duration
	laps := 120 // 1 minutes

	cmd := exec.Command("ffmpeg", "-i", path, "-ss", Utility.ToString(start), "-t", Utility.ToString(laps), "-vf", "scale="+Utility.ToString(height)+":-1,fps=.250", "preview_%05d.jpg")
	cmd.Dir = output // the output directory...

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return err
	}

	path_ = strings.ReplaceAll(path, config.GetDataDir()+"/files", "")
	path_ = path_[0:strings.LastIndex(path_, "/")]

	return nil
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

func getVideoDuration(path string) float64 {
	path = strings.ReplaceAll(path, "\\", "/")
	// original command...
	// ffprobe -v quiet -print_format compact=print_section=0:nokey=1:escape=csv -show_entries format=duration bob_ross_img-0-Animated.mp4
	cmd := exec.Command("ffprobe", `-v`, `quiet`, `-print_format`, `compact=print_section=0:nokey=1:escape=csv`, `-show_entries`, `format=duration`, path)

	cmd.Dir = os.TempDir()

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

// Create a viedeo Preview...
func (file_server *server) CreateVideoPreview(ctx context.Context, rqst *filepb.CreateVideoPreviewRequest) (*filepb.CreateVideoPreviewResponse, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	err := createVideoPreview(rqst.Path, int(rqst.Nb), int(rqst.Height))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.CreateVideoPreviewResponse{}, nil

}

// Create video time line
func (file_server *server) CreateVideoTimeLine(ctx context.Context, rqst *filepb.CreateVideoTimeLineRequest) (*filepb.CreateVideoTimeLineResponse, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	err := createVideoTimeLine(rqst.Path, int(rqst.Width), rqst.Fps)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.CreateVideoTimeLineResponse{}, nil
}

// Convert a file from mkv, avi or other format to MPEG-4 AVC
func (file_server *server) ConvertVideoToMpeg4H264(ctx context.Context, rqst *filepb.ConvertVideoToMpeg4H264Request) (*filepb.ConvertVideoToMpeg4H264Response, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	_, err := createVideoMpeg4H264(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.ConvertVideoToMpeg4H264Response{}, nil
}

// Convert a video file (must be  MPEG-4 H264) to HLS stream... That will automatically generate the
// the streams for various resolutions. (see script create-vod-hls.sh for more info)
func (file_server *server) ConvertVideoToHls(ctx context.Context, rqst *filepb.ConvertVideoToHlsRequest) (*filepb.ConvertVideoToHlsResponse, error) {
	if !Utility.Exists(rqst.Path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	// in case of a mkv Need conversion before...
	if strings.HasSuffix(rqst.Path, ".mkv") || strings.HasPrefix(rqst.Path, ".MKV") {
		var err error
		rqst.Path, err = createVideoMpeg4H264(rqst.Path)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Create the hls stream from MPEG-4 H264 file.
	err := createHlsStreamFromMpeg4H264(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &filepb.ConvertVideoToHlsResponse{}, nil
}
