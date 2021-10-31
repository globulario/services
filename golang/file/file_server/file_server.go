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
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	wkhtml "github.com/SebastiaanKlippert/go-wkhtmltopdf"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/file/file_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/file/filepb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/rbac/rbacpb"
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

	rbac_client_  *rbac_client.Rbac_Client
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

	// The grpc server.
	grpcServer *grpc.Server

	// Specific to file server.
	Root string
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

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", file_server)
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
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", file_server)
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
	_fileName = _fileName[strings.LastIndex(_fileName, "/")+1 : strings.LastIndex(_fileName, ".")]
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

	// Cut the Root part of the part.
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
			if recursive || f.Name() == ".hidden" || strings.Contains(path, ".hidden") {
				info_, err := readDir(s, path+string(os.PathSeparator)+f.Name(), recursive, thumbnailMaxWidth, thumbnailMaxHeight, true)
				if err != nil {
					return nil, err
				}
				info.Files = append(info.Files, info_)
			} else {
				info_, err := readDir(s, path+string(os.PathSeparator)+f.Name(), recursive, thumbnailMaxWidth, thumbnailMaxHeight, false)
				if err != nil {
					return nil, err
				}
				info.Files = append(info.Files, info_)
			}
		} else if readFiles {

			info_, err := getFileInfo(s, path+string(os.PathSeparator)+f.Name())
			if err != nil {
				return nil, err
			}

			f_, err := os.Open(path + string(os.PathSeparator) + f.Name())
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
				path = file_server.Root + path
			} else if !strings.HasSuffix(path, "/") {
				path = file_server.Root + path
			} else {
				path = file_server.Root + "/" + path
			}
		} else {
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
		return err
	}

	// Here I will serialyse the data into JSON.
	jsonStr, err := json.Marshal(info)

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
		err = stream.Send(&filepb.ReadDirResponse{
			Data: data,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// Create a new directory
func (file_server *server) CreateDir(ctx context.Context, rqst *filepb.CreateDirRequest) (*filepb.CreateDirResponse, error) {
	path := file_server.formatPath(rqst.GetPath())
	err := Utility.CreateDirIfNotExist(path + string(os.PathSeparator) + rqst.GetName())
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

// Create an archive from a given dir and set it with name.
func (file_server *server) CreateAchive(ctx context.Context, rqst *filepb.CreateArchiveRequest) (*filepb.CreateArchiveResponse, error) {

	var user string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			user, _, _, _, _, err = security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no token was given")))
		}
	}

	// Here I will create the directory...
	tmp := os.TempDir() + "/" + rqst.GetName()
	createTempDir := true

	if len(rqst.Paths) == 1 {
		info, _ := os.Stat(file_server.Root + rqst.Paths[0])
		if info.IsDir() {
			tmp = file_server.Root + rqst.Paths[0]
			createTempDir = false
		}
	}

	// This will create a temporary directory...
	if createTempDir {
		Utility.CreateDirIfNotExist(tmp)
		defer os.RemoveAll(tmp)

		//defer os.Remove(tmp)
		for i := 0; i < len(rqst.Paths); i++ {
			// The file or directory must be in the path.
			if Utility.Exists(file_server.Root + rqst.Paths[i]) {
				info, _ := os.Stat(file_server.Root + rqst.Paths[i])
				fileName := rqst.Paths[i][strings.LastIndex(rqst.Paths[i], "/"):]
				if info.IsDir() {
					Utility.CopyDir(file_server.Root+rqst.Paths[i], tmp+"/"+fileName)
				} else {
					Utility.CopyFile(file_server.Root+rqst.Paths[i], tmp+"/"+fileName)
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
			clientId, _, _, _, _, err = security.ValidateToken(token)
			if err != nil {
				return err
			}
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
	rbac_client_, err = file_server.GetRbacClient()
	if err != nil {
		return err
	}

	err = rbac_client_.SetResourcePermissions(path, permissions)

	fmt.Println("Set permission to ", path, clientId)

	if err != nil {
		return err
	}

	return nil
}

// Rename a file or a directory.
func (file_server *server) Rename(ctx context.Context, rqst *filepb.RenameRequest) (*filepb.RenameResponse, error) {
	path := file_server.formatPath(rqst.GetPath())
	err := os.Rename(path+string(os.PathSeparator)+rqst.OldName, path+string(os.PathSeparator)+rqst.NewName)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rbac_client_, err = file_server.GetRbacClient()
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
	}else{
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
	}else{
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

	rbac_client_, err = file_server.GetRbacClient()
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
		}else{
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
		return err
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
				return err
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
				stream.SendAndClose(&filepb.SaveFileResponse{
					Result: true,
				})

				return nil
			} else {
				return err
			}
		}

		// Receive message informations.
		switch msg := rqst.File.(type) {
		case *filepb.SaveFileRequest_Path:
			// The roo will be the Root specefied by the server.
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

	rbac_client_, err = file_server.GetRbacClient()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}


	// I will remove the permission from the db.
	rbac_client_.DeleteResourcePermissions(rqst.GetPath())

	// Also delete informations from .hidden
	path_ := path[0:strings.LastIndex(path, "/")]
	fileName := path[strings.LastIndex(path, "/")+1 : strings.LastIndex(path, ".")]

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


func (server *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	var err error
	if rbac_client_ == nil {
		rbac_client_, err = rbac_client.NewRbacService_Client(server.Domain, "rbac.RbacService")
		if err != nil {
			return nil, err
		}

	}
	return rbac_client_, nil
}

func (server *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := server.GetRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
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

	// Set the permissions
	s_impl.setActionResourcesPermissions(s_impl.Permissions[0].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[1].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[2].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[3].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[4].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[5].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[6].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[7].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[8].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[9].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[10].(map[string]interface{}))
	s_impl.setActionResourcesPermissions(s_impl.Permissions[11].(map[string]interface{}))
	
	// Set the root path if is pass as argument.
	if len(s_impl.Root) == 0 {
		s_impl.Root = os.TempDir()
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the echo services
	filepb.RegisterFileServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)


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
			fileName := path[strings.LastIndex(path, "/")+1 : strings.LastIndex(path, ".")]
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
					fileName := f[strings.LastIndex(f, "/")+1 : strings.LastIndex(f, ".")]
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

	// The roo will be the Root specefied by the server.
	if strings.HasPrefix(path, "/") {
		path = file_server.Root + path
		// Set the path separator...
		path = strings.Replace(path, "/", string(os.PathSeparator), -1)
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
		path = strings.Replace(path, "/", string(os.PathSeparator), -1)
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
