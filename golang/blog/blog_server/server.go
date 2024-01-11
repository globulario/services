package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/blevesearch/bleve"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/blog/blog_client"
	"github.com/globulario/services/golang/blog/blogpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/storage/storage_store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id                   string
	Mac                  string
	Name                 string
	Domain               string
	Address              string
	Path                 string
	Proto                string
	Port                 int
	Proxy                int
	AllowAllOrigins      bool
	AllowedOrigins       string // comma separated string.
	Protocol             string
	Version              string
	PublisherId          string
	KeepUpToDate         bool
	Checksum             string
	Plaform              string
	KeepAlive            bool
	Description          string
	Keywords             []string
	Repositories         []string
	Discoveries          []string
	Process              int
	ProxyProcess         int
	ConfigPath           string
	LastError            string
	ModTime              int64
	State                string
	TLS                  bool
	DynamicMethodRouting []interface{} // contains the method name and it routing policy. (ex: ["GetFile", "round-robin"])

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	// Specific configuration.
	Root string // Where to look for conversation data, file.. etc.

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The grpc server.
	grpcServer *grpc.Server

	// Store global conversation information like conversation owner's participant...
	store storage_store.Store

	// keep in map active conversation db connections.
	blogs *sync.Map

	// Contain indexation.
	indexs map[string]bleve.Index
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
func (srv *server) GetPublisherId() string {
	return srv.PublisherId
}
func (srv *server) SetPublisherId(publisherId string) {
	srv.PublisherId = publisherId
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

	// Create a new local store.
	srv.store = storage_store.NewBadger_store()
	return srv.store.Open(`{"path":"` + srv.Root + `", "name":"blogs"}`)

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

/**
 * Return indexation for a given path...
 */
func (srv *server) getIndex(path string) (bleve.Index, error) {
	if srv.indexs[path] == nil {
		index, err := bleve.Open(path) // try to open existing index.
		if err != nil {
			mapping := bleve.NewIndexMapping()
			var err error
			index, err = bleve.New(path, mapping)
			if err != nil {
				return nil, err
			}
		}

		if srv.indexs == nil {
			srv.indexs = make(map[string]bleve.Index, 0)
		}

		srv.indexs[path] = index
	}

	return srv.indexs[path], nil
}

////////////////////////////////////////////////////////////////////////////////////////
// Logger function
////////////////////////////////////////////////////////////////////////////////////////
/**
 * Get the log client.
 */
func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	client, err := globular_client.GetClient(srv.Address, "log.LogService", "NewLogService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*log_client.Log_Client), nil
}

func (srv *server) logServiceInfo(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (srv *server) logServiceError(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Address, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

// /////////////////// resource service functions ////////////////////////////////////
func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")

	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) publish(event string, data []byte) error {
	eventClient, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return eventClient.Publish(event, data)
}

func (srv *server) subscribe(evt string, listener func(evt *eventpb.Event)) error {
	eventClient, err := srv.getEventClient()
	if err != nil {
		return err
	}

	// register a listener...
	return eventClient.Subscribe(evt, srv.Name, listener)
}

//////////////////////////////////////// RBAC Functions ///////////////////////////////////////////////
/**
 * Get the rbac client.
 */
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}
func (srv *server) getResourcePermissions(path string) (*rbacpb.Permissions, error) {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return nil, err
	}

	return rbac_client_.GetResourcePermissions(path)
}

func (srv *server) setResourcePermissions(token, path, resource_type string, permissions *rbacpb.Permissions) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetResourcePermissions(token, path, resource_type, permissions)
}

func (srv *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return false, false, err
	}

	return rbac_client_.ValidateAccess(subject, subjectType, name, path)

}

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.AddResourceOwner(path, resourceType, subject, subjectType)
}

func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
}

// //////////////////////////////////////////////////////////////////////////////////////////////
// Blogger specific functions.
// //////////////////////////////////////////////////////////////////////////////////////////////
func (srv *server) deleteAccountListener(evt *eventpb.Event) {
	accountId := string(evt.Data)
	blogs, err := srv.getBlogPostByAuthor(accountId)
	if err == nil {
		for i := 0; i < len(blogs); i++ {
			// remove the post...
			err := srv.deleteBlogPost(accountId, blogs[i].Uuid)
			if err != nil {
				fmt.Println("post ", blogs[i].Uuid, "was removed")
			}
		}
	}
}

/**
 * Return a new blogPost
 */
func (srv *server) getBlogPost(uuid string) (*blogpb.BlogPost, error) {
	// Delete a blog...
	blog := new(blogpb.BlogPost)
	jsonStr, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, err
	}

	err = protojson.Unmarshal(jsonStr, blog)
	if err != nil {
		return nil, err
	}

	return blog, nil
}

func (srv *server) getBlogPostByAuthor(author string) ([]*blogpb.BlogPost, error) {

	blog_posts := make([]*blogpb.BlogPost, 0)
	blogs_, err := srv.store.GetItem(author)

	ids := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(blogs_, &ids)
		if err != nil {
			return nil, err
		}
	}

	// Retreive the list of blogs.
	for i := 0; i < len(ids); i++ {
		jsonStr, err := srv.store.GetItem(ids[i])
		instance := new(blogpb.BlogPost)
		if err == nil {
			err := protojson.Unmarshal(jsonStr, instance)
			if err == nil {
				blog_posts = append(blog_posts, instance)
			}
		}
	}

	return blog_posts, nil
}

/**
 * Retreive a sub-comment in a comment.
 */
func (srv *server) getSubComment(uuid string, comment *blogpb.Comment) (*blogpb.Comment, error) {
	if comment.Comments == nil {
		return nil, errors.New("no answer was found for that comment")
	}

	for i := 0; i < len(comment.Comments); i++ {
		comment := comment.Comments[i]
		if uuid == comment.Uuid {
			return comment, nil
		}
		if comment.Comments != nil {
			comment_, err := srv.getSubComment(uuid, comment)
			if err == nil && comment != nil {
				return comment_, nil
			}
		}
	}

	return nil, errors.New("no answer was found for that comment")
}

/**
 * Retreive a comment inside a blog
 */
func (srv *server) getBlogComment(parentUuid string, blog *blogpb.BlogPost) (*blogpb.Comment, error) {
	// Here I will try to find the comment...
	for i := 0; i < len(blog.Comments); i++ {
		comment := blog.Comments[i]
		if comment.Uuid == parentUuid {
			return comment, nil
		}

		// try to get the comment in sub-comment (answer)
		comment, err := srv.getSubComment(parentUuid, comment)
		if err == nil && comment != nil {
			return comment, nil
		}
	}

	return nil, errors.New("no comment was found for that blog")
}

/**
 * So here I will delete the
 */
func (srv *server) deleteBlogPost(author, uuid string) error {

	blog, err := srv.getBlogPost(uuid)
	if err != nil {
		return err
	}

	if author != blog.Author {
		return errors.New("only blog author can delete it blog")
	}

	// first I will remove it from it author indexation.
	blogs_, err := srv.store.GetItem(blog.Author)
	ids := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(blogs_, &ids)
		if err != nil {
			return err
		}
	}

	ids = Utility.RemoveString(ids, uuid)

	// Now I will save the value.
	blogs__, err := Utility.ToJson(ids)
	if err != nil {
		return err
	}

	err = srv.store.SetItem(blog.Author, []byte(blogs__))
	if err != nil {
		return err
	}

	// Now I will delete the blog.
	return srv.store.RemoveItem(uuid)
}

/**
 * Save a blog post.
 */
func (srv *server) saveBlogPost(author string, blogPost *blogpb.BlogPost) error {

	// Set the domain
	blogPost.Domain = srv.Domain

	// set the mac address to...
	blogPost.Mac = srv.Mac

	jsonStr, err := protojson.Marshal(blogPost)
	if err != nil {
		return err
	}

	// set the new one.
	err = srv.store.SetItem(blogPost.Uuid, []byte(jsonStr))
	if err != nil {
		return err
	}

	// I will asscociate the author with that post...
	blogs_, err := srv.store.GetItem(author)
	blogs := make([]string, 0)
	if err == nil {
		json.Unmarshal(blogs_, &blogs)
	}

	if !Utility.Contains(blogs, blogPost.Uuid) {
		blogs = append(blogs, blogPost.Uuid)
	}

	// Now I will save the value.
	blogs__, err := Utility.ToJson(blogs)
	if err != nil {
		return err
	}

	err = srv.store.SetItem(author, []byte(blogs__))
	if err != nil {
		return err
	}

	return nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(blogpb.File_blog_proto.Services().Get(0).FullName())
	s_impl.Proto = blogpb.File_blog_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "localhost"
	s_impl.Description = "The Hello world of gRPC service!"
	s_impl.Keywords = []string{"Example", "Blog", "Post", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 3)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.DynamicMethodRouting = make([]interface{}, 0)

	// Register the client function, so it can be use for dynamic routing, (ex: ["GetFile", "round-robin"])
	Utility.RegisterFunction("NewBlogService_Client", blog_client.NewBlogService_Client)

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
	}

	// Set the root path if is pass as argument.
	if len(s_impl.Root) == 0 {
		s_impl.Root = os.TempDir()
	}

	// specific permissions.
	s_impl.Permissions[0] = map[string]interface{}{"action": "/blog.BlogService/SaveBlogPost", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[1] = map[string]interface{}{"action": "/blog.BlogService/DeleteBlogPost", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s with error %s", s_impl.Name, s_impl.Id, err.Error())
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Open the connetion with the store.
	Utility.CreateDirIfNotExist(s_impl.Root + "/blogs")
	s_impl.store.Open(`{"path":"` + s_impl.Root + "/blogs" + `", "name":"index"}`)

	// Register the blog services
	blogpb.RegisterBlogServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start listen for event...
	go func() {
		// subscribe to account delete event events
		s_impl.subscribe("delete_account_evt", s_impl.deleteAccountListener)
	}()

	// Start the service.
	s_impl.StartService()

}
