package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/blog/blog_client"
	"github.com/globulario/services/golang/blog/blogpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// Comma separated values.
	allowed_origins string = ""
)

// -----------------------------------------------------------------------------
// Service implementation
// -----------------------------------------------------------------------------

// server holds Globular service metadata and dependencies required to run the
// Blog gRPC microservice. Although the type is unexported, many of its methods
// are exported to satisfy the Globular runtime interfaces.
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
	AllowedOrigins  string // comma-separated
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
	Permissions  []interface{} // action permissions for the service
	Dependencies []string      // names of required services

	// Runtime components.
	grpcServer *grpc.Server
	store      storage_store.Store // persistent KV store

	// Cached/active resources.
	blogs  *sync.Map
	indexs map[string]bleve.Index
}

// GetConfigurationPath returns the path where the service configuration is stored.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path where the service configuration is stored.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP(S) address where configuration is exposed (e.g., "/config").
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP(S) address where configuration is exposed.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the current service PID (or -1 if not started by supervisor).
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the service process ID.
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the current reverse proxy PID (or -1 if none).
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the reverse proxy process ID.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state string (e.g. "starting", "running", "stopped").
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state string.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last recorded error message (if any).
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError records the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the configuration modification time (unix epoch ms).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the configuration modification time (unix epoch ms).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the unique instance identifier.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the unique instance identifier.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetMac returns the host MAC address recorded for this service.
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the host MAC address recorded for this service.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetChecksum returns the binary checksum string.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the binary checksum string.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the platform string (e.g., "linux/amd64").
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the platform string.
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetKeywords returns the keyword list associated with the service.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the keyword list associated with the service.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns the list of repository URIs.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets the list of repository URIs.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns discovery endpoints used by the service.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets discovery endpoints used by the service.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// Dist packages and distributes the service from the given path (delegates to Globular).
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of dependent service names.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency adds a dependency name if not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetPath returns the executable path for the service binary.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the executable path for the service binary.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the path to the .proto file describing the service API.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the path to the .proto file describing the service API.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC service port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC service port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse proxy port (gRPC-Web).
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse proxy port (gRPC-Web).
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the transport protocol ("grpc", "http", "https", "tls").
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the transport protocol ("grpc", "http", "https", "tls").
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins reports whether all origins are allowed for CORS.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins configures whether all origins are allowed for CORS.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated list of allowed origins used when
// GetAllowAllOrigins() is false.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed origins for CORS.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the configured domain name or IP.
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the configured domain name or IP.
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// TLS section.

// GetTls returns true if the service is configured to run with TLS.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS mode.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the path to the CA trust bundle.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the path to the CA trust bundle.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the path to the X.509 certificate file.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the path to the X.509 certificate file.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the path to the private key file.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the path to the private key file.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the semantic version for the service.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the semantic version for the service.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher/owner identifier.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher/owner identifier.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns whether the supervisor should auto-update this service.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate configures auto-update behavior (note: name preserved for compatibility).
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns whether the service should be kept alive by a supervisor.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive configures keep-alive behavior in the supervisor.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the action/resource permissions for the service.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the action/resource permissions for the service.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// Init initializes the service configuration, gRPC server, and backing store.
// It must be called before StartService.
func (srv *server) Init() error {
	// Initialize service config with Globular runtime.
	if err := globular.InitService(srv); err != nil {
		slog.Error("init service failed", "service", srv.Name, "id", srv.Id, "err", err)
		return err
	}

	// Initialize gRPC server with interceptors.
	grpcSrv, err := globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		slog.Error("init grpc server failed", "service", srv.Name, "id", srv.Id, "err", err)
		return err
	}
	srv.grpcServer = grpcSrv

	// Create and open local KV store.
	srv.store = storage_store.NewBadger_store()
	if err := srv.store.Open(`{"path":"` + srv.Root + `", "name":"blogs"}`); err != nil {
		slog.Error("open store failed", "path", srv.Root, "name", "blogs", "err", err)
		return err
	}

	slog.Info("service initialized", "service", srv.Name, "id", srv.Id, "address", srv.Address)
	return nil
}

// Save persists the current service configuration to disk.
func (srv *server) Save() error {
	return globular.SaveService(srv)
}

// StartService starts the gRPC server and associated reverse proxy (if configured).
func (srv *server) StartService() error {
	slog.Info("starting service", "service", srv.Name, "port", srv.Port, "proxy", srv.Proxy, "protocol", srv.Protocol)
	return globular.StartService(srv, srv.grpcServer)
}

// StopService gracefully stops the gRPC server and reverse proxy (if running).
func (srv *server) StopService() error {
	slog.Info("stopping service", "service", srv.Name)
	return globular.StopService(srv, srv.grpcServer)
}

// getIndex opens or creates a Bleve index at the given path and caches it.
func (srv *server) getIndex(path string) (bleve.Index, error) {
	if srv.indexs == nil {
		srv.indexs = make(map[string]bleve.Index, 0)
	}
	if srv.indexs[path] == nil {
		index, err := bleve.Open(path)
		if err != nil {
			// Create a new index if opening failed.
			mapping := bleve.NewIndexMapping()
			index, err = bleve.New(path, mapping)
			if err != nil {
				slog.Error("create bleve index failed", "path", path, "err", err)
				return nil, err
			}
			slog.Info("created new bleve index", "path", path)
		} else {
			slog.Info("opened existing bleve index", "path", path)
		}
		srv.indexs[path] = index
	}
	return srv.indexs[path], nil
}


// -----------------------------------------------------------------------------
// Event helpers
// -----------------------------------------------------------------------------

func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) subscribe(evt string, listener func(evt *eventpb.Event)) error {
	eventClient, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return eventClient.Subscribe(evt, srv.Name, listener)
}

// -----------------------------------------------------------------------------
// RBAC helpers
// -----------------------------------------------------------------------------

// GetRbacClient returns an RBAC client to manage permissions and ACLs.
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) getResourcePermissions(path string) (*rbacpb.Permissions, error) {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return nil, err
	}
	return c.GetResourcePermissions(path)
}

func (srv *server) setResourcePermissions(token, path, resource_type string, permissions *rbacpb.Permissions) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return c.SetResourcePermissions(token, path, resource_type, permissions)
}

func (srv *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return false, false, err
	}
	return c.ValidateAccess(subject, subjectType, name, path)
}

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return c.AddResourceOwner(path, resourceType, subject, subjectType)
}

func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return c.SetActionResourcesPermissions(permissions)
}

// -----------------------------------------------------------------------------
// Blog helpers
// -----------------------------------------------------------------------------

func (srv *server) deleteAccountListener(evt *eventpb.Event) {
	accountId := string(evt.Data)
	blogs, err := srv.getBlogPostByAuthor(accountId)
	if err != nil {
		slog.Error("get blogs by author failed", "author", accountId, "err", err)
		return
	}

	for i := 0; i < len(blogs); i++ {
		if err := srv.deleteBlogPost(accountId, blogs[i].Uuid); err != nil {
			slog.Error("delete blog post failed", "author", accountId, "uuid", blogs[i].Uuid, "err", err)
		} else {
			slog.Info("deleted blog post for removed account", "author", accountId, "uuid", blogs[i].Uuid)
		}
	}
}

// getBlogPost returns the blog post with the given uuid.
func (srv *server) getBlogPost(uuid string) (*blogpb.BlogPost, error) {
	blog := new(blogpb.BlogPost)
	jsonStr, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, err
	}

	if err := protojson.Unmarshal(jsonStr, blog); err != nil {
		return nil, err
	}
	return blog, nil
}

// getBlogPostByAuthor returns all blog posts authored by the given account id.
func (srv *server) getBlogPostByAuthor(author string) ([]*blogpb.BlogPost, error) {
	blogPosts := make([]*blogpb.BlogPost, 0)

	blogsBytes, err := srv.store.GetItem(author)
	ids := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(blogsBytes, &ids); err != nil {
			return nil, err
		}
	}

	for i := 0; i < len(ids); i++ {
		jsonStr, err := srv.store.GetItem(ids[i])
		if err != nil {
			continue
		}
		instance := new(blogpb.BlogPost)
		if err := protojson.Unmarshal(jsonStr, instance); err == nil {
			blogPosts = append(blogPosts, instance)
		}
	}

	return blogPosts, nil
}

// getSubComment searches recursively for a sub-comment inside a comment tree.
func (srv *server) getSubComment(uuid string, comment *blogpb.Comment) (*blogpb.Comment, error) {
	if comment.Comments == nil {
		return nil, errors.New("no answer was found for that comment")
	}

	for i := 0; i < len(comment.Comments); i++ {
		c := comment.Comments[i]
		if uuid == c.Uuid {
			return c, nil
		}
		if c.Comments != nil {
			if found, err := srv.getSubComment(uuid, c); err == nil && found != nil {
				return found, nil
			}
		}
	}

	return nil, errors.New("no answer was found for that comment")
}

// getBlogComment finds a comment by uuid within a blog post (searching answers recursively).
func (srv *server) getBlogComment(parentUuid string, blog *blogpb.BlogPost) (*blogpb.Comment, error) {
	for i := 0; i < len(blog.Comments); i++ {
		c := blog.Comments[i]
		if c.Uuid == parentUuid {
			return c, nil
		}
		if found, err := srv.getSubComment(parentUuid, c); err == nil && found != nil {
			return found, nil
		}
	}
	return nil, errors.New("no comment was found for that blog")
}

// deleteBlogPost deletes a blog post if requested by its author.
func (srv *server) deleteBlogPost(author, uuid string) error {
	blog, err := srv.getBlogPost(uuid)
	if err != nil {
		return err
	}

	if author != blog.Author {
		return errors.New("only blog author can delete it blog")
	}

	// Remove from author indexation list.
	blogsBytes, err := srv.store.GetItem(blog.Author)
	ids := make([]string, 0)
	if err == nil {
		if err := json.Unmarshal(blogsBytes, &ids); err != nil {
			return err
		}
	}
	ids = Utility.RemoveString(ids, uuid)

	// Save updated list.
	idsJSON, err := Utility.ToJson(ids)
	if err != nil {
		return err
	}
	if err := srv.store.SetItem(blog.Author, []byte(idsJSON)); err != nil {
		return err
	}

	// Delete the post object.
	return srv.store.RemoveItem(uuid)
}

// saveBlogPost persists a blog post and maintains the author's index list.
func (srv *server) saveBlogPost(author string, blogPost *blogpb.BlogPost) error {
	blogPost.Domain = srv.Domain
	blogPost.Mac = srv.Mac

	jsonStr, err := protojson.Marshal(blogPost)
	if err != nil {
		return err
	}

	if err := srv.store.SetItem(blogPost.Uuid, []byte(jsonStr)); err != nil {
		return err
	}

	// Update author index.
	blogsBytes, err := srv.store.GetItem(author)
	blogs := make([]string, 0)
	if err == nil {
		_ = json.Unmarshal(blogsBytes, &blogs)
	}
	if !Utility.Contains(blogs, blogPost.Uuid) {
		blogs = append(blogs, blogPost.Uuid)
	}

	blogsJSON, err := Utility.ToJson(blogs)
	if err != nil {
		return err
	}
	return srv.store.SetItem(author, []byte(blogsJSON))
}

// -----------------------------------------------------------------------------
// Entrypoint
// -----------------------------------------------------------------------------

// main boots the Blog service and blocks until the gRPC server is stopped.
func main() {
	// Structured logger to stdout.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Initialize service implementation with defaults.
	s := new(server)
	s.Name = string(blogpb.File_blog_proto.Services().Get(0).FullName())
	s.Proto = blogpb.File_blog_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Domain, _ = config.GetDomain()
	s.Address, _ = config.GetAddress()
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "The Hello world of gRPC service!"
	s.Keywords = []string{"Example", "Blog", "Post", "Service"}
	s.Repositories = make([]string, 0)
	s.Discoveries = make([]string, 0)
	s.Dependencies = make([]string, 0)
	s.Permissions = make([]interface{}, 3)
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allow_all_origins
	s.AllowedOrigins = allowed_origins

	// Dynamic client registration for routing.
	Utility.RegisterFunction("NewBlogService_Client", blog_client.NewBlogService_Client)

	// ID and optional config path from args.
	if len(os.Args) == 2 {
		s.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s.Id = os.Args[1]
		s.ConfigPath = os.Args[2]
	}

	// Ensure Root dir.
	if len(s.Root) == 0 {
		s.Root = os.TempDir()
	}

	// Specific permissions.
	s.Permissions[0] = map[string]interface{}{"action": "/blog.BlogService/SaveBlogPost", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s.Permissions[1] = map[string]interface{}{"action": "/blog.BlogService/DeleteBlogPost", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}

	// Init service (config, gRPC, store).
	if err := s.Init(); err != nil {
		slog.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// Ensure blogs storage path and open secondary index store.
	if err := Utility.CreateDirIfNotExist(s.Root + "/blogs"); err != nil {
		slog.Error("create blogs dir failed", "path", s.Root+"/blogs", "err", err)
		os.Exit(1)
	}
	if err := s.store.Open(`{"path":"` + s.Root + "/blogs" + `", "name":"index"}`); err != nil {
		slog.Error("open blogs index store failed", "path", s.Root+"/blogs", "err", err)
		os.Exit(1)
	}

	// Register the gRPC service.
	blogpb.RegisterBlogServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	slog.Info("gRPC service registered", "service", s.Name, "port", s.Port)

	// Subscribe to account deletion events.
	go func() {
		if err := s.subscribe("delete_account_evt", s.deleteAccountListener); err != nil {
			slog.Error("event subscription failed", "event", "delete_account_evt", "err", err)
		} else {
			slog.Info("subscribed to event", "event", "delete_account_evt")
		}
	}()

	// Start serving (blocks).
	if err := s.StartService(); err != nil {
		slog.Error("service start failed", "service", s.Name, "err", err)
		os.Exit(1)
	}
}
