package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/catalog/catalog_client"
	"github.com/globulario/services/golang/catalog/catalogpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	defaultPort       = 10017
	defaultProxy      = 10018
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// Permissions as JSON to avoid hand-maintaining nested maps.
const permissionsJSON = `[
  {"action":"/catalog.CatalogService/Stop","resources":[],"permission":"write"},
  {"action":"/catalog.CatalogService/CreateConnection","resources":[{"index":0,"field":"Id","permission":"write"}]},
  {"action":"/catalog.CatalogService/DeleteConnection","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/SaveUnitOfMeasure","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SavePropertyDefinition","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SaveItemDefinition","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SaveItemInstance","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SaveInventory","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SaveManufacturer","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SaveSupplier","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SaveLocalisation","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SavePackage","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SavePackageSupplier","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SaveItemManufacturer","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/SaveCategory","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/AppendItemDefinitionCategory","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/RemoveItemDefinitionCategory","resources":[{"index":0,"permission":"write"}]},
  {"action":"/catalog.CatalogService/getSupplier","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getSuppliers","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getManufacturer","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getManufacturers","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getSupplierPackages","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getPackage","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getPackages","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getUnitOfMeasure","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getUnitOfMeasures","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getItemDefinition","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getItemDefinitions","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getItemInstance","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getItemInstances","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getLocalisation","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getLocalisations","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getCategory","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getCategories","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/getInventories","resources":[{"index":0,"permission":"read"}]},
  {"action":"/catalog.CatalogService/deleteInventory","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deletePackage","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deletePackageSupplier","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deleteSupplier","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deletePropertyDefinition","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deleteUnitOfMeasure","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deleteItemInstance","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deleteManufacturer","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deleteItemManufacturer","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deleteCategory","resources":[{"index":0,"permission":"delete"}]},
  {"action":"/catalog.CatalogService/deleteLocalisation","resources":[{"index":0,"permission":"delete"}]}
]`

type server struct {
	Id                 string
	Name               string
	Mac                string
	Port               int
	Proxy              int
	Path               string
	Proto              string
	AllowAllOrigins    bool
	AllowedOrigins     string
	Protocol           string
	Domain             string
	Address            string
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	State              string
	LastError          string
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherID        string
	KeepUpToDate       bool
	KeepAlive          bool
	Checksum           string
	Plaform            string
	ModTime            int64

	Services     map[string]interface{}
	Permissions  []interface{}
	Dependencies []string

	persistenceClient *persistence_client.Persistence_Client
	eventClient       *event_client.Event_Client

	grpcServer *grpc.Server

	persistenceFactory func(address string) (*persistence_client.Persistence_Client, error)
	eventFactory       func(address string) (*event_client.Event_Client, error)
}

// Getters / Setters required by Globular
func (srv *server) GetConfigurationPath() string      { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)  { srv.ConfigPath = path }
func (srv *server) GetAddress() string                { return srv.Address }
func (srv *server) SetAddress(address string)         { srv.Address = address }
func (srv *server) GetProcess() int                   { return srv.Process }
func (srv *server) SetProcess(pid int)                { srv.Process = pid }
func (srv *server) GetProxyProcess() int              { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)           { srv.ProxyProcess = pid }
func (srv *server) GetState() string                  { return srv.State }
func (srv *server) SetState(state string)             { srv.State = state }
func (srv *server) GetLastError() string              { return srv.LastError }
func (srv *server) SetLastError(err string)           { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)          { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                 { return srv.ModTime }
func (srv *server) GetId() string                     { return srv.Id }
func (srv *server) SetId(id string)                   { srv.Id = id }
func (srv *server) GetName() string                   { return srv.Name }
func (srv *server) SetName(name string)               { srv.Name = name }
func (srv *server) GetMac() string                    { return srv.Mac }
func (srv *server) SetMac(mac string)                 { srv.Mac = mac }
func (srv *server) GetChecksum() string               { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)       { srv.Checksum = checksum }
func (srv *server) GetPlatform() string               { return srv.Plaform }
func (srv *server) SetPlatform(platform string)       { srv.Plaform = platform }
func (srv *server) GetDescription() string            { return srv.Description }
func (srv *server) SetDescription(description string) { srv.Description = description }
func (srv *server) GetKeywords() []string             { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)     { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string         { return srv.Repositories }
func (srv *server) SetRepositories(v []string)        { srv.Repositories = v }
func (srv *server) GetDiscoveries() []string          { return srv.Discoveries }
func (srv *server) SetDiscoveries(v []string)         { srv.Discoveries = v }
func (srv *server) Dist(path string) (string, error)  { return globular.Dist(path, srv) }
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
func (srv *server) GetPath() string                 { return srv.Path }
func (srv *server) SetPath(path string)             { srv.Path = path }
func (srv *server) GetProto() string                { return srv.Proto }
func (srv *server) SetProto(proto string)           { srv.Proto = proto }
func (srv *server) GetPort() int                    { return srv.Port }
func (srv *server) SetPort(port int)                { srv.Port = port }
func (srv *server) GetProxy() int                   { return srv.Proxy }
func (srv *server) SetProxy(proxy int)              { srv.Proxy = proxy }
func (srv *server) GetProtocol() string             { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)     { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool        { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)       { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string       { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)      { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string               { return srv.Domain }
func (srv *server) SetDomain(domain string)         { srv.Domain = domain }
func (srv *server) GetTls() bool                    { return srv.TLS }
func (srv *server) SetTls(hasTls bool)              { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string   { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string             { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)     { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string              { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)       { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string              { return srv.Version }
func (srv *server) SetVersion(version string)       { srv.Version = version }
func (srv *server) GetPublisherID() string          { return srv.PublisherID }
func (srv *server) SetPublisherID(v string)         { srv.PublisherID = v }
func (srv *server) GetKeepUpToDate() bool           { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)        { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool              { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)           { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})  { srv.Permissions = p }

func (srv *server) RolesDefault() []resourcepb.Role { /* unchanged content */
	domain, _ := config.GetDomain()

	reader := resourcepb.Role{
		Id:          "role:catalog.reader",
		Name:        "Catalog Reader",
		Domain:      domain,
		Description: "Read catalog data across connections.",
		Actions: []string{
			"/catalog.CatalogService/getSupplier",
			"/catalog.CatalogService/getSuppliers",
			"/catalog.CatalogService/getManufacturer",
			"/catalog.CatalogService/getManufacturers",
			"/catalog.CatalogService/getSupplierPackages",
			"/catalog.CatalogService/getPackage",
			"/catalog.CatalogService/getPackages",
			"/catalog.CatalogService/getUnitOfMeasure",
			"/catalog.CatalogService/getUnitOfMeasures",
			"/catalog.CatalogService/getItemDefinition",
			"/catalog.CatalogService/getItemDefinitions",
			"/catalog.CatalogService/getItemInstance",
			"/catalog.CatalogService/getItemInstances",
			"/catalog.CatalogService/getLocalisation",
			"/catalog.CatalogService/getLocalisations",
			"/catalog.CatalogService/getCategory",
			"/catalog.CatalogService/getCategories",
			"/catalog.CatalogService/getInventories",
		},
		TypeName: "resource.Role",
	}

	editor := resourcepb.Role{
		Id:          "role:catalog.editor",
		Name:        "Catalog Editor",
		Domain:      domain,
		Description: "Create/update catalog entities and manage item-category links.",
		Actions: []string{
			"/catalog.CatalogService/SaveUnitOfMeasure",
			"/catalog.CatalogService/SavePropertyDefinition",
			"/catalog.CatalogService/SaveItemDefinition",
			"/catalog.CatalogService/SaveItemInstance",
			"/catalog.CatalogService/SaveInventory",
			"/catalog.CatalogService/SaveManufacturer",
			"/catalog.CatalogService/SaveSupplier",
			"/catalog.CatalogService/SaveLocalisation",
			"/catalog.CatalogService/SavePackage",
			"/catalog.CatalogService/SavePackageSupplier",
			"/catalog.CatalogService/SaveItemManufacturer",
			"/catalog.CatalogService/SaveCategory",
			"/catalog.CatalogService/AppendItemDefinitionCategory",
			"/catalog.CatalogService/RemoveItemDefinitionCategory",
		},
		TypeName: "resource.Role",
	}

	moderator := resourcepb.Role{
		Id:          "role:catalog.moderator",
		Name:        "Catalog Moderator",
		Domain:      domain,
		Description: "Delete catalog entities.",
		Actions: []string{
			"/catalog.CatalogService/deleteInventory",
			"/catalog.CatalogService/deletePackage",
			"/catalog.CatalogService/deletePackageSupplier",
			"/catalog.CatalogService/deleteSupplier",
			"/catalog.CatalogService/deletePropertyDefinition",
			"/catalog.CatalogService/deleteUnitOfMeasure",
			"/catalog.CatalogService/deleteItemInstance",
			"/catalog.CatalogService/deleteManufacturer",
			"/catalog.CatalogService/deleteItemManufacturer",
			"/catalog.CatalogService/deleteCategory",
			"/catalog.CatalogService/deleteLocalisation",
		},
		TypeName: "resource.Role",
	}

	connAdmin := resourcepb.Role{
		Id:          "role:catalog.connadmin",
		Name:        "Catalog Connection Admin",
		Domain:      domain,
		Description: "Create and remove persistence connections used by the catalog.",
		Actions: []string{
			"/catalog.CatalogService/CreateConnection",
			"/catalog.CatalogService/DeleteConnection",
		},
		TypeName: "resource.Role",
	}

	admin := resourcepb.Role{
		Id:          "role:catalog.admin",
		Name:        "Catalog Admin",
		Domain:      domain,
		Description: "Full control over catalog data and service lifecycle.",
		Actions: append(append(append(reader.Actions, editor.Actions...), moderator.Actions...),
			"/catalog.CatalogService/CreateConnection",
			"/catalog.CatalogService/DeleteConnection",
			"/catalog.CatalogService/Stop",
		),
		TypeName: "resource.Role",
	}

	return []resourcepb.Role{reader, editor, moderator, connAdmin, admin}
}

func defaultPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

func defaultEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}

	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

func (srv *server) Save() error { return globular.SaveService(srv) }

func (srv *server) StartService() error {
	if srv.Services == nil {
		srv.Services = map[string]interface{}{
			"Persistence": map[string]interface{}{"Address": srv.Address},
			"Event":       map[string]interface{}{"Address": srv.Address},
		}
	}

	if srv.persistenceClient == nil {
		if raw, ok := srv.Services["Persistence"]; ok {
			if cfg, cast := raw.(map[string]interface{}); cast {
				if addr, ok := cfg["Address"].(string); ok && strings.TrimSpace(addr) != "" {
					factory := srv.persistenceFactory
					if factory == nil {
						factory = defaultPersistenceClient
					}
					if cli, err := factory(addr); err == nil {
						srv.persistenceClient = cli
					} else {
						logger.Warn("connect persistence failed", "address", addr, "err", err)
					}
				}
			}
		}
	}

	if srv.eventClient == nil {
		if raw, ok := srv.Services["Event"]; ok {
			if cfg, cast := raw.(map[string]interface{}); cast {
				if addr, ok := cfg["Address"].(string); ok && strings.TrimSpace(addr) != "" {
					factory := srv.eventFactory
					if factory == nil {
						factory = defaultEventClient
					}
					if cli, err := factory(addr); err == nil {
						srv.eventClient = cli
					} else {
						logger.Warn("connect event failed", "address", addr, "err", err)
					}
				}
			}
		}
	}

	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }

func initializeServerDefaults() *server {
	s := new(server)
	s.Name = string(catalogpb.File_catalog_proto.Services().Get(0).FullName())
	s.Proto = catalogpb.File_catalog_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Catalog service"
	s.Keywords = []string{}
	s.Repositories = []string{}
	s.Discoveries = []string{}
	s.Dependencies = []string{}
	s.Permissions = loadDefaultPermissions()
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.Services = map[string]interface{}{}

	domain, addr := globular.GetDefaultDomainAddress(s.Port)
	s.Domain = domain
	if host, _, ok := strings.Cut(addr, ":"); ok {
		s.Address = host
	} else {
		s.Address = addr
	}

	return s
}

func loadDefaultPermissions() []interface{} {
	var out []interface{}
	_ = json.Unmarshal([]byte(permissionsJSON), &out)
	return out
}

func setupGrpcService(srv *server) {
	catalogpb.RegisterCatalogServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
}

func printUsage() {
	exe := filepath.Base(os.Args[0])
	os.Stdout.WriteString(`
Usage: ` + exe + ` [options] <id> [configPath]

Options:
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

Arguments:
  <id>            Service instance ID
  [configPath]    Optional path to configuration file

Example:
  ` + exe + ` catalog-1 /etc/globular/catalog/config.json

`)
}

func main() {
	srv := initializeServerDefaults()
	args := os.Args[1:]

	for _, a := range args {
		if strings.ToLower(a) == "--debug" {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			break
		}
	}

	if globular.HandleInformationalFlags(srv, args, logger, printUsage) {
		return
	}

	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	Utility.RegisterFunction("NewCatalogService_Client", catalog_client.NewCatalogService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	if srv.Services == nil || len(srv.Services) == 0 {
		srv.Services = map[string]interface{}{
			"Persistence": map[string]interface{}{"Address": srv.Address},
			"Event":       map[string]interface{}{"Address": srv.Address},
		}
	}

	setupGrpcService(srv)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "err", err)
		os.Exit(1)
	}
}
