package load_balancing_client

import (
	"strconv"

	"context"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/lb/lbpb"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// Ca Client Service
////////////////////////////////////////////////////////////////////////////////

type Lb_Client struct {
	cc *grpc.ClientConn
	c  lbpb.LoadBalancingServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The mac address of the server
	mac string

	// The port
	port int

	// The client domain
	domain string

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string

	// when the client need to close.
	close_channel chan bool

	// the channel to report load info.
	report_load_info_channel chan *lbpb.LoadInfo
}

// Create a connection to the service.
func NewLbService_Client(address string, id string) (*Lb_Client, error) {
	client := new(Lb_Client)

	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = lbpb.NewLoadBalancingServiceClient(client.cc)

	// Start processing load info.
	go func() {
		client.startReportLoadInfo()
	}()

	return client, nil
}

func (client *Lb_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(client)
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

// Return the address
func (client *Lb_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the domain
func (client *Lb_Client) GetDomain() string {
	return client.domain
}

// Return the id of the service instance
func (client *Lb_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Lb_Client) GetName() string {
	return client.name
}

func (client *Lb_Client) GetMac() string {
	return client.mac
}


// must be close when no more needed.
func (client *Lb_Client) Close() {
	// Close the load report loop.
	client.close_channel <- true
	client.cc.Close()
}

// Set grpc_service port.
func (client *Lb_Client) SetPort(port int) {
	client.port = port
}

// Set the client instance id.
func (client *Lb_Client) SetId(id string) {
	client.id = id
}

func (client *Lb_Client) SetName(name string) {
	client.name = name
}

// Set the client name.
func (client *Lb_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Lb_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Lb_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Lb_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Lb_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Lb_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Lb_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Lb_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Lb_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Lb_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////////////////////////////////////////////////////////////////////
// Load balancing functions.
////////////////////////////////////////////////////////////////////////////////

// Start reporting client load infos.
func (client *Lb_Client) startReportLoadInfo() error {
	client.close_channel = make(chan bool)
	client.report_load_info_channel = make(chan *lbpb.LoadInfo)

	// Open the stream...
	stream, err := client.c.ReportLoadInfo(globular.GetClientContext(client))
	if err != nil {
		return err
	}

	for {
		select {
		case <-client.close_channel:
			// exit.
			break

		case load_info := <-client.report_load_info_channel:
			rqst := &lbpb.ReportLoadInfoRequest{
				Info: load_info,
			}
			stream.Send(rqst)
		}

	}

	// Close the stream.
	_, err = stream.CloseAndRecv()

	return err

}

// Simply report the load info to the load balancer service.
func (client *Lb_Client) ReportLoadInfo(load_info *lbpb.LoadInfo) {
	if client.report_load_info_channel == nil {
		return // the service is not ready to get info.
	}
	client.report_load_info_channel <- load_info
}

// Get the list of candidate for a given services.
func (client *Lb_Client) GetCandidates(serviceName string) ([]*lbpb.ServerInfo, error) {
	rqst := &lbpb.GetCanditatesRequest{
		ServiceName: serviceName,
	}

	resp, err := client.c.GetCanditates(globular.GetClientContext(client), rqst)
	if err != nil {
		return nil, err
	}

	return resp.GetServers(), nil
}
