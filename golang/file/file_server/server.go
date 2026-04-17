package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/file/file_client"
	"github.com/globulario/services/golang/file/filepb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/media/media_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	resource_client "github.com/globulario/services/golang/resource/resource_client"
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
	Version   = "0.1.0"
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

	// Cluster-wide public dir cache (populated by etcd watcher).
	clusterDirs clusterDirCache
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
// Uses stable action keys as the RBAC permission identifiers.
// Roles are defined in cluster-roles.json (owned by RBAC); this method
// RolesDefault returns an empty set — roles are defined externally in
// cluster-roles.json and per-service policy files.
func (srv *server) RolesDefault() []resourcepb.Role {
	return []resourcepb.Role{}
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

func getResourceClient() (*resource_client.Resource_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	c, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*resource_client.Resource_Client), nil
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

// Indexing by MIME type and file extension.
func (srv *server) indexFile(path string, force bool) error {
	fileInfos, err := getFileInfo(srv, path, -1, -1)
	if err != nil {
		return err
	}

	mime := strings.ToLower(fileInfos.Mime)
	ext := strings.ToLower(filepath.Ext(path))

	// PDF
	if mime == "application/pdf" {
		return srv.indexPdfFile(path, fileInfos, force)
	}

	// Document formats handled by indexing_docs.go (extension-based,
	// because Go's mime package doesn't recognize many of these).
	switch ext {
	case ".docx", ".xlsx", ".odt", ".ods", ".odp",
		".epub", ".rtf", ".csv", ".tsv",
		".md", ".markdown":
		return srv.indexDocumentFile(path, fileInfos, force)
	}

	// HTML (may come as text/html or via extension)
	if mime == "text/html" || ext == ".html" || ext == ".htm" || ext == ".xhtml" {
		return srv.indexDocumentFile(path, fileInfos, force)
	}

	// RTF can also be detected by MIME
	if mime == "application/rtf" || mime == "text/rtf" {
		return srv.indexDocumentFile(path, fileInfos, force)
	}

	// Plain text and any text/* MIME
	if strings.HasPrefix(mime, "text") {
		return srv.indexTextFile(path, fileInfos, force)
	}

	return errors.New("no indexer for file type " + mime + " (" + ext + ")")
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

	// Public storage always uses local filesystem rooted at srv.Root.
	srv.publicStorage = storage_backend.NewOSStorage(srv.Root)

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

	// Non-Minio: user dirs live under srv.Root on the local filesystem.
	srv.storage = storage_backend.NewOSStorage(srv.Root)
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

	opts := &minio.Options{Creds: creds, Secure: cfg.Secure}
	// Cluster DNS dialer for *.globular.internal names.
	transport := &http.Transport{DialContext: config.ClusterDialContext}
	if cfg.Secure {
		tlsCfg, err := buildMinioTLSConfig(cfg)
		if err != nil {
			return fmt.Errorf("build minio TLS config: %w", err)
		}
		if tlsCfg != nil {
			transport.TLSClientConfig = tlsCfg
		}
	}
	opts.Transport = transport

	client, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		return err
	}

	srv.minioClient = client
	return nil
}

// buildMinioTLSConfig returns a tls.Config for the MinIO endpoint.
// If CABundlePath is set, it is loaded for server-cert verification.
// For loopback endpoints with no CA bundle, InsecureSkipVerify is used
// (acceptable because traffic is local-only).
func buildMinioTLSConfig(cfg *config.MinioProxyConfig) (*tls.Config, error) {
	// Loopback endpoints always skip verification — traffic is local-only and
	// after a backup restore the CA may not match MinIO's current cert.
	host, _, _ := net.SplitHostPort(cfg.Endpoint)
	if host == "127.0.0.1" || host == "::1" || host == "localhost" {
		return &tls.Config{InsecureSkipVerify: true}, nil //nolint:gosec // loopback only
	}
	if cfg.CABundlePath != "" {
		caCert, err := os.ReadFile(cfg.CABundlePath)
		if err != nil {
			return nil, fmt.Errorf("read CA bundle %q: %w", cfg.CABundlePath, err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)
		return &tls.Config{RootCAs: pool}, nil
	}
	return nil, nil
}

// minioContractPath is the well-known path where the installer writes the Minio config.
const minioContractPath = "/var/lib/globular/objectstore/minio.json"

// minioCredentialsPath is written by the MinIO package installer (format: "access:secret").
const minioCredentialsPath = "/var/lib/globular/minio/credentials"

// loadMinioConfig reads MinIO configuration from etcd. etcd is the only
// source of truth — no env vars, no disk contracts, no localhost fallbacks.
// The endpoint is a DNS name (minio.<cluster-domain>) served by the cluster
// DNS reconciler.
func (srv *server) loadMinioConfig() *config.MinioProxyConfig {
	cfg, err := config.BuildMinioProxyConfig()
	if err != nil {
		logger.Warn("minio config unavailable", "err", err)
		return nil
	}
	return cfg
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
//
// Routing rules:
//   - Registered public dirs (e.g. /mnt/syno/...): raw OS storage with no root prefix,
//     because these are real filesystem paths that must not be re-rooted under srv.Root.
//   - All other non-user paths (/applications/, /templates/, etc.): publicStorage rooted
//     at srv.Root so virtual paths resolve correctly.
//   - /users/ paths: the configured backend (MinIO or local OS rooted at srv.Root).
func (srv *server) storageForPath(path string) storage_backend.Storage {
	path = srv.formatPath(path)
	if srv.isPublic(path) && !strings.HasPrefix(path, "/public/") {
		// Real OS path registered as a public directory; use it as-is without root translation.
		// Paths starting with /public/ are MinIO-backed and handled below.
		return storage_backend.NewOSStorage("")
	}
	// MinIO-backed paths: /public/, /webroot/, and all non-user content.
	// MinIO is the distributed source of truth for web content.
	if (strings.HasPrefix(path, "/public/") || strings.HasPrefix(path, "/webroot/")) && srv.minioEnabled() {
		if err := srv.ensureMinioClient(); err == nil {
			m, err := storage_backend.NewMinioStorage(srv.minioClient, srv.MinioConfig.Bucket, "")
			if err == nil {
				return m
			}
		}
	}
	if !strings.HasPrefix(path, "/users/") {
		// Non-user paths: prefer MinIO (distributed) over local filesystem.
		// This ensures all nodes serve the same content from shared storage.
		if srv.minioEnabled() {
			if err := srv.ensureMinioClient(); err == nil {
				m, err := storage_backend.NewMinioStorage(srv.minioClient, srv.MinioConfig.Bucket, "")
				if err == nil {
					return m
				}
			}
		}
		// Fallback to local OS only if MinIO is unavailable.
		if srv.publicStorage == nil {
			srv.publicStorage = storage_backend.NewOSStorage(srv.Root)
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
	s.CacheAddress, _ = config.GetAddress()

	// Load permissions from manifest or compiled fallback, and auto-register
	// method→action mappings with the global resolver for interceptor use.
	if extPerms, _ := policy.LoadAndRegisterPermissions("file"); extPerms != nil {
		s.Permissions = extPerms
	} else {
		s.Permissions = make([]any, 0)
		policy.GlobalResolver().RegisterFromInterface(s.Permissions)
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
			s.Id = Utility.GenerateUUID(s.Name + ":" + s.Version + ":" + s.Mac)
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
		s.Id = Utility.GenerateUUID(s.Name + ":" + s.Version + ":" + s.Mac)
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
	// Always override Root after Init() so the stored etcd config can never point
	// user dirs at a stale path (e.g. /var/lib/globular/data/files from an old install).
	s.Root = config.GetDataDir()
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

	// Start cluster-wide public dir watcher and background migration.
	clusterCtx, clusterCancel := context.WithCancel(context.Background())
	defer clusterCancel()
	s.startClusterDirWatcher(clusterCtx)
	go s.migrateLocalUsersToMinio(clusterCtx)

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
				if err := s.indexFile(pp, false); err != nil {
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

		// Create user home dir in storage when a new account is registered.
		subscribeWithRetry("create_account_evt", func(evt *eventpb.Event) {
			if len(evt.Data) == 0 {
				return
			}
			var acc struct {
				Id string `json:"id"`
			}
			if err := json.Unmarshal(evt.Data, &acc); err != nil || acc.Id == "" {
				return
			}
			path := "/users/" + acc.Id
			if err := s.storageMkdirAll(context.Background(), path, 0o755); err != nil {
				logger.Error("create_account_evt: mkdir failed", "path", path, "err", err)
			} else {
				logger.Info("created user home dir", "path", path)
				// Register RBAC ownership so the user can write to their own home dir.
				if rbac, err := getRbacClient(); err == nil {
					if err := rbac.AddResourceOwner(token, path, acc.Id, "file", rbacpb.SubjectType_ACCOUNT); err != nil {
						logger.Error("create_account_evt: set owner failed", "path", path, "owner", acc.Id, "err", err)
					}
				} else {
					logger.Error("create_account_evt: get rbac client failed", "err", err)
				}
			}
		})
	}()

	// Ensure home dirs exist for all accounts already in the system (best-effort, background).
	// Retries with backoff so that a slow Minio startup or missing bucket is recovered automatically.
	go func() {
		ctx := context.Background()

		// Get a local token for RBAC calls (best-effort; log and skip if unavailable).
		bootstrapToken, tokenErr := security.GetLocalToken(s.Mac)

		// Retry /users/sa creation until storage is ready (up to ~5 min).
		const maxAttempts = 30
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			time.Sleep(10 * time.Second)
			if err := s.storageMkdirAll(ctx, "/users/sa", 0o755); err != nil {
				logger.Warn("could not ensure /users/sa dir, will retry",
					"attempt", attempt, "max", maxAttempts, "err", err)
				continue
			}
			logger.Info("ensured /users/sa home dir", "attempt", attempt)
			// Register RBAC ownership for sa so it can write to its home dir.
			if tokenErr == nil {
				if rbac, err := getRbacClient(); err == nil {
					if err := rbac.AddResourceOwner(bootstrapToken, "/users/sa", "sa", "file", rbacpb.SubjectType_ACCOUNT); err != nil {
						logger.Error("bootstrap: set owner for /users/sa failed", "err", err)
					}
				} else {
					logger.Error("bootstrap: get rbac client failed", "err", err)
				}
			}
			break
		}

		// Bootstrap dirs for every registered account.
		rc, err := getResourceClient()
		if err != nil {
			logger.Warn("resource client unavailable for user dir init", "err", err)
			return
		}
		accounts, err := rc.GetAccounts("")
		if err != nil {
			logger.Warn("GetAccounts failed for user dir init", "err", err)
			return
		}
		for _, acc := range accounts {
			if acc.GetId() == "" {
				continue
			}
			path := "/users/" + acc.GetId()
			if err := s.storageMkdirAll(ctx, path, 0o755); err != nil {
				logger.Warn("could not ensure user dir", "path", path, "err", err)
				continue
			}
			// Register RBAC ownership so each user can write to their home dir.
			if tokenErr == nil {
				if rbac, err := getRbacClient(); err == nil {
					if err := rbac.AddResourceOwner(bootstrapToken, path, acc.GetId(), "file", rbacpb.SubjectType_ACCOUNT); err != nil {
						logger.Warn("bootstrap: set owner failed", "path", path, "owner", acc.GetId(), "err", err)
					}
				}
			}
		}
		logger.Info("user home dir init complete", "count", len(accounts))
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
