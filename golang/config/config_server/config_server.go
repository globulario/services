package main

import (
	"context"
	"encoding/json"
	"errors"

	//"fmt"
	"log"
	"os"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/config/configpb"
	globular "github.com/globulario/services/golang/globular_service"

	//"github.com/globulario/services/golang/interceptors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""
	domain          string = "localhost"
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Mac             string
	Name            string
	Domain          string
	Address         string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Version         string
	PublisherId     string
	KeepUpToDate    bool
	KeepAlive       bool
	Checksum        string
	Plaform         string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	ModTime         int64
	State           string
	TLS             bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The grpc server.
	grpcServer *grpc.Server
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

// The path of the configuration.
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
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
func (svr *server) GetId() string {
	return svr.Id
}
func (svr *server) SetId(id string) {
	svr.Id = id
}

// The name of a service, must be the gRpc Service name.
func (svr *server) GetName() string {
	return svr.Name
}
func (svr *server) SetName(name string) {
	svr.Name = name
}

// The description of the service
func (svr *server) GetDescription() string {
	return svr.Description
}
func (svr *server) SetDescription(description string) {
	svr.Description = description
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
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

// The list of keywords of the services.
func (svr *server) GetKeywords() []string {
	return svr.Keywords
}
func (svr *server) SetKeywords(keywords []string) {
	svr.Keywords = keywords
}

func (svr *server) GetRepositories() []string {
	return svr.Repositories
}
func (svr *server) SetRepositories(repositories []string) {
	svr.Repositories = repositories
}

func (svr *server) GetDiscoveries() []string {
	return svr.Discoveries
}
func (svr *server) SetDiscoveries(discoveries []string) {
	svr.Discoveries = discoveries
}

// Dist
func (svr *server) Dist(path string) (string, error) {

	return globular.Dist(path, svr)
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

// The path of the executable.
func (svr *server) GetPath() string {
	return svr.Path
}
func (svr *server) SetPath(path string) {
	svr.Path = path
}

// The path of the .proto file.
func (svr *server) GetProto() string {
	return svr.Proto
}
func (svr *server) SetProto(proto string) {
	svr.Proto = proto
}

// The gRpc port.
func (svr *server) GetPort() int {
	return svr.Port
}
func (svr *server) SetPort(port int) {
	svr.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (svr *server) GetProxy() int {
	return svr.Proxy
}
func (svr *server) SetProxy(proxy int) {
	svr.Proxy = proxy
}

// Can be one of http/https/tls
func (svr *server) GetProtocol() string {
	return svr.Protocol
}
func (svr *server) SetProtocol(protocol string) {
	svr.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (svr *server) GetAllowAllOrigins() bool {
	return svr.AllowAllOrigins
}
func (svr *server) SetAllowAllOrigins(allowAllOrigins bool) {
	svr.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (svr *server) GetAllowedOrigins() string {
	return svr.AllowedOrigins
}

func (svr *server) SetAllowedOrigins(allowedOrigins string) {
	svr.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (svr *server) GetDomain() string {
	return svr.Domain
}
func (svr *server) SetDomain(domain string) {
	svr.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (svr *server) GetTls() bool {
	return svr.TLS
}
func (svr *server) SetTls(hasTls bool) {
	svr.TLS = hasTls
}

// The certificate authority file
func (svr *server) GetCertAuthorityTrust() string {
	return svr.CertAuthorityTrust
}
func (svr *server) SetCertAuthorityTrust(ca string) {
	svr.CertAuthorityTrust = ca
}

// The certificate file.
func (svr *server) GetCertFile() string {
	return svr.CertFile
}
func (svr *server) SetCertFile(certFile string) {
	svr.CertFile = certFile
}

// The key file.
func (svr *server) GetKeyFile() string {
	return svr.KeyFile
}
func (svr *server) SetKeyFile(keyFile string) {
	svr.KeyFile = keyFile
}

// The service version
func (svr *server) GetVersion() string {
	return svr.Version
}
func (svr *server) SetVersion(version string) {
	svr.Version = version
}

// The publisher id.
func (svr *server) GetPublisherId() string {
	return svr.PublisherId
}
func (svr *server) SetPublisherId(publisherId string) {
	svr.PublisherId = publisherId
}

func (svr *server) GetKeepUpToDate() bool {
	return svr.KeepUpToDate
}
func (svr *server) SetKeepUptoDate(val bool) {
	svr.KeepUpToDate = val
}

func (svr *server) GetKeepAlive() bool {
	return svr.KeepAlive
}
func (svr *server) SetKeepAlive(val bool) {
	svr.KeepAlive = val
}

func (svr *server) GetPermissions() []interface{} {
	return svr.Permissions
}
func (svr *server) SetPermissions(permissions []interface{}) {
	svr.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (svr *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewConfigService_Client", config_client.NewConfigService_Client)

	// Get the configuration path.
	err := globular.InitService(svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr /*interceptors.ServerUnaryInterceptor, interceptors.ServerStreamIntercepto*/, nil, nil)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (svr *server) Save() error {
	// Create the file...
	return globular.SaveService(svr)
}

func (svr *server) StartService() error {
	return globular.StartService(svr, svr.grpcServer)
}

func (svr *server) StopService() error {
	return globular.StopService(svr, svr.grpcServer)
}

/////////////////////// Config service specific function /////////////////////////////////

// Get the list of all services configurations
func (svr *server) GetServiceConfiguration(ctx context.Context, rqst *configpb.GetServiceConfigurationRequest) (*configpb.GetServiceConfigurationResponse, error) {
	config__, err := config.GetServiceConfigurationById(rqst.Path)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// convert to the a protobuffer struct...
	return &configpb.GetServiceConfigurationResponse{Config: Utility.ToString(config__)}, nil
}

// Set a service configuration.
func (svr *server) SetServiceConfiguration(ctx context.Context, rqst *configpb.SetServiceConfigurationRequest) (*configpb.SetServiceConfigurationResponse, error) {

	config_ := make(map[string]interface{}, 0)
	json.Unmarshal([]byte(rqst.Config), &config_)

	if config_["ConfigPath"] == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no configuration path was foud for service "+config_["Name"].(string)+":"+config_["Id"].(string))))
	}

	configPath := config_["ConfigPath"].(string)
	if !strings.HasSuffix(configPath, "config.json") {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the configuration file must have name config.json")))
	}

	if !Utility.Exists(configPath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no configuration file found at "+configPath)))
	}

	err := config.SaveServiceConfiguration(config_)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &configpb.SetServiceConfigurationResponse{}, nil
}

// Get service configuration with a given id.
func (svr *server) GetServiceConfigurationById(ctx context.Context, rqst *configpb.GetServiceConfigurationByIdRequest) (*configpb.GetServiceConfigurationByIdResponse, error) {
	config_, err := config.GetServiceConfigurationById(rqst.Id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &configpb.GetServiceConfigurationByIdResponse{Config: Utility.ToString(config_)}, nil
}

// Get list of service configuration with a given name
func (svr *server) GetServicesConfigurationsByName(ctx context.Context, rqst *configpb.GetServicesConfigurationsByNameRequest) (*configpb.GetServicesConfigurationsByNameResponse, error) {

	configs, err := config.GetServicesConfigurationsByName(rqst.Name)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rsp := &configpb.GetServicesConfigurationsByNameResponse{}
	rsp.Configs = make([]string, 0)
	for i := 0; i < len(configs); i++ {
		rsp.Configs = append(rsp.Configs, Utility.ToString(configs[i]))
	}

	return rsp, nil
}

// Get the list of all services configurations
func (svr *server) GetServicesConfigurations(ctx context.Context, rqst *configpb.GetServicesConfigurationsRequest) (*configpb.GetServicesConfigurationsResponse, error) {
	configs, err := config.GetServicesConfigurations()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	rsp := &configpb.GetServicesConfigurationsResponse{}
	rsp.Configs = make([]string, 0)
	for i := 0; i < len(configs); i++ {
		rsp.Configs = append(rsp.Configs, Utility.ToString(configs[i]))
	}

	return rsp, nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(configpb.File_config_proto.Services().Get(0).FullName())
	s_impl.Proto = configpb.File_config_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Configuration manager to get synchronize access to services configurations"
	s_impl.Keywords = []string{"Configurations", "Synchronization", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
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
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	// Register the config services
	configpb.RegisterConfigServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
