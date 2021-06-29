package dns_client

import (
	"context"
	"strconv"

	"github.com/globulario/services/golang/dns/dnspb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type DNS_Client struct {
	cc *grpc.ClientConn
	c  dnspb.DnsServiceClient

	// The id of the service
	id string

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
func NewDnsService_Client(address string, id string) (*DNS_Client, error) {
	client := new(DNS_Client)
	err := globular.InitClient(client, address, id)

	if err != nil {

		return nil, err
	}

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = dnspb.NewDnsServiceClient(client.cc)
	return client, nil
}

func (dns_client *DNS_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(dns_client)
	}
	return globular.InvokeClientRequest(dns_client.c, ctx, method, rqst)
}

// Return the domain
func (dns_client *DNS_Client) GetDomain() string {
	return dns_client.domain
}

// Return the address
func (dns_client *DNS_Client) GetAddress() string {
	return dns_client.domain + ":" + strconv.Itoa(dns_client.port)
}

// Return the id of the service instance
func (dns_client *DNS_Client) GetId() string {
	return dns_client.id
}

// Return the name of the service
func (dns_client *DNS_Client) GetName() string {
	return dns_client.name
}

// must be close when no more needed.
func (dns_client *DNS_Client) Close() {
	dns_client.cc.Close()
}

// Set grpc_service port.
func (dns_client *DNS_Client) SetPort(port int) {
	dns_client.port = port
}

// Set the client instance id.
func (dns_client *DNS_Client) SetId(id string) {
	dns_client.id = id
}

// Set the client name.
func (dns_client *DNS_Client) SetName(name string) {
	dns_client.name = name
}

// Set the domain.
func (dns_client *DNS_Client) SetDomain(domain string) {
	dns_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (dns_client *DNS_Client) HasTLS() bool {
	return dns_client.hasTLS
}

// Get the TLS certificate file path
func (dns_client *DNS_Client) GetCertFile() string {
	return dns_client.certFile
}

// Get the TLS key file path
func (dns_client *DNS_Client) GetKeyFile() string {
	return dns_client.keyFile
}

// Get the TLS key file path
func (dns_client *DNS_Client) GetCaFile() string {
	return dns_client.caFile
}

// Set the client is a secure client.
func (dns_client *DNS_Client) SetTLS(hasTls bool) {
	dns_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (dns_client *DNS_Client) SetCertFile(certFile string) {
	dns_client.certFile = certFile
}

// Set TLS key file path
func (dns_client *DNS_Client) SetKeyFile(keyFile string) {
	dns_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (dns_client *DNS_Client) SetCaFile(caFile string) {
	dns_client.caFile = caFile
}

// The domain of the globule responsible to do resource validation.
// That domain will be use by the interceptor and access validation will
// be evaluated by the resource manager at the domain address.
func (dns_client *DNS_Client) getDomainContext(domain string) context.Context {
	// Here I will set the targeted domain as domain in the context.
	md := metadata.New(map[string]string{"domain": domain})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	return ctx
}

///////////////// API ////////////////////
// Stop the service.
func (dns_client *DNS_Client) StopService() {
	dns_client.c.Stop(globular.GetClientContext(dns_client), &dnspb.StopRequest{})
}

func (dns_client *DNS_Client) GetA(domain string) (string, error) {

	rqst := &dnspb.GetARequest{
		Domain: domain,
	}

	rsp, err := dns_client.c.GetA(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.A, nil
}

// Register a subdomain to a domain.
// ex: toto.globular.io is the subdomain to globular.io, so here
// domain will be globular.io and subdomain toto.globular.io. The validation will
// be done by globular.io and not the dns itdns_client.
func (dns_client *DNS_Client) SetA(token, domain, subdomain, ipv4 string, ttl uint32) (string, error) {

	rqst := &dnspb.SetARequest{
		Domain: subdomain,
		A:      ipv4,
		Ttl:    ttl,
	}
	ctx := globular.GetClientContext(dns_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := dns_client.c.SetA(ctx, rqst)
	if err != nil {
		return "", err
	}
	
	return rsp.Message, nil
}

func (dns_client *DNS_Client) RemoveA(domain string) error {
	
	rqst := &dnspb.RemoveARequest{
		Domain: domain,
	}

	_, err := dns_client.c.RemoveA(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetAAAA(domain string) (string, error) {

	rqst := &dnspb.GetAAAARequest{
		Domain: domain,
	}

	rsp, err := dns_client.c.GetAAAA(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return "", err
	}
	return rsp.Aaaa, nil
}

func (dns_client *DNS_Client) SetAAAA(domain string, ipv6 string, ttl uint32) (string, error) {

	rqst := &dnspb.SetAAAARequest{
		Domain: domain,
		Aaaa:   ipv6,
		Ttl:    ttl,
	}

	rsp, err := dns_client.c.SetAAAA(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return "", err
	}
	return rsp.Message, nil
}

func (dns_client *DNS_Client) RemoveAAAA(domain string) error {

	rqst := &dnspb.RemoveAAAARequest{
		Domain: domain,
	}

	_, err := dns_client.c.RemoveAAAA(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetText(domain string, id string) ([]string, error) {

	rqst := &dnspb.GetTextRequest{
		Id: id,
	}

	rsp, err := dns_client.c.GetText(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.GetValues(), nil
}

func (dns_client *DNS_Client) SetText(domain string, id string, values []string, ttl uint32) error {

	rqst := &dnspb.SetTextRequest{
		Id:     id,
		Values: values,
		Ttl:    ttl,
	}

	_, err := dns_client.c.SetText(globular.GetClientContext(dns_client), rqst)
	return err
}

func (dns_client *DNS_Client) RemoveText(domain string, id string) error {

	rqst := &dnspb.RemoveTextRequest{
		Id: id,
	}

	_, err := dns_client.c.RemoveText(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetNs(domain string, id string) (string, error) {

	rqst := &dnspb.GetNsRequest{
		Id: id,
	}

	rsp, err := dns_client.c.GetNs(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetNs(), nil
}

func (dns_client *DNS_Client) SetNs(domain string, id string, ns string, ttl uint32) error {

	rqst := &dnspb.SetNsRequest{
		Id:  id,
		Ns:  ns,
		Ttl: ttl,
	}

	_, err := dns_client.c.SetNs(globular.GetClientContext(dns_client), rqst)
	return err
}

func (dns_client *DNS_Client) RemoveNs(domain string, id string) error {

	rqst := &dnspb.RemoveNsRequest{
		Id: id,
	}

	_, err := dns_client.c.RemoveNs(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetCName(domain string, id string) (string, error) {

	rqst := &dnspb.GetCNameRequest{
		Id: id,
	}

	rsp, err := dns_client.c.GetCName(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetCname(), nil
}

func (dns_client *DNS_Client) SetCName(domain string, id string, cname string, ttl uint32) error {

	rqst := &dnspb.SetCNameRequest{
		Id:    id,
		Cname: cname,
		Ttl:   ttl,
	}

	_, err := dns_client.c.SetCName(globular.GetClientContext(dns_client), rqst)
	return err
}

func (dns_client *DNS_Client) RemoveCName(domain string, id string) error {

	rqst := &dnspb.RemoveCNameRequest{
		Id: id,
	}

	_, err := dns_client.c.RemoveCName(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetMx(domain string, id string) (map[string]interface{}, error) {

	rqst := &dnspb.GetMxRequest{
		Id: id,
	}

	rsp, err := dns_client.c.GetMx(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return nil, err
	}

	mx := make(map[string]interface{})
	mx["Preference"] = uint16(rsp.GetResult().Preference)
	mx["Mx"] = rsp.GetResult().Mx

	return mx, nil
}

func (dns_client *DNS_Client) SetMx(domain string, id string, preference uint16, mx string, ttl uint32) error {

	rqst := &dnspb.SetMxRequest{
		Id: id,
		Mx: &dnspb.MX{
			Preference: int32(preference),
			Mx:         mx,
		},
		Ttl: ttl,
	}

	_, err := dns_client.c.SetMx(globular.GetClientContext(dns_client), rqst)
	return err
}

func (dns_client *DNS_Client) RemoveMx(domain string, id string) error {

	rqst := &dnspb.RemoveMxRequest{
		Id: id,
	}

	_, err := dns_client.c.RemoveMx(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetSoa(domain string, id string) (map[string]interface{}, error) {

	rqst := &dnspb.GetSoaRequest{
		Id: id,
	}

	rsp, err := dns_client.c.GetSoa(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return nil, err
	}

	soa := make(map[string]interface{})
	soa["Ns"] = rsp.GetResult().Ns
	soa["Mbox"] = rsp.GetResult().Mbox
	soa["Serial"] = rsp.GetResult().Serial
	soa["Refresh"] = rsp.GetResult().Refresh
	soa["Retry"] = rsp.GetResult().Retry
	soa["Expire"] = rsp.GetResult().Expire
	soa["Minttl"] = rsp.GetResult().Minttl

	return soa, nil
}

func (dns_client *DNS_Client) SetSoa(domain string, id string, ns string, mbox string, serial uint32, refresh uint32, retry uint32, expire uint32, minttl uint32, ttl uint32) error {

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

	_, err := dns_client.c.SetSoa(globular.GetClientContext(dns_client), rqst)
	return err
}

func (dns_client *DNS_Client) RemoveSoa(domain string, id string) error {

	rqst := &dnspb.RemoveSoaRequest{
		Id: id,
	}

	_, err := dns_client.c.RemoveSoa(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetUri(domain string, id string) (map[string]interface{}, error) {

	rqst := &dnspb.GetUriRequest{
		Id: id,
	}

	rsp, err := dns_client.c.GetUri(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return nil, err
	}

	uri := make(map[string]interface{})
	uri["Priority"] = rsp.GetResult().Priority
	uri["Weight"] = rsp.GetResult().Weight
	uri["Target"] = rsp.GetResult().Target

	return uri, nil
}

func (dns_client *DNS_Client) SetUri(domain string, id string, priority uint32, weight uint32, target string, ttl uint32) error {

	rqst := &dnspb.SetUriRequest{
		Id: id,
		Uri: &dnspb.URI{
			Priority: priority,
			Weight:   weight,
			Target:   target,
		},
		Ttl: ttl,
	}

	_, err := dns_client.c.SetUri(globular.GetClientContext(dns_client), rqst)
	return err
}

func (dns_client *DNS_Client) RemoveUri(domain string, id string) error {

	rqst := &dnspb.RemoveUriRequest{
		Id: id,
	}

	_, err := dns_client.c.RemoveUri(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetCaa(domain string, id string) (map[string]interface{}, error) {

	rqst := &dnspb.GetCaaRequest{
		Id: id,
	}

	rsp, err := dns_client.c.GetCaa(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return nil, err
	}

	caa := make(map[string]interface{})
	caa["Flag"] = rsp.GetResult().Flag
	caa["Tag"] = rsp.GetResult().Tag
	caa["Value"] = rsp.GetResult().Value

	return caa, nil
}

func (dns_client *DNS_Client) SetCaa(domain string, id string, flag uint32, tag string, value string, ttl uint32) error {

	rqst := &dnspb.SetCaaRequest{
		Id: id,
		Caa: &dnspb.CAA{
			Flag:  flag,
			Tag:   tag,
			Value: value,
		},
		Ttl: ttl,
	}

	_, err := dns_client.c.SetCaa(globular.GetClientContext(dns_client), rqst)
	return err
}

func (dns_client *DNS_Client) RemoveCaa(domain string, id string) error {

	rqst := &dnspb.RemoveCaaRequest{
		Id: id,
	}

	_, err := dns_client.c.RemoveCaa(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}

func (dns_client *DNS_Client) GetAfsdb(domain string, id string) (map[string]interface{}, error) {

	rqst := &dnspb.GetAfsdbRequest{
		Id: id,
	}

	rsp, err := dns_client.c.GetAfsdb(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return nil, err
	}

	afsdb := make(map[string]interface{})
	afsdb["Subtype"] = rsp.GetResult().Subtype
	afsdb["Hostname"] = rsp.GetResult().Hostname

	return afsdb, nil
}

func (dns_client *DNS_Client) SetAfsdb(domain string, id string, subtype uint32, hostname string, ttl uint32) error {

	rqst := &dnspb.SetAfsdbRequest{
		Id: id,
		Afsdb: &dnspb.AFSDB{
			Subtype:  subtype,
			Hostname: hostname,
		},
		Ttl: ttl,
	}

	_, err := dns_client.c.SetAfsdb(globular.GetClientContext(dns_client), rqst)
	return err
}

func (dns_client *DNS_Client) RemoveAfsdb(domain string, id string) error {

	rqst := &dnspb.RemoveAfsdbRequest{
		Id: id,
	}

	_, err := dns_client.c.RemoveAfsdb(globular.GetClientContext(dns_client), rqst)
	if err != nil {
		return err
	}
	return nil
}
