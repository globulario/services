package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/monitoring/monitoring_store"
	"github.com/globulario/services/golang/monitoring/monitoringpb"

	"github.com/davecourtois/Utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10019
	defaultProxy = 10020

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""
)

// Keep connection information here.
type connection struct {
	Id   string // The connection id
	Host string // can also be ipv4 addresse.
	Port int32
	Type monitoringpb.StoreType // Only Prometheus at this time.
}

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Mac             string
	Name            string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Domain          string
	Address         string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string

	// monitoring_server-signed X.509 public keys for distribution
	CertFile string
	// a private RSA key to sign and authenticate the public key
	KeyFile string
	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string
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

	// That map contain the list of active connections.
	Connections map[string]connection
	stores      map[string]monitoring_store.Store
}

// The path of the configuration.
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
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
func (monitoring_server *server) GetId() string {
	return monitoring_server.Id
}
func (monitoring_server *server) SetId(id string) {
	monitoring_server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (monitoring_server *server) GetName() string {
	return monitoring_server.Name
}
func (monitoring_server *server) SetName(name string) {
	monitoring_server.Name = name
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The description of the service
func (monitoring_server *server) GetDescription() string {
	return monitoring_server.Description
}
func (monitoring_server *server) SetDescription(description string) {
	monitoring_server.Description = description
}

// The list of keywords of the services.
func (monitoring_server *server) GetKeywords() []string {
	return monitoring_server.Keywords
}
func (monitoring_server *server) SetKeywords(keywords []string) {
	monitoring_server.Keywords = keywords
}

// Dist
func (monitoring_server *server) Dist(path string) (string, error) {

	return globular.Dist(path, monitoring_server)
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

func (monitoring_server *server) GetRepositories() []string {
	return monitoring_server.Repositories
}
func (monitoring_server *server) SetRepositories(repositories []string) {
	monitoring_server.Repositories = repositories
}

func (monitoring_server *server) GetDiscoveries() []string {
	return monitoring_server.Discoveries
}
func (monitoring_server *server) SetDiscoveries(discoveries []string) {
	monitoring_server.Discoveries = discoveries
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
func (monitoring_server *server) GetPath() string {
	return monitoring_server.Path
}
func (monitoring_server *server) SetPath(path string) {
	monitoring_server.Path = path
}

// The path of the .proto file.
func (monitoring_server *server) GetProto() string {
	return monitoring_server.Proto
}
func (monitoring_server *server) SetProto(proto string) {
	monitoring_server.Proto = proto
}

// The gRpc port.
func (monitoring_server *server) GetPort() int {
	return monitoring_server.Port
}
func (monitoring_server *server) SetPort(port int) {
	monitoring_server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (monitoring_server *server) GetProxy() int {
	return monitoring_server.Proxy
}
func (monitoring_server *server) SetProxy(proxy int) {
	monitoring_server.Proxy = proxy
}

// Can be one of http/https/tls
func (monitoring_server *server) GetProtocol() string {
	return monitoring_server.Protocol
}
func (monitoring_server *server) SetProtocol(protocol string) {
	monitoring_server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (monitoring_server *server) GetAllowAllOrigins() bool {
	return monitoring_server.AllowAllOrigins
}
func (monitoring_server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	monitoring_server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (monitoring_server *server) GetAllowedOrigins() string {
	return monitoring_server.AllowedOrigins
}

func (monitoring_server *server) SetAllowedOrigins(allowedOrigins string) {
	monitoring_server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (monitoring_server *server) GetDomain() string {
	return monitoring_server.Domain
}
func (monitoring_server *server) SetDomain(domain string) {
	monitoring_server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (monitoring_server *server) GetTls() bool {
	return monitoring_server.TLS
}
func (monitoring_server *server) SetTls(hasTls bool) {
	monitoring_server.TLS = hasTls
}

// The certificate authority file
func (monitoring_server *server) GetCertAuthorityTrust() string {
	return monitoring_server.CertAuthorityTrust
}
func (monitoring_server *server) SetCertAuthorityTrust(ca string) {
	monitoring_server.CertAuthorityTrust = ca
}

// The certificate file.
func (monitoring_server *server) GetCertFile() string {
	return monitoring_server.CertFile
}
func (monitoring_server *server) SetCertFile(certFile string) {
	monitoring_server.CertFile = certFile
}

// The key file.
func (monitoring_server *server) GetKeyFile() string {
	return monitoring_server.KeyFile
}
func (monitoring_server *server) SetKeyFile(keyFile string) {
	monitoring_server.KeyFile = keyFile
}

// The service version
func (monitoring_server *server) GetVersion() string {
	return monitoring_server.Version
}
func (monitoring_server *server) SetVersion(version string) {
	monitoring_server.Version = version
}

// The publisher id.
func (monitoring_server *server) GetPublisherId() string {
	return monitoring_server.PublisherId
}
func (monitoring_server *server) SetPublisherId(publisherId string) {
	monitoring_server.PublisherId = publisherId
}

func (monitoring_server *server) GetKeepUpToDate() bool {
	return monitoring_server.KeepUpToDate
}
func (monitoring_server *server) SetKeepUptoDate(val bool) {
	monitoring_server.KeepUpToDate = val
}

func (monitoring_server *server) GetKeepAlive() bool {
	return monitoring_server.KeepAlive
}
func (monitoring_server *server) SetKeepAlive(val bool) {
	monitoring_server.KeepAlive = val
}

func (monitoring_server *server) GetPermissions() []interface{} {
	return monitoring_server.Permissions
}
func (monitoring_server *server) SetPermissions(permissions []interface{}) {
	monitoring_server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (monitoring_server *server) Init() error {
	monitoring_server.stores = make(map[string]monitoring_store.Store)
	monitoring_server.Connections = make(map[string]connection)

	err := globular.InitService(monitoring_server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	monitoring_server.grpcServer, err = globular.InitGrpcServer(monitoring_server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// init store for existiong connection.
	for _, c := range monitoring_server.Connections {

		var store monitoring_store.Store
		var err error
		address := "http://" + c.Host + ":" + Utility.ToString(c.Port)

		if c.Type == monitoringpb.StoreType_PROMETHEUS {
			store, err = monitoring_store.NewPrometheusStore(address)
		}

		if err != nil {
			return err
		}

		if store == nil {
			return errors.New("Fail to connect to store!")
		}

		// Keep the ref to the store.
		monitoring_server.stores[c.Id] = store
	}

	return nil

}

// Save the configuration values.
func (monitoring_server *server) Save() error {

	return globular.SaveService(monitoring_server)
}

func (monitoring_server *server) StartService() error {
	return globular.StartService(monitoring_server, monitoring_server.grpcServer)
}

func (monitoring_server *server) StopService() error {
	return globular.StopService(monitoring_server, monitoring_server.grpcServer)
}

func (monitoring_server *server) Stop(context.Context, *monitoringpb.StopRequest) (*monitoringpb.StopResponse, error) {
	return &monitoringpb.StopResponse{}, monitoring_server.StopService()
}

// /////////////////// Monitoring specific functions ////////////////////////////
func (monitoring_server *server) createConnection(id string, host string, port int32, storeType monitoringpb.StoreType) error {
	var c connection

	// Set the connection info from the request.
	c.Id = id
	c.Host = host
	c.Port = port
	c.Type = storeType

	if monitoring_server.Connections == nil {
		monitoring_server.Connections = make(map[string]connection, 0)
	}

	// set or update the connection and save it in json file.
	monitoring_server.Connections[c.Id] = c

	var store monitoring_store.Store
	var err error
	address := "http://" + c.Host + ":" + Utility.ToString(c.Port)

	if c.Type == monitoringpb.StoreType_PROMETHEUS {
		store, err = monitoring_store.NewPrometheusStore(address)
	}

	if err != nil {
		return err
	}

	if store == nil {
		return errors.New("Fail to connect to store!")
	}

	// Keep the ref to the store.
	monitoring_server.stores[c.Id] = store

	// In that case I will save it in file.
	err = monitoring_server.Save()
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	// Print the success message here.
	return nil
}

// Create a connection.
func (monitoring_server *server) CreateConnection(ctx context.Context, rqst *monitoringpb.CreateConnectionRqst) (*monitoringpb.CreateConnectionRsp, error) {

	err := monitoring_server.createConnection(rqst.Connection.Id, rqst.Connection.Host, rqst.Connection.Port, rqst.Connection.Store)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Print the success message here.
	return &monitoringpb.CreateConnectionRsp{
		Result: true,
	}, nil
}

// Delete a connection.
func (monitoring_server *server) DeleteConnection(ctx context.Context, rqst *monitoringpb.DeleteConnectionRqst) (*monitoringpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := monitoring_server.Connections[id]; !ok {
		return &monitoringpb.DeleteConnectionRsp{
			Result: true,
		}, nil
	}

	delete(monitoring_server.Connections, id)

	// In that case I will save it in file.
	err := monitoring_server.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// return success.
	return &monitoringpb.DeleteConnectionRsp{
		Result: true,
	}, nil

}

// Alerts returns a list of all active alerts.
func (monitoring_server *server) Alerts(ctx context.Context, rqst *monitoringpb.AlertsRequest) (*monitoringpb.AlertsResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	str, err := store.Alerts(ctx)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.AlertsResponse{
		Results: str,
	}, nil
}

// AlertManagers returns an overview of the current state of the Prometheus alert manager discovery.
func (monitoring_server *server) AlertManagers(ctx context.Context, rqst *monitoringpb.AlertManagersRequest) (*monitoringpb.AlertManagersResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	str, err := store.AlertManagers(ctx)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.AlertManagersResponse{
		Results: str,
	}, nil
}

// CleanTombstones removes the deleted data from disk and cleans up the existing tombstones.
func (monitoring_server *server) CleanTombstones(ctx context.Context, rqst *monitoringpb.CleanTombstonesRequest) (*monitoringpb.CleanTombstonesResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err := store.CleanTombstones(ctx)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.CleanTombstonesResponse{
		Result: true,
	}, nil
}

// Config returns the current Prometheus configuration.
func (monitoring_server *server) Config(ctx context.Context, rqst *monitoringpb.ConfigRequest) (*monitoringpb.ConfigResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	configStr, err := store.Config(ctx)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.ConfigResponse{
		Results: configStr,
	}, nil
}

// DeleteSeries deletes data for a selection of series in a time range.
func (monitoring_server *server) DeleteSeries(ctx context.Context, rqst *monitoringpb.DeleteSeriesRequest) (*monitoringpb.DeleteSeriesResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// convert input arguments...
	startTime := time.Unix(int64(rqst.GetStartTime()), 0)
	endTime := time.Unix(int64(rqst.GetEndTime()), 0)

	err := store.DeleteSeries(ctx, rqst.GetMatches(), startTime, endTime)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.DeleteSeriesResponse{
		Result: true,
	}, nil
}

// Flags returns the flag values that Prometheus was launched with.
func (monitoring_server *server) Flags(ctx context.Context, rqst *monitoringpb.FlagsRequest) (*monitoringpb.FlagsResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	str, err := store.Flags(ctx)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.FlagsResponse{
		Results: str,
	}, nil
}

// LabelNames returns all the unique label names present in the block in sorted order.
func (monitoring_server *server) LabelNames(ctx context.Context, rqst *monitoringpb.LabelNamesRequest) (*monitoringpb.LabelNamesResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	strs, str, err := store.LabelNames(ctx)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.LabelNamesResponse{
		Labels:   strs,
		Warnings: str,
	}, nil
}

// LabelValues performs a query for the values of the given label.
func (monitoring_server *server) LabelValues(ctx context.Context, rqst *monitoringpb.LabelValuesRequest) (*monitoringpb.LabelValuesResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resultStr, warnings, err := store.LabelValues(ctx, rqst.Label, rqst.Values, rqst.StartTime, rqst.EndTime)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.LabelValuesResponse{
		LabelValues: resultStr,
		Warnings:    warnings,
	}, nil
}

// Query performs a query for the given time.
func (monitoring_server *server) Query(ctx context.Context, rqst *monitoringpb.QueryRequest) (*monitoringpb.QueryResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	ts := time.Unix(int64(rqst.GetTs()), 0)
	resultStr, warnings, err := store.Query(ctx, rqst.Query, ts)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.QueryResponse{
		Value:    resultStr,
		Warnings: warnings,
	}, nil
}

// QueryRange performs a query for the given range.
func (monitoring_server *server) QueryRange(rqst *monitoringpb.QueryRangeRequest, stream monitoringpb.MonitoringService_QueryRangeServer) error {

	store := monitoring_server.stores[rqst.ConnectionId]
	ctx := stream.Context()

	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	startTime := time.Unix(int64(rqst.GetStartTime()), 0)
	endTime := time.Unix(int64(rqst.GetEndTime()), 0)
	step := rqst.Step

	resultStr, warnings, err := store.QueryRange(ctx, rqst.GetQuery(), startTime, endTime, step)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	maxSize := 2000
	for i := 0; i < len(resultStr); i += maxSize {
		rsp := new(monitoringpb.QueryRangeResponse)
		rsp.Warnings = warnings
		if i+maxSize < len(resultStr) {
			rsp.Value = resultStr[i : i+maxSize]
		} else {
			rsp.Value = resultStr[i:]
		}
		stream.Send(rsp)
	}

	return nil
}

// Series finds series by label matchers.
func (monitoring_server *server) Series(ctx context.Context, rqst *monitoringpb.SeriesRequest) (*monitoringpb.SeriesResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	startTime := time.Unix(int64(rqst.GetStartTime()), 0)
	endTime := time.Unix(int64(rqst.GetEndTime()), 0)

	resultStr, warnings, err := store.Series(ctx, rqst.GetMatches(), startTime, endTime)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.SeriesResponse{
		LabelSet: resultStr,
		Warnings: warnings,
	}, nil
}

// Snapshot creates a snapshot of all current data into snapshots/<datetime>-<rand>
// under the TSDB's data directory and returns the directory as response.
func (monitoring_server *server) Snapshot(ctx context.Context, rqst *monitoringpb.SnapshotRequest) (*monitoringpb.SnapshotResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resultStr, err := store.Snapshot(ctx, rqst.GetSkipHead())

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.SnapshotResponse{
		Result: resultStr,
	}, nil
}

// Rules returns a list of alerting and recording rules that are currently loaded.
func (monitoring_server *server) Rules(ctx context.Context, rqst *monitoringpb.RulesRequest) (*monitoringpb.RulesResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resultStr, err := store.Rules(ctx)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.RulesResponse{
		Result: resultStr,
	}, nil
}

// Targets returns an overview of the current state of the Prometheus target discovery.
func (monitoring_server *server) Targets(ctx context.Context, rqst *monitoringpb.TargetsRequest) (*monitoringpb.TargetsResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resultStr, err := store.Targets(ctx)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.TargetsResponse{
		Result: resultStr,
	}, nil
}

// TargetsMetadata returns metadata about metrics currently scraped by the target.
func (monitoring_server *server) TargetsMetadata(ctx context.Context, rqst *monitoringpb.TargetsMetadataRequest) (*monitoringpb.TargetsMetadataResponse, error) {
	store := monitoring_server.stores[rqst.ConnectionId]
	if store == nil {
		err := errors.New("No store connection exist for id " + rqst.ConnectionId)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resultStr, err := store.TargetsMetadata(ctx, rqst.GetMatchTarget(), rqst.GetMetric(), rqst.GetLimit())

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &monitoringpb.TargetsMetadataResponse{
		Result: resultStr,
	}, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "monitoring_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(monitoringpb.File_monitoring_proto.Services().Get(0).FullName())
	s_impl.Proto = monitoringpb.File_monitoring_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.PublisherId = "globulario@globule-dell.globular.cloud"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	// TODO set it from the program arguments...
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
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
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	// Register the echo services
	monitoringpb.RegisterMonitoringServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
