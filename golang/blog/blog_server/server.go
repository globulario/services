package main

import (
	"encoding/json"
	"errors"
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
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
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
	defaultPort       = 10029
	defaultProxy      = 10030
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// -----------------------------------------------------------------------------
// Service implementation (consumed by Globular)
// Keep all public method signatures unchanged.
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
	Permissions  []interface{} // action permissions for the service
	Dependencies []string      // names of required services

	// Runtime components.
	grpcServer *grpc.Server
	store      storage_store.Store // persistent KV store

	// Cached/active resources.
	blogs  *sync.Map
	indexs map[string]bleve.Index
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
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(v []interface{})  { srv.Permissions = v }

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

// Lifecycle
func (srv *server) Init() error {

	// Initialize service config with Globular runtime.
	if err := globular.InitService(srv); err != nil {
		return err
	}
	// Initialize gRPC server (interceptors wired internally like in the auth template).
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs

	// Create and open local KV store (single open here).
	if srv.store == nil {
		srv.store = storage_store.NewBadger_store()
	}
	// Default location if not set.
	if srv.Root == "" {
		srv.Root = os.TempDir()
	}
	if err := srv.store.Open(`{"path":"` + srv.Root + `", "name":"blogs"}`); err != nil {
		return err
	}

	return nil
}
func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }

// --- logger to STDERR so stdout stays clean for JSON outputs ---
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

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

func getRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(token, path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	c, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return c.AddResourceOwner(token, path, resourceType, subject, subjectType)
}

// -----------------------------------------------------------------------------
// Bleve helpers
// -----------------------------------------------------------------------------

// getIndex opens or creates a Bleve index at the given path and caches it.
func (srv *server) getIndex(path string) (bleve.Index, error) {
	if srv.indexs == nil {
		srv.indexs = make(map[string]bleve.Index)
	}
	if srv.indexs[path] == nil {
		index, err := bleve.Open(path)
		if err != nil {
			// Create a new index if opening failed.
			mapping := bleve.NewIndexMapping()
			index, err = bleve.New(path, mapping)
			if err != nil {
				logger.Error("create bleve index failed", "path", path, "err", err)
				return nil, err
			}
			logger.Info("created new bleve index", "path", path)
		} else {
			logger.Info("opened existing bleve index", "path", path)
		}
		srv.indexs[path] = index
	}
	return srv.indexs[path], nil
}

// -----------------------------------------------------------------------------
// Blog helpers
// -----------------------------------------------------------------------------

func (srv *server) deleteAccountListener(evt *eventpb.Event) {
	accountId := string(evt.Data)
	blogs, err := srv.getBlogPostByAuthor(accountId)
	if err != nil {
		logger.Error("get blogs by author failed", "author", accountId, "err", err)
		return
	}
	for i := 0; i < len(blogs); i++ {
		if err := srv.deleteBlogPost(accountId, blogs[i].Uuid); err != nil {
			logger.Error("delete blog post failed", "author", accountId, "uuid", blogs[i].Uuid, "err", err)
		} else {
			logger.Info("deleted blog post for removed account", "author", accountId, "uuid", blogs[i].Uuid)
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

	// Remove from author index list.
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
// Usage
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Entrypoint
// -----------------------------------------------------------------------------

func main() {
	// Skeleton only (no etcd access yet)
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
	// Use generic verbs and only protect parameters that are real resource paths (UUIDs / index paths).
	s.Permissions = []interface{}{
		// --- Posts: create / update / delete / read ---

		// Create: writes to the Bleve index on disk.
		map[string]interface{}{
			"action": "/blog.BlogService/CreateBlogPost",
			"resources": []interface{}{
				// CreateBlogPostRequest.indexPath
				map[string]interface{}{"index": 0, "permission": "write"},
			},
		},

		// Save: write the specific blog post + write index.
		map[string]interface{}{
			"action": "/blog.BlogService/SaveBlogPost",
			"resources": []interface{}{
				// SaveBlogPostRequest.blog_post.Uuid (prefer binding the message subfield)
				map[string]interface{}{"index": 1, "field": "Uuid", "permission": "write"},
				// SaveBlogPostRequest.indexPath
				map[string]interface{}{"index": 2, "permission": "write"},
			},
		},

		// Read specific posts by UUID.
		map[string]interface{}{
			"action": "/blog.BlogService/GetBlogPosts",
			"resources": []interface{}{
				// GetBlogPostsRequest.uuids (list expansion handled by interceptor)
				map[string]interface{}{"index": 0, "permission": "read"},
			},
		},

		// Search: read access to the index path.
		map[string]interface{}{
			"action": "/blog.BlogService/SearchBlogPosts",
			"resources": []interface{}{
				// SearchBlogPostsRequest.indexPath
				map[string]interface{}{"index": 2, "permission": "read"},
			},
		},

		// Delete post: delete the post + write index.
		map[string]interface{}{
			"action": "/blog.BlogService/DeleteBlogPost",
			"resources": []interface{}{
				// DeleteBlogPostRequest.uuid
				map[string]interface{}{"index": 0, "permission": "delete"},
				// DeleteBlogPostRequest.indexPath
				map[string]interface{}{"index": 1, "permission": "write"},
			},
		},

		// --- Reactions & comments (write/delete on the target post/comment UUID) ---

		// Add emoji on a post or comment (targeted by rqst.uuid).
		map[string]interface{}{
			"action": "/blog.BlogService/AddEmoji",
			"resources": []interface{}{
				// AddEmojiRequest.uuid (target blog or comment thread owner post)
				map[string]interface{}{"index": 0, "permission": "write"},
			},
		},

		// Remove emoji (target post uuid).
		map[string]interface{}{
			"action": "/blog.BlogService/RemoveEmoji",
			"resources": []interface{}{
				// RemoveEmojiRequest.uuid
				map[string]interface{}{"index": 0, "permission": "delete"},
			},
		},

		// Add comment (target post uuid).
		map[string]interface{}{
			"action": "/blog.BlogService/AddComment",
			"resources": []interface{}{
				// AddCommentRequest.uuid
				map[string]interface{}{"index": 0, "permission": "write"},
			},
		},

		// Remove comment (target post uuid).
		map[string]interface{}{
			"action": "/blog.BlogService/RemoveComment",
			"resources": []interface{}{
				// RemoveCommentRequest.uuid
				map[string]interface{}{"index": 0, "permission": "delete"},
			},
		},

		// Note: GetBlogPostsByAuthors is intentionally left permissive (no resource path to bind that maps to blog ownership).
	}

	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.blogs = &sync.Map{}

	// Dynamic client registration for routing.
	Utility.RegisterFunction("NewBlogService_Client", blog_client.NewBlogService_Client)

	// CLI flags BEFORE touching config
	args := os.Args[1:]
	if len(args) == 0 {
		s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			fmt.Println("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(s.Id)
		if err != nil {
			fmt.Println("fail to allocate port", "error", err)
			os.Exit(1)
		}
		s.Port = p
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// Minimal fields for description
			s.Process = os.Getpid()
			s.State = "starting"
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && strings.TrimSpace(v) != "" {
				s.Domain = strings.ToLower(v)
			} else {
				s.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && strings.TrimSpace(v) != "" {
				s.Address = strings.ToLower(v)
			} else {
				s.Address = "localhost:" + Utility.ToString(s.Port)
			}
			b, err := globular.DescribeJSON(s)
			if err != nil {
				logger.Error("describe error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return
		case "--health":
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
			return
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--version", "-v":
			os.Stdout.WriteString(s.Version + "\n")
			return
		case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Println("unknown option:", a)
				printUsage()
				os.Exit(1)
			}

		}
	}

	// Optional positional args: <id> [configPath]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		s.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		s.Id = args[0]
		s.ConfigPath = args[1]
	}

	// Safe to touch config now
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
	} else {
		s.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
	}

	// Ensure Root dir (if not set by config/Init).
	if len(s.Root) == 0 {
		s.Root = os.TempDir()
	}

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// Ensure blogs/index storage path (Bleve indices)
	if err := Utility.CreateDirIfNotExist(filepath.Join(s.Root, "blogs")); err != nil {
		logger.Error("create blogs dir failed", "path", filepath.Join(s.Root, "blogs"), "err", err)
		os.Exit(1)
	}

	// Register the gRPC service.
	blogpb.RegisterBlogServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	logger.Info("gRPC service registered",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"listen_ms", time.Since(start).Milliseconds())

	// Subscribe to account deletion events.
	go func() {
		consumerID := s.Name + "@" + s.Domain + ":delete"
		backoff := time.Second
		for {
			evtClient, err := s.getEventClient()
			if err != nil {
				logger.Warn("event client unavailable; retrying", "err", err)
				time.Sleep(backoff)
				if backoff < 10*time.Second {
					backoff *= 2
				}
				continue
			}
			err = evtClient.Subscribe("delete_account_evt", consumerID, func(evt *eventpb.Event) {
				s.deleteAccountListener(evt)
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
	}()

	// Start serving (blocks).
	if err := s.StartService(); err != nil {
		logger.Error("service start failed", "service", s.Name, "err", err)
		os.Exit(1)
	}
}
