// Package main implements the Search gRPC service for Globular.
// This refactor standardizes structure (like the Echo template),
// adds --describe / --health, switches to slog, clarifies errors,
// and preserves all public method prototypes.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/search/search_engine"
	"github.com/globulario/services/golang/search/searchpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------
var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	// Allow all origins by default.
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// -----------------------------------------------------------------------------
// Service type (matches Globular contract) + Search engine field
// -----------------------------------------------------------------------------
type server struct {
	// Core metadata
	Id           string
	Mac          string
	Name         string
	Domain       string
	Address      string
	Path         string
	Proto        string
	Port         int
	Proxy        int
	Protocol     string
	Version      string
	PublisherID  string
	Description  string
	Keywords     []string
	Repositories []string
	Discoveries  []string

	// Policy / ops
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Permissions     []interface{}
	Dependencies    []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64

	// TLS
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// Runtime
	grpcServer *grpc.Server

	// Search engine implementation
	search_engine search_engine.SearchEngine
}

// GetKeywords implements globular_service.Service.
func (srv *server) GetKeywords() []string {
	return srv.Keywords
}

// SetKeywords implements globular_service.Service.
func (srv *server) SetKeywords(keywords []string) {
	srv.Keywords = keywords
}

// -----------------------------------------------------------------------------
// Globular getters/setters (unchanged prototypes)
// -----------------------------------------------------------------------------
func (srv *server) GetConfigurationPath() string          { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)      { srv.ConfigPath = path }
func (srv *server) GetAddress() string                    { return srv.Address }
func (srv *server) SetAddress(address string)             { srv.Address = address }
func (srv *server) GetProcess() int                       { return srv.Process }
func (srv *server) SetProcess(pid int)                    { srv.Process = pid }
func (srv *server) GetProxyProcess() int                  { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)               { srv.ProxyProcess = pid }
func (srv *server) GetState() string                      { return srv.State }
func (srv *server) SetState(state string)                 { srv.State = state }
func (srv *server) GetLastError() string                  { return srv.LastError }
func (srv *server) SetLastError(err string)               { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)              { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                     { return srv.ModTime }
func (srv *server) GetId() string                         { return srv.Id }
func (srv *server) SetId(id string)                       { srv.Id = id }
func (srv *server) GetName() string                       { return srv.Name }
func (srv *server) SetName(name string)                   { srv.Name = name }
func (srv *server) GetMac() string                        { return srv.Mac }
func (srv *server) SetMac(mac string)                     { srv.Mac = mac }
func (srv *server) GetDescription() string                { return srv.Description }
func (srv *server) SetDescription(description string)     { srv.Description = description }
func (srv *server) GetRepositories() []string             { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string              { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)   { srv.Discoveries = discoveries }
func (srv *server) Dist(path string) (string, error)      { return globular.Dist(path, srv) }
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
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}
func (srv *server) GetChecksum() string             { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)     { srv.Checksum = checksum }
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
func (srv *server) SetPublisherID(id string)        { srv.PublisherID = id }
func (srv *server) GetKeepUpToDate() bool           { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)        { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool              { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)           { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})  { srv.Permissions = p }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:search.viewer",
			Name:        "Search Viewer",
			Domain:      domain,
			Description: "Read-only access to search indexes and engine info.",
			Actions: []string{
				"/search.SearchService/GetEngineVersion",
				"/search.SearchService/Count",
				"/search.SearchService/SearchDocuments",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:search.indexer",
			Name:        "Search Indexer",
			Domain:      domain,
			Description: "Can index and delete documents, plus all viewer capabilities.",
			Actions: []string{
				// Viewer
				"/search.SearchService/GetEngineVersion",
				"/search.SearchService/Count",
				"/search.SearchService/SearchDocuments",
				// Write
				"/search.SearchService/IndexJsonObject",
				"/search.SearchService/DeleteDocument",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:search.admin",
			Name:        "Search Admin",
			Domain:      domain,
			Description: "Full control over SearchService, including stopping the service.",
			Actions: []string{
				"/search.SearchService/Stop",
				"/search.SearchService/GetEngineVersion",
				"/search.SearchService/IndexJsonObject",
				"/search.SearchService/Count",
				"/search.SearchService/DeleteDocument",
				"/search.SearchService/SearchDocuments",
			},
			TypeName: "resource.Role",
		},
	}
}


// Init initializes configuration, gRPC server, and the search engine.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	// init the search engine (Bleve based)
	srv.search_engine = search_engine.NewBleveSearchEngine()
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService begins serving gRPC (and proxy if configured).
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the running gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// -----------------------------------------------------------------------------
// Search API (public prototypes preserved)
// -----------------------------------------------------------------------------

// Stop stops the service via gRPC.
func (srv *server) Stop(context.Context, *searchpb.StopRequest) (*searchpb.StopResponse, error) {
	return &searchpb.StopResponse{}, srv.StopService()
}

// GetEngineVersion returns the underlying engine version.
func (srv *server) GetEngineVersion(ctx context.Context, rqst *searchpb.GetEngineVersionRequest) (*searchpb.GetEngineVersionResponse, error) {
	return &searchpb.GetEngineVersionResponse{Message: srv.search_engine.GetVersion()}, nil
}

// DeleteDocument removes a document from the index.
func (srv *server) DeleteDocument(ctx context.Context, rqst *searchpb.DeleteDocumentRequest) (*searchpb.DeleteDocumentResponse, error) {
	if err := srv.search_engine.DeleteDocument(rqst.Path, rqst.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &searchpb.DeleteDocumentResponse{}, nil
}

// Count returns the number of documents in a database.
func (srv *server) Count(ctx context.Context, rqst *searchpb.CountRequest) (*searchpb.CountResponse, error) {
	return &searchpb.CountResponse{Result: srv.search_engine.Count(rqst.Path)}, nil
}

// SearchDocuments performs a search and streams back the results.
func (srv *server) SearchDocuments(rqst *searchpb.SearchDocumentsRequest, stream searchpb.SearchService_SearchDocumentsServer) error {
	results, err := srv.search_engine.SearchDocuments(rqst.Paths, rqst.Language, rqst.Fields, rqst.Query, rqst.Offset, rqst.PageSize, rqst.SnippetLength)
	if err != nil {
		return status.Errorf(codes.Internal, "search failed: %v", err)
	}
	return stream.Send(&searchpb.SearchDocumentsResponse{Results: results})
}

// IndexJsonObject indexes a JSON object/array of objects.
func (srv *server) IndexJsonObject(ctx context.Context, rqst *searchpb.IndexJsonObjectRequest) (*searchpb.IndexJsonObjectResponse, error) {
	if err := srv.search_engine.IndexJsonObject(rqst.Path, rqst.JsonStr, rqst.Language, rqst.Id, rqst.Indexs, rqst.Data); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &searchpb.IndexJsonObjectResponse{}, nil
}

// -----------------------------------------------------------------------------
// Runtime
// -----------------------------------------------------------------------------
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

func main() {
	srv := new(server)

	// Minimal bootstrap values that don't require contacting etcd/config.
	srv.Name = string(searchpb.File_search_proto.Services().Get(0).FullName())
	srv.Proto = searchpb.File_search_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Keywords = []string{"Search", "Index", "Bleve", "Service"}
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)
	srv.Permissions = []interface{}{
		// ---- Stop the service
		map[string]interface{}{
			"action":     "/search.SearchService/Stop",
			"permission": "admin",
			"resources":  []interface{}{},
		},

		// ---- Engine info (read-only)
		map[string]interface{}{
			"action":     "/search.SearchService/GetEngineVersion",
			"permission": "read",
			"resources":  []interface{}{},
		},

		// ---- Index JSON (writes to an index at Path)
		map[string]interface{}{
			"action":     "/search.SearchService/IndexJsonObject",
			"permission": "write",
			"resources": []interface{}{
				// IndexJsonObjectRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Count docs in an index (Path)
		map[string]interface{}{
			"action":     "/search.SearchService/Count",
			"permission": "read",
			"resources": []interface{}{
				// CountRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "read"},
			},
		},

		// ---- Delete a document (requires write on Path)
		map[string]interface{}{
			"action":     "/search.SearchService/DeleteDocument",
			"permission": "write",
			"resources": []interface{}{
				// DeleteDocumentRequest.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Search across one or more Paths (read)
		map[string]interface{}{
			"action":     "/search.SearchService/SearchDocuments",
			"permission": "read",
			"resources": []interface{}{
				// SearchDocumentsRequest.paths
				map[string]interface{}{"index": 0, "field": "Paths", "permission": "read"},
			},
		},
	}

	srv.Process = -1
	srv.ProxyProcess = -1
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.KeepAlive = true
	srv.KeepUpToDate = true

	args := os.Args[1:]
	if len(args) == 0 {
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			logger.Error("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(srv.Id)
		if err != nil {
			logger.Error("fail to allocate port", "error", err)
			os.Exit(1)
		}
		srv.Port = p
	}

	// Handle lightweight CLI paths BEFORE touching config/etcd.
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// best-effort runtime fields
			srv.Process = os.Getpid()
			srv.State = "starting"

			// Provide domain/address without etcd dependency
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
				srv.Domain = strings.ToLower(v)
			} else {
				srv.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
				srv.Address = strings.ToLower(v)
			} else {
				srv.Address = "localhost:" + Utility.ToString(srv.Port)
			}

			b, err := globular.DescribeJSON(srv)
			if err != nil {
				logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if srv.Port == 0 || srv.Name == "" {
				logger.Error("health error: uninitialized", "service", srv.Name, "port", srv.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--version", "-v":
			logger.Info(srv.Name + " version " + srv.Version)
			return
		default:
			// skip unknown flags for now (e.g. positional args)
		}
	}

	// Optional positional args for id and config path (unchanged behavior)
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Safe to query local config (file/etcd) now.
	if d, err := config.GetDomain(); err == nil {
		srv.Domain = d
	} else {
		srv.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register gRPC service
	searchpb.RegisterSearchServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  search_server [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("Examples:")
	fmt.Println("  search_server my-search-id /etc/globular/search/config.json")
	fmt.Println("  search_server --describe")
	fmt.Println("  search_server --health")
}
