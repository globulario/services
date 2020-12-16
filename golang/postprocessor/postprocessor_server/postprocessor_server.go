package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/Globular/Interceptors"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/postprocessor/postprocessor_client"
	"github.com/globulario/services/golang/postprocessor/postprocessorpb"
	"google.golang.org/grpc"

	//	"google.golang.org/grpc/codes"

	//"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
	//	"google.golang.org/grpc/status"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	domain string = "localhost"
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
	Domain          string
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
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string

	TLS bool

	// self-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	// The grpc server.
	grpcServer *grpc.Server
}

// Globular services implementation...
// The id of a particular service instance.
func (self *server) GetId() string {
	return self.Id
}
func (self *server) SetId(id string) {
	self.Id = id
}

// The name of a service, must be the gRpc Service name.
func (self *server) GetName() string {
	return self.Name
}
func (self *server) SetName(name string) {
	self.Name = name
}

// The description of the service
func (self *server) GetDescription() string {
	return self.Description
}
func (self *server) SetDescription(description string) {
	self.Description = description
}

// The list of keywords of the services.
func (self *server) GetKeywords() []string {
	return self.Keywords
}
func (self *server) SetKeywords(keywords []string) {
	self.Keywords = keywords
}

func (self *server) GetRepositories() []string {
	return self.Repositories
}
func (self *server) SetRepositories(repositories []string) {
	self.Repositories = repositories
}

func (self *server) GetDiscoveries() []string {
	return self.Discoveries
}
func (self *server) SetDiscoveries(discoveries []string) {
	self.Discoveries = discoveries
}

// Dist
func (self *server) Dist(path string) error {

	return globular.Dist(path, self)
}

func (self *server) GetPlatform() string {
	return globular.GetPlatform()
}

func (self *server) PublishService(address string, user string, password string) error {
	return globular.PublishService(address, user, password, self)
}

// The path of the executable.
func (self *server) GetPath() string {
	return self.Path
}
func (self *server) SetPath(path string) {
	self.Path = path
}

// The path of the .proto file.
func (self *server) GetProto() string {
	return self.Proto
}
func (self *server) SetProto(proto string) {
	self.Proto = proto
}

// The gRpc port.
func (self *server) GetPort() int {
	return self.Port
}
func (self *server) SetPort(port int) {
	self.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (self *server) GetProxy() int {
	return self.Proxy
}
func (self *server) SetProxy(proxy int) {
	self.Proxy = proxy
}

// Can be one of http/https/tls
func (self *server) GetProtocol() string {
	return self.Protocol
}
func (self *server) SetProtocol(protocol string) {
	self.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (self *server) GetAllowAllOrigins() bool {
	return self.AllowAllOrigins
}
func (self *server) SetAllowAllOrigins(allowAllOrigins bool) {
	self.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (self *server) GetAllowedOrigins() string {
	return self.AllowedOrigins
}

func (self *server) SetAllowedOrigins(allowedOrigins string) {
	self.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (self *server) GetDomain() string {
	return self.Domain
}
func (self *server) SetDomain(domain string) {
	self.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (self *server) GetTls() bool {
	return self.TLS
}
func (self *server) SetTls(hasTls bool) {
	self.TLS = hasTls
}

// The certificate authority file
func (self *server) GetCertAuthorityTrust() string {
	return self.CertAuthorityTrust
}
func (self *server) SetCertAuthorityTrust(ca string) {
	self.CertAuthorityTrust = ca
}

// The certificate file.
func (self *server) GetCertFile() string {
	return self.CertFile
}
func (self *server) SetCertFile(certFile string) {
	self.CertFile = certFile
}

// The key file.
func (self *server) GetKeyFile() string {
	return self.KeyFile
}
func (self *server) SetKeyFile(keyFile string) {
	self.KeyFile = keyFile
}

// The service version
func (self *server) GetVersion() string {
	return self.Version
}
func (self *server) SetVersion(version string) {
	self.Version = version
}

// The publisher id.
func (self *server) GetPublisherId() string {
	return self.PublisherId
}
func (self *server) SetPublisherId(publisherId string) {
	self.PublisherId = publisherId
}

func (self *server) GetKeepUpToDate() bool {
	return self.KeepUpToDate
}
func (self *server) SetKeepUptoDate(val bool) {
	self.KeepUpToDate = val
}

func (self *server) GetKeepAlive() bool {
	return self.KeepAlive
}
func (self *server) SetKeepAlive(val bool) {
	self.KeepAlive = val
}

func (self *server) GetPermissions() []interface{} {
	return self.Permissions
}
func (self *server) SetPermissions(permissions []interface{}) {
	self.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (self *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewPostprocessorService_Client", postprocessor_client.NewPostprocessorService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", self)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	self.grpcServer, err = globular.InitGrpcServer(self, Interceptors.ServerUnaryInterceptor, Interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (self *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", self)
}

func (self *server) StartService() error {
	return globular.StartService(self, self.grpcServer)
}

func (self *server) StopService() error {
	return globular.StopService(self, self.grpcServer)
}

func (self *server) Stop(context.Context, *postprocessorpb.StopRequest) (*postprocessorpb.StopResponse, error) {
	return &postprocessorpb.StopResponse{}, self.StopService()
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "postprocessor_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(postprocessorpb.File_proto_postprocessor_proto.Services().Get(0).FullName())
	s_impl.Proto = postprocessorpb.File_proto_postprocessor_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "The Hello world of gRPC service!"
	s_impl.Keywords = []string{"Example", "postprocessor", "Test", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 0)

	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id, err)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	if len(os.Args) > 2 {
		// Subcommands

		// Intall globular as service/demon
		publishCommand := flag.NewFlagSet("publish", flag.ExitOnError)
		publishCommand_address := publishCommand.String("a", "", "The domain where to publish the service")
		publishCommand_user := publishCommand.String("u", "", "The user (must be register on the domain)")
		publishCommand_password := publishCommand.String("p", "", "The user password")

		switch os.Args[1] {
		case "publish":
			publishCommand.Parse(os.Args[2:])
		}

		// Check if the command was parsed
		if publishCommand.Parsed() {
			// Required Flags
			if *publishCommand_address == "" {
				publishCommand.PrintDefaults()
				os.Exit(1)
			}
			if *publishCommand_user == "" {
				publishCommand.PrintDefaults()
				os.Exit(1)
			}
			if *publishCommand_password == "" {
				publishCommand.PrintDefaults()
				os.Exit(1)
			}

			err := s_impl.PublishService(*publishCommand_address, *publishCommand_user, *publishCommand_password)
			if err != nil {
				fmt.Println(err.Error())
			} else {
				fmt.Println("Your service was publish successfuly!")
			}

		}
	} else {
		// Register the postprocessor services
		postprocessorpb.RegisterPostprocessorServiceServer(s_impl.grpcServer, s_impl)
		reflection.Register(s_impl.grpcServer)

		// Start the service.
		s_impl.StartService()
	}

}
