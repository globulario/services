package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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
	Plaform         string
	Checksum        string
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

/**
 *  The character - can't be part of uuid, bleeve use it in it query syntax so I will get rid of it
 **/
func generateUUID(id string) string {
	uuid := Utility.GenerateUUID(id)
	uuid = strings.ReplaceAll(uuid, "-", "_")
	return uuid
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

// The path of the configuration.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
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

func (svr *server) GetChecksum() string {

	return svr.Checksum
}

func (svr *server) SetChecksum(checksum string) {
	svr.Checksum = checksum
}

func (svr *server) GetPlatform() string {
	return svr.Plaform
}

func (svr *server) SetPlatform(platform string) {
	svr.Plaform = platform
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

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewTitleService_Client", title_client.NewTitleService_Client)

	// Get the configuration path.
	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	srv.grpcServer, err = globular.InitGrpcServer(srv /*interceptors.ServerUnaryInterceptor, interceptors.ServerStreamIntercepto*/, nil, nil)
	if err != nil {
		return err
	}

	return nil

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

/////////////////////// title specific function /////////////////////////////////
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

func (srv *server) getTitleById(indexPath, titleId string) (*titlepb.Title, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	index, err := srv.getIndex(indexPath)
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
		if titleId == val.ID {
			raw, err := index.GetInternal([]byte(val.ID))
			if err != nil {
				return nil, err
			}
			title = new(titlepb.Title)
			jsonpb.UnmarshalString(string(raw), title)
			break
		}

	}

	return title, nil
}

// Get a title by a given id.
func (srv *server) GetTitleById(ctx context.Context, rqst *titlepb.GetTitleByIdRequest) (*titlepb.GetTitleByIdResponse, error) {

	title, err := srv.getTitleById(rqst.IndexPath, generateUUID(rqst.TitleId))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	filePaths := []string{}
	// get the list of associated files if there some...
	if srv.associations != nil {
		if srv.associations[rqst.IndexPath] != nil {
			data, err := srv.associations[rqst.IndexPath].GetItem(rqst.TitleId)
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
func (srv *server) CreateTitle(ctx context.Context, rqst *titlepb.CreateTitleRequest) (*titlepb.CreateTitleResponse, error) {
	if rqst.Title == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no title was given")))

	}

	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Index the title and put it in the search engine.
	rqst.Title.UUID = generateUUID(rqst.Title.ID)
	err = index.Index(rqst.Title.UUID, rqst.Title)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will get the title thumbnail...
	thumbnail_path := os.TempDir() + "/" + rqst.Title.Poster.URL[strings.LastIndex(rqst.Title.Poster.URL, "/")+1:]
	defer os.Remove(thumbnail_path)

	// Dowload the file.
	err = Utility.DownloadFile(rqst.Title.Poster.URL, thumbnail_path)
	if err == nil {
		thumbnail, err := Utility.CreateThumbnail(thumbnail_path, 300, 180)
		if err == nil {
			rqst.Title.Poster.ContentUrl = thumbnail
		}
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Title)

	if err == nil {
		index.SetInternal([]byte(rqst.Title.UUID), []byte(jsonStr))
	}

	return &titlepb.CreateTitleResponse{}, nil
}

// Delete a title from the database.
func (srv *server) DeleteTitle(ctx context.Context, rqst *titlepb.DeleteTitleRequest) (*titlepb.DeleteTitleResponse, error) {

	// Remove all file indexation...
	paths, err := srv.getTitleFiles(rqst.IndexPath, rqst.TitleId)
	if err == nil {
		for i := 0; i < len(paths); i++ {
			srv.dissociateFileWithTitle(rqst.IndexPath, rqst.TitleId, paths[i])
		}
	}

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := generateUUID(rqst.TitleId)
	err = index.Delete(id)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(id))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.DeleteTitleResponse{}, nil
}

// File and title association.
type fileTileAssociation struct {
	ID     string
	Titles []string // contain the titles ids
	Paths  []string // list of file path's where file can be found on the local disck.
}

// Associate a file and a title info, so file can be found from title informations...
func (srv *server) AssociateFileWithTitle(ctx context.Context, rqst *titlepb.AssociateFileWithTitleRequest) (*titlepb.AssociateFileWithTitleResponse, error) {

	// so the first thing I will do is to get the file on the disc.
	absolutefilePath := rqst.FilePath
	absolutefilePath = strings.ReplaceAll(absolutefilePath, "\\", "/")

	if !Utility.Exists(absolutefilePath) {
		// Here I will try to get it from the users dirs...
		if strings.HasPrefix(absolutefilePath, "/users/") || strings.HasPrefix(absolutefilePath, "/applications/") {
			absolutefilePath = config.GetDataDir() + "/files" + absolutefilePath
		}

		if !Utility.Exists(absolutefilePath) {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+absolutefilePath)))
		}
	}

	fileInfo, err := os.Stat(absolutefilePath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will set the title url in the media file to keep track of the title
	// information. If the title is lost it will be possible to recreate it from
	// that url.
	if strings.HasSuffix(rqst.IndexPath, "/search/titles") {
		title, err := srv.getTitleById(rqst.IndexPath, generateUUID(rqst.TitleId))
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if title.Poster != nil {
			title.Poster.ContentUrl = title.Poster.URL // set the Content url with the lnk instead of data url to save space.
		}

		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(title)
		if err != nil {
			return nil, err
		}

		encoded := base64.StdEncoding.EncodeToString([]byte(jsonStr))
		if fileInfo.IsDir() {
			err = os.WriteFile(absolutefilePath+"/infos.json", []byte(jsonStr), 0664)
			if err != nil {
				return nil, err
			}
		} else {
			err = Utility.SetMetadata(absolutefilePath, "comment", encoded)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}

	} else if strings.HasSuffix(rqst.IndexPath, "/search/videos") {
		video, err := srv.getVideoById(rqst.IndexPath, generateUUID(rqst.TitleId))
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if video == nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("video with id "+rqst.TitleId+" was not found")))
		}

		if video.Poster != nil {
			video.Poster.ContentUrl = video.Poster.URL // set the Content url with the lnk instead of data url to save space.
		}

		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(video)
		encoded := base64.StdEncoding.EncodeToString([]byte(jsonStr))
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if fileInfo.IsDir() {
			err = os.WriteFile(absolutefilePath+"/infos.json", []byte(jsonStr), 0664)
			if err != nil {
				return nil, err
			}
		} else {
			err = Utility.SetMetadata(absolutefilePath, "comment", encoded)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	var uuid string
	filePath := strings.ReplaceAll(rqst.FilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")

	// Depending if the filePath point to a dir or a file...
	if fileInfo.IsDir() {
		// is a directory
		uuid = generateUUID(filePath)
	} else {
		// is not a directory
		uuid = Utility.CreateFileChecksum(absolutefilePath)
	}

	fmt.Println("associate file ", absolutefilePath, uuid)

	if srv.associations == nil {
		srv.associations = make(map[string]*storage_store.Badger_store)
	}

	if srv.associations[rqst.IndexPath] == nil {
		srv.associations[rqst.IndexPath] = storage_store.NewBadger_store()
		// open in it own thread
		srv.associations[rqst.IndexPath].Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := srv.associations[rqst.IndexPath].GetItem(uuid)
	association := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Append the path if not already there.
	if !Utility.Contains(association.Paths, filePath) {
		association.Paths = append(association.Paths, filePath)
	}

	// Append the title if not aready exist.
	if !Utility.Contains(association.Titles, rqst.TitleId) {
		association.Titles = append(association.Titles, rqst.TitleId)
	}

	// Now I will set back the item in the store.
	data, _ = json.Marshal(association)
	err = srv.associations[rqst.IndexPath].SetItem(uuid, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will get the association for that title.
	data, err = srv.associations[rqst.IndexPath].GetItem(rqst.TitleId)
	association = &fileTileAssociation{ID: rqst.TitleId, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Append the path if not already there.
	if !Utility.Contains(association.Paths, filePath) {
		association.Paths = append(association.Paths, filePath)
	}

	if !Utility.Contains(association.Titles, rqst.TitleId) {
		association.Titles = append(association.Titles, rqst.TitleId)
	}

	data, _ = json.Marshal(association)
	err = srv.associations[rqst.IndexPath].SetItem(rqst.TitleId, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return empty response.
	return &titlepb.AssociateFileWithTitleResponse{}, nil
}

func (srv *server) dissociateFileWithTitle(indexPath, titleId, absoluteFilePath string) error {
	if !Utility.Exists(indexPath) {
		return errors.New("no database found at path " + indexPath)
	}

	// I will use the file checksum as file id...
	var uuid string
	fileInfo, err := os.Stat(absoluteFilePath)
	if err != nil {
		return err
	}

	// here I will remove the absolute part in case of /users /applications
	filePath := strings.ReplaceAll(absoluteFilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")

	// Depending if the filePath point to a dir or a file...
	if fileInfo.IsDir() {
		// is a directory
		uuid = generateUUID(filePath)
	} else {
		// is not a directory
		uuid = Utility.CreateFileChecksum(absoluteFilePath)
	}

	if srv.associations == nil {
		srv.associations = make(map[string]*storage_store.Badger_store)
	}

	if srv.associations[indexPath] == nil {
		srv.associations[indexPath] = storage_store.NewBadger_store()
		// open in it own thread
		srv.associations[indexPath].Open(`{"path":"` + indexPath + `", "name":"titles"}`)
	}

	file_data, err := srv.associations[indexPath].GetItem(uuid)
	file_association := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(file_data, file_association)
		if err != nil {
			return err
		}
	}

	// so here i will remove the path from the list of path.
	file_association.Paths = Utility.RemoveString(file_association.Paths, filePath)
	file_association.Paths = Utility.RemoveString(file_association.Titles, titleId)

	if len(file_association.Paths) == 0 || len(file_association.Titles) == 0 {
		srv.associations[indexPath].RemoveItem(uuid)
	} else {
		// Now I will set back the item in the store.
		data, _ := json.Marshal(file_association)
		err = srv.associations[indexPath].SetItem(uuid, data)
		if err != nil {
			return err
		}
	}

	title_data, err := srv.associations[indexPath].GetItem(titleId)
	title_association := &fileTileAssociation{ID: titleId, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(title_data, title_association)
		if err != nil {
			return err
		}
	}

	// so here i will remove the path from the list of path.
	title_association.Paths = Utility.RemoveString(title_association.Paths, filePath)

	if len(title_association.Paths) == 0 {
		srv.associations[indexPath].RemoveItem(titleId)

		// I will also delete the title itself from the search engine...
		index, err := srv.getIndex(indexPath)
		if err != nil {
			return err
		}

		err = index.Delete(titleId)
		if err != nil {
			return err
		}

		err = index.DeleteInternal([]byte(titleId))
		if err != nil {
			return err
		}

	} else {
		// Now I will set back the item in the store.
		data, _ := json.Marshal(title_association)
		err = srv.associations[indexPath].SetItem(titleId, data)
		if err != nil {
			return err
		}
	}

	return nil

}

// Dissociate a file and a title info, so file can be found from title informations...
func (srv *server) DissociateFileWithTitle(ctx context.Context, rqst *titlepb.DissociateFileWithTitleRequest) (*titlepb.DissociateFileWithTitleResponse, error) {

	// so the first thing I will do is to get the file on the disc.
	absolutefilePath := rqst.FilePath
	absolutefilePath = strings.ReplaceAll(absolutefilePath, "\\", "/")
	if !Utility.Exists(absolutefilePath) {
		// Here I will try to get it from the users dirs...
		if strings.HasPrefix(absolutefilePath, "/users/") || strings.HasPrefix(absolutefilePath, "/applications/") {
			absolutefilePath = config.GetDataDir() + "/files" + absolutefilePath
		}
	}
	if !Utility.Exists(absolutefilePath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+absolutefilePath)))
	}

	err := srv.dissociateFileWithTitle(rqst.IndexPath, rqst.TitleId, absolutefilePath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.DissociateFileWithTitleResponse{}, nil
}

func (srv *server) getFileTitles(indexPath, filePath, absolutePath string) ([]*titlepb.Title, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	// I will use the file checksum as file id...
	var uuid string
	fileInfo, err := os.Stat(absolutePath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if fileInfo.IsDir() {
		// is a directory
		uuid = generateUUID(filePath)
	} else {
		// is not a directory
		uuid = Utility.CreateFileChecksum(absolutePath)
	}

	if srv.associations == nil {
		srv.associations = make(map[string]*storage_store.Badger_store)
	}

	if srv.associations[indexPath] == nil {
		srv.associations[indexPath] = storage_store.NewBadger_store()
		// open in it own thread
		srv.associations[indexPath].Open(`{"path":"` + indexPath + `", "name":"titles"}`)
	}

	data, err := srv.associations[indexPath].GetItem(uuid)
	association := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
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
		title, err := srv.getTitleById(indexPath, generateUUID(association.Titles[i]))
		if err == nil {
			titles = append(titles, title)
		}
	}

	// In case of a dir I need to recursivly get the list of title from sub-folder...
	if fileInfo.IsDir() && !Utility.Exists(absolutePath+"/playlist.m3u8") {
		files, err := ioutil.ReadDir(absolutePath)
		if err == nil {
			for _, f := range files {
				titles_, err := srv.getFileTitles(indexPath, filePath+"/"+f.Name(), absolutePath+"/"+f.Name())
				if err == nil {
					// append all found title.
					titles = append(titles, titles_...)
				}
			}
		}
	}

	// Return the list of all related titles.
	return titles, nil
}

// Return the list of titles asscociate with a file.
func (srv *server) GetFileTitles(ctx context.Context, rqst *titlepb.GetFileTitlesRequest) (*titlepb.GetFileTitlesResponse, error) {

	filePath := strings.ReplaceAll(rqst.FilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")

	// So here I will get the list of titles asscociated with a file...
	absolutefilePath := rqst.FilePath
	absolutefilePath = strings.ReplaceAll(absolutefilePath, "\\", "/")
	if !Utility.Exists(absolutefilePath) {
		// Here I will try to get it from the users dirs...
		if strings.HasPrefix(absolutefilePath, "/users/") || strings.HasPrefix(absolutefilePath, "/applications/") {
			absolutefilePath = config.GetDataDir() + "/files" + absolutefilePath
		}
	}

	if !Utility.Exists(absolutefilePath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+absolutefilePath)))
	}

	titles, err := srv.getFileTitles(rqst.IndexPath, filePath, absolutefilePath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.GetFileTitlesResponse{Titles: &titlepb.Titles{Titles: titles}}, nil
}

// ////////////////////////////////////////////////////// Publisher ////////////////////////////////////////////////////////
// Create a publisher...
func (srv *server) CreatePublisher(ctx context.Context, rqst *titlepb.CreatePublisherRequest) (*titlepb.CreatePublisherResponse, error) {
	if rqst.Publisher == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no publisher was given")))

	}

	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Index the title and put it in the search engine.
	err = index.Index(rqst.Publisher.ID, rqst.Publisher)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Publisher)

	if err == nil {
		err = index.SetInternal([]byte(rqst.Publisher.ID), []byte(jsonStr))
	}

	return &titlepb.CreatePublisherResponse{}, nil
}

// Delete a publisher...
func (srv *server) DeletePublisher(ctx context.Context, rqst *titlepb.DeletePublisherRequest) (*titlepb.DeletePublisherResponse, error) {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.Delete(rqst.PublisherId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(rqst.PublisherId))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO remove asscociated videos...

	return &titlepb.DeletePublisherResponse{}, nil
}

func (srv *server) getPublisherById(indexPath, id string) (*titlepb.Publisher, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}

	query := bleve.NewQueryStringQuery(id)
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// Now from the result I will
	if searchResult.Total == 0 {
		return nil, errors.New("No matches")
	}

	var publisher *titlepb.Publisher
	// Now I will return the data
	for _, val := range searchResult.Hits {
		id := val.ID
		raw, err := index.GetInternal([]byte(id))
		if err != nil {
			return nil, err
		}
		publisher = new(titlepb.Publisher)
		jsonpb.UnmarshalString(string(raw), publisher)

	}

	return publisher, nil
}

// Retrun a publisher.
func (srv *server) GetPublisherById(ctx context.Context, rqst *titlepb.GetPublisherByIdRequest) (*titlepb.GetPublisherByIdResponse, error) {
	publisher, err := srv.getPublisherById(rqst.IndexPath, rqst.PublisherId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.GetPublisherByIdResponse{
		Publisher: publisher,
	}, nil
}

// ////////////////////////////////////////////////////////// Cast ////////////////////////////////////////////////////////////
// Create a person...
func (srv *server) CreatePerson(ctx context.Context, rqst *titlepb.CreatePersonRequest) (*titlepb.CreatePersonResponse, error) {
	if rqst.Person == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no publisher was given")))

	}

	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Index the title and put it in the search engine.
	err = index.Index(rqst.Person.ID, rqst.Person)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Person)

	if err == nil {
		err = index.SetInternal([]byte(rqst.Person.ID), []byte(jsonStr))
	}

	return &titlepb.CreatePersonResponse{}, nil
}

// Delete a person...
func (srv *server) DeletePerson(ctx context.Context, rqst *titlepb.DeletePersonRequest) (*titlepb.DeletePersonResponse, error) {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.Delete(rqst.PersonId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(rqst.PersonId))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO remove asscociated videos...

	return &titlepb.DeletePersonResponse{}, nil
}

func (srv *server) getPersonById(indexPath, id string) (*titlepb.Person, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}

	query := bleve.NewQueryStringQuery(id)
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// Now from the result I will
	if searchResult.Total == 0 {
		return nil, errors.New("No matches")
	}

	var person *titlepb.Person
	// Now I will return the data
	for _, val := range searchResult.Hits {
		id := val.ID
		raw, err := index.GetInternal([]byte(id))
		if err != nil {
			return nil, err
		}
		person = new(titlepb.Person)
		jsonpb.UnmarshalString(string(raw), person)
	}

	return person, nil
}

// Retrun a person with a given id.
func (srv *server) GetPersonById(ctx context.Context, rqst *titlepb.GetPersonByIdRequest) (*titlepb.GetPersonByIdResponse, error) {
	person, err := srv.getPersonById(rqst.IndexPath, rqst.PersonId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.GetPersonByIdResponse{
		Person: person,
	}, nil
}

// Insert a video in the database or update it if it already exist.
func (srv *server) CreateVideo(ctx context.Context, rqst *titlepb.CreateVideoRequest) (*titlepb.CreateVideoResponse, error) {

	if rqst.Video == nil {
		fmt.Println("no video was given")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no video was given")))

	}

	fmt.Println("try to create video", rqst.Video.ID)

	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rqst.Video.UUID = generateUUID(rqst.Video.ID)
	err = index.Index(rqst.Video.UUID, rqst.Video)

	if err != nil {
		fmt.Println("fail to index video with error", rqst.Video.ID, err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Video)

	if err == nil {
		index.SetInternal([]byte(rqst.Video.UUID), []byte(jsonStr))
	} else {
		fmt.Println("1353 fail to index video with error  ", rqst.Video.ID, err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	test_, err := srv.getVideoById(rqst.IndexPath, rqst.Video.UUID)
	if err != nil || test_ == nil {
		fmt.Println("1362 fail to index video with error  ", rqst.Video.ID, err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.CreateVideoResponse{}, nil
}

func (srv *server) getVideoById(indexPath, id string) (*titlepb.Video, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}

	query := bleve.NewQueryStringQuery(id)
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// Now from the result I will
	if searchResult.Total == 0 {
		return nil, errors.New("No matches")
	}

	var video *titlepb.Video

	// Now I will return the data
	for _, val := range searchResult.Hits {
		if val.ID == id {
			raw, err := index.GetInternal([]byte(val.ID))
			if err != nil {
				return nil, err
			}

			video = new(titlepb.Video)
			jsonpb.UnmarshalString(string(raw), video)
			break
		}

	}

	return video, nil
}

// Get a video by a given id.
func (srv *server) GetVideoById(ctx context.Context, rqst *titlepb.GetVideoByIdRequest) (*titlepb.GetVideoByIdResponse, error) {
	video, err := srv.getVideoById(rqst.IndexPath, generateUUID(rqst.VidoeId))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	filePaths := []string{}

	// get the list of associated files if there some...
	if srv.associations != nil {
		if srv.associations[rqst.IndexPath] != nil {
			data, err := srv.associations[rqst.IndexPath].GetItem(rqst.VidoeId)
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

	return &titlepb.GetVideoByIdResponse{
		Video:      video,
		FilesPaths: filePaths,
	}, nil
}

// Delete a video from the database.
func (srv *server) DeleteVideo(ctx context.Context, rqst *titlepb.DeleteVideoRequest) (*titlepb.DeleteVideoResponse, error) {

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := generateUUID(rqst.VideoId)
	err = index.Delete(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(id))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	val, err := index.GetInternal([]byte(id))
	if err == nil {
		return nil, errors.New("fail to remove " + rqst.VideoId)
	}

	if val != nil {
		return nil, errors.New("expected nil, got" + string(val))
	}

	// Now I will remove the file association...

	return &titlepb.DeleteVideoResponse{}, nil
}

// Return the list of videos asscociate with a file.
func (srv *server) GetFileVideos(ctx context.Context, rqst *titlepb.GetFileVideosRequest) (*titlepb.GetFileVideosResponse, error) {

	// relative path...
	filePath := strings.ReplaceAll(rqst.FilePath, config.GetConfigDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")

	// So here I will get the list of titles asscociated with a file...
	absolutefilePath := rqst.FilePath
	absolutefilePath = strings.ReplaceAll(absolutefilePath, "\\", "/")
	if !Utility.Exists(absolutefilePath) {
		// Here I will try to get it from the users dirs...
		if strings.HasPrefix(absolutefilePath, "/users/") || strings.HasPrefix(absolutefilePath, "/applications/") {
			absolutefilePath = config.GetDataDir() + "/files" + absolutefilePath
		}
	}

	if !Utility.Exists(absolutefilePath) {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+filePath)))
	}

	// I will use the file checksum as file id...
	var uuid string
	fileInfo, err := os.Stat(absolutefilePath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if fileInfo.IsDir() {
		// is a directory
		uuid = generateUUID(filePath)
	} else {
		// is not a directory
		uuid = Utility.CreateFileChecksum(absolutefilePath)
	}

	if srv.associations == nil {
		srv.associations = make(map[string]*storage_store.Badger_store)
	}

	if srv.associations[rqst.IndexPath] == nil {
		srv.associations[rqst.IndexPath] = storage_store.NewBadger_store()
		// open in it own thread
		srv.associations[rqst.IndexPath].Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := srv.associations[rqst.IndexPath].GetItem(uuid)
	association := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	videos := make([]*titlepb.Video, 0)
	for i := 0; i < len(association.Titles); i++ {
		video, err := srv.getVideoById(rqst.IndexPath, generateUUID(association.Titles[i]))
		if err == nil {
			videos = append(videos, video)
		}
	}

	return &titlepb.GetFileVideosResponse{Videos: &titlepb.Videos{Videos: videos}}, nil
}

// Return the list of files associate with a title
func (srv *server) getTitleFiles(indexPath, titleId string) ([]string, error) {

	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	// I will use the file checksum as file id...
	if srv.associations == nil {
		srv.associations = make(map[string]*storage_store.Badger_store)
	}

	if srv.associations[indexPath] == nil {
		srv.associations[indexPath] = storage_store.NewBadger_store()
		// open in it own thread
		srv.associations[indexPath].Open(`{"path":"` + indexPath + `", "name":"titles"}`)
	}

	data, err := srv.associations[indexPath].GetItem(titleId)
	if err != nil {
		fmt.Println("no files association found for title: ", titleId, err)
		return nil, err
	}

	association := &fileTileAssociation{ID: "", Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			fmt.Println("fail to read association: ", titleId, err)
			return nil, err
		}
	}

	// Here I will remove path that no more exist...
	paths := make([]string, 0)
	for i := 0; i < len(association.Paths); i++ {
		if Utility.Exists(association.Paths[i]) {
			paths = append(paths, association.Paths[i])
		} else if Utility.Exists(config.GetDataDir() + "/files" + association.Paths[i]) {
			paths = append(paths, association.Paths[i])
		}
	}

	if len(paths) != len(association.Paths) {
		association.Paths = paths
		// if no more file are link I will remove the association...
		if len(association.Paths) == 0 {
			srv.associations[indexPath].RemoveItem(titleId)
			srv.associations[indexPath].RemoveItem(association.ID)
		} else {
			data, _ = json.Marshal(association)
			srv.associations[indexPath].SetItem(association.ID, data)
			srv.associations[indexPath].SetItem(titleId, data)
		}
	}

	return association.Paths, nil
}

// Return the list of files associate with a title
func (srv *server) GetTitleFiles(ctx context.Context, rqst *titlepb.GetTitleFilesRequest) (*titlepb.GetTitleFilesResponse, error) {

	// I will use the file checksum as file id...
	paths, err := srv.getTitleFiles(rqst.IndexPath, rqst.TitleId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Return the store association values...
	return &titlepb.GetTitleFilesResponse{FilePaths: paths}, nil

}

// Search titles infos...
func (srv *server) SearchTitles(rqst *titlepb.SearchTitlesRequest, stream titlepb.TitleService_SearchTitlesServer) error {

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := bleve.NewQueryStringQuery(rqst.Query)
	request := bleve.NewSearchRequest(query)
	request.Size = int(rqst.Size) //
	request.From = int(rqst.Offset)

	if request.Size == 0 {
		request.Size = 50
	}

	// Now I will add the facets for type and genre.

	// The genre facet.
	genres := bleve.NewFacetRequest("Genres", int(rqst.Size))
	request.AddFacet("Genres", genres)

	// The type facet...
	types := bleve.NewFacetRequest("Type", int(rqst.Size))
	request.AddFacet("Types", types)

	tags := bleve.NewFacetRequest("Tags", int(rqst.Size))
	request.AddFacet("Tags", tags)

	// The rating facet
	var zero = 0.0
	var lowToMidRating = 3.5
	var midToHighRating = 7.0
	var ten = 10.0

	rating := bleve.NewFacetRequest("Rating", int(rqst.Size))
	rating.AddNumericRange("low", &zero, &lowToMidRating)
	rating.AddNumericRange("medium", &lowToMidRating, &midToHighRating)
	rating.AddNumericRange("high", &midToHighRating, &ten)
	request.AddFacet("Rating", rating)

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

	if srv.associations[rqst.IndexPath] == nil {
		srv.associations[rqst.IndexPath] = storage_store.NewBadger_store()
		// open in it own thread
		srv.associations[rqst.IndexPath].Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

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
			// append to the results.
			hit_.Snippets = append(hit_.Snippets, snippet)
		}

		// Here I will get the title itself.
		raw, err := index.GetInternal([]byte(id))
		if err == nil {
			// the title must contain file association...

			//if err == nil {

			title := new(titlepb.Title)
			err = jsonpb.UnmarshalString(string(raw), title)
			if err == nil {
				hit_.Result = &titlepb.SearchHit_Title{
					Title: title,
				}

				hasFile := true
				files, err := srv.getTitleFiles(rqst.IndexPath, title.ID)
				if err != nil {
					hasFile = false
				} else {
					hasFile = len(files) > 0
				}
				
				// Here I will send the search result...
				if hasFile {
					stream.Send(&titlepb.SearchTitlesResponse{
						Result: &titlepb.SearchTitlesResponse_Hit{
							Hit: hit_,
						},
					})
				} else {

					index.Delete(id)
					index.DeleteInternal([]byte(id))
				}

			} else {
				// Here I will try with a video...
				video := new(titlepb.Video)
				err := jsonpb.UnmarshalString(string(raw), video)
				if err == nil {
					hit_.Result = &titlepb.SearchHit_Video{
						Video: video,
					}

					hasFile := true
					files, err := srv.getTitleFiles(rqst.IndexPath, video.ID)
					if err != nil {
						hasFile = false
					} else {
						hasFile = len(files) > 0
					}
					
					// Here I will send the search result...
					if hasFile {
						stream.Send(&titlepb.SearchTitlesResponse{
							Result: &titlepb.SearchTitlesResponse_Hit{
								Hit: hit_,
							},
						})
					} else {

						index.Delete(id)
						index.DeleteInternal([]byte(id))
					}

				} else {
					audio := new(titlepb.Audio)
					err := jsonpb.UnmarshalString(string(raw), audio)
					if err == nil {
						hit_.Result = &titlepb.SearchHit_Audio{
							Audio: audio,
						}

						hasFile := true
						files, err := srv.getTitleFiles(rqst.IndexPath, audio.ID)
						if err != nil {
							hasFile = false
						} else {
							hasFile = len(files) > 0
						}
						
						if hasFile {
							// Here I will send the search result...
							stream.Send(&titlepb.SearchTitlesResponse{
								Result: &titlepb.SearchTitlesResponse_Hit{
									Hit: hit_,
								},
							})
						} else {
							index.Delete(id)
							index.DeleteInternal([]byte(id))
						}

					}
				}

			}
		}
	}

	// Finaly I will send the facets...
	facets := new(titlepb.SearchFacets)
	facets.Facets = make([]*titlepb.SearchFacet, 0)

	for _, f := range result.Facets {
		facet_ := new(titlepb.SearchFacet)
		facet_.Field = f.Field
		facet_.Total = int32(f.Total)
		facet_.Other = int32(f.Other)
		facet_.Terms = make([]*titlepb.SearchFacetTerm, 0)
		// Regular terms...
		for _, t := range f.Terms {
			term := new(titlepb.SearchFacetTerm)
			term.Count = int32(t.Count)
			term.Term = t.Term

			facet_.Terms = append(facet_.Terms, term)
		}

		// Numeric Range terms...
		for _, t := range f.NumericRanges {
			term := new(titlepb.SearchFacetTerm)
			term.Count = int32(t.Count)

			// Here I will set a json string...
			numeric := make(map[string]interface{}, 0)
			numeric["name"] = t.Name
			numeric["min"] = t.Min
			numeric["max"] = t.Max
			jsonStr, _ := Utility.ToJson(numeric)
			term.Term = string(jsonStr)
			facet_.Terms = append(facet_.Terms, term)
		}

		facets.Facets = append(facets.Facets, facet_)
	}

	// send the facets...
	stream.Send(&titlepb.SearchTitlesResponse{
		Result: &titlepb.SearchTitlesResponse_Facets{
			Facets: facets,
		},
	})

	return nil
}

/////////////////////////////////// Audio specific functions ////////////////////////////////////////

// Insert a audio information in the database or update it if it already exist.
func (srv *server) CreateAudio(ctx context.Context, rqst *titlepb.CreateAudioRequest) (*titlepb.CreateAudioResponse, error) {
	if rqst.Audio == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no audio was given")))

	}

	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rqst.Audio.UUID = generateUUID(rqst.Audio.ID)

	// Index the title and put it in the search engine.
	err = index.Index(rqst.Audio.UUID, rqst.Audio)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Audio)

	if err == nil {
		err = index.SetInternal([]byte(generateUUID(rqst.Audio.ID)), []byte(jsonStr))
	}

	// Now I will create the album from the track info...
	_, err = srv.getAlbum(rqst.IndexPath, rqst.Audio.Album)
	if err != nil {
		// In that case the album dosent exist... so I will create it.
		album := &titlepb.Album{ID: rqst.Audio.Album, Artist: rqst.Audio.AlbumArtist, Year: rqst.Audio.Year, Genres: rqst.Audio.Genres, Poster: rqst.Audio.Poster}
		jsonStr, err := marshaler.MarshalToString(album)
		if err == nil {
			err = index.SetInternal([]byte(album.ID), []byte(jsonStr))
		}
	}

	return &titlepb.CreateAudioResponse{}, nil
}

func (srv *server) getAudioById(indexPath, id string) (*titlepb.Audio, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}

	query := bleve.NewQueryStringQuery(id)
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// Now from the result I will
	if searchResult.Total == 0 {
		return nil, errors.New("No matches")
	}

	var audio *titlepb.Audio

	// Now I will return the data
	for _, val := range searchResult.Hits {
		if val.ID == id {
			raw, err := index.GetInternal([]byte(val.ID))
			if err != nil {
				return nil, err
			}

			audio = new(titlepb.Audio)
			jsonpb.UnmarshalString(string(raw), audio)
			break
		}

	}

	return audio, nil
}

// Get a audio by a given id.
func (srv *server) GetAudioById(ctx context.Context, rqst *titlepb.GetAudioByIdRequest) (*titlepb.GetAudioByIdResponse, error) {
	audio, err := srv.getAudioById(rqst.IndexPath, generateUUID(rqst.AudioId))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	filePaths := []string{}

	// get the list of associated files if there some...
	if srv.associations != nil {
		if srv.associations[rqst.IndexPath] != nil {
			data, err := srv.associations[rqst.IndexPath].GetItem(rqst.AudioId)
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

	return &titlepb.GetAudioByIdResponse{
		Audio:      audio,
		FilesPaths: filePaths,
	}, nil
}

func (srv *server) getAlbum(indexPath, id string) (*titlepb.Album, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}

	query := bleve.NewQueryStringQuery(id)
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, err := index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// Now from the result I will
	if searchResult.Total == 0 {
		return nil, errors.New("No matches")
	}

	var album *titlepb.Album

	// Now I will return the data
	for _, val := range searchResult.Hits {
		if val.ID == id {
			raw, err := index.GetInternal([]byte(val.ID))
			if err != nil {
				return nil, err
			}

			album_ := new(titlepb.Album)
			err_ := jsonpb.UnmarshalString(string(raw), album_)
			if err_ == nil {
				album = album_
			} else {
				track := new(titlepb.Audio)
				err_ := jsonpb.UnmarshalString(string(raw), track)

				if err_ == nil {
					if track.Album == id {
						if album.Tracks == nil {
							album.Tracks = new(titlepb.Audios)
							album.Tracks.Audios = make([]*titlepb.Audio, 0)
						}
						album.Tracks.Audios = append(album.Tracks.Audios, track)
					}
				}
			}
			break
		}
	}

	return album, nil
}

// Return the album information for a given id.
func (srv *server) GetAlbum(ctx context.Context, rqst *titlepb.GetAlbumRequest) (*titlepb.GetAlbumResponse, error) {

	indexPath := rqst.IndexPath
	id := rqst.AlbumId

	album, err := srv.getAlbum(indexPath, id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.GetAlbumResponse{Album: album}, nil
}

// Delete a audio from the database.
func (srv *server) DeleteAudio(ctx context.Context, rqst *titlepb.DeleteAudioRequest) (*titlepb.DeleteAudioResponse, error) {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := generateUUID(rqst.AudioId)

	err = index.Delete(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(id))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO remove asscociated files...

	return &titlepb.DeleteAudioResponse{}, nil
}

func (srv *server) DeleteAlbum(ctx context.Context, rqst *titlepb.DeleteAlbumRequest) (*titlepb.DeleteAlbumResponse, error) {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.Delete(rqst.AlbumId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(rqst.AlbumId))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO remove asscociated files...

	return &titlepb.DeleteAlbumResponse{}, nil
}

// Return the list of audios asscociate with a file.
func (srv *server) GetFileAudios(ctx context.Context, rqst *titlepb.GetFileAudiosRequest) (*titlepb.GetFileAudiosResponse, error) {

	// remove keep the part after /applications or /users
	filePath := strings.ReplaceAll(rqst.FilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")

	// So here I will get the list of titles asscociated with a file...
	absolutefilePath := rqst.FilePath
	absolutefilePath = strings.ReplaceAll(absolutefilePath, "\\", "/")
	if !Utility.Exists(absolutefilePath) {
		// Here I will try to get it from the users dirs...
		if strings.HasPrefix(absolutefilePath, "/users/") || strings.HasPrefix(absolutefilePath, "/applications/") {
			absolutefilePath = config.GetDataDir() + "/files" + absolutefilePath
		}
	}

	if !Utility.Exists(absolutefilePath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found with path "+absolutefilePath)))
	}

	// I will use the file checksum as file id...
	var uuid string
	fileInfo, err := os.Stat(absolutefilePath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if fileInfo.IsDir() {
		// is a directory
		uuid = generateUUID(filePath)
	} else {
		// is not a directory
		uuid = Utility.CreateFileChecksum(absolutefilePath)
	}

	if srv.associations == nil {
		srv.associations = make(map[string]*storage_store.Badger_store)
	}

	if srv.associations[rqst.IndexPath] == nil {
		srv.associations[rqst.IndexPath] = storage_store.NewBadger_store()
		// open in it own thread
		srv.associations[rqst.IndexPath].Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := srv.associations[rqst.IndexPath].GetItem(uuid)
	association := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	audios := make([]*titlepb.Audio, 0)
	for i := 0; i < len(association.Titles); i++ {
		audio, err := srv.getAudioById(rqst.IndexPath, generateUUID(association.Titles[i]))
		if err == nil {
			audios = append(audios, audio)
		}
	}

	return &titlepb.GetFileAudiosResponse{Audios: &titlepb.Audios{Audios: audios}}, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

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
	s_impl.associations = make(map[string]*storage_store.Badger_store)

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

	// Register the title services
	titlepb.RegisterTitleServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
