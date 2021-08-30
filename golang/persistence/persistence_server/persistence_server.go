package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	//"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/persistence/persistencepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const (
	BufferSize = 1024 * 5 // the chunck size.
)

var (
	defaultPort  = 10035
	defaultProxy = 10036

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// The default domain
	domain string = "localhost"

	// The grpc server.
	grpcServer *grpc.Server
)

// This is the connction to a datastore.
type connection struct {
	Id       string
	Name     string
	Host     string
	Store    persistencepb.StoreType
	User     string
	Port     int32
	Timeout  int32
	Options  string
	Password string
}

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id                 string
	Mac                string
	Name               string
	Path               string
	Port               int
	Proto              string
	Proxy              int
	Protocol           string
	AllowAllOrigins    bool
	AllowedOrigins     string // comma separated string.
	Domain             string
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	CertAuthorityTrust string
	CertFile           string
	KeyFile            string
	TLS                bool
	Version            string
	PublisherId        string
	KeepUpToDate       bool
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process	int
	ProxyProcess int
	ConfigPath string
	LastError string

	// The grpc server.
	grpcServer *grpc.Server

	// saved connections
	Connections map[string]connection

	// unsaved connections
	connections map[string]connection

	// The map of store (also connections...)
	stores map[string]persistence_store.Store
}

// Globular services implementation...
// The id of a particular service instance.
func (persistence_server *server) GetId() string {
	return persistence_server.Id
}
func (persistence_server *server) SetId(id string) {
	persistence_server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (persistence_server *server) GetName() string {
	return persistence_server.Name
}
func (persistence_server *server) SetName(name string) {
	persistence_server.Name = name
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The description of the service
func (persistence_server *server) GetDescription() string {
	return persistence_server.Description
}
func (persistence_server *server) SetDescription(description string) {
	persistence_server.Description = description
}

// The list of keywords of the services.
func (persistence_server *server) GetKeywords() []string {
	return persistence_server.Keywords
}
func (persistence_server *server) SetKeywords(keywords []string) {
	persistence_server.Keywords = keywords
}

func (persistence_server *server) GetRepositories() []string {
	return persistence_server.Repositories
}
func (persistence_server *server) SetRepositories(repositories []string) {
	persistence_server.Repositories = repositories
}

func (persistence_server *server) GetDiscoveries() []string {
	return persistence_server.Discoveries
}
func (persistence_server *server) SetDiscoveries(discoveries []string) {
	persistence_server.Discoveries = discoveries
}

// Dist
func (persistence_server *server) Dist(path string) (string, error) {

	return globular.Dist(path, persistence_server)
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

func (persistence_server *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (persistence_server *server) GetPath() string {
	return persistence_server.Path
}
func (persistence_server *server) SetPath(path string) {
	persistence_server.Path = path
}

// The path of the .proto file.
func (persistence_server *server) GetProto() string {
	return persistence_server.Proto
}
func (persistence_server *server) SetProto(proto string) {
	persistence_server.Proto = proto
}

// The gRpc port.
func (persistence_server *server) GetPort() int {
	return persistence_server.Port
}
func (persistence_server *server) SetPort(port int) {
	persistence_server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (persistence_server *server) GetProxy() int {
	return persistence_server.Proxy
}
func (persistence_server *server) SetProxy(proxy int) {
	persistence_server.Proxy = proxy
}

// Can be one of http/https/tls
func (persistence_server *server) GetProtocol() string {
	return persistence_server.Protocol
}
func (persistence_server *server) SetProtocol(protocol string) {
	persistence_server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (persistence_server *server) GetAllowAllOrigins() bool {
	return persistence_server.AllowAllOrigins
}
func (persistence_server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	persistence_server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (persistence_server *server) GetAllowedOrigins() string {
	return persistence_server.AllowedOrigins
}

func (persistence_server *server) SetAllowedOrigins(allowedOrigins string) {
	persistence_server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (persistence_server *server) GetDomain() string {
	return persistence_server.Domain
}
func (persistence_server *server) SetDomain(domain string) {
	persistence_server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (persistence_server *server) GetTls() bool {
	return persistence_server.TLS
}
func (persistence_server *server) SetTls(hasTls bool) {
	persistence_server.TLS = hasTls
}

// The certificate authority file
func (persistence_server *server) GetCertAuthorityTrust() string {
	return persistence_server.CertAuthorityTrust
}
func (persistence_server *server) SetCertAuthorityTrust(ca string) {
	persistence_server.CertAuthorityTrust = ca
}

// The certificate file.
func (persistence_server *server) GetCertFile() string {
	return persistence_server.CertFile
}
func (persistence_server *server) SetCertFile(certFile string) {
	persistence_server.CertFile = certFile
}

// The key file.
func (persistence_server *server) GetKeyFile() string {
	return persistence_server.KeyFile
}
func (persistence_server *server) SetKeyFile(keyFile string) {
	persistence_server.KeyFile = keyFile
}

// The service version
func (persistence_server *server) GetVersion() string {
	return persistence_server.Version
}
func (persistence_server *server) SetVersion(version string) {
	persistence_server.Version = version
}

// The publisher id.
func (persistence_server *server) GetPublisherId() string {
	return persistence_server.PublisherId
}
func (persistence_server *server) SetPublisherId(publisherId string) {
	persistence_server.PublisherId = publisherId
}

func (persistence_server *server) GetKeepUpToDate() bool {
	return persistence_server.KeepUpToDate
}
func (persistence_server *server) SetKeepUptoDate(val bool) {
	persistence_server.KeepUpToDate = val
}

func (persistence_server *server) GetKeepAlive() bool {
	return persistence_server.KeepAlive
}
func (persistence_server *server) SetKeepAlive(val bool) {
	persistence_server.KeepAlive = val
}

func (persistence_server *server) GetPermissions() []interface{} {
	return persistence_server.Permissions
}
func (persistence_server *server) SetPermissions(permissions []interface{}) {
	persistence_server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (persistence_server *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)

	// init the connections
	persistence_server.connections = make(map[string]connection)

	// initialyse store connection here.
	persistence_server.stores = make(map[string]persistence_store.Store)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", persistence_server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	persistence_server.grpcServer, err = globular.InitGrpcServer(persistence_server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// Here I will initialyse the connection.
	for _, c := range persistence_server.Connections {

		if c.Store == persistencepb.StoreType_MONGO {
			// here I will create a new mongo data store.
			s := new(persistence_store.MongoStore)

			// Now I will try to connect...
			err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options)
			// keep the store for futur call...
			if err == nil {
				persistence_server.stores[c.Id] = s
			} else {
				return err
			}
		}
	}

	return nil
}

// Save the configuration values.
func (persistence_server *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", persistence_server)
}

func (persistence_server *server) StartService() error {
	return globular.StartService(persistence_server, persistence_server.grpcServer)
}

func (persistence_server *server) StopService() error {
	return globular.StopService(persistence_server, persistence_server.grpcServer)
}


// Singleton.
var (
	log_client_    *log_client.Log_Client
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
		log_client_, err = log_client.NewLogService_Client(server.Domain, "log.LogService")
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

////////////////////////////////////////////////////////////////////////////////////////
// Resource manager function
////////////////////////////////////////////////////////////////////////////////////////
func (persistence_server *server) createConnection(ctx context.Context,user, password, id, name, host string, port int32, store persistencepb.StoreType, save bool ) error {

	var c connection
	var err error

	// use existing connection as we can.
	if _, ok := persistence_server.connections[id]; ok {
		c = persistence_server.connections[id]
	} else if _, ok := persistence_server.Connections[id]; ok {
		c = persistence_server.Connections[id]
	} else {

		// Set the connection info from the request.
		c.Id = id
		c.Name =name
		c.Host = host
		c.Port = port
		c.User = user
		c.Password = password
		c.Store = store

		if c.Store == persistencepb.StoreType_MONGO {
			// here I will create a new mongo data store.
			s := new(persistence_store.MongoStore)

			// Now I will try to connect...
			err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options)
			if err != nil {
				// codes.
				return err
			}

			// keep the store for futur call...
			persistence_server.stores[c.Id] = s
		}

		// If the connection need to save in the server configuration.
		if save {
			if persistence_server.Connections == nil {
				persistence_server.Connections = make(map[string]connection)
			}
			persistence_server.Connections[c.Id] = c
			// In that case I will save it in file.
			err = persistence_server.Save()
			if err != nil {
				return err
			}
		} else {
			persistence_server.connections[c.Id] = c
		}
	}

	// test if the connection is reacheable.
	err = persistence_server.stores[c.Id].Ping(ctx, c.Id)

	if err != nil {
		persistence_server.stores[c.Id].Disconnect(c.Id)
		if _, ok := persistence_server.connections[id]; ok {
			delete(persistence_server.connections, id)
		} else if _, ok := persistence_server.Connections[id]; ok {
			delete(persistence_server.Connections, id)
		}
		return err
	}

	// Print the success message here.
	return nil
}

// Create a new Store connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (persistence_server *server) CreateConnection(ctx context.Context, rqst *persistencepb.CreateConnectionRqst) (*persistencepb.CreateConnectionRsp, error) {

	err := persistence_server.createConnection(ctx, rqst.Connection.User, rqst.Connection.Password, rqst.Connection.Id, rqst.Connection.Name, rqst.Connection.Host, rqst.Connection.Port, rqst.Connection.Store, rqst.Save)
	if err != nil {
		// codes.
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Print the success message here.
	return &persistencepb.CreateConnectionRsp{
		Result: true,
	}, nil
}

func (persistence_server *server) Connect(ctx context.Context, rqst *persistencepb.ConnectRqst) (*persistencepb.ConnectRsp, error) {

	store := persistence_server.stores[rqst.GetConnectionId()]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.GetConnectionId())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if c, ok := persistence_server.Connections[rqst.ConnectionId]; ok {

		// So here I will open the connection.
		c.Password = rqst.Password
		if c.Store == persistencepb.StoreType_MONGO {
			// here I will create a new mongo data store.
			s := new(persistence_store.MongoStore)

			// Now I will try to connect...
			err := s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options)

			if err != nil {
				// codes.
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// keep the store for futur call...
			persistence_server.stores[c.Id] = s
		}

		// set or update the connection and save it in json file.
		persistence_server.Connections[c.Id] = c

		return &persistencepb.ConnectRsp{
			Result: true,
		}, nil
	} else {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No connection found with id "+rqst.ConnectionId)))
	}

}

// Close connection.
func (persistence_server *server) Disconnect(ctx context.Context, rqst *persistencepb.DisconnectRqst) (*persistencepb.DisconnectRsp, error) {
	store := persistence_server.stores[rqst.GetConnectionId()]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.GetConnectionId())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.Disconnect(rqst.GetConnectionId())
	if err != nil {
		// codes.
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.DisconnectRsp{
		Result: true,
	}, nil
}

// Create a database
func (persistence_server *server) CreateDatabase(ctx context.Context, rqst *persistencepb.CreateDatabaseRqst) (*persistencepb.CreateDatabaseRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("CreateDatabase No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.CreateDatabase(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.CreateDatabaseRsp{
		Result: true,
	}, nil
}

// Delete a database
func (persistence_server *server) DeleteDatabase(ctx context.Context, rqst *persistencepb.DeleteDatabaseRqst) (*persistencepb.DeleteDatabaseRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("DeleteDatabase No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.DeleteDatabase(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.DeleteDatabaseRsp{
		Result: true,
	}, nil
}

// Create a Collection
func (persistence_server *server) CreateCollection(ctx context.Context, rqst *persistencepb.CreateCollectionRqst) (*persistencepb.CreateCollectionRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("CreateCollection No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.CreateCollection(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.OptionsStr)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.CreateCollectionRsp{
		Result: true,
	}, nil
}

// Delete collection
func (persistence_server *server) DeleteCollection(ctx context.Context, rqst *persistencepb.DeleteCollectionRqst) (*persistencepb.DeleteCollectionRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("DeleteCollection No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.DeleteCollection(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.DeleteCollectionRsp{
		Result: true,
	}, nil
}

// Ping a sql connection.
func (persistence_server *server) Ping(ctx context.Context, rqst *persistencepb.PingConnectionRqst) (*persistencepb.PingConnectionRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("ping No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.Ping(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.PingConnectionRsp{
		Result: "pong",
	}, nil
}

// Get the number of entry in a collection
func (persistence_server *server) Count(ctx context.Context, rqst *persistencepb.CountRqst) (*persistencepb.CountRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("Count No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	count, err := store.Count(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.CountRsp{
		Result: count,
	}, nil
}

// Implementation of the Persistence method.
func (persistence_server *server) InsertOne(ctx context.Context, rqst *persistencepb.InsertOneRqst) (*persistencepb.InsertOneRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("InsertOne No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	entity := make(map[string]interface{})
	err := json.Unmarshal([]byte(rqst.Data), &entity)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	var id interface{}
	id, err = store.InsertOne(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, entity, rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := json.Marshal(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.InsertOneRsp{
		Id: string(jsonStr),
	}, nil
}

func (persistence_server *server) InsertMany(stream persistencepb.PersistenceService_InsertManyServer) error {

	var buffer bytes.Buffer
	var rqst *persistencepb.InsertManyRqst
	var err error
	var connectionId string
	var database string
	var collection string

	for {
		rqst, err = stream.Recv()
		if err == io.EOF {
			// end of stream...
			stream.SendAndClose(&persistencepb.InsertManyRsp{})
			break
		} else if err != nil {
			return err
		} else if len(rqst.Data) > 0 {
			if len(rqst.Collection) > 0 {
				collection = rqst.Collection
			}
			if len(rqst.Id) > 0 {
				connectionId = rqst.Id
			}
			if len(rqst.Database) > 0 {
				database = rqst.Database
			}
			buffer.Write(rqst.Data)
		} else {
			break
		}
	}

	// The buffer that contain the
	entities := make([]interface{}, 0)
	err = json.Unmarshal(buffer.Bytes(), &entities)
	if err != nil {
		return err
	}

	connectionId = strings.ReplaceAll(strings.ReplaceAll(connectionId, "@", "_"), ".", "_")
	database = strings.ReplaceAll(strings.ReplaceAll(database, "@", "_"), ".", "_")

	_, err = persistence_server.stores[connectionId].InsertMany(stream.Context(), connectionId, database, collection, entities, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// Find many
func (persistence_server *server) Find(rqst *persistencepb.FindRqst, stream persistencepb.PersistenceService_FindServer) error {

	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]

	if store == nil {
		err := errors.New("Find No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	results, err := store.Find(stream.Context(), strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	err = enc.Encode(results)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for {
		var data [BufferSize]byte
		bytesread, err := buffer.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &persistencepb.FindResp{
				Data: data[0:bytesread],
			}
			// send the data to the server.
			err = stream.Send(rqst)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}
	return nil
}

func (persistence_server *server) Aggregate(rqst *persistencepb.AggregateRqst, stream persistencepb.PersistenceService_AggregateServer) error {

	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]

	if store == nil {
		err := errors.New("Aggregate No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	results, err := store.Aggregate(stream.Context(), strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Pipeline, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	err = enc.Encode(results)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for {
		var data [BufferSize]byte
		bytesread, err := buffer.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &persistencepb.AggregateResp{
				Data: data[0:bytesread],
			}
			// send the data to the server.
			err = stream.Send(rqst)
		}

		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
	}

	return nil
}

// Find one
func (persistence_server *server) FindOne(ctx context.Context, rqst *persistencepb.FindOneRqst) (*persistencepb.FindOneResp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("FindOne No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	result, err := store.FindOne(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		err = errors.New(rqst.Collection + " " + rqst.Query + " " + err.Error())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	obj_, err := Utility.ToMap(result)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	obj, err := structpb.NewStruct(obj_)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.FindOneResp{
		Result: obj,
	}, nil
}

// Update a single or many value depending of the query
func (persistence_server *server) Update(ctx context.Context, rqst *persistencepb.UpdateRqst) (*persistencepb.UpdateRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("Update No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.Update(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Value, rqst.Options)
	if err != nil {
		return nil, err
	}

	return &persistencepb.UpdateRsp{
		Result: true,
	}, nil
}

// Update a single docuemnt value(s)
func (persistence_server *server) UpdateOne(ctx context.Context, rqst *persistencepb.UpdateOneRqst) (*persistencepb.UpdateOneRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("UpdateOne No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.UpdateOne(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Value, rqst.Options)
	if err != nil {
		return nil, err
	}

	return &persistencepb.UpdateOneRsp{
		Result: true,
	}, nil
}

// Replace one document by another.
func (persistence_server *server) ReplaceOne(ctx context.Context, rqst *persistencepb.ReplaceOneRqst) (*persistencepb.ReplaceOneRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("ReplaceOne No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_") + " collection: " + rqst.Collection + " query: " + rqst.Query)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.ReplaceOne(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Value, rqst.Options)
	if err != nil {
		return nil, err
	}

	return &persistencepb.ReplaceOneRsp{
		Result: true,
	}, nil
}

// Delete many or one.
func (persistence_server *server) Delete(ctx context.Context, rqst *persistencepb.DeleteRqst) (*persistencepb.DeleteRsp, error) {
	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("Delete No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.Delete(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		return nil, err
	}

	return &persistencepb.DeleteRsp{
		Result: true,
	}, nil
}

// Delete one document at time
func (persistence_server *server) DeleteOne(ctx context.Context, rqst *persistencepb.DeleteOneRqst) (*persistencepb.DeleteOneRsp, error) {

	store := persistence_server.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
	if store == nil {
		err := errors.New("DeleteOne No store connection exist for id " + strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"))
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.DeleteOne(ctx, strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_"), strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		return nil, err
	}
	return &persistencepb.DeleteOneRsp{
		Result: true,
	}, nil
}

// Remove a connection from the map and the file.
func (persistence_server *server) DeleteConnection(ctx context.Context, rqst *persistencepb.DeleteConnectionRqst) (*persistencepb.DeleteConnectionRsp, error) {

	id := strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")
	if _, ok := persistence_server.Connections[id]; !ok {
		return &persistencepb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	delete(persistence_server.Connections, id)

	// In that case I will save it in file.
	err := persistence_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return success.
	return &persistencepb.DeleteConnectionRsp{
		Result: true,
	}, nil
}

// Create a new user.
func (persistence_server *server) RunAdminCmd(ctx context.Context, rqst *persistencepb.RunAdminCmdRqst) (*persistencepb.RunAdminCmdRsp, error) {
	store := persistence_server.stores[rqst.GetConnectionId()]
	if store == nil {
		err := errors.New("RunAdminCmd No store connection exist for id " + rqst.GetConnectionId())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.RunAdminCmd(ctx, rqst.GetConnectionId(), rqst.User, rqst.Password, rqst.Script)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.RunAdminCmdRsp{
		Result: "",
	}, nil
}

func (persistence_server *server) Stop(context.Context, *persistencepb.StopRequest) (*persistencepb.StopResponse, error) {
	return &persistencepb.StopResponse{}, persistence_server.StopService()
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(persistencepb.File_persistence_proto.Services().Get(0).FullName())
	s_impl.Port = defaultPort
	s_impl.Proto = persistencepb.File_persistence_proto.Path()
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "globulario"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"log.LogService"}
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the echo services
	persistencepb.RegisterPersistenceServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
