package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/file/file_client"
	"github.com/globulario/services/golang/file/filepb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/media/media_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/search/search_client"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/storage_backend"
	"github.com/globulario/services/golang/title/title_client"
	Utility "github.com/globulario/utility"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// -------------------- Defaults & globals --------------------

var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""

	// RBAC client (lazily created)
	rbac_client_ *rbac_client.Rbac_Client

	// File metadata cache store
	cache storage_store.Store
)

// STDERR logger (keeps STDOUT clean for --describe/--health)
// Note: Can be reconfigured for debug level via --debug flag in main()
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// -------------------- Service type --------------------

type server struct {
	Id                 string
	Mac                string
	Name               string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	AllowAllOrigins    bool
	AllowedOrigins     string
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
	Permissions        []interface{}
	Dependencies       []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	State              string

	grpcServer *grpc.Server

	Root                   string
	storage                storage_backend.Storage
	publicStorage          storage_backend.Storage
	CacheType              string
	CacheAddress           string
	CacheReplicationFactor int
	Public                 []string

	MinioConfig *config.MinioProxyConfig

	minioClient *minio.Client

	checksumCache sync.Map
}

// -------------------- Globular getters/setters --------------------

func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }
func (srv *server) GetAddress() string               { return srv.Address }
func (srv *server) SetAddress(address string)        { srv.Address = address }
func (srv *server) GetProcess() int                  { return srv.Process }
func (srv *server) SetProcess(pid int) {
	if pid == -1 && cache != nil {
		_ = cache.Close()
	}
	srv.Process = pid
}
func (srv *server) GetProxyProcess() int              { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)           { srv.ProxyProcess = pid }
func (srv *server) GetState() string                  { return srv.State }
func (srv *server) SetState(state string)             { srv.State = state }
func (srv *server) GetLastError() string              { return srv.LastError }
func (srv *server) SetLastError(err string)           { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)          { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                 { return srv.ModTime }
func (srv *server) GetId() string                     { return srv.Id }
func (srv *server) SetId(id string)                   { srv.Id = id }
func (srv *server) GetName() string                   { return srv.Name }
func (srv *server) SetName(name string)               { srv.Name = name }
func (srv *server) GetDescription() string            { return srv.Description }
func (srv *server) SetDescription(description string) { srv.Description = description }
func (srv *server) GetMac() string                    { return srv.Mac }
func (srv *server) SetMac(mac string)                 { srv.Mac = mac }
func (srv *server) GetKeywords() []string             { return srv.Keywords }
func (srv *server) SetKeywords(k []string)            { srv.Keywords = k }
func (srv *server) GetRepositories() []string         { return srv.Repositories }
func (srv *server) SetRepositories(v []string)        { srv.Repositories = v }
func (srv *server) GetDiscoveries() []string          { return srv.Discoveries }
func (srv *server) SetDiscoveries(v []string)         { srv.Discoveries = v }
func (srv *server) Dist(path string) (string, error)  { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if !Utility.Contains(srv.GetDependencies(), dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
	}
}
func (srv *server) GetChecksum() string             { return srv.Checksum }
func (srv *server) SetChecksum(sum string)          { srv.Checksum = sum }
func (srv *server) GetPlatform() string             { return srv.Plaform }
func (srv *server) SetPlatform(platform string)     { srv.Plaform = platform }
func (srv *server) GetPath() string                 { return srv.Path }
func (srv *server) SetPath(path string)             { srv.Path = path }
func (srv *server) GetProto() string                { return srv.Proto }
func (srv *server) SetProto(proto string)           { srv.Proto = proto }
func (srv *server) GetPort() int                    { return srv.Port }
func (srv *server) SetPort(port int)                { srv.Port = port }
func (srv *server) GetProxy() int                   { return srv.Proxy }
func (srv *server) SetProxy(proxy int)              { srv.Proxy = proxy }
func (srv *server) GetProtocol() string             { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)     { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool        { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)       { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string       { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)      { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string               { return srv.Domain }
func (srv *server) SetDomain(domain string)         { srv.Domain = domain }
func (srv *server) GetTls() bool                    { return srv.TLS }
func (srv *server) SetTls(hasTls bool)              { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string   { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string             { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)     { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string              { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)       { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string              { return srv.Version }
func (srv *server) SetVersion(version string)       { srv.Version = version }
func (srv *server) GetPublisherID() string          { return srv.PublisherID }
func (srv *server) SetPublisherID(p string)         { srv.PublisherID = p }
func (srv *server) GetKeepUpToDate() bool           { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)        { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool              { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)           { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})  { srv.Permissions = p }
func (srv *server) SetPublicDirs(dirs []string) {
	srv.Public = dirs
}

// -------------------- Lifecycle --------------------

func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	logger.Info("grpc server initialized", "service", srv.Name, "port", srv.Port, "proxy", srv.Proxy)
	return nil
}
func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }
func (srv *server) Stop(context.Context, *filepb.StopRequest) (*filepb.StopResponse, error) {
	return &filepb.StopResponse{}, srv.StopService()
}

// RolesDefault returns a curated set of roles for FileService.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:file.viewer",
			Name:        "File Viewer",
			Domain:      domain,
			Description: "Read-only access to files and directories.",
			Actions: []string{
				"/file.FileService/ReadDir",
				"/file.FileService/GetFileInfo",
				"/file.FileService/GetFileMetadata",
				"/file.FileService/ReadFile",
				"/file.FileService/GetThumbnails",
				"/file.FileService/GetPublicDirs",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:file.uploader",
			Name:        "File Uploader",
			Domain:      domain,
			Description: "Upload/create content; no destructive ops.",
			Actions: []string{
				"/file.FileService/UploadFile",
				"/file.FileService/SaveFile",
				"/file.FileService/CreateDir",
				"/file.FileService/CreateLnk",
				"/file.FileService/WriteExcelFile",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:file.editor",
			Name:        "File Editor",
			Domain:      domain,
			Description: "Create, modify, move/copy, and delete files/dirs.",
			Actions: []string{
				"/file.FileService/CreateDir",
				"/file.FileService/DeleteDir",
				"/file.FileService/Rename",
				"/file.FileService/Move",
				"/file.FileService/Copy",
				"/file.FileService/CreateArchive",
				"/file.FileService/SaveFile",
				"/file.FileService/DeleteFile",
				"/file.FileService/UploadFile",
				"/file.FileService/CreateLnk",
				"/file.FileService/WriteExcelFile",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:file.publisher",
			Name:        "File Publisher",
			Domain:      domain,
			Description: "Manage public directories (publish/unpublish).",
			Actions: []string{
				"/file.FileService/GetPublicDirs",
				"/file.FileService/AddPublicDir",
				"/file.FileService/RemovePublicDir",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:file.admin",
			Name:        "File Admin",
			Domain:      domain,
			Description: "Full control over FileService.",
			Actions: []string{
				"/file.FileService/Stop",
				"/file.FileService/GetPublicDirs",
				"/file.FileService/AddPublicDir",
				"/file.FileService/RemovePublicDir",
				"/file.FileService/ReadDir",
				"/file.FileService/CreateDir",
				"/file.FileService/DeleteDir",
				"/file.FileService/Rename",
				"/file.FileService/Move",
				"/file.FileService/Copy",
				"/file.FileService/CreateArchive",
				"/file.FileService/GetFileInfo",
				"/file.FileService/GetFileMetadata",
				"/file.FileService/ReadFile",
				"/file.FileService/SaveFile",
				"/file.FileService/DeleteFile",
				"/file.FileService/GetThumbnails",
				"/file.FileService/UploadFile",
				"/file.FileService/WriteExcelFile",
				"/file.FileService/HtmlToPdf",
			},
			TypeName: "resource.Role",
		},
	}
}

// -------------------- Clients & helpers --------------------

func getEventClient() (*event_client.Event_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		logger.Error("connect event client failed", "err", err)
		return nil, err
	}
	return c.(*event_client.Event_Client), nil
}
func getTitleClient() (*title_client.Title_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewTitleService_Client", title_client.NewTitleService_Client)
	c, err := globular_client.GetClient(address, "title.TitleService", "NewTitleService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*title_client.Title_Client), nil
}
func getRbacClient() (*rbac_client.Rbac_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	c, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*rbac_client.Rbac_Client), nil
}
func (srv *server) setOwner(token, path string) error {
	var clientId string
	if len(token) > 0 {
		claims, err := security.ValidateToken(token)
		if err != nil {
			return err
		}
		clientId = claims.ID
	} else {
		return errors.New("no token was given")
	}
	rbac, err := getRbacClient()
	if err != nil {
		return err
	}
	if strings.Contains(path, "/files/users/") {
		path = path[strings.Index(path, "/users/"):]
	}
	return rbac.AddResourceOwner(token, path, clientId, "file", rbacpb.SubjectType_ACCOUNT)
}

func getMediaClient() (*media_client.Media_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewMediaService_Client", media_client.NewMediaService_Client)
	c, err := globular_client.GetClient(address, "media.MediaService", "NewMediaService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*media_client.Media_Client), nil
}
func (srv *server) GetSearchClient() (*search_client.Search_Client, error) {
	Utility.RegisterFunction("NewSearchService_Client", search_client.NewSearchService_Client)
	c, err := globular_client.GetClient(srv.Address, "search.SearchService", "NewSearchService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*search_client.Search_Client), nil
}
func (srv *server) IndexJsonObject(indexationPath, jsonStr string, lang string, idField string, textFields []string, data string) error {
	c, err := srv.GetSearchClient()
	if err != nil {
		return err
	}
	return c.IndexJsonObject(indexationPath, jsonStr, lang, idField, textFields, data)
}

// Temp file cleanup
func removeTempFiles(rootDir string) error {
	return filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".temp.mp4") {
			logger.Info("removing temp file", "path", path)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("remove %s: %v", path, err)
			}
		}
		return nil
	})
}
func (srv *server) startRemoveTempFiles() {
	go func() {
		dirs := append([]string{}, config.GetPublicDirs()...)
		dirs = append(dirs, config.GetDataDir()+"/users", config.GetDataDir()+"/applications")
		for _, d := range dirs {
			if err := removeTempFiles(d); err != nil {
				logger.Error("temp file cleanup failed", "dir", d, "err", err)
			}
		}
		logger.Info("temp file cleanup complete")
	}()
}

// Indexing by MIME
func (srv *server) indexFile(path string) error {
	fileInfos, err := getFileInfo(srv, path, -1, -1)
	if err != nil {
		return err
	}
	if fileInfos.Mime == "application/pdf" {
		return srv.indexPdfFile(path, fileInfos)
	} else if strings.HasPrefix(fileInfos.Mime, "text") {
		return srv.indexTextFile(path, fileInfos)
	}
	return errors.New("no indexer for file type " + fileInfos.Mime)
}

// -------------------- CLI UX --------------------

// printUsage prints comprehensive command-line usage information.
func printUsage() {
	fmt.Println("Globular File Service")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  file-service [OPTIONS] [<id> [configPath]]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("POSITIONAL ARGUMENTS:")
	fmt.Println("  id          Service instance ID (optional, auto-generated if not provided)")
	fmt.Println("  configPath  Path to service configuration file (optional)")
	fmt.Println()
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("  GLOBULAR_DOMAIN       Override service domain")
	fmt.Println("  GLOBULAR_ADDRESS      Override service address")
	fmt.Println("  MINIO_ENDPOINT        MinIO/S3 endpoint (e.g., localhost:9000)")
	fmt.Println("  MINIO_BUCKET          MinIO bucket name (default: globular)")
	fmt.Println("  MINIO_PREFIX          MinIO key prefix (default: /users)")
	fmt.Println("  MINIO_USE_SSL         Enable SSL for MinIO (true/false)")
	fmt.Println("  MINIO_ACCESS_KEY      MinIO access key")
	fmt.Println("  MINIO_SECRET_KEY      MinIO secret key")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with auto-generated ID and default config")
	fmt.Println("  file-service")
	fmt.Println()
	fmt.Println("  # Start with specific service ID")
	fmt.Println("  file-service my-file-service-id")
	fmt.Println()
	fmt.Println("  # Enable debug logging")
	fmt.Println("  file-service --debug")
	fmt.Println()
	fmt.Println("  # Print service metadata")
	fmt.Println("  file-service --describe")
	fmt.Println()
	fmt.Println("  # Check service health")
	fmt.Println("  file-service --health")
	fmt.Println()
}

// printVersion prints version information as JSON.
func printVersion() {
	info := map[string]string{
		"service":    "file",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}

// initStorage selects the default and public storage implementations based on config.
func (srv *server) initStorage() error {

	// Public storage always uses local filesystem
	srv.publicStorage = storage_backend.NewOSStorage("")

	if srv.minioEnabled() {
		if err := srv.ensureMinioClient(); err != nil {
			return err
		}
		m, err := storage_backend.NewMinioStorage(
			srv.minioClient,
			srv.MinioConfig.Bucket,
			srv.MinioConfig.Prefix,
		)
		if err != nil {
			return err
		}
		srv.storage = m
		logger.Info("minio storage initialized",
			"endpoint", srv.MinioConfig.Endpoint,
			"bucket", srv.MinioConfig.Bucket)
		return nil
	}

	srv.storage = storage_backend.NewOSStorage("")
	return nil
}

func (srv *server) minioEnabled() bool {
	return srv.MinioConfig != nil && srv.MinioConfig.Endpoint != "" && srv.MinioConfig.Bucket != ""
}

func (srv *server) ensureMinioClient() error {
	if !srv.minioEnabled() {
		return fmt.Errorf("minio is not enabled")
	}
	if srv.minioClient != nil {
		return nil
	}

	cfg := srv.MinioConfig
	auth := cfg.Auth
	if auth == nil {
		auth = &config.MinioProxyAuth{Mode: config.MinioProxyAuthModeNone}
	}

	var creds *credentials.Credentials
	switch auth.Mode {
	case config.MinioProxyAuthModeAccessKey:
		creds = credentials.NewStaticV4(auth.AccessKey, auth.SecretKey, "")
	case config.MinioProxyAuthModeFile:
		data, err := os.ReadFile(auth.CredFile)
		if err != nil {
			return fmt.Errorf("read minio credentials file: %w", err)
		}
		parts := strings.Split(strings.TrimSpace(string(data)), ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid minio credentials file format")
		}
		creds = credentials.NewStaticV4(parts[0], parts[1], "")
	case config.MinioProxyAuthModeNone:
		creds = credentials.NewStaticV4("", "", "")
	default:
		return fmt.Errorf("unknown minio auth mode: %s", auth.Mode)
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: cfg.Secure,
	})
	if err != nil {
		return err
	}

	srv.minioClient = client
	return nil
}

// loadMinioConfig reads MinIO configuration from the service config or environment.
func (srv *server) loadMinioConfig() *config.MinioProxyConfig {
	if cfg, err := config.GetServiceConfigurationById(srv.Id); err == nil && cfg != nil {
		if minioRaw, ok := cfg["MinioConfig"]; ok {
			if minioMap, ok := minioRaw.(map[string]interface{}); ok {
				return parseMinioConfigFromMap(minioMap)
			}
		}
	}

	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		return nil
	}

	return &config.MinioProxyConfig{
		Endpoint: endpoint,
		Bucket:   getEnvOrDefault("MINIO_BUCKET", "globular"),
		Prefix:   getEnvOrDefault("MINIO_PREFIX", "/users"),
		Secure:   getEnvOrDefault("MINIO_USE_SSL", "false") == "true",
		Auth: &config.MinioProxyAuth{
			Mode:      config.MinioProxyAuthModeAccessKey,
			AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
			SecretKey: os.Getenv("MINIO_SECRET_KEY"),
		},
	}
}

func parseMinioConfigFromMap(m map[string]interface{}) *config.MinioProxyConfig {
	cfg := &config.MinioProxyConfig{}

	if v, ok := m["endpoint"].(string); ok {
		cfg.Endpoint = v
	}
	if v, ok := m["bucket"].(string); ok {
		cfg.Bucket = v
	}
	if v, ok := m["prefix"].(string); ok {
		cfg.Prefix = v
	}
	if v, ok := m["secure"].(bool); ok {
		cfg.Secure = v
	}
	if v, ok := m["caBundlePath"].(string); ok {
		cfg.CABundlePath = v
	}

	if authRaw, ok := m["auth"].(map[string]interface{}); ok {
		cfg.Auth = &config.MinioProxyAuth{}
		if mode, ok := authRaw["mode"].(string); ok {
			cfg.Auth.Mode = mode
		}
		if ak, ok := authRaw["accessKey"].(string); ok {
			cfg.Auth.AccessKey = ak
		}
		if sk, ok := authRaw["secretKey"].(string); ok {
			cfg.Auth.SecretKey = sk
		}
		if cf, ok := authRaw["credFile"].(string); ok {
			cfg.Auth.CredFile = cf
		}
	}

	return cfg
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// Storage returns the configured backend (defaulting to local filesystem).
func (srv *server) Storage() storage_backend.Storage {
	if srv.storage == nil {
		if err := srv.initStorage(); err != nil {
			logger.Error("failed to initialize storage; falling back to local filesystem", "err", err)
			srv.storage = storage_backend.NewOSStorage("")
		}
	}
	return srv.storage
}

// storageForPath returns the appropriate storage implementation for a specific path.
// Public directories are always served from the local filesystem; everything else uses the default backend.
func (srv *server) storageForPath(path string) storage_backend.Storage {
	path = srv.formatPath(path)
	if srv.isPublic(path) || !strings.HasPrefix(path, "/users/") {
		if srv.publicStorage == nil {
			srv.publicStorage = storage_backend.NewOSStorage("")
		}
		return srv.publicStorage
	}
	return srv.Storage()
}

// pathExists reports whether a path exists within the selected backend.
func (srv *server) pathExists(ctx context.Context, path string) bool {
	if _, err := srv.storageForPath(path).Stat(ctx, path); err == nil {
		return true
	}
	return false
}

// -------------------- main --------------------

func main() {
	// Define CLI flags (BEFORE any arg parsing)
	var (
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
	)

	flag.Usage = printUsage
	flag.Parse()

	// Handle --debug flag (reconfigure logger level)
	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		logger.Debug("debug logging enabled")
	}

	// Handle informational flags that exit early
	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	// Initialize service skeleton (no etcd/config yet)
	s := new(server)
	s.Name = string(filepb.File_file_proto.Services().Get(0).FullName())
	s.Proto = filepb.File_file_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = Version // Use build-time version
	s.PublisherID = "localhost"
	s.Description = "File service providing filesystem and object storage"
	s.Keywords = []string{"File", "FS", "Storage", "MinIO", "S3"}
	s.Repositories = []string{}
	s.Discoveries = []string{}
	s.Dependencies = []string{
		"rbac.RbacService",
		"event.EventService",
		"authentication.AuthenticationService",
	}
	s.Public = []string{}
	s.CacheReplicationFactor = 1
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.Root = config.GetDataDir()
	s.CacheAddress, _ = config.GetAddress()

	s.Permissions = []interface{}{
		// ---- Directory listing
		map[string]interface{}{
			"action":     "/file.FileService/ReadDir",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "read"},
			},
		},

		// ---- Create / Delete directory
		map[string]interface{}{
			"action":     "/file.FileService/CreateDir",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"}, // parent dir
			},
		},
		map[string]interface{}{
			"action":     "/file.FileService/DeleteDir",
			"permission": "delete",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "delete"},
			},
		},

		// ---- Rename (inside a directory)
		map[string]interface{}{
			"action":     "/file.FileService/Rename",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "OldName", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "NewName", "permission": "write"},
			},
		},

		// ---- Copy (read sources, write destination)
		map[string]interface{}{
			"action":     "/file.FileService/Copy",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Files", "permission": "read"}, // files[]
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"}, // destination dir
			},
		},

		// ---- Move (delete sources, write destination)
		map[string]interface{}{
			"action":     "/file.FileService/Move",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Files", "permission": "delete"}, // files[]
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},   // destination dir
			},
		},

		// ---- Create archive (read sources; server writes into caller's area)
		map[string]interface{}{
			"action":     "/file.FileService/CreateArchive",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Paths", "permission": "read"}, // paths[]
				// NOTE: destination is implicit (user home) in server impl; no request field to reference.
			},
		},

		// ---- File info & metadata
		map[string]interface{}{
			"action":     "/file.FileService/GetFileInfo",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/file.FileService/GetFileMetadata",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "read"},
			},
		},

		// ---- Read / Save / Delete file
		map[string]interface{}{
			"action":     "/file.FileService/ReadFile",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/file.FileService/SaveFile",
			"permission": "write",
			"resources": []interface{}{
				// SaveFile is client-streaming; enforce when a message contains Path in the oneof.
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/file.FileService/DeleteFile",
			"permission": "delete",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "delete"},
			},
		},

		// ---- Link (.lnk) creation
		map[string]interface{}{
			"action":     "/file.FileService/CreateLnk",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"}, // directory where link is created
				map[string]interface{}{"index": 0, "field": "Name", "permission": "write"},
				// "Lnk" is payload metadata; no FS permission required.
			},
		},

		// ---- Thumbnails & transforms
		map[string]interface{}{
			"action":     "/file.FileService/GetThumbnails",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/file.FileService/WriteExcelFile",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/file.FileService/HtmlToPdf",
			"permission": "read",
			"resources":  []interface{}{}, // no FS resource in request
		},

		// ---- Remote ingest (download to dest)
		map[string]interface{}{
			"action":     "/file.FileService/UploadFile",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Dest", "permission": "write"},
				// "Url" is external; validate-only (no FS permission).
			},
		},

		// ---- Public directory management
		map[string]interface{}{
			"action":     "/file.FileService/AddPublicDir",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/file.FileService/RemovePublicDir",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Path", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/file.FileService/GetPublicDirs",
			"permission": "read",
			"resources":  []interface{}{}, // config read; no path param
		},

		// ---- Control plane
		map[string]interface{}{
			"action":     "/file.FileService/Stop",
			"permission": "admin",
			"resources":  []interface{}{},
		},
	}

	// Dynamic client registration
	Utility.RegisterFunction("NewFileService_Client", file_client.NewFileService_Client)

	// Handle --describe flag (requires minimal service setup, no config access)
	if *showDescribe {
		s.Process = os.Getpid()
		s.State = "starting"
		if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
			s.Domain = strings.ToLower(v)
		} else {
			s.Domain = "localhost"
		}
		if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
			s.Address = strings.ToLower(v)
		} else {
			s.Address = "localhost:" + Utility.ToString(s.Port)
		}
		if s.Id == "" {
			s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
		}
		b, err := globular.DescribeJSON(s)
		if err != nil {
			logger.Error("describe error", "service", s.Name, "id", s.Id, "err", err)
			os.Exit(2)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		os.Exit(0)
	}

	// Handle --health flag (requires minimal service setup, no config access)
	if *showHealth {
		if s.Port == 0 || s.Name == "" {
			logger.Error("health error: uninitialized", "service", s.Name, "port", s.Port)
			os.Exit(2)
		}
		b, err := globular.HealthJSON(s, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
		if err != nil {
			logger.Error("health error", "service", s.Name, "id", s.Id, "err", err)
			os.Exit(2)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		os.Exit(0)
	}

	// Parse positional arguments: [<id> [configPath]]
	args := flag.Args()
	if len(args) == 0 {
		// No args: auto-generate ID and allocate port
		s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			logger.Error("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(s.Id)
		if err != nil {
			logger.Error("fail to allocate port", "error", err)
			os.Exit(1)
		}
		s.Port = p
		logger.Debug("auto-allocated service", "id", s.Id, "port", s.Port)
	} else if len(args) == 1 {
		// One arg: service ID
		s.Id = args[0]
		logger.Debug("using provided service id", "id", s.Id)
	} else if len(args) >= 2 {
		// Two+ args: service ID and config path
		s.Id = args[0]
		s.ConfigPath = args[1]
		logger.Debug("using provided service id and config", "id", s.Id, "config", s.ConfigPath)
	}

	// Load configuration (safe to touch config now)
	logger.Debug("loading service configuration")
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
		logger.Debug("loaded domain from config", "domain", d)
	} else {
		s.Domain = "localhost"
		logger.Debug("using default domain", "domain", "localhost")
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
		logger.Debug("loaded address from config", "address", a)
	}

	// Initialize service
	logger.Info("initializing file service", "id", s.Id, "domain", s.Domain)
	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("initialization failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
	logger.Debug("service initialized", "duration_ms", time.Since(start).Milliseconds())
	s.MinioConfig = s.loadMinioConfig()
	if s.MinioConfig != nil {
		logger.Info("minio storage configured",
			"endpoint", s.MinioConfig.Endpoint,
			"bucket", s.MinioConfig.Bucket,
			"secure", s.MinioConfig.Secure)
	}
	if err := s.initStorage(); err != nil {
		logger.Error("storage initialization failed", "err", err)
		os.Exit(1)
	}
	if s.Address == "" {
		if addr, _ := config.GetAddress(); addr != "" {
			s.Address = addr
		}
	}

	// Select cache backend
	logger.Debug("selecting cache backend", "type", s.CacheType)
	switch strings.ToUpper(s.CacheType) {
	case "BADGER":
		cache = storage_store.NewBadger_store()
		logger.Info("using badger cache backend")
	case "SCYLLA":
		cache = storage_store.NewScylla_store(s.CacheAddress, "files", s.CacheReplicationFactor)
		logger.Info("using scylla cache backend", "address", s.CacheAddress, "replication", s.CacheReplicationFactor)
	case "LEVELDB":
		cache = storage_store.NewLevelDB_store()
		logger.Info("using leveldb cache backend")
	default:
		cache = storage_store.NewBigCache_store() // in-memory
		logger.Info("using bigcache backend (in-memory)")
	}

	// Register gRPC
	filepb.RegisterFileServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	Utility.CreateDirIfNotExist(s.Root + "/cache")
	if err := cache.Open(`{"path":"` + s.Root + `", "name":"files"}`); err != nil {
		logger.Error("cache open failed", "err", err)
	} else {
		logger.Info("cache opened", "backend", s.CacheType, "root", s.Root)
	}

	// Event-driven indexing pipeline (robust subscribe; force channel bootstrap; rotate UUID)
	go func() {
		evtClient, err := getEventClient()
		if err != nil {
			logger.Warn("event client unavailable; indexing events disabled", "err", err)
			return
		}

		channel0 := make(chan string, 64) // owner-set stage
		channel1 := make(chan string, 64) // index stage
		token, err := security.GetLocalToken(s.Mac)
		if err != nil {
			logger.Error("failed to get local token", "err", err)
			return
		}

		// Stage 1: set owner, then enqueue for indexing
		go func() {
			for path := range channel0 {
				if strings.HasPrefix(path, "/users/") {
					parts := strings.Split(path, "/")
					if len(parts) > 2 {
						owner := parts[2] // user@domain
						if rbac, err := getRbacClient(); err == nil {
							if err := rbac.AddResourceOwner(token, path, owner, "file", rbacpb.SubjectType_ACCOUNT); err != nil {
								logger.Error("set file owner failed", "path", path, "owner", owner, "err", err)
							}
						} else {
							logger.Error("get rbac client failed", "err", err)
						}
					}
				}
				select {
				case channel1 <- path:
				default:
					logger.Warn("index queue full; dropping", "path", path)
				}
			}
		}()

		// Stage 2: indexer
		go func() {
			for path := range channel1 {
				pp := path
				if err := s.indexFile(pp); err != nil {
					logger.Error("index file failed", "path", pp, "err", err)
				} else {
					logger.Info("indexed file", "path", pp)
				}
			}
		}()

		// Helper: subscribe with exponential backoff; ensure channel exists; rotate UUID each attempt
		subscribeWithRetry := func(ch string, cb func(*eventpb.Event)) {
			backoff := 1 * time.Second
			for {
				// Best-effort channel bootstrap (no-op if already exists)
				if err := evtClient.Publish(ch, []byte("__bootstrap__")); err != nil {
					logger.Debug("bootstrap publish failed (will still try subscribe)", "channel", ch, "err", err)
				}
				// Rotate consumer UUID to avoid colliding with stale server-side state
				uuid := fmt.Sprintf("%s:%s", s.GetId(), Utility.RandomUUID())

				if err := evtClient.Subscribe(ch, uuid, cb); err != nil {
					logger.Warn("subscribe failed; will retry", "channel", ch, "uuid", uuid, "err", err, "backoff", backoff)
					time.Sleep(backoff)
					if backoff < 30*time.Second {
						backoff *= 2
					}
					continue
				}
				logger.Info("subscribed to channel", "channel", ch)
				return
			}
		}

		// Subscribe; ignore non-path bootstrap/noise
		subscribeWithRetry("index_file_event", func(evt *eventpb.Event) {
			path := string(evt.Data)
			if len(path) == 0 || path[0] != '/' {
				return // ignore bootstrap or unexpected payloads
			}
			select {
			case channel0 <- path:
			default:
				logger.Warn("owner-set queue full; dropping", "path", path)
			}
		})
	}()

	// Start cleanup pass for temporary files
	logger.Debug("starting temp file cleanup background task")
	s.startRemoveTempFiles()

	// Service ready - log comprehensive startup info
	logger.Info("file service ready",
		"id", s.Id,
		"version", s.Version,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"address", s.Address,
		"root", s.Root,
		"startup_ms", time.Since(start).Milliseconds())

	// Start gRPC server
	logger.Info("starting grpc server", "port", s.Port)
	if err := s.StartService(); err != nil {
		logger.Error("service start failed", "err", err)
		os.Exit(1)
	}
}
