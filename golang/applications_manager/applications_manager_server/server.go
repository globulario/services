package main

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/applications_manager/applications_manager_client"
	"github.com/globulario/services/golang/applications_manager/applications_managerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	// "google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/reflection"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separated values.
	allowed_origins string = ""
)

// server contains the values needed by Globular to start the service.
type server struct {
	// The global attributes of the service.
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
	AllowedOrigins  string // comma-separated string
	Protocol        string
	Version         string
	PublisherID     string
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

	// trust anchor for the certificate authority
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permissions for the service.

	Dependencies []string // The list of services needed by this service.

	// The gRPC server.
	grpcServer *grpc.Server

	// The webroot
	WebRoot string
}

// GetConfigurationPath returns the path to the service configuration.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path to the service configuration.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where the configuration can be found.
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP address where the configuration can be found.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the service process ID.
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the service process ID.
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the reverse-proxy process ID.
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the reverse-proxy process ID.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state.
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message recorded by the service.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError sets the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification time (unix seconds).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the last modification time (unix seconds).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the ID of this service instance.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the ID of this service instance.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetMac returns the MAC address recorded for the service.
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address recorded for the service.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetChecksum returns the service checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the service checksum.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the platform string.
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the platform string.
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetKeywords returns the list of service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the list of service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns the configured repositories.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets the configured repositories.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns the configured discovery endpoints.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets the configured discovery endpoints.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// Dist packages/distributes the service at the given path using Globular helpers.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the service dependencies (guaranteed non-nil).
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency appends a dependency if not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetPath returns the executable path.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the executable path.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the .proto file path.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the .proto file path.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse-proxy port.
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse-proxy port.
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the protocol (http/https/tls).
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the protocol (http/https/tls).
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins returns whether all origins are allowed to access the service.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether all origins are allowed to access the service.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated list of allowed origins (if not allowing all).
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the service domain (hostname).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the service domain (hostname).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls reports whether the service uses TLS.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS for the service.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA trust file path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA trust file path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the certificate file path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the certificate file path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the key file path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the key file path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher ID.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher ID.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate reports whether the service should keep applications up to date.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate sets the keep-up-to-date flag.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive reports whether the service should be kept alive.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive sets the keep-alive flag.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the service permissions payload.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the service permissions payload.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// Init creates the configuration file (if missing) and initializes the gRPC server.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

// Save persists the current configuration values.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the service gRPC server.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService stops the service gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// //////////////////////////////////////////////////////////////////////////////////////
// Resource manager helpers
// //////////////////////////////////////////////////////////////////////////////////////

func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

func (srv *server) getApplication(applicationId string) (*resourcepb.Application, error) {
	// Accept "name@domain" form, but only allow same-domain access.
	if strings.Contains(applicationId, "@") {
		if strings.Split(applicationId, "@")[1] != srv.Domain {
			return nil, errors.New("you can only get application in your own domain")
		}
		applicationId = strings.Split(applicationId, "@")[0]
	}

	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	applications, err := resourceClient.GetApplications(`{"_id":"` + applicationId + `"}`)
	if err != nil {
		return nil, err
	}
	if len(applications) == 0 {
		return nil, errors.New("no application found with name or _id " + applicationId)
	}
	return applications[0], nil
}

func (srv *server) deleteApplication(token, applicationId string) error {
	if _, err := security.ValidateToken(token); err != nil {
		return err
	}
	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return err
	}
	return resourceClient.DeleteApplication(token, applicationId)
}

func (srv *server) createApplication(token, id, name, domain, password, path, PublisherID, version, description, alias, icon string, actions, keywords []string) error {
	if domain != srv.Domain {
		return errors.New("you can only create application in your own domain")
	}
	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return err
	}
	return resourceClient.CreateApplication(token, id, name, domain, password, path, PublisherID, version, description, alias, icon, actions, keywords)
}

func (srv *server) createRole(token, id, name string, actions []string) error {
	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return err
	}
	return resourceClient.CreateRole(token, id, name, actions)
}

func (srv *server) createGroup(token, id, name, description string) error {
	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return err
	}
	return resourceClient.CreateGroup(token, id, name, description)
}

// ////////////////////// rbac service ///////////////////////////////////////

// GetRbacClient returns an RBAC client connected to the local RBAC service.
func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbacClient, err := srv.GetRbacClient()
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(path, resourceType, subject, subjectType)
}

// /////////////////// event service functions ////////////////////////////////////

func (srv *server) getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) subscribe(address, evt string, listener func(evt *eventpb.Event)) error {
	eventClient, err := srv.getEventClient(address)
	if err != nil {
		slog.Error("get event client failed", "addr", address, "err", err)
		return err
	}
	if err := eventClient.Subscribe(evt, srv.Id, listener); err != nil {
		slog.Error("subscribe failed", "event", evt, "id", srv.Id, "err", err)
		return err
	}
	return nil
}

func updateApplication(srv *server, application *resourcepb.Application) func(evt *eventpb.Event) {
	return func(evt *eventpb.Event) {
		descriptor := new(resourcepb.PackageDescriptor)
		err := protojson.Unmarshal(evt.Data, descriptor)

		applicationId := Utility.GenerateUUID(application.PublisherID + "%" + application.Name + "%" + application.Version)
		application.Id = applicationId
		descriptor.Id = application.Id

		if err == nil {
			token, terr := security.GetLocalToken(srv.Mac)
			if terr != nil {
				return
			}

			// uninstall current
			if ierr := srv.uninstallApplication(token, applicationId); ierr == nil {
				repository := descriptor.Repositories[0]
				packageRepository, gerr := GetRepositoryClient(repository)
				if gerr != nil {
					return
				}

				bundle, derr := packageRepository.DownloadBundle(descriptor, "webapp")
				if derr != nil {
					slog.Error("install application: download bundle failed", "app", applicationId, "err", derr)
					return
				}

				r := bytes.NewReader(bundle.Binairies)
				if ierr := srv.installApplication(
					token, srv.Domain, descriptor.Id, descriptor.Name,
					descriptor.PublisherID, descriptor.Version, descriptor.Description,
					descriptor.Icon, descriptor.Alias, r, descriptor.Actions,
					descriptor.Keywords, descriptor.Roles, descriptor.Groups, false,
				); ierr != nil {
					slog.Error("install application failed", "app", applicationId, "err", ierr)
					return
				}
			}
		}
	}
}

// main runs the Application Manager service.
func main() {
	// If you prefer grpc logger, you can set it here:
	// grpclog.SetLoggerV2(grpclog.NewLoggerV2(os.Stdout, os.Stdout, os.Stderr))

	// Initialize service with default values.
	s := new(server)
	s.Name = string(applications_managerpb.File_applications_manager_proto.Services().Get(0).FullName())
	s.Proto = applications_managerpb.File_applications_manager_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Domain, _ = config.GetDomain()
	s.Address, _ = config.GetAddress()
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Application manager service"
	s.Keywords = []string{"Install, Uninstall, Deploy applications"}
	s.Repositories = make([]string, 0)
	s.Discoveries = make([]string, 0)
	s.Dependencies = []string{"discovery.PackageDiscovery", "event.EventService", "resource.ResourceService"}
	s.Permissions = make([]interface{}, 1)
	s.WebRoot = config.GetWebRootDir()
	s.AllowAllOrigins = allow_all_origins
	s.AllowedOrigins = allowed_origins
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true

	// Register the client constructor for dynamic routing
	Utility.RegisterFunction("NewApplicationsManager_Client", applications_manager_client.NewApplicationsManager_Client)

	// ID / optional ConfigPath from args
	if len(os.Args) == 2 {
		s.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s.Id = os.Args[1]
		s.ConfigPath = os.Args[2]
	}

	// Init service (loads/creates config, builds gRPC server)
	if err := s.Init(); err != nil {
		slog.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	if s.Address == "" {
		if addr, err := config.GetAddress(); err == nil {
			s.Address = addr
		}
	}

	// Register service and reflection
	applications_managerpb.RegisterApplicationManagerServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	// Require owner on resource(0) to deploy an application
	s.Permissions[0] = map[string]interface{}{
		"action": "/applications_manager.ApplicationManagerService/DeployApplication",
		"resources": []interface{}{
			map[string]interface{}{"index": 0, "permission": "owner"},
		},
	}

	// Keep applications up-to-date by subscribing to publisher:name events
	go func() {
		resourceClient, err := s.getResourceClient(s.Address)
		if err != nil {
			slog.Warn("resource client not ready; skipping auto-update hook", "err", err)
			return
		}
		apps, err := resourceClient.GetApplications("")
		if err != nil {
			slog.Warn("get applications failed; skipping auto-update hook", "err", err)
			return
		}
		for _, app := range apps {
			evt := app.PublisherID + ":" + app.Name
			_ = s.subscribe(s.Address, evt, updateApplication(s, app))
		}
	}()

	// Start the service (blocking)
	if err := s.StartService(); err != nil {
		slog.Error("service start failed", "service", s.Name, "err", err)
		os.Exit(1)
	}
}
