package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	//"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	Utility "github.com/globulario/utility"

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
	Address            string
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
	Plaform            string
	Checksum           string
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	State              string

	// The grpc server.
	grpcServer *grpc.Server

	// saved connections
	Connections map[string]connection

	// unsaved connections
	connections map[string]connection

	// The map of store (also connections...)
	stores map[string]persistence_store.Store
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

func (srv *server) GetMac() string {
	return srv.Mac
}

func (srv *server) SetMac(mac string) {
	srv.Mac = mac
}

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}

func (srv *server) SetDescription(description string) {
	srv.Description = description
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

	// init the connections
	srv.connections = make(map[string]connection)

	// initialyse store connection here.
	srv.stores = make(map[string]persistence_store.Store)

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

	// Here I will initialyse the connection.
	for _, c := range srv.Connections {

		if c.Store == persistencepb.StoreType_MONGO {
			// here I will create a new mongo data store.
			s := new(persistence_store.MongoStore)

			// Now I will try to connect...
			err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options)
			// keep the store for futur call...
			if err == nil {
				srv.stores[c.Id] = s
			} else {
				return err
			}
		} else if c.Store == persistencepb.StoreType_SQL {
			s := new(persistence_store.SqlStore)
			err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options)
			if err == nil {
				srv.stores[c.Id] = s
			} else {
				return err
			}
		}
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

// Singleton.
var (
	log_client_ *log_client.Log_Client
)

////////////////////////////////////////////////////////////////////////////////////////
// Logger function
////////////////////////////////////////////////////////////////////////////////////////
/**
 * Get the log client.
 */
func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	// validate the port has not change...
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

// //////////////////////////////////////////////////////////////////////////////////////
// Resource manager function
// //////////////////////////////////////////////////////////////////////////////////////
func (srv *server) createConnection(ctx context.Context, user, password, id, name, host string, port int32, store persistencepb.StoreType, save bool, options string) error {

	fmt.Println("createConnection", id, name, host, port, store, save, options)

	var c connection
	var err error

	if host == "0.0.0.0" || host == "localhost" {
		host, _ = config.GetDomain()
	}

	// use existing connection as we can.
	if _, ok := srv.connections[id]; ok {
		c = srv.connections[id]
		if c.Password != password {
			return errors.New("a connection with id " + id + " already exist")
		} else {
			return nil // the connection already exist.
		}

	} else {

		// Set the connection info from the request.
		c.Id = id
		c.Name = name
		c.Host = host
		c.Port = port
		c.User = user
		c.Password = password
		c.Store = store
		c.Options = options

		// If the connection need to save in the server configuration.
		if save {
			if srv.Connections == nil {
				srv.Connections = make(map[string]connection)
			}

			srv.Connections[c.Id] = c

			// In that case I will save it in file.
			err = srv.Save()
			if err != nil {
				return err
			}

		} else {
			srv.connections[c.Id] = c
		}
	}

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
		srv.stores[c.Id] = s
	} else if c.Store == persistencepb.StoreType_SQL {
		s := new(persistence_store.SqlStore)
		err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options)
		if err != nil {

			// codes.
			return err
		}
		srv.stores[c.Id] = s
	} else if c.Store == persistencepb.StoreType_SCYLLA {
		s := new(persistence_store.ScyllaStore)
		err = s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options)
		if err != nil {
			fmt.Println("fail to connect with error ", err)
			// codes.
			return err
		}
		srv.stores[c.Id] = s
	} else {
		err := errors.New("Store type not supported")
		return err
	}

	// test if the connection is reacheable.
	err = srv.stores[c.Id].Ping(ctx, c.Id)

	// fail to connect with error
	if err != nil {
		srv.stores[c.Id].Disconnect(c.Id)
		return err
	}

	// Print the success message here.
	return nil
}

// Create a new Store connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (srv *server) CreateConnection(ctx context.Context, rqst *persistencepb.CreateConnectionRqst) (*persistencepb.CreateConnectionRsp, error) {

	if rqst.Connection == nil {
		err := errors.New("no connection provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Connection.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := srv.createConnection(ctx, rqst.Connection.User, rqst.Connection.Password, rqst.Connection.Id, rqst.Connection.Name, rqst.Connection.Host, rqst.Connection.Port, rqst.Connection.Store, rqst.Save, rqst.Connection.Options)
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

func (srv *server) Connect(ctx context.Context, rqst *persistencepb.ConnectRqst) (*persistencepb.ConnectRsp, error) {
	if len(rqst.GetConnectionId()) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[rqst.GetConnectionId()]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.GetConnectionId())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if c, ok := srv.Connections[rqst.ConnectionId]; ok {

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
			srv.stores[c.Id] = s
		} else if c.Store == persistencepb.StoreType_SQL {

			s := new(persistence_store.SqlStore)
			err := s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options)
			if err != nil {
				// codes.
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

		}

		// set or update the connection and save it in json file.
		srv.Connections[c.Id] = c

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
func (srv *server) Disconnect(ctx context.Context, rqst *persistencepb.DisconnectRqst) (*persistencepb.DisconnectRsp, error) {
	if len(rqst.GetConnectionId()) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[rqst.GetConnectionId()]
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
func (srv *server) CreateDatabase(ctx context.Context, rqst *persistencepb.CreateDatabaseRqst) (*persistencepb.CreateDatabaseRsp, error) {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) DeleteDatabase(ctx context.Context, rqst *persistencepb.DeleteDatabaseRqst) (*persistencepb.DeleteDatabaseRsp, error) {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) CreateCollection(ctx context.Context, rqst *persistencepb.CreateCollectionRqst) (*persistencepb.CreateCollectionRsp, error) {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) DeleteCollection(ctx context.Context, rqst *persistencepb.DeleteCollectionRqst) (*persistencepb.DeleteCollectionRsp, error) {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) Ping(ctx context.Context, rqst *persistencepb.PingConnectionRqst) (*persistencepb.PingConnectionRsp, error) {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) Count(ctx context.Context, rqst *persistencepb.CountRqst) (*persistencepb.CountRsp, error) {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) InsertOne(ctx context.Context, rqst *persistencepb.InsertOneRqst) (*persistencepb.InsertOneRsp, error) {

	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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

	jsonStr, err := Utility.ToJson(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &persistencepb.InsertOneRsp{
		Id: string(jsonStr),
	}, nil
}

func (srv *server) InsertMany(stream persistencepb.PersistenceService_InsertManyServer) error {

	var buffer bytes.Buffer
	var rqst *persistencepb.InsertManyRqst
	var err error
	var connectionId string
	var database string
	var collection string

	for {

		rqst, err = stream.Recv()
		if len(rqst.Id) == 0 {
			err := errors.New("no connection id provided")
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if len(rqst.Database) == 0 {
			err := errors.New("no database provided")
			return status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if err == io.EOF {
			// end of stream...
			err_ := stream.SendAndClose(&persistencepb.InsertManyRsp{})
			if err_ != nil {
				fmt.Println("fail send response and close stream with error ", err_)
				return err_
			}
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

	_, err = srv.stores[connectionId].InsertMany(stream.Context(), connectionId, database, collection, entities, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// Find many
func (srv *server) Find(rqst *persistencepb.FindRqst, stream persistencepb.PersistenceService_FindServer) error {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	connectionId := strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")
	store := srv.stores[connectionId]

	if store == nil {
		err := errors.New("Find No store connection exist for id " + connectionId)
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	results, err := store.Find(stream.Context(), connectionId, strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Options)
	if err != nil {
		err_ := errors.New(connectionId + " " + rqst.Collection + " " + rqst.Query + " " + err.Error())
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err_))
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
			// send the data to the srv.
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

func (srv *server) Aggregate(rqst *persistencepb.AggregateRqst, stream persistencepb.PersistenceService_AggregateServer) error {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]

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
			// send the data to the srv.
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
func (srv *server) FindOne(ctx context.Context, rqst *persistencepb.FindOneRqst) (*persistencepb.FindOneResp, error) {

	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	connectionId := strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")
	store := srv.stores[connectionId]
	if store == nil {
		err := errors.New("FindOne No store connection exist for id " + connectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	result, err := store.FindOne(ctx, connectionId, strings.ReplaceAll(strings.ReplaceAll(rqst.Database, "@", "_"), ".", "_"), rqst.Collection, rqst.Query, rqst.Options)
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
func (srv *server) Update(ctx context.Context, rqst *persistencepb.UpdateRqst) (*persistencepb.UpdateRsp, error) {
	if len(rqst.Id) == 0 {
		err := errors.New("FindOne No connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("FindOne No database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) UpdateOne(ctx context.Context, rqst *persistencepb.UpdateOneRqst) (*persistencepb.UpdateOneRsp, error) {
	if len(rqst.Id) == 0 {
		err := errors.New("no connection id provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(rqst.Database) == 0 {
		err := errors.New("no database provided")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) ReplaceOne(ctx context.Context, rqst *persistencepb.ReplaceOneRqst) (*persistencepb.ReplaceOneRsp, error) {

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) Delete(ctx context.Context, rqst *persistencepb.DeleteRqst) (*persistencepb.DeleteRsp, error) {
	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) DeleteOne(ctx context.Context, rqst *persistencepb.DeleteOneRqst) (*persistencepb.DeleteOneRsp, error) {

	store := srv.stores[strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")]
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
func (srv *server) DeleteConnection(ctx context.Context, rqst *persistencepb.DeleteConnectionRqst) (*persistencepb.DeleteConnectionRsp, error) {

	id := strings.ReplaceAll(strings.ReplaceAll(rqst.Id, "@", "_"), ".", "_")
	if _, ok := srv.Connections[id]; !ok {
		return &persistencepb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	delete(srv.Connections, id)

	// In that case I will save it in file.
	err := srv.Save()
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
func (srv *server) RunAdminCmd(ctx context.Context, rqst *persistencepb.RunAdminCmdRqst) (*persistencepb.RunAdminCmdRsp, error) {
	store := srv.stores[rqst.GetConnectionId()]
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

func (srv *server) Stop(context.Context, *persistencepb.StopRequest) (*persistencepb.StopResponse, error) {
	return &persistencepb.StopResponse{}, srv.StopService()
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(persistencepb.File_persistence_proto.Services().Get(0).FullName())
	s_impl.Port = defaultPort
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Proto = persistencepb.File_persistence_proto.Path()
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "localhost"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"log.LogService", "authentication.AuthenticationService", "event.EventService"}
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

	// register new client creator.
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)

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
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Register the echo services
	persistencepb.RegisterPersistenceServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
