package main

import (
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

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort       = 10017
	defaultProxy      = 10018
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// --- logger to STDERR so stdout stays clean for JSON outputs ---
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// -----------------------------------------------------------------------------
// Service implementation
// -----------------------------------------------------------------------------

// server implements the Catalog gRPC microservice and the Globular runtime interface.
type server struct {
	// Generic service attributes required by Globular runtime.
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

	// Service configuration and dependencies.
	Services     map[string]interface{}
	Permissions  []interface{}
	Dependencies []string

	// External clients.
	persistenceClient *persistence_client.Persistence_Client
	eventClient       *event_client.Event_Client

	// Runtime component.
	grpcServer *grpc.Server
}

// --- Globular getters/setters (unchanged signatures) ---

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

func (srv *server) RolesDefault() []resourcepb.Role {
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

// -----------------------------------------------------------------------------
// Clients & Init
// -----------------------------------------------------------------------------

func getPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}
func getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) Init() error {
	// Initialize config (no interceptors args here—wired internally like your auth template).
	if err := globular.InitService(srv); err != nil {
		return err
	}

	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs

	// Optional dependency wiring from Services map (if provided by config).
	var addr string
	var ok bool

	if srv.Services != nil {
		if raw, found := srv.Services["Persistence"]; found {
			if cfg, cast := raw.(map[string]interface{}); cast {
				if addr, ok = cfg["Address"].(string); ok && strings.TrimSpace(addr) != "" {
					if cli, e := getPersistenceClient(addr); e == nil {
						srv.persistenceClient = cli
					} else {
						logger.Warn("connect persistence failed", "address", addr, "err", e)
					}
				}
			}
		}
		if raw, found := srv.Services["Event"]; found {
			if cfg, cast := raw.(map[string]interface{}); cast {
				if addr, ok = cfg["Address"].(string); ok && strings.TrimSpace(addr) != "" {
					if cli, e := getEventClient(addr); e == nil {
						srv.eventClient = cli
					} else {
						logger.Warn("connect event failed", "address", addr, "err", e)
					}
				}
			}
		}
	}

	return nil
}

func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }

// -----------------------------------------------------------------------------
// Usage
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Entrypoint
// -----------------------------------------------------------------------------

func main() {
	// Build a skeleton service (no etcd/config yet)
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
	// Default RBAC permissions for CatalogService.
	// We bind to concrete resource fields only when they truly represent
	// an access-controlled resource. Here, the primary protected resource
	// is the target persistence connection (connectionId). For connection
	// management, we bind to Connection.Id. For harmful, parameterless ops,
	// we protect the action itself.
	s.Permissions = []interface{}{

		// ---- Service control
		map[string]interface{}{
			"action":     "/catalog.CatalogService/Stop",
			"resources":  []interface{}{}, // harmful even without params
			"permission": "write",
		},

		// ---- Connections
		map[string]interface{}{
			"action": "/catalog.CatalogService/CreateConnection",
			"resources": []interface{}{
				// CreateConnectionRqst.connection.Id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/catalog.CatalogService/DeleteConnection",
			"resources": []interface{}{
				// DeleteConnectionRqst.id
				map[string]interface{}{"index": 0, "permission": "delete"},
			},
		},

		// ---- Saves / upserts (write on connectionId)
		map[string]interface{}{"action": "/catalog.CatalogService/SaveUnitOfMeasure", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SavePropertyDefinition", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SaveItemDefinition", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SaveItemInstance", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SaveInventory", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SaveManufacturer", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SaveSupplier", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SaveLocalisation", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SavePackage", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SavePackageSupplier", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SaveItemManufacturer", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/SaveCategory", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},

		// Category links (write on connectionId)
		map[string]interface{}{"action": "/catalog.CatalogService/AppendItemDefinitionCategory", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/RemoveItemDefinitionCategory", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},

		// ---- Getters (READ on connectionId)
		map[string]interface{}{"action": "/catalog.CatalogService/getSupplier", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getSuppliers", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getManufacturer", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getManufacturers", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getSupplierPackages", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getPackage", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getPackages", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getUnitOfMeasure", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getUnitOfMeasures", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getItemDefinition", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getItemDefinitions", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getItemInstance", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getItemInstances", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getLocalisation", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getLocalisations", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getCategory", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getCategories", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/getInventories", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "read"}}},

		// ---- Deletes (DELETE on connectionId)
		map[string]interface{}{"action": "/catalog.CatalogService/deleteInventory", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deletePackage", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deletePackageSupplier", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deleteSupplier", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deletePropertyDefinition", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deleteUnitOfMeasure", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deleteItemInstance", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deleteManufacturer", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deleteItemManufacturer", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deleteCategory", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/catalog.CatalogService/deleteLocalisation", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
	}

	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr

	// Register client ctor for dynamic routing
	Utility.RegisterFunction("NewCatalogService_Client", catalog_client.NewCatalogService_Client)

	// CLI flags BEFORE touching config
	args := os.Args[1:]
	if len(args) == 0 {
		s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			logger.Error("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(s.Id)
		if err != nil {
			logger.Error("fail to allocate port", "error", err)
			os.Exit(1)
		}
		s.Port = p
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
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
						if s.Id == "" {
				s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
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
		case "--help", "-h", "/h", "/help":
			printUsage()
			return
		case "--version", "-v", "/v", "/version":
			os.Stdout.WriteString(s.Version + "\n")
			return
			case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		default:
			// skip unknown flags for now (e.g. positional args)
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

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// Default dependencies set to local address if not provided by config
	if s.Services == nil {
		s.Services = map[string]interface{}{
			"Persistence": map[string]interface{}{"Address": s.Address},
			"Event":       map[string]interface{}{"Address": s.Address},
		}
	}

	// Bind again now that Services likely loaded from config during Init
	// (if Init read a config file that overrides Services).
	if s.persistenceClient == nil || s.eventClient == nil {
		if raw, ok := s.Services["Persistence"]; ok {
			if cfg, cast := raw.(map[string]interface{}); cast {
				if addr, ok := cfg["Address"].(string); ok && strings.TrimSpace(addr) != "" {
					if cli, e := getPersistenceClient(addr); e == nil {
						s.persistenceClient = cli
					}
				}
			}
		}
		if raw, ok := s.Services["Event"]; ok {
			if cfg, cast := raw.(map[string]interface{}); cast {
				if addr, ok := cfg["Address"].(string); ok && strings.TrimSpace(addr) != "" {
					if cli, e := getEventClient(addr); e == nil {
						s.eventClient = cli
					}
				}
			}
		}
	}

	// gRPC registration
	catalogpb.RegisterCatalogServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := s.StartService(); err != nil {
		logger.Error("service start failed", "err", err)
		os.Exit(1)
	}
}
