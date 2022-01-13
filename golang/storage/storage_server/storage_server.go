package main

import (
	"context"

	"bytes"
	"errors"
	"io"
	"log"
	"os"
	"strconv"

	//	"time"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/storage/storage_client"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/storage/storagepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	// "google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

const BufferSize = 1024 * 5 // the chunck size.

var (
	defaultPort  = 10013
	defaultProxy = 10014

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// The domain
	domain string = "localhost"
)

// Keep connection information here.
type connection struct {
	Id   string // The connection id
	Name string // The kv store name
	Type storagepb.StoreType
}

type server struct {

	// The global attribute of the services.
	Id              string
	Name            string
	Mac             string
	Path            string
	Proto           string
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
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	ModTime 		int64

	// storage_server-signed X.509 public keys for distribution
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

	// The map of connection...
	Connections map[string]connection

	// the map of store
	stores map[string]storage_store.Store
}

func (svr *server) GetProcess() int {
	return svr.Process
}

func (svr *server) SetProcess(pid int) {
	svr.SetProcess(pid)
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
func (storage_server *server) GetId() string {
	return storage_server.Id
}
func (storage_server *server) SetId(id string) {
	storage_server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (storage_server *server) GetName() string {
	return storage_server.Name
}
func (storage_server *server) SetName(name string) {
	storage_server.Name = name
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The description of the service
func (storage_server *server) GetDescription() string {
	return storage_server.Description
}
func (storage_server *server) SetDescription(description string) {
	storage_server.Description = description
}

// The list of keywords of the services.
func (storage_server *server) GetKeywords() []string {
	return storage_server.Keywords
}
func (storage_server *server) SetKeywords(keywords []string) {
	storage_server.Keywords = keywords
}

func (storage_server *server) GetRepositories() []string {
	return storage_server.Repositories
}
func (storage_server *server) SetRepositories(repositories []string) {
	storage_server.Repositories = repositories
}

func (storage_server *server) GetDiscoveries() []string {
	return storage_server.Discoveries
}
func (storage_server *server) SetDiscoveries(discoveries []string) {
	storage_server.Discoveries = discoveries
}

// Dist
func (storage_server *server) Dist(path string) (string, error) {

	return globular.Dist(path, storage_server)
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

func (storage_server *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (storage_server *server) GetPath() string {
	return storage_server.Path
}
func (storage_server *server) SetPath(path string) {
	storage_server.Path = path
}

// The path of the .proto file.
func (storage_server *server) GetProto() string {
	return storage_server.Proto
}
func (storage_server *server) SetProto(proto string) {
	storage_server.Proto = proto
}

// The gRpc port.
func (storage_server *server) GetPort() int {
	return storage_server.Port
}
func (storage_server *server) SetPort(port int) {
	storage_server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (storage_server *server) GetProxy() int {
	return storage_server.Proxy
}
func (storage_server *server) SetProxy(proxy int) {
	storage_server.Proxy = proxy
}

// Can be one of http/https/tls
func (storage_server *server) GetProtocol() string {
	return storage_server.Protocol
}
func (storage_server *server) SetProtocol(protocol string) {
	storage_server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (storage_server *server) GetAllowAllOrigins() bool {
	return storage_server.AllowAllOrigins
}
func (storage_server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	storage_server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (storage_server *server) GetAllowedOrigins() string {
	return storage_server.AllowedOrigins
}

func (storage_server *server) SetAllowedOrigins(allowedOrigins string) {
	storage_server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (storage_server *server) GetDomain() string {
	return storage_server.Domain
}
func (storage_server *server) SetDomain(domain string) {
	storage_server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (storage_server *server) GetTls() bool {
	return storage_server.TLS
}
func (storage_server *server) SetTls(hasTls bool) {
	storage_server.TLS = hasTls
}

// The certificate authority file
func (storage_server *server) GetCertAuthorityTrust() string {
	return storage_server.CertAuthorityTrust
}
func (storage_server *server) SetCertAuthorityTrust(ca string) {
	storage_server.CertAuthorityTrust = ca
}

// The certificate file.
func (storage_server *server) GetCertFile() string {
	return storage_server.CertFile
}
func (storage_server *server) SetCertFile(certFile string) {
	storage_server.CertFile = certFile
}

// The key file.
func (storage_server *server) GetKeyFile() string {
	return storage_server.KeyFile
}
func (storage_server *server) SetKeyFile(keyFile string) {
	storage_server.KeyFile = keyFile
}

// The service version
func (storage_server *server) GetVersion() string {
	return storage_server.Version
}
func (storage_server *server) SetVersion(version string) {
	storage_server.Version = version
}

// The publisher id.
func (storage_server *server) GetPublisherId() string {
	return storage_server.PublisherId
}
func (storage_server *server) SetPublisherId(publisherId string) {
	storage_server.PublisherId = publisherId
}

func (storage_server *server) GetKeepUpToDate() bool {
	return storage_server.KeepUpToDate
}
func (storage_server *server) SetKeepUptoDate(val bool) {
	storage_server.KeepUpToDate = val
}

func (storage_server *server) GetKeepAlive() bool {
	return storage_server.KeepAlive
}
func (storage_server *server) SetKeepAlive(val bool) {
	storage_server.KeepAlive = val
}

func (storage_server *server) GetPermissions() []interface{} {
	return storage_server.Permissions
}
func (storage_server *server) SetPermissions(permissions []interface{}) {
	storage_server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (storage_server *server) Init() error {

	storage_server.stores = make(map[string]storage_store.Store)
	storage_server.Connections = make(map[string]connection)

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewStorageService_Client", storage_client.NewStorageService_Client)

	err := globular.InitService(storage_server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	storage_server.grpcServer, err = globular.InitGrpcServer(storage_server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (storage_server *server) Save() error {
	// Create the file...
	return globular.SaveService(storage_server)
}
 
func (storage_server *server) StartService() error {
	return globular.StartService(storage_server, storage_server.grpcServer)
}

func (storage_server *server) StopService() error {
	return globular.StopService(storage_server, storage_server.grpcServer)
}

func (storage_server *server) Stop(context.Context, *storagepb.StopRequest) (*storagepb.StopResponse, error) {
	return &storagepb.StopResponse{}, storage_server.StopService()
}

//////////////////////// Storage specific functions ////////////////////////////

// Create a new KV connection and store it for futur use. If the connection already
// exist it will be replace by the new one.
func (storage_server *server) CreateConnection(ctx context.Context, rsqt *storagepb.CreateConnectionRqst) (*storagepb.CreateConnectionRsp, error) {
	if rsqt.Connection == nil {
		return nil, errors.New("The request dosent contain connection object!")
	}

	if storage_server.stores == nil {
		storage_server.stores = make(map[string]storage_store.Store)
	}

	if storage_server.Connections == nil {
		storage_server.Connections = make(map[string]connection)
	} else {
		if _, ok := storage_server.Connections[rsqt.Connection.Id]; ok {
			if storage_server.stores[rsqt.Connection.Id] != nil {
				storage_server.stores[rsqt.Connection.Id].Close() // close the previous connection.
			}
		}
	}

	var c connection
	var err error

	// Set the connection info from the request.
	c.Id = rsqt.Connection.Id
	c.Name = rsqt.Connection.Name

	// set or update the connection and save it in json file.
	storage_server.Connections[c.Id] = c

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// In that case I will save it in file.
	err = storage_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &storagepb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Remove a connection from the map and the file.
func (storage_server *server) DeleteConnection(ctx context.Context, rqst *storagepb.DeleteConnectionRqst) (*storagepb.DeleteConnectionRsp, error) {

	id := rqst.GetId()
	if _, ok := storage_server.Connections[id]; !ok {
		return &storagepb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	delete(storage_server.Connections, id)

	// In that case I will save it in file.
	err := storage_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return success.
	return &storagepb.DeleteConnectionRsp{
		Result: true,
	}, nil

}

// Open the store and set options...
func (storage_server *server) Open(ctx context.Context, rqst *storagepb.OpenRqst) (*storagepb.OpenRsp, error) {
	if _, ok := storage_server.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.GetId())))
	}

	var store storage_store.Store
	conn := storage_server.Connections[rqst.GetId()]

	// Create the store object.
	if conn.Type == storagepb.StoreType_LEVEL_DB {
		store = storage_store.NewLevelDB_store()
	} else if conn.Type == storagepb.StoreType_BIG_CACHE {
		store = storage_store.NewBigCache_store()
	}

	if store == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	err := store.Open(rqst.GetOptions())

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	storage_server.stores[rqst.GetId()] = store

	return &storagepb.OpenRsp{
		Result: true,
	}, nil
}

// Close the data store.
func (storage_server *server) Close(ctx context.Context, rqst *storagepb.CloseRqst) (*storagepb.CloseRsp, error) {
	if _, ok := storage_server.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.GetId())))
	}

	if storage_server.stores[rqst.GetId()] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	err := storage_server.stores[rqst.GetId()].Close()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &storagepb.CloseRsp{
		Result: true,
	}, nil
}

// Save an item in the kv store
func (storage_server *server) SetItem(ctx context.Context, rqst *storagepb.SetItemRequest) (*storagepb.SetItemResponse, error) {

	if _, ok := storage_server.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.GetId())))
	}

	store := storage_server.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	err := store.SetItem(rqst.GetKey(), rqst.GetValue())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &storagepb.SetItemResponse{
		Result: true,
	}, nil
}

// Save an item in the kv store
func (storage_server *server) SetLargeItem(stream storagepb.StorageService_SetLargeItemServer) error {

	var rqst *storagepb.SetLargeItemRequest
	var buffer bytes.Buffer
	var err error

	for {
		rqst, err = stream.Recv()
		if err == io.EOF {
			// end of stream...
			stream.SendAndClose(&storagepb.SetLargeItemResponse{})
			break
		} else if err != nil {
			return err
		} else {
			buffer.Write(rqst.Value)
		}
	}

	if _, ok := storage_server.Connections[rqst.GetId()]; !ok {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.GetId())))
	}

	store := storage_server.stores[rqst.GetId()]
	if store == nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	err = store.SetItem(rqst.GetKey(), buffer.Bytes())
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// Retreive a value with a given key
func (storage_server *server) GetItem(rqst *storagepb.GetItemRequest, stream storagepb.StorageService_GetItemServer) error {

	if _, ok := storage_server.Connections[rqst.GetId()]; !ok {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.GetId())))
	}

	store := storage_server.stores[rqst.GetId()]
	if store == nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	value, err := store.GetItem(rqst.GetKey())
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	buffer := bytes.NewReader(value) // create the buffer and set it data.

	for {
		var data [BufferSize]byte
		bytesread, err := buffer.Read(data[0:BufferSize])
		if bytesread > 0 {
			rqst := &storagepb.GetItemResponse{
				Result: data[0:bytesread],
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

// Remove an item with a given key
func (storage_server *server) RemoveItem(ctx context.Context, rqst *storagepb.RemoveItemRequest) (*storagepb.RemoveItemResponse, error) {
	if _, ok := storage_server.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.GetId())))
	}

	store := storage_server.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	if store == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	err := store.RemoveItem(rqst.GetKey())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &storagepb.RemoveItemResponse{
		Result: true,
	}, nil

}

// Remove all items
func (storage_server *server) Clear(ctx context.Context, rqst *storagepb.ClearRequest) (*storagepb.ClearResponse, error) {
	if _, ok := storage_server.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.GetId())))
	}

	store := storage_server.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	err := store.Clear()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &storagepb.ClearResponse{
		Result: true,
	}, nil
}

// Drop a store
func (storage_server *server) Drop(ctx context.Context, rqst *storagepb.DropRequest) (*storagepb.DropResponse, error) {
	if _, ok := storage_server.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no connection found with id "+rqst.GetId())))
	}

	store := storage_server.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no store found for connection with id "+rqst.GetId())))
	}

	err := store.Drop()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &storagepb.DropResponse{
		Result: true,
	}, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "storage_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Connections = make(map[string]connection)
	s_impl.Name = string(storagepb.File_storage_proto.Services().Get(0).FullName())
	s_impl.Proto = storagepb.File_storage_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.PublisherId = "globulario"
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	
	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the echo services
	storagepb.RegisterStorageServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)
	// Start the service.
	s_impl.StartService()

}
