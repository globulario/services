package main

import (
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"

    "github.com/blevesearch/bleve/v2"
    "github.com/globulario/services/golang/blog/blog_client"
    "github.com/globulario/services/golang/blog/blogpb"
    "github.com/globulario/services/golang/config"
    "github.com/globulario/services/golang/event/eventpb"
    globular "github.com/globulario/services/golang/globular_service"
    "github.com/globulario/services/golang/resource/resourcepb"
    "github.com/globulario/services/golang/storage/storage_store"
    Utility "github.com/globulario/utility"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
    defaultPort       = 10029
    defaultProxy      = 10030
    allowAllOrigins   = true
    allowedOriginsStr = ""
)

// --- logger to STDERR so stdout stays clean for JSON outputs ---
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// -----------------------------------------------------------------------------
// Service implementation (consumed by Globular)
// -----------------------------------------------------------------------------

type server struct {
    // Generic service attributes required by Globular runtime.
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
    AllowedOrigins  string
    Protocol        string
    Version         string
    PublisherID     string
    KeepUpToDate    bool
    Checksum        string
    Plaform         string
    KeepAlive       bool
    Description     string
    Keywords        []string
    Repositories    []string
    Discoveries     []string
    Process         int
    ProxyProcess    int
    ConfigPath      string
    LastError       string
    ModTime         int64
    State           string
    TLS             bool

    // TLS material.
    CertFile           string
    KeyFile            string
    CertAuthorityTrust string

    // Service-specific configuration.
    Root string // Where to store conversation data, files, etc.

    // Permissions and dependencies.
    Permissions  []any // action permissions for the service
    Dependencies []string

    // Runtime components.
    grpcServer *grpc.Server
    store      storage_store.Store // persistent KV store

    // Cached/active resources.
    blogs  *sync.Map
    indexs map[string]bleve.Index

    // Test hooks / dependency injection.
    eventClientFactory eventClientFactory
    rbacClientFactory  rbacClientFactory
}

// --- Getters/Setters required by Globular (unchanged signatures) ---

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
func (srv *server) GetDescription() string                { return srv.Description }
func (srv *server) SetDescription(description string)     { srv.Description = description }
func (srv *server) GetMac() string                        { return srv.Mac }
func (srv *server) SetMac(mac string)                     { srv.Mac = mac }
func (srv *server) GetChecksum() string                   { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)           { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                   { return srv.Plaform }
func (srv *server) SetPlatform(platform string)           { srv.Plaform = platform }
func (srv *server) GetKeywords() []string                 { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)         { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string             { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string              { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)   { srv.Discoveries = discoveries }
func (srv *server) Dist(path string) (string, error)      { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
    if srv.Dependencies == nil {
        srv.Dependencies = []string{}
    }
    return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
    if srv.Dependencies == nil {
        srv.Dependencies = []string{}
    }
    if !Utility.Contains(srv.Dependencies, dep) {
        srv.Dependencies = append(srv.Dependencies, dep)
    }
}
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
func (srv *server) GetPermissions() []any           { return srv.Permissions }
func (srv *server) SetPermissions(v []any)          { srv.Permissions = v }

// RolesDefault returns curated roles for BlogService.
func (srv *server) RolesDefault() []resourcepb.Role {
    domain, _ := config.GetDomain()

    return []resourcepb.Role{
        {
            Id:          "role:blog.reader",
            Name:        "Blog Reader",
            Domain:      domain,
            Description: "Read and search blog posts.",
            Actions: []string{
                "/blog.BlogService/GetBlogPosts",
                "/blog.BlogService/SearchBlogPosts",
                "/blog.BlogService/GetBlogPostsByAuthors", // left permissive; included for completeness
            },
            TypeName: "resource.Role",
        },
        {
            Id:          "role:blog.contributor",
            Name:        "Blog Contributor",
            Domain:      domain,
            Description: "Create and update posts; add comments and reactions.",
            Actions: []string{
                "/blog.BlogService/CreateBlogPost",
                "/blog.BlogService/SaveBlogPost",
                "/blog.BlogService/AddComment",
                "/blog.BlogService/AddEmoji",
            },
            TypeName: "resource.Role",
        },
        {
            Id:          "role:blog.moderator",
            Name:        "Blog Moderator",
            Domain:      domain,
            Description: "Moderate content: delete posts, comments, and reactions.",
            Actions: []string{
                "/blog.BlogService/DeleteBlogPost",
                "/blog.BlogService/RemoveComment",
                "/blog.BlogService/RemoveEmoji",
            },
            TypeName: "resource.Role",
        },
        {
            Id:          "role:blog.admin",
            Name:        "Blog Admin",
            Domain:      domain,
            Description: "Full control over blogging features.",
            Actions: []string{
                "/blog.BlogService/CreateBlogPost",
                "/blog.BlogService/SaveBlogPost",
                "/blog.BlogService/GetBlogPosts",
                "/blog.BlogService/SearchBlogPosts",
                "/blog.BlogService/GetBlogPostsByAuthors",
                "/blog.BlogService/DeleteBlogPost",
                "/blog.BlogService/AddComment",
                "/blog.BlogService/RemoveComment",
                "/blog.BlogService/AddEmoji",
                "/blog.BlogService/RemoveEmoji",
            },
            TypeName: "resource.Role",
        },
    }
}

// Init initializes the service configuration and gRPC server.
func (srv *server) Init() error {
    if err := globular.InitService(srv); err != nil {
        return err
    }

    gs, err := globular.InitGrpcServer(srv)
    if err != nil {
        return err
    }
    srv.grpcServer = gs
    storage_store.SetLogger(logger)

    return nil
}

func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService prepares storage, subscriptions, and starts the gRPC server.
func (srv *server) StartService() error {
    if srv.store == nil {
        srv.store = storage_store.NewBadger_store()
    }

    if srv.Root == "" {
        srv.Root = os.TempDir()
    }

    if err := srv.store.Open(`{"path":"` + srv.Root + `", "name":"blogs"}`); err != nil {
        return err
    }

    if err := Utility.CreateDirIfNotExist(filepath.Join(srv.Root, "blogs")); err != nil {
        return err
    }

    // Subscribe to account deletion events.
    go srv.startDeleteAccountSubscription()

    return globular.StartService(srv, srv.grpcServer)
}

// StopService stops gRPC server and cleans resources.
func (srv *server) StopService() error {
    var firstErr error

    if srv.indexs != nil {
        for path, idx := range srv.indexs {
            if idx != nil {
                if err := idx.Close(); err != nil && firstErr == nil {
                    firstErr = fmt.Errorf("close index %s: %w", path, err)
                }
            }
        }
    }

    if srv.store != nil {
        if err := srv.store.Close(); err != nil && firstErr == nil {
            firstErr = err
        }
    }

    if err := globular.StopService(srv, srv.grpcServer); err != nil && firstErr == nil {
        firstErr = err
    }

    return firstErr
}

// GetGrpcServer returns the gRPC server instance (LifecycleService requirement).
func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }

// startDeleteAccountSubscription retries subscription until it succeeds.
func (srv *server) startDeleteAccountSubscription() {
    consumerID := srv.Name + "@" + srv.Domain + ":delete"
    backoff := time.Second
    for {
        evtClient, err := srv.getEventClient()
        if err != nil {
            logger.Warn("event client unavailable; retrying", "err", err)
            time.Sleep(backoff)
            if backoff < 10*time.Second {
                backoff *= 2
            }
            continue
        }
        err = evtClient.Subscribe("delete_account_evt", consumerID, func(evt *eventpb.Event) {
            srv.deleteAccountListener(evt)
        })
        if err != nil {
            logger.Warn("subscribe failed; retrying", "channel", "delete_account_evt", "err", err)
            time.Sleep(backoff)
            if backoff < 10*time.Second {
                backoff *= 2
            }
            continue
        }
        logger.Info("subscribed to event", "channel", "delete_account_evt", "consumer", consumerID)
        break
    }
}

// initializeServerDefaults sets up baseline values before config loading.
func initializeServerDefaults() *server {
    s := new(server)

    s.Name = string(blogpb.File_blog_proto.Services().Get(0).FullName())
    s.Proto = blogpb.File_blog_proto.Path()
    s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
    s.Port = defaultPort
    s.Proxy = defaultProxy
    s.Protocol = "grpc"
    s.Version = "0.0.1"
    s.PublisherID = "localhost"
    s.Description = "Blog service"
    s.Keywords = []string{"Example", "Blog", "Post", "Service"}
    s.Repositories = []string{}
    s.Discoveries = []string{}
    s.Dependencies = []string{"event.EventService", "rbac.RbacService", "log.LogService"}

    // Default RBAC permissions for BlogService.
    s.Permissions = []any{
        // Create: writes to the Bleve index on disk.
        map[string]any{
            "action": "/blog.BlogService/CreateBlogPost",
            "resources": []any{
                map[string]any{"index": 0, "permission": "write"},
            },
        },

        // Save: write the specific blog post + write index.
        map[string]any{
            "action": "/blog.BlogService/SaveBlogPost",
            "resources": []any{
                map[string]any{"index": 1, "field": "Uuid", "permission": "write"},
                map[string]any{"index": 2, "permission": "write"},
            },
        },

        // Read specific posts by UUID.
        map[string]any{
            "action": "/blog.BlogService/GetBlogPosts",
            "resources": []any{
                map[string]any{"index": 0, "permission": "read"},
            },
        },

        // Search: read access to the index path.
        map[string]any{
            "action": "/blog.BlogService/SearchBlogPosts",
            "resources": []any{
                map[string]any{"index": 2, "permission": "read"},
            },
        },

        // Delete post: delete the post + write index.
        map[string]any{
            "action": "/blog.BlogService/DeleteBlogPost",
            "resources": []any{
                map[string]any{"index": 0, "permission": "delete"},
                map[string]any{"index": 1, "permission": "write"},
            },
        },

        // Add emoji on a post or comment (targeted by rqst.uuid).
        map[string]any{
            "action": "/blog.BlogService/AddEmoji",
            "resources": []any{
                map[string]any{"index": 0, "permission": "write"},
            },
        },

        // Remove emoji (target post uuid).
        map[string]any{
            "action": "/blog.BlogService/RemoveEmoji",
            "resources": []any{
                map[string]any{"index": 0, "permission": "delete"},
            },
        },

        // Add comment (target post uuid).
        map[string]any{
            "action": "/blog.BlogService/AddComment",
            "resources": []any{
                map[string]any{"index": 0, "permission": "write"},
            },
        },

        // Remove comment (target post uuid).
        map[string]any{
            "action": "/blog.BlogService/RemoveComment",
            "resources": []any{
                map[string]any{"index": 0, "permission": "delete"},
            },
        },

        // Note: GetBlogPostsByAuthors is intentionally left permissive (no resource path binding).
    }

    s.Process = -1
    s.ProxyProcess = -1
    s.KeepAlive = true
    s.KeepUpToDate = true
    s.AllowAllOrigins = allowAllOrigins
    s.AllowedOrigins = allowedOriginsStr
    s.blogs = &sync.Map{}

    domain, addr := globular.GetDefaultDomainAddress(s.Port)
    s.Domain = domain
    if host, _, ok := strings.Cut(addr, ":"); ok {
        s.Address = host
    } else {
        s.Address = addr
    }

    return s
}

func setupGrpcService(srv *server) {
    blogpb.RegisterBlogServiceServer(srv.grpcServer, srv)
    reflection.Register(srv.grpcServer)
}

func printUsage() {
    exe := filepath.Base(os.Args[0])
    os.Stdout.WriteString(`
Usage: ` + exe + ` [options] <id> [configPath]

Options:
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

Arguments:
  <id>            Service instance ID
  [configPath]    Optional path to configuration file

Example:
  ` + exe + ` blog-1 /etc/globular/blog/config.json

`)
}

func validateFlags(args []string) error {
    allowed := map[string]bool{
        "--describe": true,
        "--health":   true,
        "--help":     true,
        "-h":         true,
        "--version":  true,
        "-v":         true,
        "--debug":    true,
    }

    for _, a := range args {
        if strings.HasPrefix(a, "-") && !allowed[strings.ToLower(a)] {
            return fmt.Errorf("unknown option: %s", a)
        }
    }
    return nil
}

// main configures and starts the Blog service.
func main() {
    srv := initializeServerDefaults()

    args := os.Args[1:]

    // Handle --debug flag first (affects logger verbosity)
    for _, a := range args {
        if strings.ToLower(a) == "--debug" {
            logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
            break
        }
    }

    if err := validateFlags(args); err != nil {
        fmt.Println(err.Error())
        printUsage()
        os.Exit(1)
    }

    if globular.HandleInformationalFlags(srv, args, logger, printUsage) {
        return
    }

    // Allocate port if needed (before etcd access)
    if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
        logger.Error("port allocation failed", "error", err)
        os.Exit(1)
    }

    // Parse positional arguments
    globular.ParsePositionalArgs(srv, args)

    // Load runtime config (domain/address)
    globular.LoadRuntimeConfig(srv)

    // Dynamic client registration for routing.
    Utility.RegisterFunction("NewBlogService_Client", blog_client.NewBlogService_Client)

    // Initialize service (creates gRPC server, loads config)
    start := time.Now()
    if err := srv.Init(); err != nil {
        logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
        os.Exit(1)
    }

    // Register the gRPC service.
    setupGrpcService(srv)
    logger.Info("gRPC service registered",
        "service", srv.Name,
        "port", srv.Port,
        "proxy", srv.Proxy,
        "protocol", srv.Protocol,
        "domain", srv.Domain,
        "listen_ms", time.Since(start).Milliseconds())

    lifecycle := globular.NewLifecycleManager(srv, logger)
    if err := lifecycle.Start(); err != nil {
        logger.Error("service start failed", "service", srv.Name, "err", err)
        os.Exit(1)
    }
}
