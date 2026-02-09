package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/storage/storagepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const BufferSize = 1024 * 5 // chunk size used for streaming values

// connection represents a configured storage backend.
type connection struct {
	Id   string
	Name string
	Type storagepb.StoreType
}

// server implements the Storage service and Globular lifecycle hooks.
type server struct {
	// Core metadata / config
	Id              string
	Name            string
	Mac             string
	Domain          string
	Address         string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	Protocol        string
	Version         string
	PublisherID     string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Permissions     []interface{}
	Dependencies    []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64

	// TLS
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// Runtime
	grpcServer *grpc.Server

	// Storage runtime
	Connections map[string]connection
	stores      map[string]storage_store.Store
	storeLocks  sync.Map // map[id]*sync.Mutex
}

// Globular contract: getters/setters
func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }
func (srv *server) GetAddress() string               { return srv.Address }
func (srv *server) SetAddress(address string)        { srv.Address = address }
func (srv *server) GetProcess() int                  { return srv.Process }
func (srv *server) SetProcess(pid int)               { srv.Process = pid }
func (srv *server) GetProxyProcess() int             { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)          { srv.ProxyProcess = pid }
func (srv *server) GetState() string                 { return srv.State }
func (srv *server) SetState(state string)            { srv.State = state }
func (srv *server) GetLastError() string             { return srv.LastError }
func (srv *server) SetLastError(err string)          { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)         { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                { return srv.ModTime }
func (srv *server) GetId() string                    { return srv.Id }
func (srv *server) SetId(id string)                  { srv.Id = id }
func (srv *server) GetName() string                  { return srv.Name }
func (srv *server) SetName(name string)              { srv.Name = name }
func (srv *server) GetMac() string                   { return srv.Mac }
func (srv *server) SetMac(mac string)                { srv.Mac = mac }
func (srv *server) GetDescription() string           { return srv.Description }
func (srv *server) SetDescription(d string)          { srv.Description = d }
func (srv *server) GetKeywords() []string            { return srv.Keywords }
func (srv *server) SetKeywords(k []string)           { srv.Keywords = k }
func (srv *server) GetRepositories() []string        { return srv.Repositories }
func (srv *server) SetRepositories(r []string)       { srv.Repositories = r }
func (srv *server) GetDiscoveries() []string         { return srv.Discoveries }
func (srv *server) SetDiscoveries(d []string)        { srv.Discoveries = d }
func (srv *server) GetChecksum() string              { return srv.Checksum }
func (srv *server) SetChecksum(cs string)            { srv.Checksum = cs }
func (srv *server) GetPlatform() string              { return srv.Plaform }
func (srv *server) SetPlatform(p string)             { srv.Plaform = p }
func (srv *server) GetPath() string                  { return srv.Path }
func (srv *server) SetPath(path string)              { srv.Path = path }
func (srv *server) GetProto() string                 { return srv.Proto }
func (srv *server) SetProto(proto string)            { srv.Proto = proto }
func (srv *server) GetPort() int                     { return srv.Port }
func (srv *server) SetPort(port int)                 { srv.Port = port }
func (srv *server) GetProxy() int                    { return srv.Proxy }
func (srv *server) SetProxy(proxy int)               { srv.Proxy = proxy }
func (srv *server) GetProtocol() string              { return srv.Protocol }
func (srv *server) SetProtocol(p string)             { srv.Protocol = p }
func (srv *server) GetAllowAllOrigins() bool         { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)        { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string        { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)       { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string                { return srv.Domain }
func (srv *server) SetDomain(d string)               { srv.Domain = d }
func (srv *server) GetTls() bool                     { return srv.TLS }
func (srv *server) SetTls(b bool)                    { srv.TLS = b }
func (srv *server) GetCertAuthorityTrust() string    { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)  { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string              { return srv.CertFile }
func (srv *server) SetCertFile(cf string)            { srv.CertFile = cf }
func (srv *server) GetKeyFile() string               { return srv.KeyFile }
func (srv *server) SetKeyFile(kf string)             { srv.KeyFile = kf }
func (srv *server) GetVersion() string               { return srv.Version }
func (srv *server) SetVersion(v string)              { srv.Version = v }
func (srv *server) GetPublisherID() string           { return srv.PublisherID }
func (srv *server) SetPublisherID(pid string)        { srv.PublisherID = pid }
func (srv *server) GetKeepUpToDate() bool            { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)         { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool               { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)            { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}    { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})   { srv.Permissions = p }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
	}
}

func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:storage.viewer",
			Name:        "Storage Viewer",
			Domain:      domain,
			Description: "Read-only: open stores and read items.",
			Actions: []string{
				"/storage.StorageService/Open",
				"/storage.StorageService/GetItem",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:storage.writer",
			Name:        "Storage Writer",
			Domain:      domain,
			Description: "Read and write items; can close/clear stores.",
			Actions: []string{
				"/storage.StorageService/Open",
				"/storage.StorageService/Close",
				"/storage.StorageService/GetItem",
				"/storage.StorageService/SetItem",
				"/storage.StorageService/SetLargeItem",
				"/storage.StorageService/RemoveItem",
				"/storage.StorageService/Clear",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:storage.admin",
			Name:        "Storage Admin",
			Domain:      domain,
			Description: "Full control over storage connections and stores.",
			Actions: []string{
				"/storage.StorageService/Stop",
				"/storage.StorageService/CreateConnection",
				"/storage.StorageService/DeleteConnection",
				"/storage.StorageService/Open",
				"/storage.StorageService/Close",
				"/storage.StorageService/GetItem",
				"/storage.StorageService/SetItem",
				"/storage.StorageService/SetLargeItem",
				"/storage.StorageService/RemoveItem",
				"/storage.StorageService/Clear",
				"/storage.StorageService/Drop",
			},
			TypeName: "resource.Role",
		},
	}
}

// Init prepares config/runtime and initializes the gRPC server.
func (srv *server) Init() error {
	srv.stores = make(map[string]storage_store.Store)
	srv.Connections = make(map[string]connection)

	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService begins serving gRPC (and proxy if configured).
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop is the gRPC endpoint to stop the service.
func (srv *server) Stop(context.Context, *storagepb.StopRequest) (*storagepb.StopResponse, error) {
	return &storagepb.StopResponse{}, srv.StopService()
}

// GetGrpcServer exposes the underlying grpc.Server for lifecycle manager.
func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }

//////////////////////// Storage-specific RPCs ////////////////////////////

// CreateConnection creates or replaces a connection definition and persists it.
func (srv *server) CreateConnection(ctx context.Context, rqst *storagepb.CreateConnectionRqst) (*storagepb.CreateConnectionRsp, error) {
	if rqst.Connection == nil {
		return nil, errors.New("create connection: request missing connection object")
	}

	if srv.stores == nil {
		srv.stores = make(map[string]storage_store.Store)
	}
	if srv.Connections == nil {
		srv.Connections = make(map[string]connection)
	}

	// Close any existing store for this connection id
	if prev, ok := srv.Connections[rqst.Connection.Id]; ok {
		if st := srv.stores[prev.Id]; st != nil {
			_ = st.Close()
		}
	}

	conn := connection{
		Id:   rqst.Connection.Id,
		Name: rqst.Connection.Name,
		Type: rqst.Connection.Type,
	}
	srv.Connections[conn.Id] = conn

	if err := srv.Save(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	logger.Info("connection created/updated", "id", conn.Id, "name", conn.Name, "type", conn.Type.String())

	return &storagepb.CreateConnectionRsp{Result: true}, nil
}

func (srv *server) withStoreLock(id string, fn func() error) error {
	muIface, _ := srv.storeLocks.LoadOrStore(id, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// DeleteConnection removes a connection and persists the config update.
func (srv *server) DeleteConnection(ctx context.Context, rqst *storagepb.DeleteConnectionRqst) (*storagepb.DeleteConnectionRsp, error) {
	id := rqst.GetId()

	_ = srv.withStoreLock(id, func() error {
		if st, ok := srv.stores[id]; ok && st != nil {
			// IMPORTANT: Do not call Close() first. Drop() must be able to handle closing.
			_ = st.Drop()
			delete(srv.stores, id)
		}
		return nil
	})

	delete(srv.Connections, id)
	// If you persist config, keep this (it was removed in your snippet).
	if err := srv.Save(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &storagepb.DeleteConnectionRsp{Result: true}, nil
}

// Open initializes the selected store with provided options.
func (srv *server) Open(ctx context.Context, rqst *storagepb.OpenRqst) (*storagepb.OpenRsp, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("open: no connection found with id "+rqst.GetId())))
	}

	var store storage_store.Store
	conn := srv.Connections[rqst.GetId()]

	switch conn.Type {
	case storagepb.StoreType_LEVEL_DB:
		store = storage_store.NewLevelDB_store()
	case storagepb.StoreType_BIG_CACHE:
		store = storage_store.NewBigCache_store()
	case storagepb.StoreType_BADGER_DB:
		store = storage_store.NewBadger_store()
	case storagepb.StoreType_SCYLLA_DB:
		store = storage_store.NewScylla_store("127.0.0.1", "", 3)
	default:
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("open: unsupported store type for connection id "+rqst.GetId())))
	}

	if err := store.Open(rqst.GetOptions()); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.stores[rqst.GetId()] = store
	logger.Info("store opened", "id", rqst.GetId(), "type", conn.Type.String())
	return &storagepb.OpenRsp{Result: true}, nil
}

// Close shuts down the store connected to the given connection id.
func (srv *server) Close(ctx context.Context, rqst *storagepb.CloseRqst) (*storagepb.CloseRsp, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("close: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("close: no store found for connection id "+rqst.GetId())))
	}

	if err := store.Close(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	logger.Info("store closed", "id", rqst.GetId())
	return &storagepb.CloseRsp{Result: true}, nil
}

// SetItem writes a small value under the given key.
func (srv *server) SetItem(ctx context.Context, rqst *storagepb.SetItemRequest) (*storagepb.SetItemResponse, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("setItem: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("setItem: no store found for connection id "+rqst.GetId())))
	}

	if err := store.SetItem(rqst.GetKey(), rqst.GetValue()); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &storagepb.SetItemResponse{Result: true}, nil
}

// SetLargeItem streams chunks and stores the concatenated value under the given key.
func (srv *server) SetLargeItem(stream storagepb.StorageService_SetLargeItemServer) error {
	var rqst *storagepb.SetLargeItemRequest
	var buffer bytes.Buffer

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			rqst = msg // keep last non-nil for id/key
			if err := stream.SendAndClose(&storagepb.SetLargeItemResponse{}); err != nil {
				logger.Error("setLargeItem: send-and-close failed", "err", err)
				return err
			}
			break
		}
		if err != nil {
			return err
		}
		rqst = msg
		buffer.Write(msg.Value)
	}

	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("setLargeItem: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("setLargeItem: no store found for connection id "+rqst.GetId())))
	}

	if err := store.SetItem(rqst.GetKey(), buffer.Bytes()); err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return nil
}

// GetItem streams back the stored value in fixed-size chunks.
func (srv *server) GetItem(rqst *storagepb.GetItemRequest, stream storagepb.StorageService_GetItemServer) error {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("getItem: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("getItem: no store found for connection id "+rqst.GetId())))
	}

	value, err := store.GetItem(rqst.GetKey())
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	reader := bytes.NewReader(value)
	for {
		var data [BufferSize]byte
		n, rerr := reader.Read(data[:])
		if n > 0 {
			if err := stream.Send(&storagepb.GetItemResponse{Result: data[:n]}); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

// RemoveItem deletes a specific key.
func (srv *server) RemoveItem(ctx context.Context, rqst *storagepb.RemoveItemRequest) (*storagepb.RemoveItemResponse, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("removeItem: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("removeItem: no store found for connection id "+rqst.GetId())))
	}

	if err := store.RemoveItem(rqst.GetKey()); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &storagepb.RemoveItemResponse{Result: true}, nil
}

// Clear removes all keys/values from the store.
func (srv *server) Clear(ctx context.Context, rqst *storagepb.ClearRequest) (*storagepb.ClearResponse, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("clear: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("clear: no store found for connection id "+rqst.GetId())))
	}

	if err := store.Clear(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &storagepb.ClearResponse{Result: true}, nil
}

// Drop destroys the underlying storage (if supported) and closes it.
func (srv *server) Drop(ctx context.Context, rqst *storagepb.DropRequest) (*storagepb.DropResponse, error) {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("drop: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("drop: no store found for connection id "+rqst.GetId())))
	}

	if err := store.Drop(); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	delete(srv.stores, rqst.GetId())

	logger.Info("store dropped", "id", rqst.GetId())
	return &storagepb.DropResponse{Result: true}, nil
}

// GetAllKeys streams back all keys in the store.
func (srv *server) GetAllKeys(rqst *storagepb.GetAllKeysRequest, stream storagepb.StorageService_GetAllKeysServer) error {
	if _, ok := srv.Connections[rqst.GetId()]; !ok {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("getAllKeys: no connection found with id "+rqst.GetId())))
	}
	store := srv.stores[rqst.GetId()]
	if store == nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(),
			errors.New("getAllKeys: no store found for connection id "+rqst.GetId())))
	}

	keys, err := store.GetAllKeys()
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	const chunkSize = 100
	for i := 0; i < len(keys); i += chunkSize {
		end := i + chunkSize
		if end > len(keys) {
			end = len(keys)
		}
		if err := stream.Send(&storagepb.GetAllKeysResponse{Keys: keys[i:end]}); err != nil {
			return err
		}
	}

	return nil
}
