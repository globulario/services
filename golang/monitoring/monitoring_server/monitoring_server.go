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

	// srv-signed X.509 public keys for distribution
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
	srv.stores = make(map[string]monitoring_store.Store)
	srv.Connections = make(map[string]connection)

	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	// init store for existiong connection.
	for _, c := range srv.Connections {

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
		srv.stores[c.Id] = store
	}

	return nil

}

// Save the configuration values.
func (srv *server) Save() error {

	return globular.SaveService(srv)
}

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

func (srv *server) Stop(context.Context, *monitoringpb.StopRequest) (*monitoringpb.StopResponse, error) {
	return &monitoringpb.StopResponse{}, srv.StopService()
}

// /////////////////// Monitoring specific functions ////////////////////////////
func (srv *server) createConnection(id string, host string, port int32, storeType monitoringpb.StoreType) error {
	var c connection

	// Set the connection info from the request.
	c.Id = id
	c.Host = host
	c.Port = port
	c.Type = storeType

	if srv.Connections == nil {
		srv.Connections = make(map[string]connection, 0)
	}

	// set or update the connection and save it in json file.
	srv.Connections[c.Id] = c

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
	srv.stores[c.Id] = store

	// In that case I will save it in file.
	err = srv.Save()
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
func (srv *server) CreateConnection(ctx context.Context, rqst *monitoringpb.CreateConnectionRqst) (*monitoringpb.CreateConnectionRsp, error) {

	err := srv.createConnection(rqst.Connection.Id, rqst.Connection.Host, rqst.Connection.Port, rqst.Connection.Store)
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
func (srv *server) DeleteConnection(ctx context.Context, rqst *monitoringpb.DeleteConnectionRqst) (*monitoringpb.DeleteConnectionRsp, error) {
	id := rqst.GetId()
	if _, ok := srv.Connections[id]; !ok {
		return &monitoringpb.DeleteConnectionRsp{
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
	return &monitoringpb.DeleteConnectionRsp{
		Result: true,
	}, nil

}

// Alerts returns a list of all active alerts.
func (srv *server) Alerts(ctx context.Context, rqst *monitoringpb.AlertsRequest) (*monitoringpb.AlertsResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) AlertManagers(ctx context.Context, rqst *monitoringpb.AlertManagersRequest) (*monitoringpb.AlertManagersResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) CleanTombstones(ctx context.Context, rqst *monitoringpb.CleanTombstonesRequest) (*monitoringpb.CleanTombstonesResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) Config(ctx context.Context, rqst *monitoringpb.ConfigRequest) (*monitoringpb.ConfigResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) DeleteSeries(ctx context.Context, rqst *monitoringpb.DeleteSeriesRequest) (*monitoringpb.DeleteSeriesResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) Flags(ctx context.Context, rqst *monitoringpb.FlagsRequest) (*monitoringpb.FlagsResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) LabelNames(ctx context.Context, rqst *monitoringpb.LabelNamesRequest) (*monitoringpb.LabelNamesResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) LabelValues(ctx context.Context, rqst *monitoringpb.LabelValuesRequest) (*monitoringpb.LabelValuesResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) Query(ctx context.Context, rqst *monitoringpb.QueryRequest) (*monitoringpb.QueryResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) QueryRange(rqst *monitoringpb.QueryRangeRequest, stream monitoringpb.MonitoringService_QueryRangeServer) error {

	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) Series(ctx context.Context, rqst *monitoringpb.SeriesRequest) (*monitoringpb.SeriesResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) Snapshot(ctx context.Context, rqst *monitoringpb.SnapshotRequest) (*monitoringpb.SnapshotResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) Rules(ctx context.Context, rqst *monitoringpb.RulesRequest) (*monitoringpb.RulesResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) Targets(ctx context.Context, rqst *monitoringpb.TargetsRequest) (*monitoringpb.TargetsResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
func (srv *server) TargetsMetadata(ctx context.Context, rqst *monitoringpb.TargetsMetadataRequest) (*monitoringpb.TargetsMetadataResponse, error) {
	store := srv.stores[rqst.ConnectionId]
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
	s_impl.KeepUpToDate = true

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
	monitoringpb.RegisterMonitoringServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
