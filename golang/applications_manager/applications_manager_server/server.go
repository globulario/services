package main

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/applications_manager/applications_manager_client"
	"github.com/globulario/services/golang/applications_manager/applications_managerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// Defaults
var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// service implementation (values consumed by Globular)
type server struct {
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
	AllowedOrigins  string
	Protocol        string
	Version         string
	PublisherID     string
	KeepUpToDate    bool
	Checksum        string
	Plaform         string
	KeepAlive       bool
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

	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	Permissions  []interface{}
	Dependencies []string

	// AppMgr-specific
	WebRoot string

	grpcServer *grpc.Server
}

// --- Getters/Setters required by Globular (unchanged signatures) ---
func (srv *server) GetAddress() string               { return srv.Address }
func (srv *server) SetAddress(a string)              { srv.Address = a }
func (srv *server) GetProcess() int                  { return srv.Process }
func (srv *server) SetProcess(pid int)               { srv.Process = pid }
func (srv *server) GetProxyProcess() int             { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)          { srv.ProxyProcess = pid }
func (srv *server) GetId() string                    { return srv.Id }
func (srv *server) SetId(id string)                  { srv.Id = id }
func (srv *server) GetName() string                  { return srv.Name }
func (srv *server) SetName(name string)              { srv.Name = name }
func (srv *server) GetMac() string                   { return srv.Mac }
func (srv *server) SetMac(mac string)                { srv.Mac = mac }
func (srv *server) GetChecksum() string              { return srv.Checksum }
func (srv *server) SetChecksum(c string)             { srv.Checksum = c }
func (srv *server) GetPlatform() string              { return srv.Plaform }
func (srv *server) SetPlatform(p string)             { srv.Plaform = p }
func (srv *server) GetState() string                 { return srv.State }
func (srv *server) SetState(st string)               { srv.State = st }
func (srv *server) GetDescription() string           { return srv.Description }
func (srv *server) SetDescription(d string)          { srv.Description = d }
func (srv *server) GetKeywords() []string            { return srv.Keywords }
func (srv *server) SetKeywords(k []string)           { srv.Keywords = k }
func (srv *server) GetRepositories() []string        { return srv.Repositories }
func (srv *server) SetRepositories(r []string)       { srv.Repositories = r }
func (srv *server) GetDiscoveries() []string         { return srv.Discoveries }
func (srv *server) SetDiscoveries(d []string)        { srv.Discoveries = d }
func (srv *server) GetLastError() string             { return srv.LastError }
func (srv *server) SetLastError(e string)            { srv.LastError = e }
func (srv *server) SetModTime(t int64)               { srv.ModTime = t }
func (srv *server) GetModTime() int64                { return srv.ModTime }
func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(p string)    { srv.ConfigPath = p }
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	if !Utility.Contains(srv.Dependencies, dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
	}
}
func (srv *server) GetPath() string                { return srv.Path }
func (srv *server) SetPath(p string)               { srv.Path = p }
func (srv *server) GetProto() string               { return srv.Proto }
func (srv *server) SetProto(p string)              { srv.Proto = p }
func (srv *server) GetPort() int                   { return srv.Port }
func (srv *server) SetPort(port int)               { srv.Port = port }
func (srv *server) GetProxy() int                  { return srv.Proxy }
func (srv *server) SetProxy(px int)                { srv.Proxy = px }
func (srv *server) GetProtocol() string            { return srv.Protocol }
func (srv *server) SetProtocol(p string)           { srv.Protocol = p }
func (srv *server) GetAllowAllOrigins() bool       { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)      { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string      { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)     { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string              { return srv.Domain }
func (srv *server) SetDomain(d string)             { srv.Domain = d }
func (srv *server) GetTls() bool                   { return srv.TLS }
func (srv *server) SetTls(b bool)                  { srv.TLS = b }
func (srv *server) GetCertAuthorityTrust() string  { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(c string) { srv.CertAuthorityTrust = c }
func (srv *server) GetCertFile() string            { return srv.CertFile }
func (srv *server) SetCertFile(c string)           { srv.CertFile = c }
func (srv *server) GetKeyFile() string             { return srv.KeyFile }
func (srv *server) SetKeyFile(k string)            { srv.KeyFile = k }
func (srv *server) GetVersion() string             { return srv.Version }
func (srv *server) SetVersion(v string)            { srv.Version = v }
func (srv *server) GetPublisherID() string         { return srv.PublisherID }
func (srv *server) SetPublisherID(p string)        { srv.PublisherID = p }
func (srv *server) GetKeepUpToDate() bool          { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(b bool)         { srv.KeepUpToDate = b }
func (srv *server) GetKeepAlive() bool             { return srv.KeepAlive }
func (srv *server) SetKeepAlive(b bool)            { srv.KeepAlive = b }
func (srv *server) GetPermissions() []interface{}  { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{}) { srv.Permissions = p }

// Lifecycle
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv) // interceptors wired inside globular
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}
func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }

// RolesDefault returns curated roles for ApplicationManagerService.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:appmgr.installer",
			Name:        "Application Installer",
			Domain:      domain,
			Description: "Install applications into a domain.",
			Actions: []string{
				"/applications_manager.ApplicationManagerService/InstallApplication",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:appmgr.uninstaller",
			Name:        "Application Uninstaller",
			Domain:      domain,
			Description: "Uninstall applications from a domain.",
			Actions: []string{
				"/applications_manager.ApplicationManagerService/UninstallApplication",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:appmgr.admin",
			Name:        "Application Manager Admin",
			Domain:      domain,
			Description: "Full control of application installation and removal.",
			Actions: []string{
				"/applications_manager.ApplicationManagerService/InstallApplication",
				"/applications_manager.ApplicationManagerService/UninstallApplication",
			},
			TypeName: "resource.Role",
		},
	}
}

// --- logger to STDERR so stdout stays clean for JSON outputs ---
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// ////////////////////// Resource manager helpers //////////////////////
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

// ////////////////////// RBAC helpers //////////////////////
func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(token, path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbacClient, err := srv.GetRbacClient()
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(token, path, subject, resourceType, subjectType)
}

// ////////////////////// Event helpers //////////////////////
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
		logger.Error("get event client failed", "addr", address, "err", err)
		return err
	}
	if err := eventClient.Subscribe(evt, srv.Id, listener); err != nil {
		logger.Error("subscribe failed", "event", evt, "id", srv.Id, "err", err)
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
					logger.Error("install application: download bundle failed", "app", applicationId, "err", derr)
					return
				}

				r := bytes.NewReader(bundle.Binairies)
				if ierr := srv.installApplication(
					token, srv.Domain, descriptor.Id, descriptor.Name,
					descriptor.PublisherID, descriptor.Version, descriptor.Description,
					descriptor.Icon, descriptor.Alias, r, descriptor.Actions,
					descriptor.Keywords, descriptor.Roles, descriptor.Groups, false,
				); ierr != nil {
					logger.Error("install application failed", "app", applicationId, "err", ierr)
					return
				}
			}
		}
	}
}

// --- logger-backed usage text ---
func printUsage() {
	fmt.Fprintf(os.Stdout, `
Usage: %s [options] <id> [configPath]

Options:
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

Arguments:
  <id>            Service instance ID
  [configPath]    Optional path to configuration file

Example:
  %s appmgr-1 /etc/globular/appmgr/config.json

`, filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
}

// main runs the Applications Manager service with --describe/--health short-circuiting.
func main() {
	// Initialize service skeleton (no etcd access yet)
	s := new(server)
	s.Name = string(applications_managerpb.File_applications_manager_proto.Services().Get(0).FullName())
	s.Proto = applications_managerpb.File_applications_manager_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Application manager service"
	s.Keywords = []string{"Install", "Uninstall", "Deploy", "applications"}
	s.Repositories = []string{}
	s.Discoveries = []string{}
	s.Dependencies = []string{"discovery.PackageDiscovery", "event.EventService", "resource.ResourceService"}

	// Default permissions for ApplicationManagerService (generic verbs).
	s.Permissions = []interface{}{
		// --- Install
		map[string]interface{}{
			"action":     "/applications_manager.ApplicationManagerService/InstallApplication",
			"permission": "write", // action itself is sensitive
			"resources": []interface{}{
				// InstallApplicationRequest
				// discoveryId (index 0) is infra; not treated as a protected resource
				map[string]interface{}{"index": 1, "permission": "write"}, // applicationId
				map[string]interface{}{"index": 2, "permission": "write"}, // PublisherID
				map[string]interface{}{"index": 3, "permission": "write"}, // version
				map[string]interface{}{"index": 4, "permission": "write"}, // domain
				// set_as_default (index 5) is a flag; not a resource
			},
		},

		// --- Uninstall
		map[string]interface{}{
			"action":     "/applications_manager.ApplicationManagerService/UninstallApplication",
			"permission": "delete", // uninstall is inherently destructive
			"resources": []interface{}{
				// UninstallApplicationRequest
				map[string]interface{}{"index": 0, "permission": "delete"}, // applicationId
				map[string]interface{}{"index": 1, "permission": "delete"}, // PublisherID
				map[string]interface{}{"index": 2, "permission": "delete"}, // version
				map[string]interface{}{"index": 4, "permission": "delete"}, // domain
				// deletePermissions (index 3) is a flag; not a resource
			},
		},
	}

	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr

	// ---- CLI flags handled BEFORE any call that might touch etcd ----
	args := os.Args[1:]

	if len(args) == 0 {
		s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			fmt.Println("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(s.Id)
		if err != nil {
			fmt.Println("fail to allocate port", "error", err)
			os.Exit(1)
		}
		s.Port = p
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// runtime snapshot without etcd
			s.Process = os.Getpid()
			s.State = "starting"

			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
				s.Domain = strings.ToLower(v)
			} else {
				s.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
				s.Address = strings.ToLower(v)
			} else {
				s.Address = "localhost:" + Utility.ToString(s.Port)
			}

			b, err := globular.DescribeJSON(s)
			if err != nil {
				logger.Error("describe error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if s.Port == 0 || s.Name == "" {
				logger.Error("health error: uninitialized", "service", s.Name, "port", s.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(s, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
			if err != nil {
				logger.Error("health error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))
			slog.SetDefault(logger)
		case "--version", "-v":
			fmt.Fprintf(os.Stdout, "%s\n", s.Version)
			return
		}
	}

	// Optional positional args: <id> [configPath]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		s.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		s.Id = args[0]
		s.ConfigPath = args[1]
	}

	// Safe to touch config now
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
	} else {
		s.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
	}
	// AppMgr-specific
	s.WebRoot = config.GetWebRootDir()

	// Register dynamic client ctor
	Utility.RegisterFunction("NewApplicationsManager_Client", applications_manager_client.NewApplicationsManager_Client)

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	applications_managerpb.RegisterApplicationManagerServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	// Auto-update hook
	go func() {
		resourceClient, err := s.getResourceClient(s.Address)
		if err != nil {
			logger.Warn("resource client not ready; skipping auto-update hook", "err", err)
			return
		}
		apps, err := resourceClient.GetApplications("")
		if err != nil {
			logger.Warn("get applications failed; skipping auto-update hook", "err", err)
			return
		}
		for _, app := range apps {
			evt := app.PublisherID + ":" + app.Name
			_ = s.subscribe(s.Address, evt, updateApplication(s, app))
		}
	}()

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := s.StartService(); err != nil {
		logger.Error("service start failed", "service", s.Name, "err", err)
		os.Exit(1)
	}
}
