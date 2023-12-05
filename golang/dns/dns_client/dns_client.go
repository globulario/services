package dns_client

import (
	"context"
	"time"

	"github.com/globulario/services/golang/dns/dnspb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Dns_Client struct {
	cc *grpc.ClientConn
	c  dnspb.DnsServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The mac address of the server
	mac string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	//  keep the last connection state of the client.
	state string

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
func NewDnsService_Client(address string, id string) (*Dns_Client, error) {
	client := new(Dns_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		
		return nil, err
	}

	err = client.Reconnect()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func (client *Dns_Client) Reconnect() error {

	var err error
	nb_try_connect := 50

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = dnspb.NewDnsServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err
}

// The address where the client can connect.
func (client *Dns_Client) SetAddress(address string) {
	client.address = address
}

func (client *Dns_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Dns_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac(), "address": client.GetAddress()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (client *Dns_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Dns_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Dns_Client) GetId() string {
	return client.id
}

// Return the last know connection state
func (client *Dns_Client) GetState() string {
	return client.state
}

// Return the name of the service
func (client *Dns_Client) GetName() string {
	return client.name
}

func (client *Dns_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Dns_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Dns_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Dns_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Dns_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Dns_Client) SetName(name string) {
	client.name = name
}

func (client *Dns_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Dns_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Dns_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Dns_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Dns_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Dns_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Dns_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Dns_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Dns_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Dns_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Dns_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

// The domain of the globule responsible to do resource validation.
// That domain will be use by the interceptor and access validation will
// be evaluated by the resource manager at the domain address.
func (client *Dns_Client) getDomainContext(domain string) context.Context {
	// Here I will set the targeted domain as domain in the context.
	md := metadata.New(map[string]string{"domain": domain})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	return ctx
}

// /////////////// API ////////////////////
// Stop the service.
func (client *Dns_Client) StopService() {
	client.c.Stop(client.GetCtx(), &dnspb.StopRequest{})
}

func (client *Dns_Client) GetA(domain string) ([]string, error) {

	rqst := &dnspb.GetARequest{
		Domain: domain,
	}

	rsp, err := client.c.GetA(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.A, nil
}

// Register a subdomain to a domain.
// ex: toto.globular.io is the subdomain to globular.io, so here
// toto.globular.io. The validation will
func (client *Dns_Client) SetA(token, domain, ipv4 string, ttl uint32) (string, error) {

	rqst := &dnspb.SetARequest{
		Domain: domain,
		A:      ipv4,
		Ttl:    ttl,
	}
	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := client.c.SetA(ctx, rqst)
	if err != nil {
		return "", err
	}

	return rsp.Message, nil
}

func (client *Dns_Client) RemoveA(token, domain string) error {

	rqst := &dnspb.RemoveARequest{
		Domain: domain,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveA(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetAAAA(domain string) ([]string, error) {

	rqst := &dnspb.GetAAAARequest{
		Domain: domain,
	}

	rsp, err := client.c.GetAAAA(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Aaaa, nil
}

func (client *Dns_Client) SetAAAA(token, domain string, ipv6 string, ttl uint32) (string, error) {

	rqst := &dnspb.SetAAAARequest{
		Domain: domain,
		Aaaa:   ipv6,
		Ttl:    ttl,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := client.c.SetAAAA(ctx, rqst)
	if err != nil {
		return "", err
	}
	return rsp.Message, nil
}

func (client *Dns_Client) RemoveAAAA(token, domain string) error {

	rqst := &dnspb.RemoveAAAARequest{
		Domain: domain,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveAAAA(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetText(id string) ([]string, error) {

	rqst := &dnspb.GetTextRequest{
		Id: id,
	}

	rsp, err := client.c.GetText(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.GetValues(), nil
}

func (client *Dns_Client) SetText(token, id string, values []string, ttl uint32) error {

	rqst := &dnspb.SetTextRequest{
		Id:     id,
		Values: values,
		Ttl:    ttl,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.SetText(ctx, rqst)
	return err
}

func (client *Dns_Client) RemoveText(token, id string) error {

	rqst := &dnspb.RemoveTextRequest{
		Id: id,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveText(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetNs(id string) ([]string, error) {

	rqst := &dnspb.GetNsRequest{
		Id: id,
	}

	rsp, err := client.c.GetNs(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.GetNs(), nil
}

func (client *Dns_Client) SetNs(token, id string, ns string, ttl uint32) error {

	rqst := &dnspb.SetNsRequest{
		Id:  id,
		Ns:  ns,
		Ttl: ttl,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.SetNs(ctx, rqst)
	return err
}

func (client *Dns_Client) RemoveNs(token, id string) error {

	rqst := &dnspb.RemoveNsRequest{
		Id: id,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveNs(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetCName(id string) (string, error) {

	rqst := &dnspb.GetCNameRequest{
		Id: id,
	}

	rsp, err := client.c.GetCName(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetCname(), nil
}

func (client *Dns_Client) SetCName(token, id string, cname string, ttl uint32) error {

	rqst := &dnspb.SetCNameRequest{
		Id:    id,
		Cname: cname,
		Ttl:   ttl,
	}
	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	_, err := client.c.SetCName(ctx, rqst)
	return err
}

func (client *Dns_Client) RemoveCName(token, id string) error {

	rqst := &dnspb.RemoveCNameRequest{
		Id: id,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveCName(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetMx(token, id string) ([]*dnspb.MX, error) {

	rqst := &dnspb.GetMxRequest{
		Id: id,
	}

	rsp, err := client.c.GetMx(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Result, nil
}

func (client *Dns_Client) SetMx(token, id string, preference uint16, mx string, ttl uint32) error {

	rqst := &dnspb.SetMxRequest{
		Id: id,
		Mx: &dnspb.MX{
			Preference: int32(preference),
			Mx:         mx,
		},
		Ttl: ttl,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.SetMx(ctx, rqst)
	return err
}

func (client *Dns_Client) RemoveMx(token, id string) error {

	rqst := &dnspb.RemoveMxRequest{
		Id: id,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveMx(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetSoa(id string) ([]*dnspb.SOA, error) {

	rqst := &dnspb.GetSoaRequest{
		Id: id,
	}

	rsp, err := client.c.GetSoa(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Result, nil
}

func (client *Dns_Client) SetSoa(token, id, ns, mbox string, serial, refresh, retry, expire, minttl, ttl uint32) error {

	rqst := &dnspb.SetSoaRequest{
		Id: id,
		Soa: &dnspb.SOA{
			Ns:      ns,
			Mbox:    mbox,
			Serial:  serial,
			Refresh: refresh,
			Retry:   retry,
			Expire:  expire,
			Minttl:  minttl,
		},
		Ttl: ttl,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.SetSoa(ctx, rqst)
	return err
}

func (client *Dns_Client) RemoveSoa(token, id string) error {

	rqst := &dnspb.RemoveSoaRequest{
		Id: id,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveSoa(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetUri(id string) ([]*dnspb.URI, error) {

	rqst := &dnspb.GetUriRequest{
		Id: id,
	}

	rsp, err := client.c.GetUri(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Result, nil
}

func (client *Dns_Client) SetUri(token, id string, priority, weight uint32, target string, ttl uint32) error {

	rqst := &dnspb.SetUriRequest{
		Id: id,
		Uri: &dnspb.URI{
			Priority: priority,
			Weight:   weight,
			Target:   target,
		},
		Ttl: ttl,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.SetUri(ctx, rqst)
	return err
}

func (client *Dns_Client) RemoveUri(token, id string) error {

	rqst := &dnspb.RemoveUriRequest{
		Id: id,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveUri(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetCaa(id string) ([]*dnspb.CAA, error) {

	rqst := &dnspb.GetCaaRequest{
		Id: id,
	}

	rsp, err := client.c.GetCaa(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Result, nil
}

func (client *Dns_Client) SetCaa(token, id string, flag uint32, tag string, domain string, ttl uint32) error {

	rqst := &dnspb.SetCaaRequest{
		Id: id,
		Caa: &dnspb.CAA{
			Flag:   flag,
			Tag:    tag,
			Domain: domain,
		},
		Ttl: ttl,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.SetCaa(ctx, rqst)
	return err
}

func (client *Dns_Client) RemoveCaa(token, id string) error {

	rqst := &dnspb.RemoveCaaRequest{
		Id: id,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveCaa(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}

func (client *Dns_Client) GetAfsdb(id string) (map[string]interface{}, error) {

	rqst := &dnspb.GetAfsdbRequest{
		Id: id,
	}

	rsp, err := client.c.GetAfsdb(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	afsdb := make(map[string]interface{})
	afsdb["Subtype"] = rsp.GetResult().Subtype
	afsdb["Hostname"] = rsp.GetResult().Hostname

	return afsdb, nil
}

func (client *Dns_Client) SetAfsdb(token, id string, subtype uint32, hostname string, ttl uint32) error {

	rqst := &dnspb.SetAfsdbRequest{
		Id: id,
		Afsdb: &dnspb.AFSDB{
			Subtype:  subtype,
			Hostname: hostname,
		},
		Ttl: ttl,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	_, err := client.c.SetAfsdb(ctx, rqst)
	return err
}

func (client *Dns_Client) RemoveAfsdb(token, id string) error {

	rqst := &dnspb.RemoveAfsdbRequest{
		Id: id,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemoveAfsdb(ctx, rqst)
	if err != nil {
		return err
	}
	return nil
}
