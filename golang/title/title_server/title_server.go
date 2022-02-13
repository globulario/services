package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/blevesearch/bleve"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/title_client"
	"github.com/globulario/services/golang/title/titlepb"
	"github.com/golang/protobuf/jsonpb"

	//"github.com/globulario/services/golang/interceptors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
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
	State           string
	ModTime         int64

	TLS bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The grpc server.
	grpcServer *grpc.Server

	// Contain indexation.
	indexs map[string]bleve.Index

	// Contain the file and title asscociation.
	associations map[string]*storage_store.Badger_store
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
	Utility.RegisterFunction("NewTitleService_Client", title_client.NewTitleService_Client)

	// Get the configuration path.
	err := globular.InitService(svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr /*interceptors.ServerUnaryInterceptor, interceptors.ServerStreamIntercepto*/, nil, nil)
	if err != nil {
		return err
	}

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

/////////////////////// title specific function /////////////////////////////////
/**
 * Return indexation for a given path...
 */
func (svr *server) getIndex(path string) (bleve.Index, error) {
	if svr.indexs[path] == nil {
		index, err := bleve.Open(path) // try to open existing index.
		if err != nil {
			mapping := bleve.NewIndexMapping()
			var err error
			index, err = bleve.New(path, mapping)
			if err != nil {
				return nil, err
			}
		}

		if svr.indexs == nil {
			svr.indexs = make(map[string]bleve.Index, 0)
		}

		svr.indexs[path] = index
	}

	return svr.indexs[path], nil
}

func (svr *server) getTitleById(indexPath, titleId string) (*titlepb.Title, error) {

	index, err := svr.getIndex(indexPath)
	if err != nil {
		return nil, err
	}

	query := bleve.NewQueryStringQuery(titleId)
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// Now from the result I will
	if searchResult.Total == 0 {
		return nil, errors.New("No matches")
	}

	var title *titlepb.Title
	// Now I will return the data
	for _, val := range searchResult.Hits {
		id := val.ID
		raw, err := index.GetInternal([]byte(id))
		if err != nil {
			return nil, err
		}
		title = new(titlepb.Title)
		jsonpb.UnmarshalString(string(raw), title)

	}

	return title, nil
}

// Get a title by a given id.
func (svr *server) GetTitleById(ctx context.Context, rqst *titlepb.GetTitleByIdRequest) (*titlepb.GetTitleByIdResponse, error) {

	title, err := svr.getTitleById(rqst.IndexPath, rqst.TitleId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	filePaths := []string{}

	// get the list of associated files if there some...
	if svr.associations != nil {
		if svr.associations[rqst.IndexPath] != nil {
			data, err := svr.associations[rqst.IndexPath].GetItem(rqst.TitleId)
			if err == nil {
				association := new(fileTileAssociation)
				err = json.Unmarshal(data, association)
				if err == nil {
					// In that case I will get the files...
					filePaths = association.Paths
				}
			}
		}
	}

	return &titlepb.GetTitleByIdResponse{
		Title:      title,
		FilesPaths: filePaths,
	}, nil
}

// Insert a title in the database or update it if it already exist.
func (svr *server) CreateTitle(ctx context.Context, rqst *titlepb.CreateTitleRequest) (*titlepb.CreateTitleResponse, error) {
	if rqst.Title == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no title was given")))

	}
	fmt.Println("create new title with name ", rqst.Title.Name)
	// So here Will create the indexation for the movie...
	index, err := svr.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Index the title and put it in the search engine.
	err = index.Index(rqst.Title.ID, rqst.Title)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Title)

	if err == nil {
		err = index.SetInternal([]byte(rqst.Title.ID), []byte(jsonStr))
	}

	return &titlepb.CreateTitleResponse{}, nil
}

// Delete a title from the database.
func (svr *server) DeleteTitle(ctx context.Context, rqst *titlepb.DeleteTitleRequest) (*titlepb.DeleteTitleResponse, error) {

	index, err := svr.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.Delete(rqst.TitleId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(rqst.TitleId))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO remove asscociated files...

	return &titlepb.DeleteTitleResponse{}, nil
}

// File and title association.
type fileTileAssociation struct {
	ID     string
	Titles []string // contain the titles ids
	Paths  []string // list of file path's where file can be found on the local disck.
}

// Associate a file and a title info, so file can be found from title informations...
func (svr *server) AssociateFileWithTitle(ctx context.Context, rqst *titlepb.AssociateFileWithTitleRequest) (*titlepb.AssociateFileWithTitleResponse, error) {

	// so the first thing I will do is to get the file on the disc.
	filePath := rqst.FilePath
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if !Utility.Exists(filePath) {
		// Here I will try to get it from the users dirs...
		if strings.HasPrefix(filePath, "/users/") || strings.HasPrefix(filePath, "/applications/") {
			filePath = config.GetDataDir() + "/files" + filePath
		}
	}
	if !Utility.Exists(filePath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+filePath)))
	}

	// I will use the file checksum as file id...
	checksum := Utility.CreateFileChecksum(filePath)

	if svr.associations == nil {
		svr.associations = make(map[string]*storage_store.Badger_store)
	}

	if svr.associations[rqst.IndexPath] == nil {
		svr.associations[rqst.IndexPath] = storage_store.NewBadger_store()
		// open in it own thread
		svr.associations[rqst.IndexPath].Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := svr.associations[rqst.IndexPath].GetItem(checksum)
	association := &fileTileAssociation{ID: checksum, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Append the path if not already there.
	if !Utility.Contains(association.Paths, rqst.FilePath) {
		association.Paths = append(association.Paths, rqst.FilePath)
	}

	// Append the title if not aready exist.
	if !Utility.Contains(association.Titles, rqst.TitleId) {
		association.Titles = append(association.Titles, rqst.TitleId)
	}

	// Now I will set back the item in the store.
	data, _ = json.Marshal(association)
	err = svr.associations[rqst.IndexPath].SetItem(checksum, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Also index it with it title.
	err = svr.associations[rqst.IndexPath].SetItem(rqst.TitleId, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return empty response.
	return &titlepb.AssociateFileWithTitleResponse{}, nil
}

// Dissociate a file and a title info, so file can be found from title informations...
func (svr *server) DissociateFileWithTitle(ctx context.Context, rqst *titlepb.DissociateFileWithTitleRequest) (*titlepb.DissociateFileWithTitleResponse, error) {

	// so the first thing I will do is to get the file on the disc.
	filePath := rqst.FilePath
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if !Utility.Exists(filePath) {
		// Here I will try to get it from the users dirs...
		if strings.HasPrefix(filePath, "/users/") || strings.HasPrefix(filePath, "/applications/") {
			filePath = config.GetDataDir() + "/files" + filePath
		}
	}
	if !Utility.Exists(filePath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+filePath)))
	}

	// I will use the file checksum as file id...
	checksum := Utility.CreateFileChecksum(filePath)
	if svr.associations == nil {
		svr.associations = make(map[string]*storage_store.Badger_store)
	}

	if svr.associations[rqst.IndexPath] == nil {
		svr.associations[rqst.IndexPath] = storage_store.NewBadger_store()
		// open in it own thread
		svr.associations[rqst.IndexPath].Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := svr.associations[rqst.IndexPath].GetItem(checksum)
	association := &fileTileAssociation{ID: checksum, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// so here i will remove the path from the list of path.
	association.Paths = Utility.RemoveString(association.Paths, rqst.FilePath)
	if len(association.Paths) == 0 {
		svr.associations[rqst.IndexPath].RemoveItem(rqst.TitleId)
		svr.associations[rqst.IndexPath].RemoveItem(checksum)
	}

	return &titlepb.DissociateFileWithTitleResponse{}, nil

}

// Return the list of titles asscociate with a file.
func (svr *server) GetFileTitles(ctx context.Context, rqst *titlepb.GetFileTitlesRequest) (*titlepb.GetFileTitlesResponse, error) {
	// So here I will get the list of titles asscociated with a file...
	filePath := rqst.FilePath
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if !Utility.Exists(filePath) {
		// Here I will try to get it from the users dirs...
		if strings.HasPrefix(filePath, "/users/") || strings.HasPrefix(filePath, "/applications/") {
			filePath = config.GetDataDir() + "/files" + filePath
		}
	}
	if !Utility.Exists(filePath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+filePath)))
	}

	// I will use the file checksum as file id...
	checksum := Utility.CreateFileChecksum(filePath)
	if svr.associations == nil {
		svr.associations = make(map[string]*storage_store.Badger_store)
	}

	if svr.associations[rqst.IndexPath] == nil {
		svr.associations[rqst.IndexPath] = storage_store.NewBadger_store()
		// open in it own thread
		svr.associations[rqst.IndexPath].Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := svr.associations[rqst.IndexPath].GetItem(checksum)
	association := &fileTileAssociation{ID: checksum, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	titles := make([]*titlepb.Title, 0)
	for i := 0; i < len(association.Titles); i++ {
		title, err := svr.getTitleById(rqst.IndexPath, association.Titles[i])
		if err == nil {
			titles = append(titles, title)
		}
	}

	return &titlepb.GetFileTitlesResponse{Titles: titles}, nil
}

// Return the list of files associate with a title
func (svr *server) GetTitleFiles(ctx context.Context, rqst *titlepb.GetTitleFilesRequest) (*titlepb.GetTitleFilesResponse, error) {

	// I will use the file checksum as file id...
	if svr.associations == nil {
		svr.associations = make(map[string]*storage_store.Badger_store)
	}

	if svr.associations[rqst.IndexPath] == nil {
		svr.associations[rqst.IndexPath] = storage_store.NewBadger_store()
		// open in it own thread
		svr.associations[rqst.IndexPath].Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := svr.associations[rqst.IndexPath].GetItem(rqst.TitleId)
	association := &fileTileAssociation{ID: "", Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Return the store association values...
	return &titlepb.GetTitleFilesResponse{FilePaths: association.Paths}, nil

}

// Search document infos...
func (svr *server) SearchTitles(rqst *titlepb.SearchTitlesRequest, stream titlepb.TitleService_SearchTitlesServer) error {

	index, err := svr.getIndex(rqst.IndexPath)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := bleve.NewQueryStringQuery(rqst.Query)
	request := bleve.NewSearchRequest(query)
	request.Highlight = bleve.NewHighlightWithStyle("html")
	request.Fields = rqst.Fields
	result, err := index.Search(request)

	if err != nil { // an empty query would cause this
		return err
	}

	// The first return message will be the summary of the result...
	summary := new(titlepb.SearchSummary)
	summary.Query = rqst.Query // set back the input query.
	summary.Took = result.Took.Milliseconds()
	summary.Total = result.Total

	// Here I will send the summary...
	stream.Send(&titlepb.SearchTitlesResponse{
		Result: &titlepb.SearchTitlesResponse_Summary{
			Summary: summary,
		},
	})

	// Now I will generate the hits informations...
	for i, hit := range result.Hits {
		id := hit.ID

		hit_ := new(titlepb.SearchHit)
		hit_.Score = hit.Score
		hit_.Index = int32(i)
		hit_.Snippets = make([]*titlepb.Snippet, 0)

		// Now I will extract fragment for fields...
		for fragmentField, fragments := range hit.Fragments {
			snippet := new(titlepb.Snippet)
			snippet.Field = fragmentField
			snippet.Fragments = make([]string, 0)
			for _, fragment := range fragments {
				snippet.Fragments = append(snippet.Fragments, fragment)
			}
		}

		// Here I will get the title itself.
		raw, err := index.GetInternal([]byte(id))
		if err != nil {
			log.Fatal("Trouble getting internal doc:", err)
		}

		title := new(titlepb.Title)
		err = jsonpb.UnmarshalString(string(raw), title)
		if err == nil {
			hit_.Title = title;
			// Here I will send the search result...
			stream.Send(&titlepb.SearchTitlesResponse{
				Result: &titlepb.SearchTitlesResponse_Hit{
					Hit: hit_,
				},
			})
		}
	}
	return nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "echo_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(titlepb.File_title_proto.Services().Get(0).FullName())
	s_impl.Proto = titlepb.File_title_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Functionalities to find Title information and asscociate it with file."
	s_impl.Keywords = []string{"Search", "Movie", "Title", "Episode", "MultiMedia", "IMDB"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.KeepAlive = true

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	// Register the echo services
	titlepb.RegisterTitleServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
