package monitoring_client

import (
	"io"
	"strconv"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/monitoring/monitoringpb"

	"context"

	"google.golang.org/grpc"
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

func (monitoring_client *Monitoring_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(monitoring_client)
	}
	return globular.InvokeClientRequest(monitoring_client.c, ctx, method, rqst)
}

// Return the domain
func (monitoring_client *Monitoring_Client) GetDomain() string {
	return monitoring_client.domain
}

// Return the address
func (monitoring_client *Monitoring_Client) GetAddress() string {
	return monitoring_client.domain + ":" + strconv.Itoa(monitoring_client.port)
}

// Return the id of the service instance
func (monitoring_client *Monitoring_Client) GetId() string {
	return monitoring_client.id
}

// Return the name of the service
func (monitoring_client *Monitoring_Client) GetName() string {
	return monitoring_client.name
}

func (monitoring_client *Monitoring_Client) GetMac() string {
	return monitoring_client.mac
}


// must be close when no more needed.
func (monitoring_client *Monitoring_Client) Close() {
	monitoring_client.cc.Close()
}

// Set grpc_service port.
func (monitoring_client *Monitoring_Client) SetPort(port int) {
	monitoring_client.port = port
}

// Set the client id.
func (monitoring_client *Monitoring_Client) SetId(id string) {
	monitoring_client.id = id
}

func (monitoring_client *Monitoring_Client) SetMac(mac string) {
	monitoring_client.name = mac
}

// Set the client name.
func (monitoring_client *Monitoring_Client) SetName(name string) {
	monitoring_client.name = name
}

// Set the domain.
func (monitoring_client *Monitoring_Client) SetDomain(domain string) {
	monitoring_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (monitoring_client *Monitoring_Client) HasTLS() bool {
	return monitoring_client.hasTLS
}

// Get the TLS certificate file path
func (monitoring_client *Monitoring_Client) GetCertFile() string {
	return monitoring_client.certFile
}

// Get the TLS key file path
func (monitoring_client *Monitoring_Client) GetKeyFile() string {
	return monitoring_client.keyFile
}

// Get the TLS key file path
func (monitoring_client *Monitoring_Client) GetCaFile() string {
	return monitoring_client.caFile
}

// Set the client is a secure client.
func (monitoring_client *Monitoring_Client) SetTLS(hasTls bool) {
	monitoring_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (monitoring_client *Monitoring_Client) SetCertFile(certFile string) {
	monitoring_client.certFile = certFile
}

// Set TLS key file path
func (monitoring_client *Monitoring_Client) SetKeyFile(keyFile string) {
	monitoring_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (monitoring_client *Monitoring_Client) SetCaFile(caFile string) {
	monitoring_client.caFile = caFile
}

////////////////// Connections management functions //////////////////////////

// Stop the service.
func (monitoring_client *Monitoring_Client) StopService() {
	monitoring_client.c.Stop(globular.GetClientContext(monitoring_client), &monitoringpb.StopRequest{})
}

// Create a new connection.
func (monitoring_client *Monitoring_Client) CreateConnection(id string, host string, storeType float64, port float64) error {
	rqst := &monitoringpb.CreateConnectionRqst{
		Connection: &monitoringpb.Connection{
			Id:    id,
			Host:  host,
			Port:  int32(port),
			Store: monitoringpb.StoreType(int32(storeType)),
		},
	}

	_, err := monitoring_client.c.CreateConnection(globular.GetClientContext(monitoring_client), rqst)

	return err
}

// Delete a connection.
func (monitoring_client *Monitoring_Client) DeleteConnection(id string) error {
	rqst := &monitoringpb.DeleteConnectionRqst{
		Id: id,
	}

	_, err := monitoring_client.c.DeleteConnection(globular.GetClientContext(monitoring_client), rqst)

	return err
}

// Config returns the current Prometheus configuration.
func (monitoring_client *Monitoring_Client) Config(connectionId string) (string, error) {
	rqst := &monitoringpb.ConfigRequest{
		ConnectionId: connectionId,
	}

	rsp, err := monitoring_client.c.Config(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResults(), nil
}

// Alerts returns a list of all active alerts.
func (monitoring_client *Monitoring_Client) Alerts(connectionId string) (string, error) {
	rqst := &monitoringpb.AlertsRequest{
		ConnectionId: connectionId,
	}

	rsp, err := monitoring_client.c.Alerts(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResults(), nil
}

// AlertManagers returns an overview of the current state of the Prometheus alert manager discovery.
func (monitoring_client *Monitoring_Client) AlertManagers(connectionId string) (string, error) {
	rqst := &monitoringpb.AlertManagersRequest{
		ConnectionId: connectionId,
	}

	rsp, err := monitoring_client.c.AlertManagers(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResults(), nil
}

// CleanTombstones removes the deleted data from disk and cleans up the existing tombstones.
func (monitoring_client *Monitoring_Client) CleanTombstones(connectionId string) error {
	rqst := &monitoringpb.CleanTombstonesRequest{
		ConnectionId: connectionId,
	}

	_, err := monitoring_client.c.CleanTombstones(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

// DeleteSeries deletes data for a selection of series in a time range.
func (monitoring_client *Monitoring_Client) DeleteSeries(connectionId string, matches []string, startTime float64, endTime float64) error {
	rqst := &monitoringpb.DeleteSeriesRequest{
		ConnectionId: connectionId,
		Matches:      matches,
		StartTime:    startTime,
		EndTime:      endTime,
	}

	_, err := monitoring_client.c.DeleteSeries(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

// Flags returns the flag values that Prometheus was launched with.
func (monitoring_client *Monitoring_Client) Flags(connectionId string) (string, error) {
	rqst := &monitoringpb.FlagsRequest{
		ConnectionId: connectionId,
	}

	rsp, err := monitoring_client.c.Flags(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResults(), nil
}

// LabelNames returns all the unique label names present in the block in sorted order.
func (monitoring_client *Monitoring_Client) LabelNames(connectionId string) ([]string, string, error) {
	rqst := &monitoringpb.LabelNamesRequest{
		ConnectionId: connectionId,
	}

	rsp, err := monitoring_client.c.LabelNames(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return nil, "", err
	}

	return rsp.GetLabels(), rsp.GetWarnings(), nil
}

// LabelValues performs a query for the values of the given label.
func (monitoring_client *Monitoring_Client) LabelValues(connectionId string, label string) (string, string, error) {
	rqst := &monitoringpb.LabelValuesRequest{
		ConnectionId: connectionId,
		Label:        label,
	}

	rsp, err := monitoring_client.c.LabelValues(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", "", err
	}

	return rsp.GetLabelValues(), rsp.GetWarnings(), nil
}

// Query performs a query for the given time.
func (monitoring_client *Monitoring_Client) Query(connectionId string, query string, ts float64) (string, string, error) {
	rqst := &monitoringpb.QueryRequest{
		ConnectionId: connectionId,
		Query:        query,
		Ts:           ts,
	}

	rsp, err := monitoring_client.c.Query(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", "", err
	}

	return rsp.GetValue(), rsp.GetWarnings(), nil
}

// QueryRange performs a query for the given range.
func (monitoring_client *Monitoring_Client) QueryRange(connectionId string, query string, startTime float64, endTime float64, step float64) (string, string, error) {
	rqst := &monitoringpb.QueryRangeRequest{
		ConnectionId: connectionId,
		Query:        query,
		StartTime:    startTime,
		EndTime:      endTime,
		Step:         step,
	}

	var value string
	var warning string
	stream, err := monitoring_client.c.QueryRange(globular.GetClientContext(monitoring_client), rqst)
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
func (monitoring_client *Monitoring_Client) Series(connectionId string, matches []string, startTime float64, endTime float64) (string, string, error) {
	rqst := &monitoringpb.SeriesRequest{
		ConnectionId: connectionId,
		Matches:      matches,
		StartTime:    startTime,
		EndTime:      endTime,
	}

	rsp, err := monitoring_client.c.Series(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", "", err
	}

	return rsp.GetLabelSet(), rsp.GetWarnings(), nil
}

// Snapshot creates a snapshot of all current data into snapshots/<datetime>-<rand>
// under the TSDB's data directory and returns the directory as response.
func (monitoring_client *Monitoring_Client) Snapshot(connectionId string, skipHead bool) (string, error) {
	rqst := &monitoringpb.SnapshotRequest{
		ConnectionId: connectionId,
		SkipHead:     skipHead,
	}

	rsp, err := monitoring_client.c.Snapshot(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}

// Rules returns a list of alerting and recording rules that are currently loaded.
func (monitoring_client *Monitoring_Client) Rules(connectionId string) (string, error) {
	rqst := &monitoringpb.RulesRequest{
		ConnectionId: connectionId,
	}

	rsp, err := monitoring_client.c.Rules(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}

// Targets returns an overview of the current state of the Prometheus target discovery.
func (monitoring_client *Monitoring_Client) Targets(connectionId string) (string, error) {
	rqst := &monitoringpb.TargetsRequest{
		ConnectionId: connectionId,
	}

	rsp, err := monitoring_client.c.Targets(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}

// TargetsMetadata returns metadata about metrics currently scraped by the target.
func (monitoring_client *Monitoring_Client) TargetsMetadata(connectionId string, matchTarget string, metric string, limit string) (string, error) {
	rqst := &monitoringpb.TargetsMetadataRequest{
		ConnectionId: connectionId,
		MatchTarget:  matchTarget,
		Metric:       metric,
		Limit:        limit,
	}

	rsp, err := monitoring_client.c.TargetsMetadata(globular.GetClientContext(monitoring_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}
