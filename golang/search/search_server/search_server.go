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
	Mac             string
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
	// search_server-signed X.509 public keys for distribution
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
	Dependencies       []string      // The list of services needed by this services.

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
func (search_server *server) GetId() string {
	return search_server.Id
}
func (search_server *server) SetId(id string) {
	search_server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (search_server *server) GetName() string {
	return search_server.Name
}
func (search_server *server) SetName(name string) {
	search_server.Name = name
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The description of the service
func (search_server *server) GetDescription() string {
	return search_server.Description
}
func (search_server *server) SetDescription(description string) {
	search_server.Description = description
}

func (search_server *server) GetRepositories() []string {
	return search_server.Repositories
}
func (search_server *server) SetRepositories(repositories []string) {
	search_server.Repositories = repositories
}

func (search_server *server) GetDiscoveries() []string {
	return search_server.Discoveries
}
func (search_server *server) SetDiscoveries(discoveries []string) {
	search_server.Discoveries = discoveries
}

// The list of keywords of the services.
func (search_server *server) GetKeywords() []string {
	return search_server.Keywords
}
func (search_server *server) SetKeywords(keywords []string) {
	search_server.Keywords = keywords
}

// Dist
func (search_server *server) Dist(path string) (string, error) {

	return globular.Dist(path, search_server)
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

func (search_server *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (search_server *server) GetPath() string {
	return search_server.Path
}
func (search_server *server) SetPath(path string) {
	search_server.Path = path
}

// The path of the .proto file.
func (search_server *server) GetProto() string {
	return search_server.Proto
}
func (search_server *server) SetProto(proto string) {
	search_server.Proto = proto
}

// The gRpc port.
func (search_server *server) GetPort() int {
	return search_server.Port
}
func (search_server *server) SetPort(port int) {
	search_server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (search_server *server) GetProxy() int {
	return search_server.Proxy
}
func (search_server *server) SetProxy(proxy int) {
	search_server.Proxy = proxy
}

// Can be one of http/https/tls
func (search_server *server) GetProtocol() string {
	return search_server.Protocol
}
func (search_server *server) SetProtocol(protocol string) {
	search_server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (search_server *server) GetAllowAllOrigins() bool {
	return search_server.AllowAllOrigins
}
func (search_server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	search_server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (search_server *server) GetAllowedOrigins() string {
	return search_server.AllowedOrigins
}

func (search_server *server) SetAllowedOrigins(allowedOrigins string) {
	search_server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (search_server *server) GetDomain() string {
	return search_server.Domain
}
func (search_server *server) SetDomain(domain string) {
	search_server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (search_server *server) GetTls() bool {
	return search_server.TLS
}
func (search_server *server) SetTls(hasTls bool) {
	search_server.TLS = hasTls
}

// The certificate authority file
func (search_server *server) GetCertAuthorityTrust() string {
	return search_server.CertAuthorityTrust
}
func (search_server *server) SetCertAuthorityTrust(ca string) {
	search_server.CertAuthorityTrust = ca
}

// The certificate file.
func (search_server *server) GetCertFile() string {
	return search_server.CertFile
}
func (search_server *server) SetCertFile(certFile string) {
	search_server.CertFile = certFile
}

// The key file.
func (search_server *server) GetKeyFile() string {
	return search_server.KeyFile
}
func (search_server *server) SetKeyFile(keyFile string) {
	search_server.KeyFile = keyFile
}

// The service version
func (search_server *server) GetVersion() string {
	return search_server.Version
}
func (search_server *server) SetVersion(version string) {
	search_server.Version = version
}

// The publisher id.
func (search_server *server) GetPublisherId() string {
	return search_server.PublisherId
}
func (search_server *server) SetPublisherId(publisherId string) {
	search_server.PublisherId = publisherId
}

func (search_server *server) GetKeepUpToDate() bool {
	return search_server.KeepUpToDate
}
func (search_server *server) SetKeepUptoDate(val bool) {
	search_server.KeepUpToDate = val
}

func (search_server *server) GetKeepAlive() bool {
	return search_server.KeepAlive
}
func (search_server *server) SetKeepAlive(val bool) {
	search_server.KeepAlive = val
}

func (search_server *server) GetPermissions() []interface{} {
	return search_server.Permissions
}
func (search_server *server) SetPermissions(permissions []interface{}) {
	search_server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (search_server *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewSearchService_Client", search_client.NewSearchService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", search_server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	search_server.grpcServer, err = globular.InitGrpcServer(search_server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// Init the seach engine.
	search_server.search_engine = new(search_engine.XapianEngine)

	return nil

}

// Save the configuration values.
func (search_server *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", search_server)
}

func (search_server *server) StartService() error {
	return globular.StartService(search_server, search_server.grpcServer)
}

func (search_server *server) StopService() error {
	return globular.StopService(search_server, search_server.grpcServer)
}

func (search_server *server) Stop(context.Context, *searchpb.StopRequest) (*searchpb.StopResponse, error) {
	return &searchpb.StopResponse{}, search_server.StopService()
}

// Return the underlying engine version.
func (search_server *server) GetEngineVersion(ctx context.Context, rqst *searchpb.GetEngineVersionRequest) (*searchpb.GetEngineVersionResponse, error) {

	return &searchpb.GetEngineVersionResponse{
		Message: search_server.search_engine.GetVersion(),
	}, nil
}

// Remove a document from the db
func (search_server *server) DeleteDocument(ctx context.Context, rqst *searchpb.DeleteDocumentRequest) (*searchpb.DeleteDocumentResponse, error) {
	err := search_server.search_engine.DeleteDocument(rqst.Path, rqst.Id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &searchpb.DeleteDocumentResponse{}, nil
}

// Return the number of document in a database.
func (search_server *server) Count(ctx context.Context, rqst *searchpb.CountRequest) (*searchpb.CountResponse, error) {

	return &searchpb.CountResponse{
		Result: search_server.search_engine.Count(rqst.Path),
	}, nil

}

// Search documents
func (search_server *server) SearchDocuments(rqst *searchpb.SearchDocumentsRequest, stream searchpb.SearchService_SearchDocumentsServer) error {
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

	data, err := search_server.cache.GetItem(resultKey)
	if err == nil {
		results = new(searchpb.SearchResults)
		err = jsonpb.UnmarshalString(string(data), results)
	} else {
		results, err := search_server.search_engine.SearchDocuments(rqst.Paths, rqst.Language, rqst.Fields, rqst.Query, rqst.Offset, rqst.PageSize, rqst.SnippetLength)
		if err != nil {
			return err
		}

		// Keep the result in the cache.
		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(results)
		if err != nil {
			return err
		}

		search_server.cache.SetItem(resultKey, []byte(jsonStr))
	}

	stream.Send(&searchpb.SearchDocumentsResponse{
		Results: results,
	})

	return nil

}

/**
 * Index the content of a dir and it content.
 */
func (search_server *server) IndexDir(ctx context.Context, rqst *searchpb.IndexDirRequest) (*searchpb.IndexDirResponse, error) {

	err := search_server.search_engine.IndexDir(rqst.DbPath, rqst.DirPath, rqst.Language)
	if err != nil {
		return nil, err
	}

	return &searchpb.IndexDirResponse{}, nil
}

// Indexation of a text (docx, pdf,xlsx...) file.
func (search_server *server) IndexFile(ctx context.Context, rqst *searchpb.IndexFileRequest) (*searchpb.IndexFileResponse, error) {

	err := search_server.search_engine.IndexFile(rqst.DbPath, rqst.FilePath, rqst.Language)
	if err != nil {
		return nil, err
	}

	return &searchpb.IndexFileResponse{}, nil
}

// That function is use to index JSON object/array of object
func (search_server *server) IndexJsonObject(ctx context.Context, rqst *searchpb.IndexJsonObjectRequest) (*searchpb.IndexJsonObjectResponse, error) {

	err := search_server.search_engine.IndexJsonObject(rqst.Path, rqst.JsonStr, rqst.Language, rqst.Id, rqst.Indexs, rqst.Data)
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
	s_impl.Dependencies = make([]string, 0)
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
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
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
