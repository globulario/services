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

	"encoding/binary"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dns/dnspb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/miekg/dns"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10033
	defaultProxy = 10034

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// pointer to the sever implementation.
	s *server
)

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
	State           string
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
	Checksum           string
	Plaform            string
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	Root               string

	// The grpc server.
	grpcServer *grpc.Server

	// Contain the configuration of the storage service use to store
	// the actual values.
	DnsPort int      // the dns port
	Domains []string // The list of domains managed by this dns.

	// The replication factor, only use by scylla.
	ReplicationFactor int

	// The storage store.
	store storage_store.Store

	connection_is_open bool
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
	if pid == -1 {
		// I will close the connection.
		srv.store.Close()
		srv.connection_is_open = false
	}
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

func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) createPermission(ctx context.Context, path string) error {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}

	// Set the owner of the conversation.
	rbac_client_, err := srv.GetRbacClient()
	if err != nil {
		return err
	}

	err = rbac_client_.AddResourceOwner(path, "domain", clientId, rbacpb.SubjectType_ACCOUNT)

	if err != nil {
		return err
	}

	return nil
}

// Create the configuration file if is not already exist.
func (srv *server) Init() error {

	// Get the configuration path.
	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	if len(srv.Root) == 0 {
		fmt.Println("The value StorageDataPath in the configuration must be given. You can use /tmp (on linux) if you don't want to keep values indefilnely on the storage srv.")
	}

	s = srv

	return nil

}

// Save the configuration values.
func (srv *server) Save() error {
	// Create the file...
	return globular.SaveService(srv)
}

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

func (srv *server) Stop(context.Context, *dnspb.StopRequest) (*dnspb.StopResponse, error) {
	return &dnspb.StopResponse{}, srv.StopService()
}

//////////////////////////////// DNS Service specific //////////////////////////

// Open the connection if it's close.
func (srv *server) openConnection() error {
	if srv.connection_is_open {
		return nil
	}

	// Open store.
	srv.store = storage_store.NewBadger_store()
	err := srv.store.Open(`{"path":"` + srv.Root + `", "name":"dns"}`)
	if err != nil {
		fmt.Println("fail to read/create permissions folder with error: ", srv.Root+"/dns", err)
	}

	srv.connection_is_open = true

	return nil
}

func (srv *server) isManaged(domain string) bool {
	for i := 0; i < len(srv.Domains); i++ {
		if strings.HasSuffix(domain, srv.Domains[i]) {
			return true
		}
	}
	return false
}

// Set a dns entry.
func (srv *server) SetA(ctx context.Context, rqst *dnspb.SetARequest) (*dnspb.SetAResponse, error) {

	if !srv.isManaged(rqst.Domain) {
		err := errors.New("The domain " + rqst.Domain + " is not manage by this dns.")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)

	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain = strings.ToLower(domain)
	uuid := Utility.GenerateUUID("A:" + domain)

	values := make([]string, 0)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will add the new value.
	if !Utility.Contains(values, rqst.A) {
		values = append(values, rqst.A)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.store.SetItem(uuid, data)
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.logServiceInfo("SetA", Utility.FileLine(), Utility.FunctionName(), "domain: A:"+domain+" with uuid"+uuid+"is set with ipv4 address"+rqst.A)
	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetAResponse{
		Message: domain, // return the full domain.
	}, nil
}

func (srv *server) RemoveA(ctx context.Context, rqst *dnspb.RemoveARequest) (*dnspb.RemoveAResponse, error) {
	//fmt.Println("Try remove dns entry ", rqst.Domain)
	srv.logServiceInfo("SetA", Utility.FileLine(), Utility.FunctionName(), "Try remove dns entry "+rqst.Domain)

	if !srv.isManaged(rqst.Domain) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("The domain "+rqst.Domain+" is not manage by this dns.")))
	}

	domain := strings.ToLower(rqst.Domain)
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain = strings.ToLower(domain)
	uuid := Utility.GenerateUUID("A:" + domain)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
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

	// I will remove the value.
	if Utility.Contains(values, rqst.A) {
		values = Utility.RemoveString(values, rqst.A)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) == 0 {
		err = srv.store.RemoveItem(uuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		rbac_client_, err := srv.GetRbacClient()
		// remove the permission
		if err == nil {
			rbac_client_.DeleteResourcePermissions(domain)
		}
	} else {
		err = srv.store.SetItem(uuid, data)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &dnspb.RemoveAResponse{
		Result: true, // return the full domain.
	}, nil
}

func (srv *server) get_ipv4(domain string) ([]string, uint32, error) {
	domain = strings.ToLower(domain)
	if strings.HasSuffix(domain, ".") {
		domain = domain[:len(domain)-1]
	}

	err := srv.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain = strings.ToLower(domain)
	uuid := Utility.GenerateUUID("A:" + domain)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
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

	return values, srv.getTtl(uuid), nil
}

func (srv *server) GetA(ctx context.Context, rqst *dnspb.GetARequest) (*dnspb.GetAResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain := strings.ToLower(rqst.Domain)
	uuid := Utility.GenerateUUID("A:" + domain)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, err
	}

	values := make([]string, 0)
	err = json.Unmarshal(data, &values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &dnspb.GetAResponse{
		A: values, // return the full domain.
	}, nil
}

// Set a dns entry.
func (srv *server) SetAAAA(ctx context.Context, rqst *dnspb.SetAAAARequest) (*dnspb.SetAAAAResponse, error) {

	srv.logServiceInfo("SetAAAA", Utility.FileLine(), Utility.FunctionName(), "Try set dns entry "+rqst.Domain)
	if !srv.isManaged(rqst.Domain) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("The domain "+rqst.Domain+" is not manage by this dns.")))
	}

	domain := strings.ToLower(rqst.Domain)

	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain = strings.ToLower(domain)
	uuid := Utility.GenerateUUID("AAAA:" + domain)
	values := make([]string, 0)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will add the new value.
	if !Utility.Contains(values, rqst.Aaaa) {
		values = append(values, rqst.Aaaa)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.store.SetItem(uuid, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetAAAAResponse{
		Message: domain, // return the full domain.
	}, nil
}

func (srv *server) RemoveAAAA(ctx context.Context, rqst *dnspb.RemoveAAAARequest) (*dnspb.RemoveAAAAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	if !srv.isManaged(rqst.Domain) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("The domain "+rqst.Domain+" is not manage by this dns.")))
	}

	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	domain = strings.ToLower(domain)
	uuid := Utility.GenerateUUID("AAAA:" + domain)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will remove the value.
	if Utility.Contains(values, rqst.Aaaa) {
		values = Utility.RemoveString(values, rqst.Aaaa)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) == 0 {
		err = srv.store.RemoveItem(uuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		rbac_client_, err := srv.GetRbacClient()
		if err == nil {
			rbac_client_.DeleteResourcePermissions(domain)
		}

	}

	return &dnspb.RemoveAAAAResponse{
		Result: true, // return the full domain.
	}, nil
}

func (srv *server) get_ipv6(domain string) ([]string, uint32, error) {
	domain = strings.ToLower(domain)
	if strings.HasSuffix(domain, ".") {
		domain = domain[:len(domain)-1]
	}

	err := srv.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	domain = strings.ToLower(domain)
	uuid := Utility.GenerateUUID("AAAA:" + domain)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, 0, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No value found for domain "+domain)))
	}

	return values, srv.getTtl(uuid), nil
}

func (srv *server) GetAAAA(ctx context.Context, rqst *dnspb.GetAAAARequest) (*dnspb.GetAAAAResponse, error) {
	domain := strings.ToLower(rqst.Domain)
	err := srv.openConnection()

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain = strings.ToLower(domain)
	uuid := Utility.GenerateUUID("AAAA:" + domain)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No value found for domain "+domain)))
	}

	return &dnspb.GetAAAAResponse{
		Aaaa: values, // return the full domain.
	}, nil
}

// Set a text entry.
func (srv *server) SetText(ctx context.Context, rqst *dnspb.SetTextRequest) (*dnspb.SetTextResponse, error) {
	err := srv.openConnection()
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

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	err = srv.store.SetItem(uuid, values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetTextResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (srv *server) getText(id string) ([]string, uint32, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	data, err := srv.store.GetItem(uuid)

	values := make([]string, 0)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, 0, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return values, srv.getTtl(uuid), nil
}

// Retreive a text value
func (srv *server) GetText(ctx context.Context, rqst *dnspb.GetTextRequest) (*dnspb.GetTextResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &dnspb.GetTextResponse{
		Values: values, // return the full domain.
	}, nil
}

// Remove a text entry
func (srv *server) RemoveText(ctx context.Context, rqst *dnspb.RemoveTextRequest) (*dnspb.RemoveTextResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("TXT:" + id)
	err = srv.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rbac_client_, err := srv.GetRbacClient()
	if err == nil {
		rbac_client_.DeleteResourcePermissions(rqst.Id)
	}

	return &dnspb.RemoveTextResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a NS entry.
func (srv *server) SetNs(ctx context.Context, rqst *dnspb.SetNsRequest) (*dnspb.SetNsResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("NS:" + id)

	// because it can be more than one NS, we store the value as json that contain aa list of string.
	values := make([]string, 0)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will add the new value.
	if !Utility.Contains(values, rqst.Ns) {
		values = append(values, rqst.Ns)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// I will save the new value.
	err = srv.store.SetItem(uuid, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetNsResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (srv *server) getNs(id string) ([]string, uint32, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, 0, err
	}

	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("NS:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, 0, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return values, srv.getTtl(uuid), nil
}

// Retreive a text value
func (srv *server) GetNs(ctx context.Context, rqst *dnspb.GetNsRequest) (*dnspb.GetNsResponse, error) {

	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("NS:" + id)

	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &dnspb.GetNsResponse{
		Ns: values, // return the full domain.
	}, nil
}

// Remove a text entry
func (srv *server) RemoveNs(ctx context.Context, rqst *dnspb.RemoveNsRequest) (*dnspb.RemoveNsResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("NS:" + id)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	values := make([]string, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No value found for domain "+id)))
	}

	// I will remove the value.
	if Utility.Contains(values, rqst.Ns) {
		values = Utility.RemoveString(values, rqst.Ns)
	}

	if len(values) == 0 {
		err = srv.store.RemoveItem(uuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		rbac_client_, err := srv.GetRbacClient()
		if err == nil {
			rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
	} else {
		// I will save the new value.
		data, err = json.Marshal(values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		err = srv.store.SetItem(uuid, data)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &dnspb.RemoveNsResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a CName entry.
func (srv *server) SetCName(ctx context.Context, rqst *dnspb.SetCNameRequest) (*dnspb.SetCNameResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	err = srv.store.SetItem(uuid, []byte(rqst.Cname))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetCNameResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the CName.
func (srv *server) getCName(id string) (string, uint32, error) {
	err := srv.openConnection()
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("CName:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return "", 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return string(data), srv.getTtl(uuid), err
}

// Retreive a CName value
func (srv *server) GetCName(ctx context.Context, rqst *dnspb.GetCNameRequest) (*dnspb.GetCNameResponse, error) {

	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	data, err := srv.store.GetItem(uuid)
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
func (srv *server) RemoveCName(ctx context.Context, rqst *dnspb.RemoveCNameRequest) (*dnspb.RemoveCNameResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CName:" + id)
	err = srv.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rbac_client_, err := srv.GetRbacClient()
	if err == nil {
		rbac_client_.DeleteResourcePermissions(rqst.Id)
	}

	return &dnspb.RemoveCNameResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a text entry.
func (srv *server) SetMx(ctx context.Context, rqst *dnspb.SetMxRequest) (*dnspb.SetMxResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("MX:" + id)

	// because it can be more than one NS, we store the value as json that contain aa list of string.

	values := make([]*dnspb.MX, 0)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will add the new value.
	for i := 0; i < len(values); i++ {
		if values[i].Mx == rqst.Mx.Mx {
			values[i] = rqst.Mx
			rqst.Mx = nil
			break
		}
	}

	if rqst.Mx != nil {
		values = append(values, rqst.Mx)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// I will save the new value.
	err = srv.store.SetItem(uuid, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetMxResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (srv *server) getMx(id, mx string) ([]*dnspb.MX, uint32, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("MX:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.MX, 0) // use a map instead of Mx struct.

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, 0, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will return the value if any.
	if len(mx) > 0 {
		for i := 0; i < len(values); i++ {
			if values[i].Mx == mx {

				return []*dnspb.MX{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}

	return values, srv.getTtl(uuid), nil
}

// Retreive a text value
func (srv *server) GetMx(ctx context.Context, rqst *dnspb.GetMxRequest) (*dnspb.GetMxResponse, error) {

	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("MX:" + id)
	data, err := srv.store.GetItem(uuid)

	values := make([]*dnspb.MX, 0)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(rqst.Mx) > 0 {
		for i := 0; i < len(values); i++ {
			if values[i].Mx == rqst.Mx {
				return &dnspb.GetMxResponse{
					Result: []*dnspb.MX{values[i]}, // return the mx.
				}, nil
			}
		}
	}

	return &dnspb.GetMxResponse{
		Result: values, // return the full domain.
	}, nil
}

// Remove a text entry
func (srv *server) RemoveMx(ctx context.Context, rqst *dnspb.RemoveMxRequest) (*dnspb.RemoveMxResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("MX:" + id)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.MX, 0)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No value found for domain "+id)))
	}

	// I will remove the value.
	for i := 0; i < len(values); i++ {
		if values[i].Mx == rqst.Mx {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		err = srv.store.SetItem(uuid, data)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		err = srv.store.RemoveItem(uuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		rbac_client_, err := srv.GetRbacClient()
		if err == nil {
			rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
	}

	return &dnspb.RemoveMxResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a text entry.
func (srv *server) SetSoa(ctx context.Context, rqst *dnspb.SetSoaRequest) (*dnspb.SetSoaResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("SOA:" + id)

	// because it can be more than one NS, we store the value as json that contain aa list of string.
	values := make([]*dnspb.SOA, 0)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will add the new value.
	for i := 0; i < len(values); i++ {
		if values[i].Ns == rqst.Soa.Ns {
			values[i] = rqst.Soa
			rqst.Soa = nil
			break
		}
	}

	if rqst.Soa != nil {
		values = append(values, rqst.Soa)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.store.SetItem(uuid, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetSoaResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (srv *server) getSoa(id, ns string) ([]*dnspb.SOA, uint32, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("SOA:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0) // use a map instead of Mx struct.

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, 0, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(ns) > 0 {
		for i := 0; i < len(values); i++ {
			if values[i].Ns == ns {
				return []*dnspb.SOA{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}

	return values, srv.getTtl(uuid), nil
}

// Retreive a text value
func (srv *server) GetSoa(ctx context.Context, rqst *dnspb.GetSoaRequest) (*dnspb.GetSoaResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("SOA:" + id)
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(rqst.Ns) > 0 {
		for i := 0; i < len(values); i++ {
			if values[i].Ns == rqst.Ns {
				return &dnspb.GetSoaResponse{
					Result: []*dnspb.SOA{values[i]}, // return the mx.
				}, nil
			}
		}
	}

	return &dnspb.GetSoaResponse{
		Result: values, // return the full domain.
	}, nil
}

// Remove a soa entry
func (srv *server) RemoveSoa(ctx context.Context, rqst *dnspb.RemoveSoaRequest) (*dnspb.RemoveSoaResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("SOA:" + id)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.SOA, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No value found for domain "+id)))
	}

	// I will remove the value.
	for i := 0; i < len(values); i++ {
		if values[i].Ns == rqst.Ns {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		err = srv.store.SetItem(uuid, data)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		err = srv.store.RemoveItem(uuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		rbac_client_, err := srv.GetRbacClient()
		if err == nil {
			rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
	}

	return &dnspb.RemoveSoaResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a text entry.
func (srv *server) SetUri(ctx context.Context, rqst *dnspb.SetUriRequest) (*dnspb.SetUriResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)

	// because it can be more than one NS, we store the value as json that contain aa list of string.
	values := make([]*dnspb.URI, 0)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will add the new value.
	for i := 0; i < len(values); i++ {
		if values[i].Target == rqst.Uri.Target {
			values[i] = rqst.Uri
			rqst.Uri = nil
			break
		}
	}

	if rqst.Uri != nil {
		values = append(values, rqst.Uri)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.store.SetItem(uuid, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetUriResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the text.
func (srv *server) getUri(id, target string) ([]*dnspb.URI, uint32, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("URI:" + id)
	data, err := srv.store.GetItem(uuid)

	values := make([]*dnspb.URI, 0) // use a map instead of Mx struct.

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, 0, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will return the value if any.
	if len(target) > 0 {
		for i := 0; i < len(values); i++ {
			if values[i].Target == target {
				return []*dnspb.URI{values[i]}, srv.getTtl(uuid), nil
			}
		}
	}

	return values, srv.getTtl(uuid), nil
}

// Retreive a text value
func (srv *server) GetUri(ctx context.Context, rqst *dnspb.GetUriRequest) (*dnspb.GetUriResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)
	data, err := srv.store.GetItem(uuid)

	values := make([]*dnspb.URI, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(rqst.Target) > 0 {
		for i := 0; i < len(values); i++ {
			if values[i].Target == rqst.Target {
				return &dnspb.GetUriResponse{
					Result: []*dnspb.URI{values[i]}, // return the uri.
				}, nil
			}
		}
	}

	return &dnspb.GetUriResponse{
		Result: values, // return the full domain.
	}, nil
}

// Remove AFSDB
func (srv *server) RemoveUri(ctx context.Context, rqst *dnspb.RemoveUriRequest) (*dnspb.RemoveUriResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("URI:" + id)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	values := make([]*dnspb.URI, 0)

	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(values) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No value found for domain "+id)))
	}

	// I will remove the value.
	for i := 0; i < len(values); i++ {
		if values[i].Target == rqst.Target {
			values = append(values[:i], values[i+1:]...)
			break
		}
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) > 0 {
		err = srv.store.SetItem(uuid, data)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		err = srv.store.RemoveItem(uuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		rbac_client_, err := srv.GetRbacClient()
		if err == nil {
			rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
	}

	return &dnspb.RemoveUriResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a AFSDB entry.
func (srv *server) SetAfsdb(ctx context.Context, rqst *dnspb.SetAfsdbRequest) (*dnspb.SetAfsdbResponse, error) {
	err := srv.openConnection()
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
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	err = srv.store.SetItem(uuid, values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetAfsdbResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the AFSDB.
func (srv *server) getAfsdb(id string) (*dnspb.AFSDB, uint32, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return nil, 0, err
	}

	afsdb := new(dnspb.AFSDB) // use a map instead of Mx struct.
	err = json.Unmarshal(data, afsdb)
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return afsdb, srv.getTtl(uuid), nil
}

// Retreive a AFSDB value
func (srv *server) GetAfsdb(ctx context.Context, rqst *dnspb.GetAfsdbRequest) (*dnspb.GetAfsdbResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	data, err := srv.store.GetItem(uuid)
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
func (srv *server) RemoveAfsdb(ctx context.Context, rqst *dnspb.RemoveAfsdbRequest) (*dnspb.RemoveAfsdbResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("AFSDB:" + id)
	err = srv.store.RemoveItem(uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rbac_client_, err := srv.GetRbacClient()
	if err == nil {
		rbac_client_.DeleteResourcePermissions(rqst.Id)
	}

	return &dnspb.RemoveAfsdbResponse{
		Result: true, // return the full domain.
	}, nil
}

// Set a CAA entry.
func (srv *server) SetCaa(ctx context.Context, rqst *dnspb.SetCaaRequest) (*dnspb.SetCaaResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)

	// because it can be more than one NS, we store the value as json that contain aa list of string.
	values := make([]*dnspb.CAA, 0)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)
	if err == nil {
		err = json.Unmarshal(data, &values)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will add the new value.
	for i := 0; i < len(values); i++ {
		if values[i].Domain == rqst.Caa.Domain {
			values[i] = rqst.Caa
			rqst.Caa = nil
			break
		}
	}

	if rqst.Caa != nil {
		values = append(values, rqst.Caa)
	}

	// I will save the new value.
	data, err = json.Marshal(values)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.store.SetItem(uuid, data)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.setTtl(uuid, rqst.Ttl)

	return &dnspb.SetCaaResponse{
		Result: true, // return the full domain.
	}, nil
}

// return the CAA.
func (srv *server) getCaa(id, domain string) ([]*dnspb.CAA, uint32, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, 0, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id = strings.ToLower(id)
	uuid := Utility.GenerateUUID("CAA:" + id)
	data, err := srv.store.GetItem(uuid)

	caa := make([]*dnspb.CAA, 0) // use a map instead of Mx struct.

	if err == nil {
		err = json.Unmarshal(data, &caa)
		if err != nil {
			return nil, 0, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// I will return the value if any.
	if len(domain) > 0 {
		for i := 0; i < len(caa); i++ {
			if caa[i].Domain == domain {
				return []*dnspb.CAA{caa[i]}, srv.getTtl(uuid), nil
			}
		}
	}

	return caa, srv.getTtl(uuid), nil
}

// Retreive a AFSDB value
func (srv *server) GetCaa(ctx context.Context, rqst *dnspb.GetCaaRequest) (*dnspb.GetCaaResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)
	data, err := srv.store.GetItem(uuid)

	// I will return the value if any.
	caa := make([]*dnspb.CAA, 0)

	if err == nil {
		err = json.Unmarshal(data, &caa)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(rqst.Domain) > 0 {
		for i := 0; i < len(caa); i++ {
			if caa[i].Domain == rqst.Domain {
				return &dnspb.GetCaaResponse{
					Result: []*dnspb.CAA{caa[i]}, // return the uri.
				}, nil
			}
		}
	}

	return &dnspb.GetCaaResponse{
		Result: caa, // return the full domain.
	}, nil
}

// Remove CAA
func (srv *server) RemoveCaa(ctx context.Context, rqst *dnspb.RemoveCaaRequest) (*dnspb.RemoveCaaResponse, error) {
	err := srv.openConnection()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id := strings.ToLower(rqst.Id)
	uuid := Utility.GenerateUUID("CAA:" + id)

	// I will retreive the current value if any.
	data, err := srv.store.GetItem(uuid)

	caa := make([]*dnspb.CAA, 0)

	if err == nil {
		err = json.Unmarshal(data, &caa)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(caa) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No value found for domain "+id)))
	}

	// I will remove the value.
	for i := 0; i < len(caa); i++ {
		if caa[i].Domain == rqst.Domain {
			caa = append(caa[:i], caa[i+1:]...)
			break
		}
	}

	// I will save the new value.
	data, err = json.Marshal(caa)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(caa) > 0 {
		err = srv.store.SetItem(uuid, data)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		err = srv.store.RemoveItem(uuid)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		rbac_client_, err := srv.GetRbacClient()
		if err == nil {
			rbac_client_.DeleteResourcePermissions(rqst.Id)
		}
	}

	return &dnspb.RemoveCaaResponse{
		Result: true, // return the full domain.
	}, nil
}

// ///////////////////// DNS Specific service //////////////////////
type handler struct{}

func (hd *handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {

	switch r.Question[0].Qtype {
	case dns.TypeA:
		msg := dns.Msg{}
		msg.SetReply(r)

		domain := msg.Question[0].Name
		msg.Authoritative = true
		addresses, ttl, err := s.get_ipv4(domain) // get the address name from the

		if err == nil {
			for _, address := range addresses {
				msg.Answer = append(msg.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
					A:   net.ParseIP(address),
				})
			}
		} else {
			fmt.Println("fail to retreive ipv4 address for "+domain, err)
		}

		w.WriteMsg(&msg)

	case dns.TypeAAAA:

		msg := dns.Msg{}
		msg.SetReply(r)

		msg.Authoritative = true
		domain := msg.Question[0].Name
		addresses, ttl, err := s.get_ipv6(domain) // get the address name from the
		if err == nil {
			for _, address := range addresses {
				msg.Answer = append(msg.Answer, &dns.AAAA{
					Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
					AAAA: net.ParseIP(address),
				})
			}
		} else {
			fmt.Println(err)
		}

		w.WriteMsg(&msg)

	case dns.TypeAFSDB:

		msg := dns.Msg{}
		msg.SetReply(r)

		msg.Authoritative = true
		id := msg.Question[0].Name
		afsdb, ttl, err := s.getAfsdb(id)
		domain, _ := config.GetDomain()
		if err == nil {
			msg.Answer = append(msg.Answer, &dns.AFSDB{
				Hdr:      dns.RR_Header{Name: domain, Rrtype: dns.TypeAFSDB, Class: dns.ClassINET, Ttl: ttl},
				Subtype:  uint16(afsdb.Subtype), //
				Hostname: afsdb.Hostname,
			})
		} else {
			fmt.Println(err)
		}

		w.WriteMsg(&msg)

	case dns.TypeCAA:

		msg := dns.Msg{}
		msg.SetReply(r)

		msg.Authoritative = true
		name := msg.Question[0].Name
		domain := ""
		if len(msg.Question) > 1 {
			domain = msg.Question[1].Name
		}

		values, ttl, err := s.getCaa(name, domain)
		if err == nil {
			for _, caa := range values {
				msg.Answer = append(msg.Answer, &dns.CAA{
					Hdr:   dns.RR_Header{Name: name, Rrtype: dns.TypeCAA, Class: dns.ClassINET, Ttl: ttl},
					Flag:  uint8(caa.Flag),
					Tag:   caa.Tag,
					Value: caa.Domain,
				})
			}
		} else {
			fmt.Println(err)
		}

		w.WriteMsg(&msg)

	case dns.TypeCNAME:

		msg := dns.Msg{}
		msg.SetReply(r)

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

		w.WriteMsg(&msg)

	case dns.TypeTXT:

		id := r.Question[0].Name
		values, ttl, err := s.getText(id)
		if err == nil {
			msg := new(dns.Msg)
			msg.SetReply(r)
			// Create and add multiple TXT records to the Answer section.
			for _, txtValue := range values {
				msg.Answer = append(msg.Answer, &dns.TXT{
					Hdr: dns.RR_Header{Name: id, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: ttl},
					Txt: []string{txtValue},
				})
			}
			// Send the response.
			w.WriteMsg(msg)
		}

	case dns.TypeNS:
		id := r.Question[0].Name

		values, ttl, err := s.getNs(id)
		msg := new(dns.Msg)
		msg.SetReply(r)

		if err == nil {
			// Create and add multiple NS records to the Answer section.
			for _, ns := range values {
				msg.Answer = append(msg.Answer, &dns.NS{
					Hdr: dns.RR_Header{Name: id, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
					Ns:  ns,
				})
			}
			// Send the response.
			w.WriteMsg(msg)
		}

	case dns.TypeMX:

		msg := dns.Msg{}
		msg.SetReply(r)

		id := msg.Question[0].Name // the id where the values is store.
		mx := ""
		if len(msg.Question) > 1 {
			mx = msg.Question[1].Name
		}

		values, ttl, err := s.getMx(id, mx)

		if err == nil {
			// in case of empty string I will return the certificate validation key.
			for _, mx := range values {
				msg.Answer = append(msg.Answer, &dns.MX{
					// keep text values.
					Hdr:        dns.RR_Header{Name: id, Rrtype: dns.TypeMX, Class: dns.ClassINET, Ttl: ttl},
					Preference: uint16(mx.Preference),
					Mx:         mx.Mx,
				})
			}
		}
		w.WriteMsg(&msg)

	case dns.TypeSOA:

		msg := dns.Msg{}
		msg.SetReply(r)

		id := msg.Question[0].Name
		ns := ""
		if len(msg.Question) > 1 {
			ns = msg.Question[1].Name
		}

		values, ttl, err := s.getSoa(id, ns)
		if err == nil {
			// in case of empty string I will return the certificate validation key.
			for _, soa := range values {
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
		}
		w.WriteMsg(&msg)

	case dns.TypeURI:

		msg := dns.Msg{}
		msg.SetReply(r)

		id := msg.Question[0].Name
		target := ""
		if len(msg.Question) > 1 {
			target = msg.Question[1].Name
		}

		values, ttl, err := s.getUri(id, target)
		if err == nil {
			// in case of empty string I will return the certificate validation key.
			for _, uri := range values {
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

}

func ServeDns(port int) error {
	// Now I will start the dns srv.
	srv := &dns.Server{Addr: ":" + strconv.Itoa(port), Net: "udp"}
	srv.Handler = &handler{}
	if err := srv.ListenAndServe(); err != nil {
		fmt.Println("Failed to set udp listener", err.Error())
		return err
	}

	return nil
}

func (srv *server) setTtl(uuid string, ttl uint32) error {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, ttl)
	uuid = Utility.GenerateUUID("TTL:" + uuid)
	err := srv.store.SetItem(uuid, data)
	return err
}

func (srv *server) getTtl(uuid string) uint32 {
	uuid = Utility.GenerateUUID("TTL:" + uuid)
	data, err := srv.store.GetItem(uuid)
	if err != nil {
		return 60 // the default value
	}
	return binary.LittleEndian.Uint32(data)
}

// /////////////////////  Log Services functions ////////////////////////////////////////////////

/**
 * Get the log client.
 */
func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	client, err := globular_client.GetClient(srv.Address, "log.LogService", "NewLogService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*log_client.Log_Client), nil
}

func (srv *server) logServiceInfo(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (srv *server) logServiceError(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Address, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

//////////////////////////////////////// RBAC Functions ///////////////////////////////////////////////
/**
 * Get the rbac client.
 */
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.DnsPort = 5353            // The default dns port.
	s_impl.PublisherId = "localhost" // value by default.
	s_impl.Permissions = make([]interface{}, 6)
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

	// Set the root path if is pass as argument.
	s_impl.Root = config.GetDataDir()

	// Create the root directory if not exist.
	Utility.CreateDirIfNotExist(s_impl.Root)

	// DNS operation on a given domain.
	s_impl.Permissions[0] = map[string]interface{}{"action": "/dns.DnsService/SetA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[1] = map[string]interface{}{"action": "/dns.DnsService/SetAAAA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[5] = map[string]interface{}{"action": "/dns.DnsService/SetText", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[2] = map[string]interface{}{"action": "/dns.DnsService/RemoveA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[3] = map[string]interface{}{"action": "/dns.DnsService/RemoveAAAA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[4] = map[string]interface{}{"action": "/dns.DnsService/RemoveText", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}

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
		fmt.Printf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
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
