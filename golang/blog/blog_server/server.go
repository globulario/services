package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/blog/blog_client"
	"github.com/globulario/services/golang/blog/blogpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/search/search_engine"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
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

	domain string = "localhost"
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
	PublisherId     string
	KeepUpToDate    bool
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

	// The search engine..
	search_engine *search_engine.XapianEngine

	// Store global conversation information like conversation owner's participant...
	store *storage_store.LevelDB_store

	// keep in map active conversation db connections.
	blogs *sync.Map
}

// The http address where the configuration can be found /config
func (svr *server) GetAddress() string {
	return svr.Address
}

func (svr *server) SetAddress(address string) {
	svr.Address = address
}

func (svr *server) GetProcess() int {
	return svr.Process
}

func (svr *server) SetProcess(pid int) {
	svr.Process = pid
}

func (svr *server) GetProxyProcess() int {
	return svr.ProxyProcess
}

func (svr *server) SetProxyProcess(pid int) {
	svr.ProxyProcess = pid
}

// The path of the configuration.
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
}

// The current service state
func (svr *server) GetState() string {
	return svr.State
}

func (svr *server) SetState(state string) {
	svr.State = state
}

// The last error
func (svr *server) GetLastError() string {
	return svr.LastError
}

func (svr *server) SetLastError(err string) {
	svr.LastError = err
}

// The modeTime
func (svr *server) SetModTime(modtime int64) {
	svr.ModTime = modtime
}
func (svr *server) GetModTime() int64 {
	return svr.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (svr *server) GetId() string {
	return svr.Id
}
func (svr *server) SetId(id string) {
	svr.Id = id
}

// The name of a service, must be the gRpc Service name.
func (svr *server) GetName() string {
	return svr.Name
}
func (svr *server) SetName(name string) {
	svr.Name = name
}

// The description of the service
func (svr *server) GetDescription() string {
	return svr.Description
}
func (svr *server) SetDescription(description string) {
	svr.Description = description
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The list of keywords of the services.
func (svr *server) GetKeywords() []string {
	return svr.Keywords
}
func (svr *server) SetKeywords(keywords []string) {
	svr.Keywords = keywords
}

func (svr *server) GetRepositories() []string {
	return svr.Repositories
}
func (svr *server) SetRepositories(repositories []string) {
	svr.Repositories = repositories
}

func (svr *server) GetDiscoveries() []string {
	return svr.Discoveries
}
func (svr *server) SetDiscoveries(discoveries []string) {
	svr.Discoveries = discoveries
}

// Dist
func (svr *server) Dist(path string) (string, error) {

	return globular.Dist(path, svr)
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

func (svr *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (svr *server) GetPath() string {
	return svr.Path
}
func (svr *server) SetPath(path string) {
	svr.Path = path
}

// The path of the .proto file.
func (svr *server) GetProto() string {
	return svr.Proto
}
func (svr *server) SetProto(proto string) {
	svr.Proto = proto
}

// The gRpc port.
func (svr *server) GetPort() int {
	return svr.Port
}
func (svr *server) SetPort(port int) {
	svr.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (svr *server) GetProxy() int {
	return svr.Proxy
}
func (svr *server) SetProxy(proxy int) {
	svr.Proxy = proxy
}

// Can be one of http/https/tls
func (svr *server) GetProtocol() string {
	return svr.Protocol
}
func (svr *server) SetProtocol(protocol string) {
	svr.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (svr *server) GetAllowAllOrigins() bool {
	return svr.AllowAllOrigins
}
func (svr *server) SetAllowAllOrigins(allowAllOrigins bool) {
	svr.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (svr *server) GetAllowedOrigins() string {
	return svr.AllowedOrigins
}

func (svr *server) SetAllowedOrigins(allowedOrigins string) {
	svr.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (svr *server) GetDomain() string {
	return svr.Domain
}
func (svr *server) SetDomain(domain string) {
	svr.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (svr *server) GetTls() bool {
	return svr.TLS
}
func (svr *server) SetTls(hasTls bool) {
	svr.TLS = hasTls
}

// The certificate authority file
func (svr *server) GetCertAuthorityTrust() string {
	return svr.CertAuthorityTrust
}
func (svr *server) SetCertAuthorityTrust(ca string) {
	svr.CertAuthorityTrust = ca
}

// The certificate file.
func (svr *server) GetCertFile() string {
	return svr.CertFile
}
func (svr *server) SetCertFile(certFile string) {
	svr.CertFile = certFile
}

// The key file.
func (svr *server) GetKeyFile() string {
	return svr.KeyFile
}
func (svr *server) SetKeyFile(keyFile string) {
	svr.KeyFile = keyFile
}

// The service version
func (svr *server) GetVersion() string {
	return svr.Version
}
func (svr *server) SetVersion(version string) {
	svr.Version = version
}

// The publisher id.
func (svr *server) GetPublisherId() string {
	return svr.PublisherId
}
func (svr *server) SetPublisherId(publisherId string) {
	svr.PublisherId = publisherId
}

func (svr *server) GetKeepUpToDate() bool {
	return svr.KeepUpToDate
}
func (svr *server) SetKeepUptoDate(val bool) {
	svr.KeepUpToDate = val
}

func (svr *server) GetKeepAlive() bool {
	return svr.KeepAlive
}
func (svr *server) SetKeepAlive(val bool) {
	svr.KeepAlive = val
}

func (svr *server) GetPermissions() []interface{} {
	return svr.Permissions
}
func (svr *server) SetPermissions(permissions []interface{}) {
	svr.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (svr *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewBlogService_Client", blog_client.NewBlogService_Client)

	// Get the configuration path.
	err := globular.InitService(svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// Initialyse the search engine.
	svr.search_engine = new(search_engine.XapianEngine)

	// Create a new local store.
	svr.store = storage_store.NewLevelDB_store()

	return nil

}

// Save the configuration values.
func (svr *server) Save() error {
	// Create the file...
	return globular.SaveService(svr)
}

func (svr *server) StartService() error {
	return globular.StartService(svr, svr.grpcServer)
}

func (svr *server) StopService() error {
	return globular.StopService(svr, svr.grpcServer)
}

// Singleton.
var (
	rbac_client_  *rbac_client.Rbac_Client
	log_client_   *log_client.Log_Client
	event_client_ *event_client.Event_Client
)

////////////////////////////////////////////////////////////////////////////////////////
// Logger function
////////////////////////////////////////////////////////////////////////////////////////
/**
 * Get the log client.
 */
func (server *server) GetLogClient() (*log_client.Log_Client, error) {
	var err error
	if log_client_ == nil {
		address, _ := config.GetAddress()
		log_client_, err = log_client.NewLogService_Client(address, "log.LogService")
		if err != nil {
			return nil, err
		}

	}
	return log_client_, nil
}
func (server *server) logServiceInfo(method, fileLine, functionName, infos string) {
	log_client_, err := server.GetLogClient()
	if err != nil {
		return
	}
	log_client_.Log(server.Name, server.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (server *server) logServiceError(method, fileLine, functionName, infos string) {
	log_client_, err := server.GetLogClient()
	if err != nil {
		return
	}
	log_client_.Log(server.Name, server.Domain, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

///////////////////// resource service functions ////////////////////////////////////
func (server *server) getEventClient() (*event_client.Event_Client, error) {
	var err error
	if event_client_ != nil {
		return event_client_, nil
	}
	address, _ := config.GetAddress()
	event_client_, err = event_client.NewEventService_Client(address, "event.EventService")
	if err != nil {
		return nil, err
	}

	return event_client_, nil
}

func (svr *server) publish(event string, data []byte) error {
	eventClient, err := svr.getEventClient()
	if err != nil {
		return err
	}
	return eventClient.Publish(event, data)
}

func (svr *server) subscribe(evt string, listener func(evt *eventpb.Event)) error {
	eventClient, err := svr.getEventClient()
	if err != nil {
		return err
	}

	// register a listener...
	return eventClient.Subscribe(evt, svr.Name, listener)
}

//////////////////////////////////////// RBAC Functions ///////////////////////////////////////////////
/**
 * Get the rbac client.
 */
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	var err error
	if rbac_client_ == nil {
		rbac_client_, err = rbac_client.NewRbacService_Client(address, "rbac.RbacService")
		if err != nil {
			return nil, err
		}

	}
	return rbac_client_, nil
}
func (server *server) getResourcePermissions(path string) (*rbacpb.Permissions, error) {
	rbac_client_, err := GetRbacClient(server.Address)
	if err != nil {
		return nil, err
	}

	return rbac_client_.GetResourcePermissions(path)
}

func (server *server) setResourcePermissions(path string, permissions *rbacpb.Permissions) error {
	rbac_client_, err := GetRbacClient(server.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetResourcePermissions(path, permissions)
}

func (server *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	rbac_client_, err := GetRbacClient(server.Address)
	if err != nil {
		return false, false, err
	}

	return rbac_client_.ValidateAccess(subject, subjectType, name, path)

}

func (svr *server) addResourceOwner(path string, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(svr.Address)
	if err != nil {
		return err
	}
	return rbac_client_.AddResourceOwner(path, subject, subjectType)
}

func (svr *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := GetRbacClient(svr.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
}

////////////////////////////////////////////////////////////////////////////////////////////////
// Blogger specific functions.
////////////////////////////////////////////////////////////////////////////////////////////////
func (svr *server) deleteAccountListener(evt *eventpb.Event) {
	accountId := string(evt.Data)
	blogs, err := svr.getBlogPostByAuthor(accountId)
	if err == nil {
		for i := 0; i < len(blogs); i++ {
			// remove the post...
			err := svr.deleteBlogPost(accountId, blogs[i].Uuid)
			if err != nil {
				fmt.Println("post ", blogs[i].Uuid, "was removed")
			}
		}
	}
}

/**
 * Return a new blogPost
 */
func (svr *server) getBlogPost(uuid string) (*blogpb.BlogPost, error) {
	// Delete a blog...
	blog := new(blogpb.BlogPost)
	jsonStr, err := svr.store.GetItem(uuid)
	if err != nil {
		return nil, err
	}

	err = jsonpb.UnmarshalString(string(jsonStr), blog)
	if err != nil {
		return nil, err
	}

	return blog, nil
}

func (svr *server) getBlogPostByAuthor(author string) ([]*blogpb.BlogPost, error) {

	blog_posts := make([]*blogpb.BlogPost, 0)
	blogs_, err := svr.store.GetItem(author)

	ids := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(blogs_, &ids)
		if err != nil {
			return nil, err
		}
	}

	// Retreive the list of blogs.
	for i := 0; i < len(ids); i++ {
		jsonStr, err := svr.store.GetItem(ids[i])
		instance := new(blogpb.BlogPost)
		if err == nil {
			err := jsonpb.UnmarshalString(string(jsonStr), instance)
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
func (svr *server) getSubComment(uuid string, comment *blogpb.Comment) (*blogpb.Comment, error) {
	if comment.Comments == nil {
		return nil, errors.New("no answer was found for that comment")
	}

	for i := 0; i < len(comment.Comments); i++ {
		comment := comment.Comments[i]
		if uuid == comment.Uuid {
			return comment, nil
		}
		if comment.Comments != nil {
			comment_, err := svr.getSubComment(uuid, comment)
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
func (svr *server) getBlogComment(parentUuid string, blog *blogpb.BlogPost) (*blogpb.Comment, error) {
	// Here I will try to find the comment...
	for i := 0; i < len(blog.Comments); i++ {
		comment := blog.Comments[i]
		if comment.Uuid == parentUuid {
			return comment, nil
		}

		// try to get the comment in sub-comment (answer)
		comment, err := svr.getSubComment(parentUuid, comment)
		if err == nil && comment != nil {
			return comment, nil
		}
	}

	return nil, errors.New("no comment was found for that blog")
}

/**
 * So here I will delete the
 */
func (svr *server) deleteBlogPost(author, uuid string) error {

	blog, err := svr.getBlogPost(uuid)
	if err != nil {
		return err
	}

	if author != blog.Author {
		return errors.New("only blog author can delete it blog")
	}

	// first I will remove it from it author indexation.
	blogs_, err := svr.store.GetItem(blog.Author)

	ids := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(blogs_, &ids)
		if err != nil {
			return err
		}
	}

	ids = Utility.RemoveString(ids, uuid)

	// Now I will save the value.
	blogs__, err := json.Marshal(ids)
	if err != nil {
		return err
	}

	err = svr.store.SetItem(blog.Author, blogs__)
	if err != nil {
		return err
	}

	// Now I will delete the blog.
	return svr.store.RemoveItem(uuid)
}

/**
 * Save a blog post.
 */
func (svr *server) saveBlogPost(author string, blogPost *blogpb.BlogPost) error {
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(blogPost)
	if err != nil {
		return err
	}

	// set the new one.
	err = svr.store.SetItem(blogPost.Uuid, []byte(jsonStr))
	if err != nil {
		return err
	}

	// I will asscociate the author with that post...
	blogs_, err := svr.store.GetItem(author)
	blogs := make([]string, 0)
	if err == nil {
		json.Unmarshal(blogs_, &blogs)
	}

	if !Utility.Contains(blogs, blogPost.Uuid) {
		blogs = append(blogs, blogPost.Uuid)
	}

	// Now I will save the value.
	blogs__, err := json.Marshal(blogs)
	if err != nil {
		return err
	}

	err = svr.store.SetItem(author, blogs__)
	if err != nil {
		return err
	}

	// Now I will set the search information for conversations...
	err = svr.search_engine.IndexJsonObject(svr.Root+"/blogs/search_data", jsonStr, blogPost.Language, "uuid", []string{"keywords"}, jsonStr)
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
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "The Hello world of gRPC service!"
	s_impl.Keywords = []string{"Example", "Blog", "Post", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 2)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

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

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	s_impl.Permissions[0] = map[string]interface{}{"action": "/blog.BlogService/SaveBlogPost", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[1] =  map[string]interface{}{"action": "/blog.BlogService/DeleteBlogPost", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}

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
