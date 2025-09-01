package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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
	"github.com/globulario/services/golang/search/search_client"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/title_client"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// ------------------------------------------------------------
// File Service - server.go
//
// This file wires the File gRPC service into Globular, sets up
// dependencies (RBAC, Event, Search, Media, etc.), configures
// a content cache backend, and exposes service lifecycle hooks.
//
// Notes:
// - All logging uses slog for structured output.
// - Public method signatures remain unchanged by design.
// - Public methods are documented with GoDoc comments.
// ------------------------------------------------------------

// Package-level logger. Other files can use slog.Default(), but we keep an
// explicit instance here to avoid surprises if the default is changed.
var logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

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

// server holds configuration and runtime state for the File service.
// It implements Globular service expectations and the filepb.FileServiceServer.
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
	PublisherID        string
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

// GetConfigurationPath returns the path of the service configuration file.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

// SetConfigurationPath sets the path of the service configuration file.
func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
}

// GetAddress returns the HTTP/HTTPS address where configuration can be found.
func (srv *server) GetAddress() string {
	return srv.Address
}

// SetAddress sets the HTTP/HTTPS address where configuration can be found.
func (srv *server) SetAddress(address string) {
	srv.Address = address
}

// GetProcess returns the current OS process id (PID) of the service.
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the process id (PID). If pid == -1, it closes cache resources.
func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		if cache != nil {
			if err := cache.Close(); err != nil {
				logger.Error("cache close failed", "err", err)
			}
		}
	}
	srv.Process = pid
}

// GetProxyProcess returns the current reverse proxy process id (PID).
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the reverse proxy process id (PID).
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state.
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message captured by the service.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError sets the last error message captured by the service.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the service modification timestamp (unix seconds).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the service modification timestamp (unix seconds).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the unique id of this service instance.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the unique id of this service instance.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetMac returns the MAC address associated with the service host.
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address associated with the service host.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetKeywords returns a list of service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the list of service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns the service repositories.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets the service repositories.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns discovery endpoints used by the service.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets discovery endpoints used by the service.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// Dist packages and distributes the service binary/assets via Globular.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of dependent services.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil { srv.Dependencies = make([]string, 0) }
	return srv.Dependencies
}

// SetDependency appends a dependency if not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the service checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the service checksum.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the target platform identifier.
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the target platform identifier.
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the path of the service executable.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the path of the service executable.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the path to the .proto file for this service.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the path to the .proto file for this service.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC listening port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC listening port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse-proxy (gRPC-Web) port.
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse-proxy (gRPC-Web) port.
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the transport protocol (http/https/tls/grpc).
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the transport protocol (http/https/tls/grpc).
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins returns true if all origins can access the service.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether all origins can access the service.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns a comma-separated list of allowed origins.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the service domain (hostname or FQDN).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the service domain (hostname or FQDN).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns true if TLS is enabled for the service.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS for the service.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA trust bundle path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA trust bundle path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the TLS certificate file path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the TLS certificate file path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the TLS private key file path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the TLS private key file path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher id (org/user).
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher id (org/user).
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns whether the service should auto-update.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate sets whether the service should auto-update.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns whether the service should keep running persistently.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive sets whether the service should keep running persistently.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the service action permissions.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the service action permissions.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// Init creates or loads configuration and initializes the gRPC server.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}

	grpcSrv, err := globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}
	srv.grpcServer = grpcSrv
	logger.Info("grpc server initialized", "service", srv.Name, "port", srv.Port, "proxy", srv.Proxy)
	return nil
}

// Save persists the current configuration values to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the gRPC server and proxy according to the config.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService stops the gRPC server and proxy.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop implements filepb.FileService.Stop to gracefully stop the service.
func (srv *server) Stop(context.Context, *filepb.StopRequest) (*filepb.StopResponse, error) {
	return &filepb.StopResponse{}, srv.StopService()
}

// getEventClient returns the Event service client.
func getEventClient() (*event_client.Event_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		logger.Error("connect event client failed", "err", err)
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// getTitleClient returns the Title service client.
func getTitleClient() (*title_client.Title_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewTitleService_Client", title_client.NewTitleService_Client)
	client, err := globular_client.GetClient(address, "title.TitleService", "NewTitleService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*title_client.Title_Client), nil
}

// getRbacClient returns the RBAC service client.
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

// getAuticationClient returns the Authentication service client.
func getAuticationClient(address string) (*authentication_client.Authentication_Client, error) {
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
	client, err := globular_client.GetClient(address, "authentication.AuthenticationService", "NewAuthenticationService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*authentication_client.Authentication_Client), nil
}

// getMediaClient returns the Media service client.
func getMediaClient() (*media_client.Media_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewMediaService_Client", media_client.NewMediaService_Client)
	client, err := globular_client.GetClient(address, "media.MediaService", "NewMediaService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*media_client.Media_Client), nil
}

// GetSearchClient returns the Search service client.
func (srv *server) GetSearchClient() (*search_client.Search_Client, error) {
	Utility.RegisterFunction("NewSearchService_Client", search_client.NewSearchService_Client)
	client, err := globular_client.GetClient(srv.Address, "search.SearchService", "NewSearchService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*search_client.Search_Client), nil
}

// IndexJsonObject indexes a JSON object into the search service.
func (srv *server) IndexJsonObject(indexationPath, json string, lang string, idField string, textFields []string, data string) error {
	client, err := srv.GetSearchClient()
	if err != nil {
		return err
	}
	return client.IndexJsonObject(indexationPath, json, lang, idField, textFields, data)
}

// removeTempFiles walks the given rootDir and removes files ending with ".temp.mp4".
func removeTempFiles(rootDir string) error {
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if it's a regular file and its name ends with ".temp.mp4"
		if info != nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".temp.mp4") {
			logger.Info("removing temp file", "path", path)
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

// startRemoveTempFiles asynchronously scans common data directories to delete leftover temp files.
func (srv *server) startRemoveTempFiles() {
	go func() {
		dirs := make([]string, 0)
		dirs = append(dirs, config.GetPublicDirs()...)
		dirs = append(dirs, config.GetDataDir()+"/files/users")
		dirs = append(dirs, config.GetDataDir()+"/files/applications")
		for _, dir := range dirs {
			if err := removeTempFiles(dir); err != nil {
				logger.Error("temp file cleanup failed", "dir", dir, "err", err)
			}
		}
		logger.Info("temp file cleanup complete")
	}()
}


// indexFile indexes a file at the specified path based on its MIME type.
// It retrieves file information and chooses the appropriate indexing method:
// - For PDF files ("application/pdf"), it calls indexPdfFile.
// - For text files (MIME type starting with "text"), it calls indexTextFile.
// Returns an error if the file type is not supported for indexing.
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

// main bootstraps configuration, initializes dependencies, and starts the service.
//
// That service is use to give access to SQL (historical comment kept from legacy).
// Port numbers can be overridden by the Globular configuration.
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
	s_impl.PublisherID = "localhost"
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

	// Default permissions (used in conjunction with RBAC resources).
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

	// Give base info to retrieve its configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]
		s_impl.ConfigPath = os.Args[2]
	}

	// Initialize the service & gRPC server.
	if err := s_impl.Init(); err != nil {
		logger.Error("initialization failed", "service", s_impl.Name, "id", s_impl.Id, "err", err)
		os.Exit(1)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Select cache backend.
	switch strings.ToUpper(s_impl.CacheType) {
	case "BADGER":
		cache = storage_store.NewBadger_store()
	case "SCYLLA":
		cache = storage_store.NewScylla_store(s_impl.CacheAddress, "files", s_impl.CacheReplicationFactor)
	case "LEVELDB":
		cache = storage_store.NewLevelDB_store()
	default:
		cache = storage_store.NewBigCache_store() // in-memory
	}

	// Register the file service.
	filepb.RegisterFileServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)
	Utility.CreateDirIfNotExist(s_impl.Root + "/cache")

	if err := cache.Open(`{"path":"` + s_impl.Root + `", "name":"files"}`); err != nil {
		logger.Error("cache open failed", "err", err)
	} else {
		logger.Info("cache opened", "backend", s_impl.CacheType, "root", s_impl.Root)
	}

	// Event-driven indexing pipeline (index_file_event & user-owned paths)
	go func() {
		evtClient, err := getEventClient()
		if err != nil {
			logger.Warn("event client unavailable; indexing events disabled", "err", err)
			return
		}

		channel_0 := make(chan string, 10)
		channel_1 := make(chan string, 10)

		// Dispatcher
		go func() {
			for {
				select {
				case path := <-channel_0:
					if strings.HasPrefix(path, "/users/") {
						values := strings.Split(path, "/")
						if len(values) > 2 {
							owner := values[2]
							rbac, err := getRbacClient()
							if err == nil {
								if err := rbac.AddResourceOwner(path, "file", owner, rbacpb.SubjectType_ACCOUNT); err != nil {
									logger.Error("set file owner failed", "path", path, "owner", owner, "err", err)
								}
							} else {
								logger.Error("get rbac client failed", "err", err)
							}
						}
					}
					channel_1 <- path
				case path := <-channel_1:
					p := s_impl.formatPath(path)
					go func(p string) {
						if err := s_impl.indexFile(p); err != nil {
							logger.Error("index file failed", "path", p, "err", err)
						} else {
							logger.Info("indexed file", "path", p)
						}
					}(p)
				}
			}
		}()

		// Subscribe to indexing events
		if err := evtClient.Subscribe("index_file_event", Utility.RandomUUID(), func(evt *eventpb.Event) {
			channel_1 <- string(evt.Data)
		}); err != nil {
			logger.Error("subscribe to index_file_event failed", "err", err)
		}
	}()

	// Clean temp files on startup (async)
	s_impl.startRemoveTempFiles()

	// Start the service.
	if err := s_impl.StartService(); err != nil {
		logger.Error("service start failed", "err", err)
		os.Exit(1)
	}
}
