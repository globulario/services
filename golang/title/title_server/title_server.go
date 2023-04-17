package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	b64 "encoding/base64"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blevesearch/bleve"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/titlepb"
	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	//"google.golang.org/grpc/grpclog"
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
	//associations map[string]*storage_store.Badger_store
	associations *sync.Map
}

// The path of the configuration.
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
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

	// Get the configuration path.
	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
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

	uuid := generateUUID(titleId)
	raw, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return nil, err
	}

	title := new(titlepb.Title)
	err = jsonpb.UnmarshalString(string(raw), title)
	if err != nil {
		return nil, err
	}

	return title, nil
}

// Get accosiations store...
func (srv *server) getAssociations(id string) *storage_store.Badger_store {
	if srv.associations != nil {
		associations, ok := srv.associations.Load(id)
		if ok {
			return associations.(*storage_store.Badger_store)
		}
	}
	return nil
}

// Get a title by a given id.
func (srv *server) GetTitleById(ctx context.Context, rqst *titlepb.GetTitleByIdRequest) (*titlepb.GetTitleByIdResponse, error) {

	title, err := srv.getTitleById(rqst.IndexPath, rqst.TitleId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	filePaths := []string{}

	// get the list of associated files if there some...
	associations := srv.getAssociations(rqst.IndexPath)
	if associations != nil {
		data, err := associations.GetItem(rqst.TitleId)
		if err == nil {
			association := new(fileTileAssociation)
			err = json.Unmarshal(data, association)
			if err == nil {
				// In that case I will get the files...
				filePaths = association.Paths
			}
		}
	}

	return &titlepb.GetTitleByIdResponse{
		Title:      title,
		FilesPaths: filePaths,
	}, nil
}

// save the title casting.
func (srv *server) saveTitleCasting(indexpath, titleId, role string, persons []*titlepb.Person) []*titlepb.Person {
	casting := make([]*titlepb.Person, 0)

	for i := 0; i < len(persons); i++ {
		person := persons[i]

		// Get existiong movie...
		existing, err := srv.getPersonById(indexpath, person.ID)
		if err == nil {
			if role == "Casting" {
				for i := 0; i < len(existing.Casting); i++ {
					if !Utility.Contains(person.Casting, existing.Casting[i]) {
						person.Casting = append(person.Casting, existing.Casting[i])
					}
				}

				if !Utility.Contains(person.Casting, titleId) {
					person.Casting = append(person.Casting, titleId)
				}
			} else if role == "Acting" {
				for i := 0; i < len(existing.Acting); i++ {
					if !Utility.Contains(person.Acting, existing.Acting[i]) {
						person.Acting = append(person.Acting, existing.Acting[i])
					}
				}

				if !Utility.Contains(person.Acting, titleId) {
					person.Acting = append(person.Acting, titleId)
				}
			} else if role == "Directing" {
				for i := 0; i < len(existing.Directing); i++ {
					if !Utility.Contains(person.Directing, existing.Directing[i]) {
						person.Directing = append(person.Directing, existing.Directing[i])
					}
				}

				if !Utility.Contains(person.Directing, titleId) {
					person.Directing = append(person.Directing, titleId)
				}
			} else if role == "Writing" {
				for i := 0; i < len(existing.Writing); i++ {
					if !Utility.Contains(person.Writing, existing.Writing[i]) {
						person.Writing = append(person.Writing, existing.Writing[i])
					}
				}

				if !Utility.Contains(person.Writing, titleId) {
					person.Writing = append(person.Writing, titleId)
				}
			}

			casting = append(casting, person)

			// save the existing cast...
			srv.createPerson(indexpath, person)
		}
	}

	return casting
}

// Insert a title in the database or update it if it already exist.
func (srv *server) CreateTitle(ctx context.Context, rqst *titlepb.CreateTitleRequest) (*titlepb.CreateTitleResponse, error) {

	if len(rqst.Title.ID) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no title id was given")))
	}

	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.Domain
		} else {
			return nil, errors.New("no token was given")
		}
	}

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

	// save the persons information...
	rqst.Title.Actors = srv.saveTitleCasting(rqst.IndexPath, rqst.Title.ID, "Acting", rqst.Title.Actors)
	rqst.Title.Writers = srv.saveTitleCasting(rqst.IndexPath, rqst.Title.ID, "Writing", rqst.Title.Writers)
	rqst.Title.Directors = srv.saveTitleCasting(rqst.IndexPath, rqst.Title.ID, "Directing", rqst.Title.Directors)

	err = index.Index(rqst.Title.UUID, rqst.Title)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will get the title thumbnail...
	if rqst.Title.Poster != nil {
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
	}

	// so here I will set the ownership...
	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	permissions, _ := rbac_client_.GetResourcePermissions(rqst.Title.ID)
	if permissions == nil {
		// set the resource path...
		err = rbac_client_.AddResourceOwner(rqst.Title.ID, "title_infos", clientId, rbacpb.SubjectType_ACCOUNT)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Title)
	if err == nil {
		index.SetInternal([]byte(rqst.Title.UUID), []byte(jsonStr))
	} else {
		fmt.Println("fail to marshall title", rqst.Title.ID, "with error: ", err)
	}

	event_client, err := srv.getEventClient()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// send event to update the audio infos
	event_client.Publish("update_title_infos_evt", []byte(jsonStr))
	return &titlepb.CreateTitleResponse{}, nil
}

func (srv *server) UpdateTitleMetadata(ctx context.Context, rqst *titlepb.UpdateTitleMetadataRequest) (*titlepb.UpdateTitleMetadataResponse, error) {

	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_, err = index.GetInternal([]byte(generateUUID(rqst.Title.ID)))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	paths, err := srv.getTitleFiles(rqst.IndexPath, rqst.Title.ID)
	if err == nil {
		for i := 0; i < len(paths); i++ {
			absolutefilePath := strings.ReplaceAll(paths[i], "\\", "/")

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

			srv.saveTitleMetadata(absolutefilePath, rqst.IndexPath, rqst.Title)
		}
	}

	return &titlepb.UpdateTitleMetadataResponse{}, nil
}

func (srv *server) deleteTitle(indexPath, titleId string) error {

	title, err := srv.getTitleById(indexPath, titleId)
	if err != nil {
		return err
	}

	// Now I will remove reference from this video from the casting.
	for i := 0; i < len(title.Actors); i++ {
		p, err := srv.getPersonById(indexPath, title.Actors[i].ID)
		if err == nil {
			p.Acting = Utility.RemoveString(p.Acting, titleId)
			// save back the person.
			srv.createPerson(indexPath, p)
		}
	}

	for i := 0; i < len(title.Writers); i++ {
		p, err := srv.getPersonById(indexPath, title.Writers[i].ID)
		if err == nil {
			p.Writing = Utility.RemoveString(p.Writing, titleId)
			// save back the person.
			srv.createPerson(indexPath, p)
		}
	}

	for i := 0; i < len(title.Directors); i++ {
		p, err := srv.getPersonById(indexPath, title.Directors[i].ID)
		if err == nil {
			p.Directing = Utility.RemoveString(p.Directing, titleId)
			// save back the person.
			srv.createPerson(indexPath, p)
		}
	}

	// dir to refresh...
	dirs := make([]string, 0)

	// Remove all file indexation...
	paths, err := srv.getTitleFiles(indexPath, titleId)
	if err == nil {
		for i := 0; i < len(paths); i++ {
			srv.dissociateFileWithTitle(indexPath, titleId, paths[i])
			dirs = append(dirs, filepath.Dir(strings.ReplaceAll(paths[i], config.GetDataDir()+"/files", "")))
		}
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}

	uuid := generateUUID(titleId)
	err = index.Delete(uuid)

	if err != nil {
		return err
	}

	err = index.DeleteInternal([]byte(uuid))
	if err != nil {
		return err
	}

	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	// remove the permission.
	err = rbac_client_.DeleteResourcePermissions(titleId)
	if err != nil {
		return err
	}

	// publish delete video event.
	err = srv.publish("delete_title_event", []byte(titleId))
	if err != nil {
		return err
	}

	for i := 0; i < len(dirs); i++ {
		srv.publish("reload_dir_event", []byte(dirs[i]))
	}

	return nil
}

// Delete a title from the database.
func (srv *server) DeleteTitle(ctx context.Context, rqst *titlepb.DeleteTitleRequest) (*titlepb.DeleteTitleResponse, error) {

	err := srv.deleteTitle(rqst.IndexPath, rqst.TitleId)
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

func (srv *server) saveTitleMetadata(absolutefilePath, indexPath string, title *titlepb.Title) error {
	if len(title.Name) == 0 {
		return errors.New("no title name was given")
	}

	fileInfo, err := os.Stat(absolutefilePath)
	if err != nil {
		return err
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(title)
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(jsonStr))
	if fileInfo.IsDir() {
		err = os.WriteFile(absolutefilePath+"/infos.json", []byte(jsonStr), 0664)
		if err != nil {
			return err
		}
		dir := filepath.Dir(strings.ReplaceAll(absolutefilePath, config.GetDataDir()+"/files", ""))
		srv.publish("reload_dir_event", []byte(dir))
	} else {
		// Here I will save metadata only if it has change...
		infos, err := Utility.ReadMetadata(absolutefilePath)
		needSave := true
		if err == nil {
			if infos["format"] != nil {
				if infos["format"].(map[string]interface{})["tags"] != nil {
					tags := infos["format"].(map[string]interface{})["tags"].(map[string]interface{})
					if tags["comment"] != nil {
						comment := tags["comment"].(string)
						if len(comment) > 0 {
							needSave = comment != encoded
						}
					}
				}
			}
		}

		if needSave {

			old_checksum := Utility.CreateFileChecksum(absolutefilePath)
			Utility.SetMetadata(absolutefilePath, "comment", encoded)

			associations := srv.getAssociations(indexPath)
			if associations != nil {
				data, err := associations.GetItem(old_checksum)
				if err == nil {
					new_checksum := Utility.CreateFileChecksum(absolutefilePath)
					if old_checksum != new_checksum {
						associations.RemoveItem(old_checksum) // remove the previous
						associations.SetItem(new_checksum, data)
					}
				}
			}
			// reload the client file infos...
			dir := filepath.Dir(strings.ReplaceAll(absolutefilePath, config.GetDataDir()+"/files", ""))
			srv.publish("reload_dir_event", []byte(dir))

		} else {
			fmt.Println("not need save title metadata ", absolutefilePath)
		}
	}

	return nil
}

func (srv *server) saveVideoMetadata(absolutefilePath, indexPath string, video *titlepb.Video) error {

	if len(video.Description) == 0 {
		return errors.New("no title description was given")
	}

	fileInfo, err := os.Stat(absolutefilePath)
	if err != nil {
		return err
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(video)
	encoded := base64.StdEncoding.EncodeToString([]byte(jsonStr))
	if err != nil {
		return err
	}

	if fileInfo.IsDir() {
		err = os.WriteFile(absolutefilePath+"/infos.json", []byte(jsonStr), 0664)
		if err != nil {
			return err
		}
		dir := filepath.Dir(strings.ReplaceAll(absolutefilePath, config.GetDataDir()+"/files", ""))
		srv.publish("reload_dir_event", []byte(dir))
	} else {
		infos, err := Utility.ReadMetadata(absolutefilePath)
		needSave := true
		if err == nil {
			if infos["format"] != nil {
				if infos["format"].(map[string]interface{})["tags"] != nil {
					tags := infos["format"].(map[string]interface{})["tags"].(map[string]interface{})
					if tags["comment"] != nil {
						comment := tags["comment"].(string)
						if len(comment) > 0 {
							needSave = comment != encoded
						}
					}
				}
			}
		}

		if needSave {
			old_checksum := Utility.CreateFileChecksum(absolutefilePath)
			Utility.SetMetadata(absolutefilePath, "comment", encoded)

			associations := srv.getAssociations(indexPath)
			if associations != nil {
				data, err := associations.GetItem(old_checksum)
				if err == nil {
					new_checksum := Utility.CreateFileChecksum(absolutefilePath)
					if old_checksum != new_checksum {
						associations.RemoveItem(old_checksum) // remove the previous
						associations.SetItem(new_checksum, data)
					}
				}
			}

			// reload the client file infos...
			dir := filepath.Dir(strings.ReplaceAll(absolutefilePath, config.GetDataDir()+"/files", ""))
			srv.publish("reload_dir_event", []byte(dir))
		}
	}

	return nil
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


	// Now I will set the title url in the media file to keep track of the title
	// information. If the title is lost it will be possible to recreate it from
	// that url.
	if strings.HasSuffix(rqst.IndexPath, "/search/titles") {

		title, err := srv.getTitleById(rqst.IndexPath, rqst.TitleId)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if title.Poster != nil && len(title.Poster.ContentUrl) == 0 {
			title.Poster.ContentUrl = title.Poster.URL // set the Content url with the lnk instead of data url to save space.
		}

		srv.saveTitleMetadata(absolutefilePath, rqst.IndexPath, title)


	} else if strings.HasSuffix(rqst.IndexPath, "/search/videos") {

		video, err := srv.getVideoById(rqst.IndexPath, rqst.TitleId)
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

		if video.Poster != nil && len(video.Poster.ContentUrl) == 0 {
			video.Poster.ContentUrl = video.Poster.URL // set the Content url with the lnk instead of data url to save space.
		}

		srv.saveVideoMetadata(absolutefilePath, rqst.IndexPath, video)

	}

	var uuid string
	filePath := strings.ReplaceAll(rqst.FilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")

	// get the file info.
	fileInfo, _ := os.Stat(absolutefilePath)

	// Depending if the filePath point to a dir or a file...
	if fileInfo.IsDir() {
		// is a directory
		uuid = generateUUID(filePath)
	} else {
		// is not a directory
		uuid = Utility.CreateFileChecksum(absolutefilePath)
	}

	associations := srv.getAssociations(rqst.IndexPath)

	if associations == nil {
		associations = storage_store.NewBadger_store()
		srv.associations.Store(rqst.IndexPath, associations)

		// open in it own thread
		associations.Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := associations.GetItem(uuid)
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
	err = associations.SetItem(uuid, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will get the association for that title.
	data, err = associations.GetItem(rqst.TitleId)
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
	err = associations.SetItem(rqst.TitleId, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	dir := filepath.Dir(strings.ReplaceAll(filePath, config.GetDataDir()+"/files", ""))
	srv.publish("reload_dir_event", []byte(dir))

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

	associations := srv.getAssociations(indexPath)

	if associations == nil {
		associations = storage_store.NewBadger_store()
		srv.associations.Store(indexPath, associations)
		// open in it own thread
		associations.Open(`{"path":"` + indexPath + `", "name":"titles"}`)
	}

	file_data, err := associations.GetItem(uuid)
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
		associations.RemoveItem(uuid)
	} else {
		// Now I will set back the item in the store.
		data, _ := json.Marshal(file_association)
		err = associations.SetItem(uuid, data)
		if err != nil {
			return err
		}
	}

	title_data, err := associations.GetItem(titleId)
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
		associations.RemoveItem(titleId)

		// I will also delete the title itself from the search engine...
		if strings.HasSuffix(indexPath, "/search/videos") {
			srv.deleteVideo(indexPath, titleId)
		} else if strings.HasSuffix(indexPath, "/search/audios") {
			srv.deleteAudio(indexPath, titleId)
		} else if strings.HasSuffix(indexPath, "/search/titltes") {
			srv.deleteTitle(indexPath, titleId)
		}

	} else {
		// Now I will set back the item in the store.
		data, _ := json.Marshal(title_association)
		err = associations.SetItem(titleId, data)
		if err != nil {
			return err
		}
	}

	dir := filepath.Dir(strings.ReplaceAll(filePath, config.GetDataDir()+"/files", ""))
	srv.publish("reload_dir_event", []byte(dir))

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

	associations := srv.getAssociations(indexPath)
	if associations == nil {
		associations = storage_store.NewBadger_store()
		srv.associations.Store(indexPath, associations)
		// open in it own thread
		associations.Open(`{"path":"` + indexPath + `", "name":"titles"}`)
	}

	data, err := associations.GetItem(uuid)
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
		title, err := srv.getTitleById(indexPath, association.Titles[i])
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

	if len(titles) == 0 {
		return nil, errors.New("no titles associations found for file " + rqst.FilePath)
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
	uuid := generateUUID(rqst.Publisher.ID)
	err = index.Index(uuid, rqst.Publisher)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Publisher)

	if err == nil {
		err = index.SetInternal([]byte(uuid), []byte(jsonStr))
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

	uuid := generateUUID(rqst.PublisherId)
	err = index.Delete(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(uuid))
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
		uuid := generateUUID(val.ID)
		raw, err := index.GetInternal([]byte(uuid))
		if err != nil {
			return nil, err
		}
		publisher = new(titlepb.Publisher)
		err = jsonpb.UnmarshalString(string(raw), publisher)
		if err != nil {
			return nil, err
		}

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
func (srv *server) createPerson(indexPath string, person *titlepb.Person) error {

	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}

	if len(person.ID) == 0 || len(person.FullName) == 0 {
		return errors.New("missing inforamation for create person")
	}

	uuid := generateUUID(person.ID)

	// Encode the biography field if is not already encoded.
	if !Utility.IsStdBase64(person.Biography) {
		person.Biography = b64.StdEncoding.EncodeToString([]byte(person.Biography))
	}

	// Index the title and put it in the search engine.
	err = index.Index(uuid, person)
	if err != nil {
		return err
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(person)
	if err != nil {
		return err
	}

	err = index.SetInternal([]byte(uuid), []byte(jsonStr))
	if err != nil {
		return err
	}

	return nil
}

// Create a person...
func (srv *server) CreatePerson(ctx context.Context, rqst *titlepb.CreatePersonRequest) (*titlepb.CreatePersonResponse, error) {

	err := srv.createPerson(rqst.IndexPath, rqst.Person)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.CreatePersonResponse{}, nil
}

// Delete a person...
func (srv *server) DeletePerson(ctx context.Context, rqst *titlepb.DeletePersonRequest) (*titlepb.DeletePersonResponse, error) {
	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.Domain
		} else {
			errors.New("CreateConversation no token was given")
		}
	}

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	person, err := srv.getPersonById(rqst.IndexPath, rqst.PersonId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := generateUUID(rqst.PersonId)
	err = index.Delete(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(uuid))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now the person dosent exist anymore I will save the video from it acting list
	// so it will be remove from the video casting list.
	for i := 0; i < len(person.Casting); i++ {
		video, err := srv.getVideoById(rqst.IndexPath, person.Casting[i])
		if err == nil {
			srv.createVideo(rqst.IndexPath, clientId, video)
		}
	}

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

	uuid := generateUUID(id)
	raw, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return nil, err
	}

	person := new(titlepb.Person)
	err = jsonpb.UnmarshalString(string(raw), person)
	if err != nil {
		return nil, err
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

func (srv *server) createVideo(indexpath, clientId string, video *titlepb.Video) error {

	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(indexpath)
	if err != nil {
		return err
	}

	if len(video.ID) == 0 {
		return errors.New("no video id was given")
	}

	video.UUID = generateUUID(video.ID)

	err = index.Index(video.UUID, video)
	if err != nil {
		return err
	}

	// refresh set back the filtred casting info.
	video.Casting = srv.saveTitleCasting(indexpath, video.ID, "Casting", video.Casting)

	// so here I will set the ownership...
	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	permissions, _ := rbac_client_.GetResourcePermissions(video.ID)
	if permissions == nil {
		// set the resource path...
		err = rbac_client_.AddResourceOwner(video.ID, "video_infos", clientId, rbacpb.SubjectType_ACCOUNT)
		if err != nil {
			return err
		}
	}

	// Associated original object here...
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(video)

	if err == nil {
		err = index.SetInternal([]byte(video.UUID), []byte(jsonStr))
		if err != nil {
			return err
		}
	} else {
		return err
	}

	event_client_, err := srv.getEventClient()
	if err != nil {
		return err
	}

	// send event to update the video infos
	return event_client_.Publish("update_video_infos_evt", []byte(jsonStr))
}

/**
 * Update the video metadata
 */
func (srv *server) UpdateVideoMetadata(ctx context.Context, rqst *titlepb.UpdateVideoMetadataRequest) (*titlepb.UpdateVideoMetadataResponse, error) {

	video := rqst.Video
	// So here Will create the indexation for the movie...
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	_, err = index.GetInternal([]byte(generateUUID(video.ID)))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// update the video metadata
	paths, err := srv.getTitleFiles(rqst.IndexPath, video.ID)
	if err == nil {
		for i := 0; i < len(paths); i++ {
			absolutefilePath := strings.ReplaceAll(paths[i], "\\", "/")
			if !Utility.Exists(absolutefilePath) {
				// Here I will try to get it from the users dirs...
				if strings.HasPrefix(absolutefilePath, "/users/") || strings.HasPrefix(absolutefilePath, "/applications/") {
					absolutefilePath = config.GetDataDir() + "/files" + absolutefilePath
				}

				if !Utility.Exists(absolutefilePath) {
					return nil, status.Errorf(
						codes.Internal,
						Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}
			}

			srv.saveVideoMetadata(absolutefilePath, rqst.IndexPath, video)
		}
	}

	return &titlepb.UpdateVideoMetadataResponse{}, nil
}

// Insert a video in the database or update it if it already exist.
func (srv *server) CreateVideo(ctx context.Context, rqst *titlepb.CreateVideoRequest) (*titlepb.CreateVideoResponse, error) {

	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.Domain
		} else {
			errors.New("CreateConversation no token was given")
		}
	}

	// So here Will create the indexation for the movie...
	err = srv.createVideo(rqst.IndexPath, clientId, rqst.Video)
	if err != nil {
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

	uuid := generateUUID(id)
	raw, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return nil, err
	}

	video := new(titlepb.Video)
	err = jsonpb.UnmarshalString(string(raw), video)
	if err != nil {
		return nil, err
	}

	// Remove cating from the list if it no more exist.
	casting := make([]*titlepb.Person, 0)
	for i := 0; i < len(video.Casting); i++ {
		c, err := srv.getPersonById(indexPath, video.Casting[i].ID)
		if err == nil {
			casting = append(casting, c)
		}
	}

	// set back.
	video.Casting = casting

	return video, nil
}

// Get a video by a given id.
func (srv *server) GetVideoById(ctx context.Context, rqst *titlepb.GetVideoByIdRequest) (*titlepb.GetVideoByIdResponse, error) {
	video, err := srv.getVideoById(rqst.IndexPath, rqst.VidoeId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	filePaths := []string{}

	// get the list of associated files if there some...
	associations := srv.getAssociations(rqst.IndexPath)
	if associations != nil {
		data, err := associations.GetItem(rqst.VidoeId)
		if err == nil {
			association := new(fileTileAssociation)
			err = json.Unmarshal(data, association)
			if err == nil {
				// In that case I will get the files...
				filePaths = association.Paths
			}
		}
	}

	// Here I will init the casting from to be sure I got the last version...
	casting := make([]*titlepb.Person, len(video.Casting))
	for i := 0; i < len(video.Casting); i++ {
		c, err := srv.getPersonById(rqst.IndexPath, video.Casting[i].ID)
		if err == nil {
			casting[i] = c
		}
	}

	// set back.
	video.Casting = casting

	return &titlepb.GetVideoByIdResponse{
		Video:      video,
		FilesPaths: filePaths,
	}, nil
}

func (srv *server) DeleteVideo(ctx context.Context, rqst *titlepb.DeleteVideoRequest) (*titlepb.DeleteVideoResponse, error) {

	err := srv.deleteVideo(rqst.IndexPath, rqst.VideoId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.DeleteVideoResponse{}, nil
}

// Delete a video from the database.
func (srv *server) deleteVideo(indexPath, videoId string) error {

	// set the video
	video, err := srv.getVideoById(indexPath, videoId)
	if err != nil {
		return err
	}

	// Now I will remove reference from this video from the casting.
	for i := 0; i < len(video.Casting); i++ {
		p, err := srv.getPersonById(indexPath, video.Casting[i].ID)
		if err == nil {
			p.Casting = Utility.RemoveString(p.Casting, video.ID)
			// save back the person.
			srv.createPerson(indexPath, p)

		}
	}

	dirs := make([]string, 0)
	paths, err := srv.getTitleFiles(indexPath, videoId)
	if err == nil {
		for i := 0; i < len(paths); i++ {
			srv.dissociateFileWithTitle(indexPath, videoId, paths[i])
			dirs = append(dirs, filepath.Dir(strings.ReplaceAll(paths[i], config.GetDataDir()+"/files", "")))
		}
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}

	uuid := generateUUID(videoId)
	err = index.Delete(uuid)
	if err != nil {
		return err
	}

	err = index.DeleteInternal([]byte(uuid))
	if err != nil {
		return err
	}

	val, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return err
	}

	if val != nil {
		return errors.New("expected nil, got" + string(val))
	}

	// so here I will set the ownership...
	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	// remove the permission.
	err = rbac_client_.DeleteResourcePermissions(videoId)
	if err != nil {
		return err
	}

	// force reload dir
	for i := 0; i < len(dirs); i++ {
		srv.publish("reload_dir_event", []byte(dirs[i]))
	}

	// publish delete video event.
	return srv.publish("delete_video_event", []byte(videoId))

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

	associations := srv.getAssociations(rqst.IndexPath)

	if associations == nil {
		associations = storage_store.NewBadger_store()
		srv.associations.Store(rqst.IndexPath, associations)
		// open in it own thread
		associations.Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := associations.GetItem(uuid)
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
		video, err := srv.getVideoById(rqst.IndexPath, association.Titles[i])
		if err == nil && video != nil {
			if video.ID != "" {
				videos = append(videos, video)
			} else {
				srv.associations.Delete(video.ID) // remove associations
			}
		}
	}

	if len(videos) == 0 {
		return nil, errors.New("no videos associations found for file " + rqst.FilePath)
	}

	return &titlepb.GetFileVideosResponse{Videos: &titlepb.Videos{Videos: videos}}, nil
}

// Return the list of files associate with a title
func (srv *server) getTitleFiles(indexPath, titleId string) ([]string, error) {

	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}

	// I will use the file checksum as file id...
	associations := srv.getAssociations(indexPath)

	if associations == nil {
		associations = storage_store.NewBadger_store()
		srv.associations.Store(indexPath, associations)
		// open in it own thread
		associations.Open(`{"path":"` + indexPath + `", "name":"titles"}`)
	}

	data, err := associations.GetItem(titleId)
	if err != nil {
		return nil, err
	}

	association := &fileTileAssociation{ID: "", Titles: []string{}, Paths: []string{}}
	if err == nil {
		err = json.Unmarshal(data, association)
		if err != nil {
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
			associations.RemoveItem(titleId)
			associations.RemoveItem(association.ID)
		} else {
			data, _ = json.Marshal(association)
			associations.SetItem(association.ID, data)
			associations.SetItem(titleId, data)
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

// Search youtuber, actor, pornstar realisator etc.
func (srv *server) SearchPersons(rqst *titlepb.SearchPersonsRequest, stream titlepb.TitleService_SearchPersonsServer) error {
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

	// The acting facet.
	acting := bleve.NewFacetRequest("Acting", int(rqst.Size))
	request.AddFacet("Acting", acting)

	// The directing facet...
	directing := bleve.NewFacetRequest("Directing", int(rqst.Size))
	request.AddFacet("Directing", directing)

	// The writing facet...
	writting := bleve.NewFacetRequest("Writting", int(rqst.Size))
	request.AddFacet("Writting", writting)

	// The casting facet...
	casting := bleve.NewFacetRequest("Casting", int(rqst.Size))
	request.AddFacet("Casting", casting)

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
	stream.Send(&titlepb.SearchPersonsResponse{
		Result: &titlepb.SearchPersonsResponse_Summary{
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
			// append to the results.
			hit_.Snippets = append(hit_.Snippets, snippet)
		}

		// Here I will get the title itself.
		raw, err := index.GetInternal([]byte(id))
		if err == nil {
			person := new(titlepb.Person)
			err = jsonpb.UnmarshalString(string(raw), person)
			if err == nil {
				hit_.Result = &titlepb.SearchHit_Person{
					Person: person,
				}
				stream.Send(&titlepb.SearchPersonsResponse{
					Result: &titlepb.SearchPersonsResponse_Hit{
						Hit: hit_,
					},
				})
			}
		}
	}

	return nil
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

	associations := srv.getAssociations(rqst.IndexPath)
	if associations == nil {
		associations = storage_store.NewBadger_store()
		srv.associations.Store(rqst.IndexPath, associations)
		// open in it own thread
		associations.Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
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
			title := new(titlepb.Title)
			err = jsonpb.UnmarshalString(string(raw), title)
			if err == nil {

				// Actors
				actors := make([]*titlepb.Person, 0)
				for i := 0; i < len(title.Actors); i++ {
					p, err := srv.getPersonById(rqst.IndexPath, title.Actors[i].GetID())
					if err == nil {
						actors = append(actors, p) // set the full information...
					}
				}
				title.Actors = actors

				// Directors
				directors := make([]*titlepb.Person, 0)
				for i := 0; i < len(title.Directors); i++ {
					p, err := srv.getPersonById(rqst.IndexPath, title.Directors[i].GetID())
					if err == nil {
						directors = append(directors, p) // set the full information...
					}
				}
				title.Directors = directors

				// Writers
				writers := make([]*titlepb.Person, 0)
				for i := 0; i < len(title.Writers); i++ {
					p, err := srv.getPersonById(rqst.IndexPath, title.Writers[i].GetID())
					if err == nil {
						writers = append(writers, p) // set the full information...
					}
				}
				title.Writers = writers

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

					// Here I will init the casting from to be sure I got the last version...
					casting := make([]*titlepb.Person, 0)
					for i := 0; i < len(video.Casting); i++ {
						c, err := srv.getPersonById(rqst.IndexPath, video.Casting[i].ID)
						if err == nil {
							casting = append(casting, c)
						}
					}

					// set back.
					video.Casting = casting

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

	if len(rqst.Audio.ID) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no audio id was given")))
	}

	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.Domain
		} else {
			errors.New("CreateConversation no token was given")
		}
	}

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

	// so here I will set the ownership of the info
	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	permissions, _ := rbac_client_.GetResourcePermissions(rqst.Audio.ID)
	if permissions == nil {
		// set the resource path...
		err = rbac_client_.AddResourceOwner(rqst.Audio.ID, "audio_infos", clientId, rbacpb.SubjectType_ACCOUNT)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
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
			err = index.SetInternal([]byte(generateUUID(album.ID)), []byte(jsonStr))
		}
	}

	event_client, err := srv.getEventClient()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// send event to update the audio infos
	event_client.Publish("update_audio_infos_evt", []byte(jsonStr))

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

	uuid := generateUUID(id)
	raw, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return nil, err
	}

	audio := new(titlepb.Audio)
	err = jsonpb.UnmarshalString(string(raw), audio)
	if err != nil {
		return nil, err
	}

	return audio, nil
}

// Get a audio by a given id.
func (srv *server) GetAudioById(ctx context.Context, rqst *titlepb.GetAudioByIdRequest) (*titlepb.GetAudioByIdResponse, error) {
	audio, err := srv.getAudioById(rqst.IndexPath, rqst.AudioId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	filePaths := []string{}

	// get the list of associated files if there some...
	associations := srv.getAssociations(rqst.IndexPath)

	if associations != nil {
		data, err := associations.GetItem(rqst.AudioId)
		if err == nil {
			association := new(fileTileAssociation)
			err = json.Unmarshal(data, association)
			if err == nil {
				// In that case I will get the files...
				filePaths = association.Paths
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
			uuid := generateUUID(val.ID)
			raw, err := index.GetInternal([]byte(uuid))
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

func (srv *server) DeleteAudio(ctx context.Context, rqst *titlepb.DeleteAudioRequest) (*titlepb.DeleteAudioResponse, error) {

	err := srv.deleteAudio(rqst.IndexPath, rqst.AudioId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &titlepb.DeleteAudioResponse{}, nil
}

// Delete a audio from the database.
func (srv *server) deleteAudio(indexPath string, audioId string) error {

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}

	dirs := make([]string, 0)
	paths, err := srv.getTitleFiles(indexPath, audioId)
	if err == nil {
		for i := 0; i < len(paths); i++ {
			srv.dissociateFileWithTitle(indexPath, audioId, paths[i])
			dirs = append(dirs, filepath.Dir(strings.ReplaceAll(paths[i], config.GetDataDir()+"/files", "")))
		}
	}

	uuid := generateUUID(audioId)
	err = index.Delete(uuid)
	if err != nil {
		return err
	}

	err = index.DeleteInternal([]byte(uuid))
	if err != nil {
		return err
	}

	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	// remove the permission.
	err = rbac_client_.DeleteResourcePermissions(audioId)
	if err != nil {
		return err
	}

	// force reload dir
	for i := 0; i < len(dirs); i++ {
		srv.publish("reload_dir_event", []byte(dirs[i]))
	}

	// publish delete video event.
	return srv.publish("delete_audio_event", []byte(audioId))
}

func (srv *server) DeleteAlbum(ctx context.Context, rqst *titlepb.DeleteAlbumRequest) (*titlepb.DeleteAlbumResponse, error) {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := generateUUID(rqst.AlbumId)
	err = index.Delete(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = index.DeleteInternal([]byte(uuid))
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

	associations := srv.getAssociations(rqst.IndexPath)

	if associations == nil {
		associations = storage_store.NewBadger_store()
		srv.associations.Store(rqst.IndexPath, associations)
		// open in it own thread
		associations.Open(`{"path":"` + rqst.IndexPath + `", "name":"titles"}`)
	}

	data, err := associations.GetItem(uuid)
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
		audio, err := srv.getAudioById(rqst.IndexPath, association.Titles[i])
		if err == nil {
			audios = append(audios, audio)
		}
	}

	return &titlepb.GetFileAudiosResponse{Audios: &titlepb.Audios{Audios: audios}}, nil
}

// /////////////////// rbac service functions ////////////////////////////////////
func (server *server) getRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(server.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// /////////////////// event service functions ////////////////////////////////////
func (server *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(server.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// Publish event on the event server.
func (svr *server) publish(event string, data []byte) error {
	eventClient, err := svr.getEventClient()
	if err != nil {
		return err
	}
	return eventClient.Publish(event, data)
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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Functionalities to find Title information and asscociate it with file."
	s_impl.Keywords = []string{"Search", "Movie", "Title", "Episode", "MultiMedia", "IMDB"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 8)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.KeepAlive = true
	s_impl.associations = new(sync.Map)

	// Set Permissions.
	s_impl.Permissions[0] = map[string]interface{}{"action": "/title.TitleService/DeleteVideo", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[1] = map[string]interface{}{"action": "/title.TitleService/CreateVideo", "resources": []interface{}{map[string]interface{}{"index": 0, "field": "ID", "permission": "write"}}}

	s_impl.Permissions[2] = map[string]interface{}{"action": "/title.TitleService/DeleteAudio", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[3] = map[string]interface{}{"action": "/title.TitleService/CreateAudio", "resources": []interface{}{map[string]interface{}{"index": 0, "field": "ID", "permission": "write"}}}

	s_impl.Permissions[4] = map[string]interface{}{"action": "/title.TitleService/DeleteTitle", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[5] = map[string]interface{}{"action": "/title.TitleService/CreateTitle", "resources": []interface{}{map[string]interface{}{"index": 0, "field": "ID", "permission": "write"}}}

	s_impl.Permissions[6] = map[string]interface{}{"action": "/title.TitleService/AssociateFileWithTitle", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}, map[string]interface{}{"index": 1, "permission": "read"}}}
	s_impl.Permissions[7] = map[string]interface{}{"action": "/title.TitleService/DissociateFileWithTitle", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}, map[string]interface{}{"index": 1, "permission": "read"}}}

	// Give base info to retreive it configuration.
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
