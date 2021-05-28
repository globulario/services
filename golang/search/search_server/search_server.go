package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/search/search_client"
	"github.com/globulario/services/golang/search/searchpb"
	"github.com/globulario/services/golang/storage/storage_store"

	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"

	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc/codes"

	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/globulario/services/golang/search/search_engine"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10041
	defaultProxy = 10042

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
	Name            string
	Proto           string
	Path            string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Domain          string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	// self-signed X.509 public keys for distribution
	CertFile string
	// a private RSA key to sign and authenticate the public key
	KeyFile string
	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherId        string
	KeepUpToDate       bool
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.

	// The grpc server.
	grpcServer *grpc.Server

	// Specific to file server.
	Root string

	// The cache will contain the result of query.
	cache *storage_store.BigCache_store

	search_engine search_engine.SearchEngine
}

// Globular services implementation...
// The id of a particular service instance.
func (self *server) GetId() string {
	return self.Id
}
func (self *server) SetId(id string) {
	self.Id = id
}

// The name of a service, must be the gRpc Service name.
func (self *server) GetName() string {
	return self.Name
}
func (self *server) SetName(name string) {
	self.Name = name
}

// The description of the service
func (self *server) GetDescription() string {
	return self.Description
}
func (self *server) SetDescription(description string) {
	self.Description = description
}

func (self *server) GetRepositories() []string {
	return self.Repositories
}
func (self *server) SetRepositories(repositories []string) {
	self.Repositories = repositories
}

func (self *server) GetDiscoveries() []string {
	return self.Discoveries
}
func (self *server) SetDiscoveries(discoveries []string) {
	self.Discoveries = discoveries
}

// The list of keywords of the services.
func (self *server) GetKeywords() []string {
	return self.Keywords
}
func (self *server) SetKeywords(keywords []string) {
	self.Keywords = keywords
}

// Dist
func (self *server) Dist(path string) (string, error) {

	return globular.Dist(path, self)
}

func (self *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (self *server) GetPath() string {
	return self.Path
}
func (self *server) SetPath(path string) {
	self.Path = path
}

// The path of the .proto file.
func (self *server) GetProto() string {
	return self.Proto
}
func (self *server) SetProto(proto string) {
	self.Proto = proto
}

// The gRpc port.
func (self *server) GetPort() int {
	return self.Port
}
func (self *server) SetPort(port int) {
	self.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (self *server) GetProxy() int {
	return self.Proxy
}
func (self *server) SetProxy(proxy int) {
	self.Proxy = proxy
}

// Can be one of http/https/tls
func (self *server) GetProtocol() string {
	return self.Protocol
}
func (self *server) SetProtocol(protocol string) {
	self.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (self *server) GetAllowAllOrigins() bool {
	return self.AllowAllOrigins
}
func (self *server) SetAllowAllOrigins(allowAllOrigins bool) {
	self.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (self *server) GetAllowedOrigins() string {
	return self.AllowedOrigins
}

func (self *server) SetAllowedOrigins(allowedOrigins string) {
	self.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (self *server) GetDomain() string {
	return self.Domain
}
func (self *server) SetDomain(domain string) {
	self.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (self *server) GetTls() bool {
	return self.TLS
}
func (self *server) SetTls(hasTls bool) {
	self.TLS = hasTls
}

// The certificate authority file
func (self *server) GetCertAuthorityTrust() string {
	return self.CertAuthorityTrust
}
func (self *server) SetCertAuthorityTrust(ca string) {
	self.CertAuthorityTrust = ca
}

// The certificate file.
func (self *server) GetCertFile() string {
	return self.CertFile
}
func (self *server) SetCertFile(certFile string) {
	self.CertFile = certFile
}

// The key file.
func (self *server) GetKeyFile() string {
	return self.KeyFile
}
func (self *server) SetKeyFile(keyFile string) {
	self.KeyFile = keyFile
}

// The service version
func (self *server) GetVersion() string {
	return self.Version
}
func (self *server) SetVersion(version string) {
	self.Version = version
}

// The publisher id.
func (self *server) GetPublisherId() string {
	return self.PublisherId
}
func (self *server) SetPublisherId(publisherId string) {
	self.PublisherId = publisherId
}

func (self *server) GetKeepUpToDate() bool {
	return self.KeepUpToDate
}
func (self *server) SetKeepUptoDate(val bool) {
	self.KeepUpToDate = val
}

func (self *server) GetKeepAlive() bool {
	return self.KeepAlive
}
func (self *server) SetKeepAlive(val bool) {
	self.KeepAlive = val
}

func (self *server) GetPermissions() []interface{} {
	return self.Permissions
}
func (self *server) SetPermissions(permissions []interface{}) {
	self.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (self *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewSearchService_Client", search_client.NewSearchService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", self)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	self.grpcServer, err = globular.InitGrpcServer(self, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// Init the seach engine.
	self.search_engine = new(search_engine.XapianEngine)

	return nil

}

// Save the configuration values.
func (self *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", self)
}

func (self *server) StartService() error {
	return globular.StartService(self, self.grpcServer)
}

func (self *server) StopService() error {
	return globular.StopService(self, self.grpcServer)
}

func (self *server) Stop(context.Context, *searchpb.StopRequest) (*searchpb.StopResponse, error) {
	return &searchpb.StopResponse{}, self.StopService()
}

// Return the underlying engine version.
func (self *server) GetEngineVersion(ctx context.Context, rqst *searchpb.GetEngineVersionRequest) (*searchpb.GetEngineVersionResponse, error) {

	return &searchpb.GetEngineVersionResponse{
		Message: self.search_engine.GetVersion(),
	}, nil
}

// Remove a document from the db
func (self *server) DeleteDocument(ctx context.Context, rqst *searchpb.DeleteDocumentRequest) (*searchpb.DeleteDocumentResponse, error) {
	err := self.search_engine.DeleteDocument(rqst.Path, rqst.Id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &searchpb.DeleteDocumentResponse{}, nil
}

// Return the number of document in a database.
func (self *server) Count(ctx context.Context, rqst *searchpb.CountRequest) (*searchpb.CountResponse, error) {

	return &searchpb.CountResponse{
		Result: self.search_engine.Count(rqst.Path),
	}, nil

}

// Search documents
func (self *server) SearchDocuments(rqst *searchpb.SearchDocumentsRequest, stream searchpb.SearchService_SearchDocumentsServer) error {
	results := new(searchpb.SearchResults)
	var err error

	resultKey := ""
	// Set the list of path
	for i := 0; i < len(rqst.Paths); i++ {
		resultKey += rqst.Paths[i]
	}

	// Set the list fields.
	for i := 0; i < len(rqst.Fields); i++ {
		resultKey += rqst.Fields[i]
	}
	resultKey += Utility.ToString(rqst.Query)
	resultKey += Utility.ToString(rqst.Offset)
	resultKey += Utility.ToString(rqst.PageSize)

	// Set as Hash key
	resultKey = Utility.GenerateUUID(resultKey)

	data, err := self.cache.GetItem(resultKey)
	if err == nil {
		results = new(searchpb.SearchResults)
		err = jsonpb.UnmarshalString(string(data), results)
	} else {
		results, err := self.search_engine.SearchDocuments(rqst.Paths, rqst.Language, rqst.Fields, rqst.Query, rqst.Offset, rqst.PageSize, rqst.SnippetLength)
		if err != nil {
			return err
		}

		// Keep the result in the cache.
		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(results)
		if err != nil {
			return err
		}

		self.cache.SetItem(resultKey, []byte(jsonStr))
	}

	stream.Send(&searchpb.SearchDocumentsResponse{
		Results: results,
	})

	return nil

}

/**
 * Index the content of a dir and it content.
 */
func (self *server) IndexDir(ctx context.Context, rqst *searchpb.IndexDirRequest) (*searchpb.IndexDirResponse, error) {

	err := self.search_engine.IndexDir(rqst.DbPath, rqst.DirPath, rqst.Language)
	if err != nil {
		return nil, err
	}

	return &searchpb.IndexDirResponse{}, nil
}

// Indexation of a text (docx, pdf,xlsx...) file.
func (self *server) IndexFile(ctx context.Context, rqst *searchpb.IndexFileRequest) (*searchpb.IndexFileResponse, error) {

	err := self.search_engine.IndexFile(rqst.DbPath, rqst.FilePath, rqst.Language)
	if err != nil {
		return nil, err
	}

	return &searchpb.IndexFileResponse{}, nil
}

// That function is use to index JSON object/array of object
func (self *server) IndexJsonObject(ctx context.Context, rqst *searchpb.IndexJsonObjectRequest) (*searchpb.IndexJsonObjectResponse, error) {

	err := self.search_engine.IndexJsonObject(rqst.Path, rqst.JsonStr, rqst.Language, rqst.Id, rqst.Indexs, rqst.Data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &searchpb.IndexJsonObjectResponse{}, nil

}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(searchpb.File_search_proto.Services().Get(0).FullName())
	s_impl.Proto = searchpb.File_search_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Permissions = make([]interface{}, 0)

	// TODO set it from the program arguments...
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)

	// Create the new big cache.
	s_impl.cache = storage_store.NewBigCache_store()
	err := s_impl.cache.Open("")
	if err != nil {
		fmt.Println(err)
	}
	// set the logger.

	// Set the root path if is pass as argument.
	if len(os.Args) > 2 {
		s_impl.Root = os.Args[2]
	}

	// Here I will retreive the list of connections from file if there are some...
	err = s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id, err)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the search services
	searchpb.RegisterSearchServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
