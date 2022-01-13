package monitoring_client

import (
	"errors"
	"io"
	"strconv"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/monitoring/monitoringpb"
	"github.com/globulario/services/golang/security"

	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// Monitoring Client Service
////////////////////////////////////////////////////////////////////////////////

type Monitoring_Client struct {
	cc *grpc.ClientConn
	c  monitoringpb.MonitoringServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The client domain
	domain string

	// The port
	port int

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewMonitoringService_Client(address string, id string) (*Monitoring_Client, error) {
	client := new(Monitoring_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = monitoringpb.NewMonitoringServiceClient(client.cc)

	return client, nil
}

// Return the configuration from the configuration server.
func (client *Monitoring_Client) GetConfiguration(address string) (map[string]interface{}, error) {
	return nil, errors.New("no implemented...")
}

func (client *Monitoring_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Monitoring_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (client *Monitoring_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Monitoring_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the id of the service instance
func (client *Monitoring_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Monitoring_Client) GetName() string {
	return client.name
}

func (client *Monitoring_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Monitoring_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Monitoring_Client) SetPort(port int) {
	client.port = port
}

// Set the client id.
func (client *Monitoring_Client) SetId(id string) {
	client.id = id
}

func (client *Monitoring_Client) SetMac(mac string) {
	client.name = mac
}

// Set the client name.
func (client *Monitoring_Client) SetName(name string) {
	client.name = name
}

// Set the domain.
func (client *Monitoring_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Monitoring_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Monitoring_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Monitoring_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Monitoring_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Monitoring_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Monitoring_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Monitoring_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Monitoring_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Connections management functions //////////////////////////

// Stop the service.
func (client *Monitoring_Client) StopService() {
	client.c.Stop(client.GetCtx(), &monitoringpb.StopRequest{})
}

// Create a new connection.
func (client *Monitoring_Client) CreateConnection(id string, host string, storeType float64, port float64) error {
	rqst := &monitoringpb.CreateConnectionRqst{
		Connection: &monitoringpb.Connection{
			Id:    id,
			Host:  host,
			Port:  int32(port),
			Store: monitoringpb.StoreType(int32(storeType)),
		},
	}

	_, err := client.c.CreateConnection(client.GetCtx(), rqst)

	return err
}

// Delete a connection.
func (client *Monitoring_Client) DeleteConnection(id string) error {
	rqst := &monitoringpb.DeleteConnectionRqst{
		Id: id,
	}

	_, err := client.c.DeleteConnection(client.GetCtx(), rqst)

	return err
}

// Config returns the current Prometheus configuration.
func (client *Monitoring_Client) Config(connectionId string) (string, error) {
	rqst := &monitoringpb.ConfigRequest{
		ConnectionId: connectionId,
	}

	rsp, err := client.c.Config(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResults(), nil
}

// Alerts returns a list of all active alerts.
func (client *Monitoring_Client) Alerts(connectionId string) (string, error) {
	rqst := &monitoringpb.AlertsRequest{
		ConnectionId: connectionId,
	}

	rsp, err := client.c.Alerts(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResults(), nil
}

// AlertManagers returns an overview of the current state of the Prometheus alert manager discovery.
func (client *Monitoring_Client) AlertManagers(connectionId string) (string, error) {
	rqst := &monitoringpb.AlertManagersRequest{
		ConnectionId: connectionId,
	}

	rsp, err := client.c.AlertManagers(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResults(), nil
}

// CleanTombstones removes the deleted data from disk and cleans up the existing tombstones.
func (client *Monitoring_Client) CleanTombstones(connectionId string) error {
	rqst := &monitoringpb.CleanTombstonesRequest{
		ConnectionId: connectionId,
	}

	_, err := client.c.CleanTombstones(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}

// DeleteSeries deletes data for a selection of series in a time range.
func (client *Monitoring_Client) DeleteSeries(connectionId string, matches []string, startTime float64, endTime float64) error {
	rqst := &monitoringpb.DeleteSeriesRequest{
		ConnectionId: connectionId,
		Matches:      matches,
		StartTime:    startTime,
		EndTime:      endTime,
	}

	_, err := client.c.DeleteSeries(client.GetCtx(), rqst)
	if err != nil {
		return err
	}
	return nil
}

// Flags returns the flag values that Prometheus was launched with.
func (client *Monitoring_Client) Flags(connectionId string) (string, error) {
	rqst := &monitoringpb.FlagsRequest{
		ConnectionId: connectionId,
	}

	rsp, err := client.c.Flags(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResults(), nil
}

// LabelNames returns all the unique label names present in the block in sorted order.
func (client *Monitoring_Client) LabelNames(connectionId string) ([]string, string, error) {
	rqst := &monitoringpb.LabelNamesRequest{
		ConnectionId: connectionId,
	}

	rsp, err := client.c.LabelNames(client.GetCtx(), rqst)
	if err != nil {
		return nil, "", err
	}

	return rsp.GetLabels(), rsp.GetWarnings(), nil
}

// LabelValues performs a query for the values of the given label.
func (client *Monitoring_Client) LabelValues(connectionId string, label string) (string, string, error) {
	rqst := &monitoringpb.LabelValuesRequest{
		ConnectionId: connectionId,
		Label:        label,
	}

	rsp, err := client.c.LabelValues(client.GetCtx(), rqst)
	if err != nil {
		return "", "", err
	}

	return rsp.GetLabelValues(), rsp.GetWarnings(), nil
}

// Query performs a query for the given time.
func (client *Monitoring_Client) Query(connectionId string, query string, ts float64) (string, string, error) {
	rqst := &monitoringpb.QueryRequest{
		ConnectionId: connectionId,
		Query:        query,
		Ts:           ts,
	}

	rsp, err := client.c.Query(client.GetCtx(), rqst)
	if err != nil {
		return "", "", err
	}

	return rsp.GetValue(), rsp.GetWarnings(), nil
}

// QueryRange performs a query for the given range.
func (client *Monitoring_Client) QueryRange(connectionId string, query string, startTime float64, endTime float64, step float64) (string, string, error) {
	rqst := &monitoringpb.QueryRangeRequest{
		ConnectionId: connectionId,
		Query:        query,
		StartTime:    startTime,
		EndTime:      endTime,
		Step:         step,
	}

	var value string
	var warning string
	stream, err := client.c.QueryRange(client.GetCtx(), rqst)
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}
		if err != nil {
			return "", "", err
		}

		// Get the result...
		value += msg.GetValue()
		warning = msg.GetWarnings()
	}

	if err != nil {
		return "", "", err
	}

	return value, warning, nil
}

// Series finds series by label matchers.
func (client *Monitoring_Client) Series(connectionId string, matches []string, startTime float64, endTime float64) (string, string, error) {
	rqst := &monitoringpb.SeriesRequest{
		ConnectionId: connectionId,
		Matches:      matches,
		StartTime:    startTime,
		EndTime:      endTime,
	}

	rsp, err := client.c.Series(client.GetCtx(), rqst)
	if err != nil {
		return "", "", err
	}

	return rsp.GetLabelSet(), rsp.GetWarnings(), nil
}

// Snapshot creates a snapshot of all current data into snapshots/<datetime>-<rand>
// under the TSDB's data directory and returns the directory as response.
func (client *Monitoring_Client) Snapshot(connectionId string, skipHead bool) (string, error) {
	rqst := &monitoringpb.SnapshotRequest{
		ConnectionId: connectionId,
		SkipHead:     skipHead,
	}

	rsp, err := client.c.Snapshot(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}

// Rules returns a list of alerting and recording rules that are currently loaded.
func (client *Monitoring_Client) Rules(connectionId string) (string, error) {
	rqst := &monitoringpb.RulesRequest{
		ConnectionId: connectionId,
	}

	rsp, err := client.c.Rules(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}

// Targets returns an overview of the current state of the Prometheus target discovery.
func (client *Monitoring_Client) Targets(connectionId string) (string, error) {
	rqst := &monitoringpb.TargetsRequest{
		ConnectionId: connectionId,
	}

	rsp, err := client.c.Targets(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}

// TargetsMetadata returns metadata about metrics currently scraped by the target.
func (client *Monitoring_Client) TargetsMetadata(connectionId string, matchTarget string, metric string, limit string) (string, error) {
	rqst := &monitoringpb.TargetsMetadataRequest{
		ConnectionId: connectionId,
		MatchTarget:  matchTarget,
		Metric:       metric,
		Limit:        limit,
	}

	rsp, err := client.c.TargetsMetadata(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}
