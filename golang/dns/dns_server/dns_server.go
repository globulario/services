package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/dns/dns_client"
	"github.com/globulario/services/golang/dns/dnspb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	//"google.golang.org/grpc/codes"
	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"

	//"google.golang.org/grpc/status"
	"encoding/binary"

	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/miekg/dns"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10033
	defaultProxy = 10034

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// Thr IPV4 address
	domain string = "localhost"

	// pointer to the sever implementation.
	s *server
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
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

	// server-signed X.509 public keys for distribution
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

	// Contain the configuration of the storage service use to store
	// the actual values.
	DnsPort         int      // the dns port
	Domains         []string // The list of domains managed by this dns.
	StorageDataPath string

	store *storage_store.LevelDB_store

	// The dns records... https://en.wikipedia.org/wiki/List_of_DNS_record_types
	// see the wikipedia page to know exactly what are the values that can
	// be use here.
	Records map[string][]interface{}

	connection_is_open bool
}

// Globular services implementation...
// The id of a particular service instance.
func (server *server) GetId() string {
	return server.Id
}
func (server *server) SetId(id string) {
	server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (server *server) GetName() string {
	return server.Name
}
func (server *server) SetName(name string) {
	server.Name = name
}

// The description of the service
func (server *server) GetDescription() string {
	return server.Description
}
func (server *server) SetDescription(description string) {
	server.Description = description
}

// The list of keywords of the services.
func (server *server) GetKeywords() []string {
	return server.Keywords
}
func (server *server) SetKeywords(keywords []string) {
	server.Keywords = keywords
}

// Dist
func (server *server) Dist(path string) (string, error) {

	return globular.Dist(path, server)
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

func (server *server) GetPlatform() string {
	return globular.GetPlatform()
}

func (server *server) GetRepositories() []string {
	return server.Repositories
}
func (server *server) SetRepositories(repositories []string) {
	server.Repositories = repositories
}

func (server *server) GetDiscoveries() []string {
	return server.Discoveries
}
func (server *server) SetDiscoveries(discoveries []string) {
	server.Discoveries = discoveries
}

// The path of the executable.
func (server *server) GetPath() string {
	return server.Path
}
func (server *server) SetPath(path string) {
	server.Path = path
}

// The path of the .proto file.
func (server *server) GetProto() string {
	return server.Proto
}
func (server *server) SetProto(proto string) {
	server.Proto = proto
}

// The gRpc port.
func (server *server) GetPort() int {
	return server.Port
}
func (server *server) SetPort(port int) {
	server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (server *server) GetProxy() int {
	return server.Proxy
}
func (server *server) SetProxy(proxy int) {
	server.Proxy = proxy
}

// Can be one of http/https/tls
func (server *server) GetProtocol() string {
	return server.Protocol
}
func (server *server) SetProtocol(protocol string) {
	server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (server *server) GetAllowAllOrigins() bool {
	return server.AllowAllOrigins
}
func (server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (server *server) GetAllowedOrigins() string {
	return server.AllowedOrigins
}

func (server *server) SetAllowedOrigins(allowedOrigins string) {
	server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (server *server) GetDomain() string {
	return server.Domain
}
func (server *server) SetDomain(domain string) {
	server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (server *server) GetTls() bool {
	return server.TLS
}
func (server *server) SetTls(hasTls bool) {
	server.TLS = hasTls
}

// The certificate authority file
func (server *server) GetCertAuthorityTrust() string {
	return server.CertAuthorityTrust
}
func (server *server) SetCertAuthorityTrust(ca string) {
	server.CertAuthorityTrust = ca
}

// The certificate file.
func (server *server) GetCertFile() string {
	return server.CertFile
}
func (server *server) SetCertFile(certFile string) {
	server.CertFile = certFile
}

// The key file.
func (server *server) GetKeyFile() string {
	return server.KeyFile
}
func (server *server) SetKeyFile(keyFile string) {
	server.KeyFile = keyFile
}

// The service version
func (server *server) GetVersion() string {
	return server.Version
}
func (server *server) SetVersion(version string) {
	server.Version = version
}

// The publisher id.
func (server *server) GetPublisherId() string {
	return server.PublisherId
}
func (server *server) SetPublisherId(publisherId string) {
	server.PublisherId = publisherId
}

func (server *server) GetKeepUpToDate() bool {
	return server.KeepUpToDate
}
func (server *server) SetKeepUptoDate(val bool) {
	server.KeepUpToDate = val
}

func (server *server) GetKeepAlive() bool {
	return server.KeepAlive
}
func (server *server) SetKeepAlive(val bool) {
	server.KeepAlive = val
}

func (server *server) GetPermissions() []interface{} {
	return server.Permissions
}
func (server *server) SetPermissions(permissions []interface{}) {
	server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (server *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewDnsService_Client", dns_client.NewDnsService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	server.grpcServer, err = globular.InitGrpcServer(server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	if len(server.StorageDataPath) == 0 {
		fmt.Println("The value StorageDataPath in the configuration must be given. You can use /tmp (on linux) if you don't want to keep values indefilnely on the storage server.")
	}

	s = server

	return nil

}

// Save the configuration values.
func (server *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", server)
}

func (server *server) StartService() error {
	return globular.StartService(server, server.grpcServer)
}

func (server *server) StopService() error {
	return globular.StopService(server, server.grpcServer)
}

func (server *server) Stop(context.Context, *dnspb.StopRequest) (*dnspb.StopResponse, error) {
	return &dnspb.StopResponse{}, server.StopService()
}

//////////////////////////////// DNS Service specific //////////////////////////

// Open the connection if it's close.
func (server *server) openConnection() error {
	if server.connection_is_open {
		return nil
	}

	// Open store.
	server.store = storage_store.NewLevelDB_store()
	err := server.store.Open(`{"path":"` + server.StorageDataPath + `", "name":"dns_data_store"}`)
	if err != nil {
		return err
	}

	server.connection_is_open = true

	// Init the records with that connection.
	server.initRecords()

	return nil
}

func (server *server) isManaged(domain string) bool {
	for i := 0; i < len(server.Domains); i++ {
		if strings.HasSuffix(domain, server.Domains[i]) {
			return true
		}
	}
	return false
}

// Set a dns entry.
func (server *server) SetA(ctx context.Context, rqst *dnspb.SetARequest) (*dnspb.SetAResponse, error) {

	server.logServiceInfo("SetA", Utility.FileLine(), Utility.FunctionName(), "Try set dns entry "+rqst.Domain)

	if !server.isManaged(rqst.Domain) {
		err := errors.New("The domain " + rqst.Domain + " is not manage by this dns.")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("A:" + domain)
	err = server.store.SetItem(uuid, []byte(rqst.A))
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.logServiceInfo("SetA", Utility.FileLine(), Utility.FunctionName(), "domain: A:"+domain+" with uuid"+uuid+"is set with ipv4 address"+rqst.A)
	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetAResponse{
		Message: domain, // return the full domain.
	}, nil
}

func (server *server) RemoveA(ctx context.Context, rqst *dnspb.RemoveARequest) (*dnspb.RemoveAResponse, error) {
	fmt.Println("Try remove dns entry ", rqst.Domain)
	server.logServiceInfo("SetA", Utility.FileLine(), Utility.FunctionName(), "Try remove dns entry "+rqst.Domain)

	if !server.isManaged(rqst.Domain) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("The domain "+rqst.Domain+" is not manage by this dns.")))
	}

	domain := strings.ToLower(rqst.Domain)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("A:" + domain)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveAResponse{
		Result: true, // return the full domain.
	}, nil
}

func (server *server) get_ipv4(domain string) (string, uint32, error) {
	domain = strings.ToLower(domain)
	if strings.HasSuffix(domain, ".") {
		domain = domain[:len(domain)-1]
	}
	err := server.openConnection()
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("A:" + domain)
	ipv4, err := server.store.GetItem(uuid)
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return string(ipv4), server.getTtl(uuid), nil
}

func (server *server) GetA(ctx context.Context, rqst *dnspb.GetARequest) (*dnspb.GetAResponse, error) {
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	domain := strings.ToLower(rqst.Domain)
	uuid := Utility.GenerateUUID("A:" + domain)
	server.logServiceInfo("SetA", Utility.FileLine(), Utility.FunctionName(), "Try to get ipv4 address for "+rqst.Domain)

	ipv4, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.logServiceInfo("SetA", Utility.FileLine(), Utility.FunctionName(), "ipv4 for "+domain+" is "+string(ipv4))

	return &dnspb.GetAResponse{
		A: string(ipv4), // return the full domain.
	}, nil
}

// Set a dns entry.
func (server *server) SetAAAA(ctx context.Context, rqst *dnspb.SetAAAARequest) (*dnspb.SetAAAAResponse, error) {

	server.logServiceInfo("SetAAAA", Utility.FileLine(), Utility.FunctionName(), "Try set dns entry "+rqst.Domain)

	if !server.isManaged(rqst.Domain) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("The domain "+rqst.Domain+" is not manage by this dns.")))
	}

	domain := strings.ToLower(rqst.Domain)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)

	err = server.store.SetItem(uuid, []byte(rqst.Aaaa))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetAAAAResponse{
		Message: domain, // return the full domain.
	}, nil
}

func (server *server) RemoveAAAA(ctx context.Context, rqst *dnspb.RemoveAAAARequest) (*dnspb.RemoveAAAAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	fmt.Println("Try remove dns entry ", domain)
	if !server.isManaged(rqst.Domain) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("The domain "+rqst.Domain+" is not manage by this dns.")))
	}

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("AAAA:" + domain)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveAAAAResponse{
		Result: true, // return the full domain.
	}, nil
}

func (server *server) get_ipv6(domain string) (string, uint32, error) {
	domain = strings.ToLower(domain)
	if strings.HasSuffix(domain, ".") {
		domain = domain[:len(domain)-1]
	}
	fmt.Println("Try get dns entry ", domain)
	err := server.openConnection()
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)
	address, err := server.store.GetItem(uuid)
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return string(address), server.getTtl(uuid), nil
}

func (server *server) GetAAAA(ctx context.Context, rqst *dnspb.GetAAAARequest) (*dnspb.GetAAAAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	fmt.Println("Try get dns entry ", domain)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("AAAA:" + domain)
	ipv6, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	fmt.Println("ipv6 for", domain, "is", string(ipv6))
	return &dnspb.GetAAAAResponse{
		Aaaa: string(ipv6), // return the full domain.
	}, nil
}

// Set a text entry.
func (server *server) SetText(ctx context.Context, rqst *dnspb.SetTextRequest) (*dnspb.SetTextResponse, error) {
	fmt.Println("Try set dns text ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := json.Marshal(rqst.Values)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("TXT:" + rqst.Id)
	err = server.store.SetItem(uuid, values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetTextResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (server *server) getText(id string) ([]string, uint32, error) {
	fmt.Println("Try get dns text ", id)
	err := server.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("TXT:" + id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, 0, err
	}
	values := make([]string, 0)

	err = json.Unmarshal(data, &values)
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return values, server.getTtl(uuid), nil
}

// Retreive a text value
func (server *server) GetText(ctx context.Context, rqst *dnspb.GetTextRequest) (*dnspb.GetTextResponse, error) {
	fmt.Println("Try get dns text ", domain)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("TXT:" + rqst.Id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values := make([]string, 0)
	err = json.Unmarshal(data, &values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetTextResponse{
		Values: values, // return the full domain.
	}, nil
}

// Remove a text entry
func (server *server) RemoveText(ctx context.Context, rqst *dnspb.RemoveTextRequest) (*dnspb.RemoveTextResponse, error) {
	fmt.Println("Try remove dns text ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("TXT:" + rqst.Id)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveTextResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a NS entry.
func (server *server) SetNs(ctx context.Context, rqst *dnspb.SetNsRequest) (*dnspb.SetNsResponse, error) {
	fmt.Println("Try set dns ns ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("NS:" + rqst.Id)
	err = server.store.SetItem(uuid, []byte(rqst.Ns))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetNsResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (server *server) getNs(id string) (string, uint32, error) {
	fmt.Println("Try get dns ns ", id)
	err := server.openConnection()
	if err != nil {
		return "", 0, err
	}
	uuid := Utility.GenerateUUID("NS:" + id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return string(data), server.getTtl(uuid), err
}

// Retreive a text value
func (server *server) GetNs(ctx context.Context, rqst *dnspb.GetNsRequest) (*dnspb.GetNsResponse, error) {

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("NS:" + rqst.Id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetNsResponse{
		Ns: string(data), // return the full domain.
	}, nil
}

// Remove a text entry
func (server *server) RemoveNs(ctx context.Context, rqst *dnspb.RemoveNsRequest) (*dnspb.RemoveNsResponse, error) {
	fmt.Println("Try remove dns text ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("NS:" + rqst.Id)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveNsResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a CName entry.
func (server *server) SetCName(ctx context.Context, rqst *dnspb.SetCNameRequest) (*dnspb.SetCNameResponse, error) {
	fmt.Println("Try set dns CName ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("CName:" + rqst.Id)
	err = server.store.SetItem(uuid, []byte(rqst.Cname))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetCNameResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the CName.
func (server *server) getCName(id string) (string, uint32, error) {
	fmt.Println("Try get CName ", id)
	err := server.openConnection()
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("CName:" + id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return string(data), server.getTtl(uuid), err
}

// Retreive a CName value
func (server *server) GetCName(ctx context.Context, rqst *dnspb.GetCNameRequest) (*dnspb.GetCNameResponse, error) {

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("CName:" + rqst.Id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetCNameResponse{
		Cname: string(data), // return the full domain.
	}, nil
}

// Remove a text entry
func (server *server) RemoveCName(ctx context.Context, rqst *dnspb.RemoveCNameRequest) (*dnspb.RemoveCNameResponse, error) {
	fmt.Println("Try remove dns CName ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("CName:" + rqst.Id)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveCNameResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a text entry.
func (server *server) SetMx(ctx context.Context, rqst *dnspb.SetMxRequest) (*dnspb.SetMxResponse, error) {
	fmt.Println("Try set dns mx ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := json.Marshal(rqst.Mx)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("MX:" + rqst.Id)
	err = server.store.SetItem(uuid, values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetMxResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (server *server) getMx(id string) (map[string]interface{}, uint32, error) {
	fmt.Println("Try get dns text ", id)
	err := server.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("MX:" + id)
	data, err := server.store.GetItem(uuid)

	values := make(map[string]interface{}) // use a map instead of Mx struct.
	err = json.Unmarshal(data, &values)
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return values, server.getTtl(uuid), nil
}

// Retreive a text value
func (server *server) GetMx(ctx context.Context, rqst *dnspb.GetMxRequest) (*dnspb.GetMxResponse, error) {
	fmt.Println("Try get dns mx ", domain)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("MX:" + rqst.Id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values := make(map[string]interface{})
	err = json.Unmarshal(data, &values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetMxResponse{
		Result: &dnspb.MX{
			Preference: values["Preference"].(int32),
			Mx:         values["Mx"].(string),
		}, // return the full domain.
	}, nil
}

// Remove a text entry
func (server *server) RemoveMx(ctx context.Context, rqst *dnspb.RemoveMxRequest) (*dnspb.RemoveMxResponse, error) {
	fmt.Println("Try remove dns text ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("MX:" + rqst.Id)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveMxResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a text entry.
func (server *server) SetSoa(ctx context.Context, rqst *dnspb.SetSoaRequest) (*dnspb.SetSoaResponse, error) {
	fmt.Println("Try set dns Soa ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := json.Marshal(rqst.Soa)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("SOA:" + rqst.Id)
	err = server.store.SetItem(uuid, values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetSoaResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (server *server) getSoa(id string) (*dnspb.SOA, uint32, error) {
	fmt.Println("Try get dns soa ", id)
	err := server.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("SOA:" + id)
	data, err := server.store.GetItem(uuid)

	soa := new(dnspb.SOA) // use a map instead of Mx struct.
	err = json.Unmarshal(data, soa)
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return soa, server.getTtl(uuid), nil
}

// Retreive a text value
func (server *server) GetSoa(ctx context.Context, rqst *dnspb.GetSoaRequest) (*dnspb.GetSoaResponse, error) {
	fmt.Println("Try get dns soa ", domain)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("SOA:" + rqst.Id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	soa := new(dnspb.SOA)
	err = json.Unmarshal(data, soa)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetSoaResponse{
		Result: soa, // return the full domain.
	}, nil
}

// Remove a text entry
func (server *server) RemoveSoa(ctx context.Context, rqst *dnspb.RemoveSoaRequest) (*dnspb.RemoveSoaResponse, error) {
	fmt.Println("Try remove dns text ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("SOA:" + rqst.Id)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveSoaResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a text entry.
func (server *server) SetUri(ctx context.Context, rqst *dnspb.SetUriRequest) (*dnspb.SetUriResponse, error) {
	fmt.Println("Try set dns uri ", rqst.Id)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := json.Marshal(rqst.Uri)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("URI:" + rqst.Id)
	err = server.store.SetItem(uuid, values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetUriResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (server *server) getUri(id string) (*dnspb.URI, uint32, error) {
	fmt.Println("Try get dns uri ", id)
	err := server.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("URI:" + id)
	data, err := server.store.GetItem(uuid)

	uri := new(dnspb.URI) // use a map instead of Mx struct.
	err = json.Unmarshal(data, uri)
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return uri, server.getTtl(uuid), nil
}

// Retreive a text value
func (server *server) GetUri(ctx context.Context, rqst *dnspb.GetUriRequest) (*dnspb.GetUriResponse, error) {
	fmt.Println("Try get dns uri ", domain)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("URI:" + rqst.Id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uri := new(dnspb.URI)
	err = json.Unmarshal(data, uri)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetUriResponse{
		Result: uri, // return the full domain.
	}, nil
}

// Remove AFSDB
func (server *server) RemoveUri(ctx context.Context, rqst *dnspb.RemoveUriRequest) (*dnspb.RemoveUriResponse, error) {
	fmt.Println("Try remove dns uri ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("URI:" + rqst.Id)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveUriResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a AFSDB entry.
func (server *server) SetAfsdb(ctx context.Context, rqst *dnspb.SetAfsdbRequest) (*dnspb.SetAfsdbResponse, error) {
	fmt.Println("Try set dns afsdb ", rqst.Id)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := json.Marshal(rqst.Afsdb)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("AFSDB:" + rqst.Id)
	err = server.store.SetItem(uuid, values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetAfsdbResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the AFSDB.
func (server *server) getAfsdb(id string) (*dnspb.AFSDB, uint32, error) {
	fmt.Println("Try get dns AFSDB ", id)
	err := server.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	data, err := server.store.GetItem(uuid)

	afsdb := new(dnspb.AFSDB) // use a map instead of Mx struct.
	err = json.Unmarshal(data, afsdb)
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return afsdb, server.getTtl(uuid), nil
}

// Retreive a AFSDB value
func (server *server) GetAfsdb(ctx context.Context, rqst *dnspb.GetAfsdbRequest) (*dnspb.GetAfsdbResponse, error) {
	fmt.Println("Try get dns AFSDB ", domain)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("AFSDB:" + rqst.Id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	afsdb := new(dnspb.AFSDB)
	err = json.Unmarshal(data, afsdb)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetAfsdbResponse{
		Result: afsdb, // return the full domain.
	}, nil
}

// Remove AFSDB
func (server *server) RemoveAfsdb(ctx context.Context, rqst *dnspb.RemoveAfsdbRequest) (*dnspb.RemoveAfsdbResponse, error) {
	fmt.Println("Try remove dns Afsdb ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("AFSDB:" + rqst.Id)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveAfsdbResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a CAA entry.
func (server *server) SetCaa(ctx context.Context, rqst *dnspb.SetCaaRequest) (*dnspb.SetCaaResponse, error) {
	fmt.Println("Try set dns caa ", rqst.Id)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := json.Marshal(rqst.Caa)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("CAA:" + rqst.Id)
	err = server.store.SetItem(uuid, values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	server.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetCaaResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the CAA.
func (server *server) getCaa(id string) (*dnspb.CAA, uint32, error) {
	fmt.Println("Try get dns CAA ", id)
	err := server.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	uuid := Utility.GenerateUUID("CAA:" + id)
	data, err := server.store.GetItem(uuid)

	caa := new(dnspb.CAA) // use a map instead of Mx struct.
	err = json.Unmarshal(data, caa)
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return caa, server.getTtl(uuid), nil
}

// Retreive a AFSDB value
func (server *server) GetCaa(ctx context.Context, rqst *dnspb.GetCaaRequest) (*dnspb.GetCaaResponse, error) {
	fmt.Println("Try get dns CAA ", domain)
	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("CAA:" + rqst.Id)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	caa := new(dnspb.CAA)
	err = json.Unmarshal(data, caa)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetCaaResponse{
		Result: caa, // return the full domain.
	}, nil
}

// Remove CAA
func (server *server) RemoveCaa(ctx context.Context, rqst *dnspb.RemoveCaaRequest) (*dnspb.RemoveCaaResponse, error) {
	fmt.Println("Try remove dns Afsdb ", rqst.Id)

	err := server.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	uuid := Utility.GenerateUUID("CAA:" + rqst.Id)
	err = server.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.RemoveCaaResponse{
		Result: true, // return the full domain.
	}, nil
}

/////////////////////// DNS Specific service //////////////////////
type handler struct{}

func (hd *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := dns.Msg{}
	msg.SetReply(r)
	fmt.Println("-----> dns resquest receive... ", msg)

	switch r.Question[0].Qtype {
	case dns.TypeA:
		domain := msg.Question[0].Name
		msg.Authoritative = true
		address, ttl, err := s.get_ipv4(domain) // get the address name from the
		fmt.Println("---> look for A ", domain)
		if err == nil {
			fmt.Println("---> ask for domain: ", domain, " address to redirect is ", address)
			msg.Answer = append(msg.Answer, &dns.A{
				Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
				A:   net.ParseIP(address),
			})
		} else {
			fmt.Println("fail to retreive ipv4 address for "+domain, err)
		}

	case dns.TypeAAAA:
		msg.Authoritative = true
		domain := msg.Question[0].Name
		address, ttl, err := s.get_ipv6(domain) // get the address name from the
		fmt.Println("---> look for AAAA ", domain)
		if err == nil {
			fmt.Println("---> ask for domain: ", domain, " address to redirect is ", address)
			msg.Answer = append(msg.Answer, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
				AAAA: net.ParseIP(address),
			})
		} else {
			fmt.Println(err)
		}

	case dns.TypeAFSDB:

		msg.Authoritative = true
		id := msg.Question[0].Name
		afsdb, ttl, err := s.getAfsdb(id)
		if err == nil {
			msg.Answer = append(msg.Answer, &dns.AFSDB{
				Hdr:      dns.RR_Header{Name: domain, Rrtype: dns.TypeAFSDB, Class: dns.ClassINET, Ttl: ttl},
				Subtype:  uint16(afsdb.Subtype), //
				Hostname: afsdb.Hostname,
			})
		} else {
			fmt.Println(err)
		}

	case dns.TypeCAA:
		msg.Authoritative = true
		name := msg.Question[0].Name
		fmt.Println("---> look for CAA ", name)
		caa, ttl, err := s.getCaa(name)

		if err == nil {
			msg.Answer = append(msg.Answer, &dns.CAA{
				Hdr:   dns.RR_Header{Name: name, Rrtype: dns.TypeCAA, Class: dns.ClassINET, Ttl: ttl},
				Flag:  uint8(caa.Flag),
				Tag:   caa.Tag,
				Value: caa.Value,
			})
		} else {
			fmt.Println(err)
		}

	case dns.TypeCNAME:
		id := msg.Question[0].Name
		cname, ttl, err := s.getCName(id)
		if err == nil {
			// in case of empty string I will return the certificate validation key.
			msg.Answer = append(msg.Answer, &dns.CNAME{
				// keep text values.
				Hdr:    dns.RR_Header{Name: id, Rrtype: dns.TypeCNAME, Class: dns.ClassINET, Ttl: ttl},
				Target: cname,
			})
		}

	case dns.TypeTXT:
		id := msg.Question[0].Name
		fmt.Println("---> look for txt ", id)
		values, ttl, err := s.getText(id)
		if err == nil {
			// in case of empty string I will return the certificate validation key.
			msg.Answer = append(msg.Answer, &dns.TXT{
				// keep text values.
				Hdr: dns.RR_Header{Name: id, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
				Txt: values,
			})
		} else {
			fmt.Println("fail to retreive txt!", err)
		}

	case dns.TypeNS:
		id := msg.Question[0].Name
		ns, ttl, err := s.getNs(id)
		if err == nil {
			// in case of empty string I will return the certificate validation key.
			msg.Answer = append(msg.Answer, &dns.NS{
				// keep text values.
				Hdr: dns.RR_Header{Name: id, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
				Ns:  ns,
			})
		}

	case dns.TypeMX:
		id := msg.Question[0].Name // the id where the values is store.
		mx, ttl, err := s.getMx(id)

		if err == nil {
			// in case of empty string I will return the certificate validation key.
			msg.Answer = append(msg.Answer, &dns.MX{
				// keep text values.
				Hdr:        dns.RR_Header{Name: id, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: ttl},
				Preference: uint16(mx["Preference"].(int32)),
				Mx:         mx["Mx"].(string),
			})
		}

	case dns.TypeSOA:
		id := msg.Question[0].Name
		fmt.Println("---> look for soa ", id)
		soa, ttl, err := s.getSoa(id)
		if err == nil {
			// in case of empty string I will return the certificate validation key.
			msg.Answer = append(msg.Answer, &dns.SOA{
				// keep text values.
				Hdr:     dns.RR_Header{Name: id, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: ttl},
				Ns:      soa.Ns,
				Mbox:    soa.Mbox,
				Serial:  soa.Serial,
				Refresh: soa.Refresh,
				Retry:   soa.Retry,
				Expire:  soa.Expire,
				Minttl:  soa.Minttl,
			})
		}

	case dns.TypeURI:
		id := msg.Question[0].Name
		fmt.Println("---> look for uri ", id)
		uri, ttl, err := s.getUri(id)
		if err == nil {
			// in case of empty string I will return the certificate validation key.
			msg.Answer = append(msg.Answer, &dns.URI{
				// keep text values.
				Hdr:      dns.RR_Header{Name: id, Rrtype: dns.TypeURI, Class: dns.ClassINET, Ttl: ttl},
				Priority: uint16(uri.Priority),
				Weight:   uint16(uri.Weight),
				Target:   uri.Target,
			})
		}
	}
	w.WriteMsg(&msg)
}

func ServeDns(port int) error {
	// Now I will start the dns server.
	srv := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}
	srv.Handler = &handler{}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Println("Failed to set udp listener", err.Error())
		return err
	}

	return nil
}

func (server *server) initStringRecords(recordType string, ttl uint32, record map[string]interface{}) error {
	uuid := Utility.GenerateUUID(recordType + ":" + record["Id"].(string))
	err := server.setTtl(uuid, ttl)
	if err != nil {
		return err
	}
	return server.store.SetItem(uuid, []byte(record["Value"].(string)))
}

func (server *server) initSructRecords(recordType string, ttl uint32, record map[string]interface{}) error {

	data, err := json.Marshal(record["Value"].(map[string]interface{}))
	if err != nil {
		return err
	}
	uuid := Utility.GenerateUUID(recordType + ":" + record["Id"].(string))
	err = server.store.SetItem(uuid, data)
	if err != nil {
		return err
	}

	return server.setTtl(uuid, ttl)
}

func (server *server) initArrayRecords(recordType string, ttl uint32, record map[string]interface{}) error {

	data, err := json.Marshal(record["Value"].([]interface{}))
	if err != nil {
		return err
	}

	uuid := Utility.GenerateUUID(recordType + ":" + record["Id"].(string))

	err = server.store.SetItem(uuid, data)
	if err != nil {
		return err
	}

	return server.setTtl(uuid, ttl)
}

func (server *server) setTtl(uuid string, ttl uint32) error {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, ttl)
	uuid = Utility.GenerateUUID("TTL:" + uuid)
	err := server.store.SetItem(uuid, data)
	return err
}

func (server *server) getTtl(uuid string) uint32 {
	uuid = Utility.GenerateUUID("TTL:" + uuid)
	data, err := server.store.GetItem(uuid)
	if err != nil {
		return 60 // the default value
	}
	return binary.LittleEndian.Uint32(data)
}

// Initialyse all the records from the configuration.
func (server *server) initRecords() error {
	if server.Records == nil {
		return nil
	}

	for name, records := range server.Records {
		for i := 0; i < len(records); i++ {
			var record = records[i].(map[string]interface{})
			var ttl uint32
			if record["ttl"] != nil {
				ttl = uint32(record["ttl"].(float64))
			} else {
				ttl = 60 // default value of time to live.
			}
			var err error
			if name == "A" {
				err = server.initStringRecords("A", ttl, record)
			} else if name == "AAAA" {
				err = server.initSructRecords("AAAA", ttl, record)
			} else if name == "AFSDB" {
				err = server.initSructRecords("AFSDB", ttl, record)
			} else if name == "CAA" {
				err = server.initSructRecords("CAA", ttl, record)
			} else if name == "CNAME" {
				err = server.initStringRecords("CNAME", ttl, record)
			} else if name == "MX" {
				err = server.initSructRecords("MX", ttl, record)
			} else if name == "SOA" {
				err = server.initSructRecords("SOA", ttl, record)
			} else if name == "TXT" {
				err = server.initArrayRecords("TXT", ttl, record)
			} else if name == "URI" {
				err = server.initSructRecords("URI", ttl, record)
			} else if name == "NS" {
				err = server.initStringRecords("NS", ttl, record)
			} else {
				return errors.New("No ns record with type" + name + "exist!")
			}
			if err != nil {
				fmt.Println("---> ", err)
				return err
			}
		}
	}
	return nil
}

///////////////////////  Log Services functions ////////////////////////////////////////////////
var (
	log_client_ *log_client.Log_Client
)

/**
 * Get the log client.
 */
func (server *server) GetLogClient() (*log_client.Log_Client, error) {
	var err error
	if log_client_ == nil {
		log_client_, err = log_client.NewLogService_Client(server.Domain, "log.LogService")
		if err != nil {
			return nil, err
		}

	}
	return log_client_, nil
}
func (server *server) logServiceInfo(method, fileLine, functionName, infos string) {
	log_client_, err := server.GetLogClient()
	if err != nil {
		return
	}
	log_client_.Log(server.Name, server.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (server *server) logServiceError(method, fileLine, functionName, infos string) {
	log_client_, err := server.GetLogClient()
	if err != nil {
		return
	}
	log_client_.Log(server.Name, server.Domain, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	// grpclog.SetLogger(log.New(os.Stdout, "dns_service: ", log.LstdFlags))

	// Set the log information in case of crash...

	// The actual server implementation.
	s_impl := new(server)
	Utility.RegisterType(s_impl) // must be call dynamically
	s_impl.Name = string(dnspb.File_dns_proto.Services().Get(0).FullName())
	s_impl.Proto = dnspb.File_dns_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.DnsPort = 53 // The default dns port.
	s_impl.StorageDataPath = os.TempDir() + "/dns"
	s_impl.PublisherId = "globulario" // value by default.
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		fmt.Printf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Start dns services
	go func() {

		ServeDns(s_impl.DnsPort)
	}()

	// Register the echo services
	dnspb.RegisterDnsServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
