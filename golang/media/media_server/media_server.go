package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	"github.com/globulario/services/golang/media/mediapb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/title_client"
	Utility "github.com/globulario/utility"
	"github.com/jasonlvhit/gocron"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"google.golang.org/grpc/reflection"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// Here I will keep files info in cache...
	cache storage_store.Store
)

const (
	MAX_FFMPEG_INSTANCE = 3
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Mac             string
	Name            string
	Domain          string
	Address         string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Version         string
	PublisherID     string
	KeepUpToDate    bool
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64

	TLS bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The grpc server.
	grpcServer *grpc.Server

	// The root path of the file server.
	Root string

	// Define the backend to use as cache it can be scylla, badger or leveldb the default is bigcache a memory cache.
	CacheType string

	// Define the cache address in case is not local.
	CacheAddress string

	// the number of replication for the cache.
	CacheReplicationFactor int

	// If set to true the gpu will be used to convert video.
	HasEnableGPU bool

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

	// If true ffmeg will use information to convert the video.
	AutomaticVideoConversion bool

	// If true video will be convert to stream
	AutomaticStreamConversion bool

	// The conversion will start at that given hour...
	StartVideoConversionHour string

	// Maximum conversion time. Conversion will not continue over this delay.
	MaximumVideoConversionDelay string
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
func (srv *server) GetPublisherID() string {
	return srv.PublisherID
}
func (srv *server) SetPublisherID(PublisherID string) {
	srv.PublisherID = PublisherID
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

	// Get the configuration path.
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

func (srv *server) Stop(context.Context, *mediapb.StopRequest) (*mediapb.StopResponse, error) {
	return &mediapb.StopResponse{}, srv.StopService()
}

// Return true if the file is found in the public path...
func (srv *server) isPublic(path string) bool {
	path = strings.ReplaceAll(path, "\\", "/")
	publics, err := srv.getPublicDirs()
	if err != nil {
		return false
	}

	if Utility.Exists(path) {
		for i := 0; i < len(publics); i++ {
			if strings.HasPrefix(path, publics[i]) {
				return true
			}
		}
	}
	return false
}

func (srv *server) formatPath(path string) string {
	path, _ = url.PathUnescape(path)
	path = strings.ReplaceAll(path, "\\", "/")

	if strings.HasPrefix(path, "/") {
		if len(path) > 1 {
			if strings.HasPrefix(path, "/") {
				if !srv.isPublic(path) {
					// Must be in the root path if it's not in public path.
					if strings.HasPrefix(path, "/users/") || strings.HasPrefix(path, "/applications/") {
						path = config.GetDataDir() + "/files" + path
					} else if Utility.Exists(config.GetWebRootDir() + path) {
						path = config.GetWebRootDir() + path
					} else if Utility.Exists(srv.Root + path) {
						path = srv.Root + path
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

func getAuticationClient(address string) (*authentication_client.Authentication_Client, error) {
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
	client, err := globular_client.GetClient(address, "authentication.AuthenticationService", "NewAuthenticationService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*authentication_client.Authentication_Client), nil
}

/**
 * Get the file service client.
 */

func (srv *server) getFileClient() (*file_client.File_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewFileService_Client", file_client.NewFileService_Client)

	client, err := globular_client.GetClient(address, "file.FileService", "NewFileService_Client")
	if err != nil {
		fmt.Println("fail to connect to file client with error: ", err)
		return nil, err
	}

	return client.(*file_client.File_Client), nil
}

func (srv *server) getPublicDirs() ([]string, error) {
	client, err := srv.getFileClient()
	if err != nil {
		return nil, err
	}

	// Get the public dir.
	public, err := client.GetPublicDirs()
	if err != nil {
		return nil, err
	}

	return public, nil
}

func (srv *server) getFileInfo(token, path string) (*filepb.FileInfo, error) {
	// Try to get the file info from the cache.
	data_, err := cache.GetItem(path)
	if err == nil {
		fileInfo := new(filepb.FileInfo)
		err = protojson.Unmarshal(data_, fileInfo)
		if err == nil {
			return fileInfo, nil
		}
	}

	// Get the file client.
	file_client, err := srv.getFileClient()
	if err != nil {
		return nil, err
	}

	// Get the file info.
	fileInfo, err := file_client.GetFileInfo(token, path, false, -1, -1)
	if err != nil {
		return nil, err
	}

	data_, err = protojson.Marshal(fileInfo)
	if err == nil {
		cache.SetItem(path, data_)
	}

	return fileInfo, nil
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

func (srv *server) publishReloadDirEvent(path string) {
	client, err := getEventClient()
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.ReplaceAll(path, config.GetDataDir()+"/files", "")
	if err == nil {
		client.Publish("reload_dir_event", []byte(path))
	}
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

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "media_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(mediapb.File_media_proto.Services().Get(0).FullName())
	s_impl.Proto = mediapb.File_media_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherID = "localhost"
	s_impl.Description = "The Hello world of gRPC service!"
	s_impl.Keywords = []string{"Example", "media", "Test", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 1)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

	// Set the root path if is pass as argument.
	s_impl.Root = config.GetDataDir() + "/files"

	// set it to true in order to enable GPU acceleration.
	s_impl.HasEnableGPU = false

	// Video conversion retalted configuration.
	s_impl.scheduler = gocron.NewScheduler()
	s_impl.videoConversionErrors = new(sync.Map)
	s_impl.videoConversionLogs = new(sync.Map)
	s_impl.AutomaticStreamConversion = false
	s_impl.AutomaticVideoConversion = false
	s_impl.MaximumVideoConversionDelay = "00:00" // convert for 8 hours...
	s_impl.StartVideoConversionHour = "00:00"    // start conversion at midnight, when every one sleep

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
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

	if len(s_impl.MaximumVideoConversionDelay) == 0 {
		s_impl.StartVideoConversionHour = "00:00"

	}

	if len(s_impl.StartVideoConversionHour) == 0 {
		s_impl.StartVideoConversionHour = "00:00"
	}

	// Set the permission for the service.
	s_impl.Permissions[0] = map[string]interface{}{"action": "/file.FileService/UploadVideo", "resources": []interface{}{map[string]interface{}{"index": 1, "permission": "write"}}}

	// Register the client function, so it can be use by the service.
	Utility.RegisterFunction("NewMediaService_Client", media_client.NewMediaService_Client)

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s with error: %s", s_impl.Name, s_impl.Id, err.Error())
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Register the media services
	mediapb.RegisterMediaServiceServer(s_impl.grpcServer, s_impl)
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
			title_client, err := getTitleClient()
			if err == nil {
				// Here I will subscribe to the event channel.
				channel_1 := make(chan string)
				channel_2 := make(chan string)

				// Process request received...
				go func() {
					for {
						select {
						case path := <-channel_1:
							path_ := s_impl.formatPath(path)
							token, _ := security.GetLocalToken(s_impl.Mac)
							restoreVideoInfos(title_client, token, path_, s_impl.Domain)

							s_impl.createVideoPreview(path_, 20, 128, false)
							dir := string(path)[0:strings.LastIndex(string(path), "/")]
							dir = strings.ReplaceAll(dir, config.GetDataDir()+"/files", "")

							// remove it from the cache.
							cache.RemoveItem(path_)

							// force client to reload their informations.
							event_client.Publish("reload_dir_event", []byte(dir))
							go func() {
								channel_2 <- path
							}()

						case path := <-channel_2:
							path_ := s_impl.formatPath(path)
							s_impl.generateVideoPreview(path_, 10, 320, 30, false)
							s_impl.createVideoTimeLine(path_, 180, .2, false) // 1 frame per 5 seconds.

						}
					}
				}()

				// generate preview event
				err := event_client.Subscribe("generate_video_preview_event", Utility.RandomUUID(), func(evt *eventpb.Event) {
					channel_1 <- string(evt.Data)
				})

				if err != nil {
					fmt.Println("Fail to connect to event channel generate_video_preview_event")
				}

				// subscribe to cancel_upload_event event...
				event_client.Subscribe("cancel_upload_event", s_impl.GetId(), cancelUploadVideoHandeler(s_impl, title_client))

				if err != nil {
					fmt.Println("Fail to connect to event channel index_file_event")
				}

			}
		}

	}()

	// Here I will sync the permission to be sure everything is inline...

	// Process video at every day at the given hour...
	s_impl.scheduler.Every(1).Day().At(s_impl.StartVideoConversionHour).Do(processVideos, s_impl)
	if s_impl.AutomaticVideoConversion {
		// Start the scheduler
		s_impl.scheduler.Start()
	}

	// Now i will be sure that users are owner of every file in their user dir.
	s_impl.startProcessAudios()

	fmt.Printf("Service %s is ready to listen on port %d\n", s_impl.Name, s_impl.Port)
	// Start the service.
	s_impl.StartService()

}
