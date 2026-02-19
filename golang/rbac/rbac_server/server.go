// Package main provides the RBAC gRPC service wired for Globular.
// It mirrors the clean structure and CLI ergonomics of the Echo example,
// adds --describe and --health, uses slog for logging, and clarifies errors.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"github.com/minio/minio-go/v7"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// -----------------------------------------------------------------------------
// Server type (Globular contract + RBAC runtime)
// -----------------------------------------------------------------------------

type server struct {
	// Embed UnimplementedRbacServiceServer for forward-compatible gRPC registration.
	rbacpb.UnimplementedRbacServiceServer

	// Core metadata
	Id           string
	Mac          string
	Name         string
	Domain       string
	Address      string
	Path         string
	Proto        string
	Port         int
	Proxy        int
	Protocol     string
	Version      string
	PublisherID  string
	Description  string
	Keywords     []string
	Repositories []string
	Discoveries  []string

	// Policy / ops
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Permissions     []interface{}
	Dependencies    []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64
	CacheAddress    string

	// TLS
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// Runtime
	grpcServer *grpc.Server

	// Local KV/cache backends used by RBAC
	cache       *storage_store.BigCache_store
	permissions storage_store.Store

	MinioConfig *config.MinioProxyConfig

	minioClient *minio.Client
}

// -----------------------------------------------------------------------------
// Key/Value helpers for RBAC state
// -----------------------------------------------------------------------------

func (srv *server) setItem(key string, val []byte) error {
	if err := srv.cache.SetItem(key, val); err != nil {
		return err
	}
	return srv.permissions.SetItem(key, val)
}

func (srv *server) getItem(key string) ([]byte, error) {
	if val, err := srv.cache.GetItem(key); err == nil {
		return val, nil
	}
	return srv.permissions.GetItem(key)
}

func (srv *server) removeItem(key string) error {
	if err := srv.cache.RemoveItem(key); err != nil {
		return err
	}
	return srv.permissions.RemoveItem(key)
}

// -----------------------------------------------------------------------------
// Globular service contract (getters / setters)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string          { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)      { srv.ConfigPath = path }
func (srv *server) GetAddress() string                    { return srv.Address }
func (srv *server) SetAddress(address string)             { srv.Address = address }
func (srv *server) GetProcess() int                       { return srv.Process }
func (srv *server) SetProcess(pid int)                    { srv.Process = pid }
func (srv *server) GetProxyProcess() int                  { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)               { srv.ProxyProcess = pid }
func (srv *server) GetState() string                      { return srv.State }
func (srv *server) SetState(state string)                 { srv.State = state }
func (srv *server) GetLastError() string                  { return srv.LastError }
func (srv *server) SetLastError(err string)               { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)              { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                     { return srv.ModTime }
func (srv *server) GetId() string                         { return srv.Id }
func (srv *server) SetId(id string)                       { srv.Id = id }
func (srv *server) GetName() string                       { return srv.Name }
func (srv *server) SetName(name string)                   { srv.Name = name }
func (srv *server) GetMac() string                        { return srv.Mac }
func (srv *server) SetMac(mac string)                     { srv.Mac = mac }
func (srv *server) GetDescription() string                { return srv.Description }
func (srv *server) SetDescription(description string)     { srv.Description = description }
func (srv *server) GetKeywords() []string                 { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)         { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string             { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string              { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)   { srv.Discoveries = discoveries }
func (srv *server) Dist(path string) (string, error)      { return globular.Dist(path, srv) }
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
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}
func (srv *server) GetChecksum() string             { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)     { srv.Checksum = checksum }
func (srv *server) GetPlatform() string             { return srv.Plaform }
func (srv *server) SetPlatform(platform string)     { srv.Plaform = platform }
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
func (srv *server) SetPublisherID(id string)        { srv.PublisherID = id }
func (srv *server) GetKeepUpToDate() bool           { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)        { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool              { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)           { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})  { srv.Permissions = p }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:rbac.viewer",
			Name:        "RBAC Viewer",
			Domain:      domain,
			Description: "Read-only access to permissions metadata and validation endpoints.",
			Actions: []string{
				"/rbac.RbacService/GetResourcePermission",
				"/rbac.RbacService/GetResourcePermissions",
				"/rbac.RbacService/GetResourcePermissionsByResourceType",
				"/rbac.RbacService/GetResourcePermissionsBySubject",
				"/rbac.RbacService/GetActionResourceInfos",
				"/rbac.RbacService/GetSharedResource",
				"/rbac.RbacService/ValidateAccess",
				"/rbac.RbacService/ValidateAction",
				"/rbac.RbacService/GetSubjectAllocatedSpace",
				"/rbac.RbacService/GetSubjectAvailableSpace",
				"/rbac.RbacService/ValidateSubjectSpace",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:rbac.editor",
			Name:        "RBAC Editor",
			Domain:      domain,
			Description: "Manage resource permissions, owners, and shares.",
			Actions: []string{
				"/rbac.RbacService/SetResourcePermission",
				"/rbac.RbacService/DeleteResourcePermission",
				"/rbac.RbacService/SetResourcePermissions",
				"/rbac.RbacService/DeleteResourcePermissions",
				"/rbac.RbacService/AddResourceOwner",
				"/rbac.RbacService/RemoveResourceOwner",
				"/rbac.RbacService/RemoveSubjectFromShare",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:rbac.admin",
			Name:        "RBAC Admin",
			Domain:      domain,
			Description: "Full control over RBAC configuration and subject quotas.",
			Actions: []string{
				// everything from viewer
				"/rbac.RbacService/GetResourcePermission",
				"/rbac.RbacService/GetResourcePermissions",
				"/rbac.RbacService/GetResourcePermissionsByResourceType",
				"/rbac.RbacService/GetResourcePermissionsBySubject",
				"/rbac.RbacService/GetActionResourceInfos",
				"/rbac.RbacService/GetSharedResource",
				"/rbac.RbacService/ValidateAccess",
				"/rbac.RbacService/ValidateAction",
				"/rbac.RbacService/GetSubjectAllocatedSpace",
				"/rbac.RbacService/GetSubjectAvailableSpace",
				"/rbac.RbacService/ValidateSubjectSpace",

				// everything from editor
				"/rbac.RbacService/SetResourcePermission",
				"/rbac.RbacService/DeleteResourcePermission",
				"/rbac.RbacService/SetResourcePermissions",
				"/rbac.RbacService/DeleteResourcePermissions",
				"/rbac.RbacService/AddResourceOwner",
				"/rbac.RbacService/RemoveResourceOwner",
				"/rbac.RbacService/RemoveSubjectFromShare",

				// admin-only knobs
				"/rbac.RbacService/SetActionResourcesPermissions",
				"/rbac.RbacService/DeleteAllAccess",
				"/rbac.RbacService/DeleteSubjectShare",
				"/rbac.RbacService/SetSubjectAllocatedSpace",
			},
			TypeName: "resource.Role",
		},
		// Phase 3: Global admin role with wildcard permissions
		// Replaces hardcoded "sa" bypass - grants full system access via RBAC
		{
			Id:          "role:globular.admin",
			Name:        "Globular Administrator",
			Domain:      domain,
			Description: "Full system administrator with unrestricted access to all services and methods. Required for system maintenance and Day-0 setup.",
			Actions: []string{
				"/*", // Wildcard: grants access to ALL methods across ALL services
			},
			TypeName: "resource.Role",
		},
	}
}

// -----------------------------------------------------------------------------
// Event / Log / Resource helpers
// -----------------------------------------------------------------------------

func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*event_client.Event_Client), nil
}

func (srv *server) publish(event string, data []byte) error {
	ec, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return ec.Publish(event, data)
}

func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	c, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*resource_client.Resource_Client), nil
}

// account/group/app/node identity/org lookup helpers (cache first)

func (srv *server) getAccount(accountId string) (*resourcepb.Account, error) {
	if data, err := srv.cache.GetItem(accountId); err == nil {
		acc := new(resourcepb.Account)
		if err := protojson.Unmarshal(data, acc); err == nil {
			return acc, nil
		}
	}
	domain := srv.Domain
	if strings.Contains(accountId, "@") {
		parts := strings.Split(accountId, "@")
		if len(parts) == 2 && parts[1] != "" {
			domain = parts[1]
		}
		accountId = parts[0]
	}
	rc, err := srv.getResourceClient(domain)
	if err != nil {
		return nil, err
	}
	acc, err := rc.GetAccount(accountId)
	if err != nil {
		return nil, err
	}
	if b, err := protojson.Marshal(acc); err == nil {
		_ = srv.cache.SetItem(accountId, b)
	}
	return acc, nil
}

func (srv *server) accountExist(id string) (bool, string) {
	acc, err := srv.getAccount(id)
	if err != nil || acc == nil {
		return false, ""
	}
	return true, acc.Id + "@" + acc.Domain
}

func (srv *server) getGroup(groupId string) (*resourcepb.Group, error) {
	if data, err := srv.cache.GetItem(groupId); err == nil {
		g := new(resourcepb.Group)
		if err := protojson.Unmarshal(data, g); err == nil {
			return g, nil
		}
	}
	domain := srv.Domain
	if strings.Contains(groupId, "@") {
		parts := strings.Split(groupId, "@")
		if len(parts) == 2 && parts[1] != "" {
			domain = parts[1]
		}
		groupId = parts[0]
	}
	rc, err := srv.getResourceClient(domain)
	if err != nil {
		return nil, err
	}
	gs, err := rc.GetGroups(`{"_id":"` + groupId + `"}`)
	if err != nil {
		return nil, err
	}
	if len(gs) == 0 {
		return nil, errors.New("group not found: " + groupId)
	}
	if b, err := protojson.Marshal(gs[0]); err == nil {
		_ = srv.cache.SetItem(groupId, b)
	}
	return gs[0], nil
}

func (srv *server) groupExist(id string) (bool, string) {
	g, err := srv.getGroup(id)
	if err != nil || g == nil {
		return false, ""
	}
	return true, g.Id + "@" + g.Domain
}

func (srv *server) getApplication(applicationId string) (*resourcepb.Application, error) {
	domain := srv.Domain
	if strings.Contains(applicationId, "@") {
		parts := strings.Split(applicationId, "@")
		if len(parts) == 2 && parts[1] != "" {
			domain = parts[1]
		}
		applicationId = parts[0]
	}
	q0 := `{"_id":"` + applicationId + `"}`
	q1 := `{"name":"` + applicationId + `"}`
	rc, err := srv.getResourceClient(domain)
	if err != nil {
		return nil, err
	}
	apps, err := rc.GetApplications(q0)
	if err != nil || len(apps) == 0 {
		apps, err = rc.GetApplications(q1)
	}
	if err != nil {
		return nil, err
	}
	if len(apps) == 0 {
		return nil, errors.New("application not found: " + applicationId)
	}
	return apps[0], nil
}

func (srv *server) applicationExist(id string) (bool, string) {
	app, err := srv.getApplication(id)
	if err != nil || app == nil {
		return false, ""
	}
	return true, app.Id + "@" + app.Domain
}

func (srv *server) getNodeIdentityByMac(mac string) (*resourcepb.NodeIdentity, error) {
	addr, _ := config.GetAddress()
	rc, err := srv.getResourceClient(addr)
	if err != nil {
		return nil, err
	}
	nodes, err := rc.ListNodeIdentities(fmt.Sprintf(`{"mac":"%s"}`, mac), "")
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, errors.New("node identity not found: " + mac)
	}
	return nodes[0], nil
}

func (srv *server) nodeIdentityExists(mac string) bool {
	p, err := srv.getNodeIdentityByMac(mac)
	return err == nil && p != nil
}

func (srv *server) getOrganization(organizationId string) (*resourcepb.Organization, error) {
	domain := srv.Domain
	if strings.Contains(organizationId, "@") {
		parts := strings.Split(organizationId, "@")
		if len(parts) == 2 && parts[1] != "" {
			domain = parts[1]
		}
		organizationId = parts[0]
	}
	rc, err := srv.getResourceClient(domain)
	if err != nil {
		return nil, err
	}
	orgs, err := rc.GetOrganizations(`{"_id":"` + organizationId + `"}`)
	if err != nil {
		return nil, err
	}
	if len(orgs) == 0 {
		return nil, errors.New("organization not found: " + organizationId)
	}
	return orgs[0], nil
}

func (srv *server) organizationExist(id string) (bool, string) {
	o, err := srv.getOrganization(id)
	if err != nil || o == nil {
		return false, ""
	}
	return true, o.Id + "@" + o.Domain
}

func (srv *server) getRoles() ([]*resourcepb.Role, error) {
	rc, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}
	rs, err := rc.GetRoles("")
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (srv *server) getGroups() ([]*resourcepb.Group, error) {
	rc, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}
	gs, err := rc.GetGroups(`{}`)
	if err != nil {
		return nil, err
	}
	return gs, nil
}

func (srv *server) getOrganizations() ([]*resourcepb.Organization, error) {
	rc, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}
	os_, err := rc.GetOrganizations("")
	if err != nil {
		return nil, err
	}
	return os_, nil
}

func (srv *server) getRole(roleId string) (*resourcepb.Role, error) {
	domain := srv.Domain
	if strings.Contains(roleId, "@") {
		parts := strings.Split(roleId, "@")
		if len(parts) == 2 && parts[1] != "" {
			domain = parts[1]
		}
		roleId = parts[0]
	}
	rc, err := srv.getResourceClient(domain)
	if err != nil {
		return nil, err
	}
	rs, err := rc.GetRoles(`{"_id":"` + roleId + `"}`)
	if err != nil {
		return nil, err
	}
	if len(rs) == 0 {
		return nil, errors.New("role not found: " + roleId)
	}
	return rs[0], nil
}

func (srv *server) roleExist(id string) (bool, string) {
	r, err := srv.getRole(id)
	if err != nil || r == nil {
		return false, ""
	}
	return true, r.Id + "@" + r.Domain
}

// listAllPermissionPaths returns every resource path that currently has a Permissions record.
// Implement this using your storage layer (Scylla/etcd/etc.) and the same key prefix your
// getResourcePermissions/setResourcePermissions helpers use.
func (srv *server) listAllPermissionPaths() ([]string, error) {
	// Example sketch — replace with your actual store scan:
	// return srv.store.ListKeys(srv.permissionsKeyPrefix)
	return srv.scanPermissionKeys() // or whatever exists in your codebase
}

func (srv *server) scanPermissionKeys() ([]string, error) {
	// Scan all keys in the permissions store that match the permissions key prefix.
	// Assuming the permissions store supports a ListKeys or similar method.
	const permissionsPrefix = "PERMISSIONS_"
	keys, err := srv.permissions.GetAllKeys()
	if err != nil {
		return nil, err
	}

	// Remove the prefix from each key to return only the resource path part.
	var paths []string
	for _, key := range keys {
		if strings.HasPrefix(key, permissionsPrefix) {
			paths = append(paths, strings.TrimPrefix(key, permissionsPrefix))
		}
	}
	return paths, nil
}

// -----------------------------------------------------------------------------
// Lifecycle (Init/Save/Start/Stop) and gRPC plumbing
// -----------------------------------------------------------------------------

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

func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }

/*
// Optional administrative RPCs (example): Stop
func (srv *server) Stop(ctx context.Context, _ *rbacpb.StopRqst) (*rbacpb.StopResponse, error) {
	return &rbacpb.StopResponse{}, srv.StopService()
}*/

func (srv *server) loadMinioConfig() *config.MinioProxyConfig {
	if cfg, err := config.GetServiceConfigurationById(srv.Id); err == nil && cfg != nil {
		if minioRaw, ok := cfg["MinioConfig"]; ok {
			if minioMap, ok := minioRaw.(map[string]interface{}); ok {
				return parseMinioConfigFromMap(minioMap)
			}
		}
	}

	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		return nil
	}

	return &config.MinioProxyConfig{
		Endpoint: endpoint,
		Bucket:   getEnvOrDefault("MINIO_BUCKET", "globular"),
		Prefix:   getEnvOrDefault("MINIO_PREFIX", "/users"),
		Secure:   getEnvOrDefault("MINIO_USE_SSL", "false") == "true",
		Auth: &config.MinioProxyAuth{
			Mode:      config.MinioProxyAuthModeAccessKey,
			AccessKey: os.Getenv("MINIO_ACCESS_KEY"),
			SecretKey: os.Getenv("MINIO_SECRET_KEY"),
		},
	}
}

func parseMinioConfigFromMap(m map[string]interface{}) *config.MinioProxyConfig {
	cfg := &config.MinioProxyConfig{}

	if v, ok := m["endpoint"].(string); ok {
		cfg.Endpoint = v
	}
	if v, ok := m["bucket"].(string); ok {
		cfg.Bucket = v
	}
	if v, ok := m["prefix"].(string); ok {
		cfg.Prefix = v
	}
	if v, ok := m["secure"].(bool); ok {
		cfg.Secure = v
	}
	if v, ok := m["caBundlePath"].(string); ok {
		cfg.CABundlePath = v
	}

	if authRaw, ok := m["auth"].(map[string]interface{}); ok {
		cfg.Auth = &config.MinioProxyAuth{}
		if mode, ok := authRaw["mode"].(string); ok {
			cfg.Auth.Mode = mode
		}
		if ak, ok := authRaw["accessKey"].(string); ok {
			cfg.Auth.AccessKey = ak
		}
		if sk, ok := authRaw["secretKey"].(string); ok {
			cfg.Auth.SecretKey = sk
		}
		if cf, ok := authRaw["credFile"].(string); ok {
			cfg.Auth.CredFile = cf
		}
	}

	return cfg
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// -----------------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------------

// STDERR logger (keeps STDOUT clean for --describe/--health)
// Note: Can be reconfigured for debug level via --debug flag in main()
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

func main() {
	// Define CLI flags (BEFORE any arg parsing)
	var (
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
	)

	flag.Usage = printUsage
	flag.Parse()

	// Handle --debug flag (reconfigure logger level)
	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		logger.Debug("debug logging enabled")
	}

	// Handle informational flags that exit early
	if *showHelp {
		printUsage()
		os.Exit(0)
	}

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	// Initialize service skeleton
	srv := new(server)

	// Fill metadata that doesn't require etcd/config yet.
	srv.Name = string(rbacpb.File_rbac_proto.Services().Get(0).FullName())
	srv.Proto = rbacpb.File_rbac_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = Version // Use build-time version
	srv.PublisherID = "localhost"
	srv.Description = "RBAC service managing role-based access control and permissions"
	srv.Keywords = []string{"rbac", "permissions", "security", "access-control", "authorization"}
	srv.Repositories, srv.Discoveries = make([]string, 0), make([]string, 0)
	srv.Dependencies = []string{"resource.ResourceService"}
	// In your service init/constructor:
	srv.Permissions = []interface{}{
		// ---- Resource permission CRUD (protected by resource path)
		map[string]interface{}{
			"action":     "/rbac.RbacService/SetResourcePermissions",
			"permission": "write",
			"resources": []interface{}{
				// SetResourcePermissionsRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
				// Optional duplicated path inside rqst.permissions.path
				map[string]interface{}{"index": 0, "field": "Permissions.Path", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/DeleteResourcePermissions",
			"permission": "write",
			"resources": []interface{}{
				// DeleteResourcePermissionsRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/DeleteResourcePermission",
			"permission": "write",
			"resources": []interface{}{
				// DeleteResourcePermissionRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/GetResourcePermission",
			"permission": "read",
			"resources": []interface{}{
				// GetResourcePermissionRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/SetResourcePermission",
			"permission": "write",
			"resources": []interface{}{
				// SetResourcePermissionRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/GetResourcePermissions",
			"permission": "read",
			"resources": []interface{}{
				// GetResourcePermissionsRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "read"},
			},
		},

		// ---- Ownership management (path-scoped)
		map[string]interface{}{
			"action":     "/rbac.RbacService/AddResourceOwner",
			"permission": "write",
			"resources": []interface{}{
				// AddResourceOwnerRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/RemoveResourceOwner",
			"permission": "write",
			"resources": []interface{}{
				// RemoveResourceOwnerRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- Sharing (path-scoped mutation)
		map[string]interface{}{
			"action":     "/rbac.RbacService/RemoveSubjectFromShare",
			"permission": "write",
			"resources": []interface{}{
				// RemoveSubjectFromShareRqst.path
				map[string]interface{}{"index": 0, "field": "Path", "permission": "write"},
			},
		},

		// ---- High-privilege/global configuration (no per-parameter resource)
		map[string]interface{}{
			"action":     "/rbac.RbacService/SetActionResourcesPermissions",
			"permission": "admin",
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/DeleteAllAccess",
			"permission": "admin",
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/DeleteSubjectShare",
			"permission": "admin",
		},
		map[string]interface{}{
			"action":     "/rbac.RbacService/SetSubjectAllocatedSpace",
			"permission": "admin",
		},

		// ---- Reads that don’t mutate resources (kept unrestricted here)
		// GetResourcePermissionsByResourceType, GetResourcePermissionsBySubject,
		// GetActionResourceInfos, GetSharedResource, ValidateAccess, ValidateAction,
		// GetSubjectAllocatedSpace, GetSubjectAvailableSpace, ValidateSubjectSpace
		// can be left out so the RBAC layer doesn’t block read/validation calls.
	}

	srv.Process, srv.ProxyProcess = -1, -1
	srv.AllowAllOrigins, srv.AllowedOrigins = allowAllOrigins, allowedOriginsStr
	srv.KeepAlive, srv.KeepUpToDate = true, true
	srv.CacheAddress = srv.Address

	// Register RBAC client ctor for other components if needed.
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)

	// Handle --describe flag (requires minimal service setup, no config access)
	if *showDescribe {
		// Best-effort runtime fields without hitting etcd
		srv.Process = os.Getpid()
		srv.State = "starting"

		// Prefer env if present; otherwise harmless defaults
		if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
			srv.Domain = strings.ToLower(v)
		} else {
			srv.Domain = "localhost"
		}
		if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
			srv.Address = strings.ToLower(v)
		} else {
			srv.Address = "localhost:" + Utility.ToString(srv.Port)
		}

		b, err := globular.DescribeJSON(srv)
		if err != nil {
			logger.Error("describe failed", "service", srv.Name, "id", srv.Id, "err", err)
			os.Exit(2)
		}
		_, _ = os.Stdout.Write(b)
		_, _ = os.Stdout.Write([]byte("\n"))
		os.Exit(0)
	}

	// Handle --health flag (requires minimal service setup, no config access)
	if *showHealth {
		if srv.Port == 0 || srv.Name == "" {
			logger.Error("health: missing required fields", "service", srv.Name, "port", srv.Port)
			os.Exit(2)
		}
		b, err := globular.HealthJSON(srv, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
		if err != nil {
			logger.Error("health probe failed", "service", srv.Name, "id", srv.Id, "err", err)
			os.Exit(2)
		}
		_, _ = os.Stdout.Write(b)
		_, _ = os.Stdout.Write([]byte("\n"))
		os.Exit(0)
	}

	// Parse positional arguments: [<id> [configPath]]
	args := flag.Args()
	if len(args) == 0 {
		// No args: auto-generate ID and allocate port
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			logger.Error("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(srv.Id)
		if err != nil {
			logger.Error("fail to allocate port", "error", err)
			os.Exit(1)
		}
		srv.Port = p
		logger.Debug("auto-allocated service", "id", srv.Id, "port", srv.Port)
	} else if len(args) == 1 {
		// One arg: service ID
		srv.Id = args[0]
		logger.Debug("using provided service id", "id", srv.Id)
	} else if len(args) >= 2 {
		// Two+ args: service ID and config path
		srv.Id = args[0]
		srv.ConfigPath = args[1]
		logger.Debug("using provided service id and config", "id", srv.Id, "config", srv.ConfigPath)
	}

	// Load configuration (safe to touch config now)
	logger.Debug("loading service configuration")
	if d, err := config.GetDomain(); err == nil {
		srv.Domain = d
		logger.Debug("loaded domain from config", "domain", d)
	} else {
		srv.Domain = "localhost"
		logger.Debug("using default domain", "domain", "localhost")
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
		logger.Debug("loaded address from config", "address", a)
	}
	if srv.CacheAddress == "localhost" || srv.CacheAddress == "" {
		srv.CacheAddress = srv.Address
	}

	// Open in-memory cache store
	logger.Debug("initializing bigcache store")
	srv.cache = storage_store.NewBigCache_store()
	if err := srv.cache.Open(""); err != nil {
		logger.Error("cache open failed", "path", config.GetDataDir()+"/cache", "err", err)
	} else {
		logger.Info("bigcache store opened successfully")
	}

	// Scylla db — always connect to localhost since Scylla runs on the same node
	// (systemd: After=scylla-server.service). Scylla is configured with
	// rpc_address: 127.0.0.1, so using the node IP causes "connection refused".
	var host string
	if !srv.GetTls() {
		host = "127.0.0.1:9042"
	} else {
		host = "127.0.0.1:9142"
	}

	// Build JSON options for the store
	opts := fmt.Sprintf(`{
  "hosts": ["%s"],
  "keyspace": "rbac_permissions",
  "table": "permissions",
  "replication_factor": 1,
  "connect_timeout_ms": 5000,
  "timeout_ms": 5000,
  "consistency": "quorum",
  "disable_initial_host_lookup": true,
  "ca_file": "%s",
  "cert_file": "%s",
  "key_file": "%s",
  "insecure_skip_verify": false,
  "ssl_port": 9142,
  "tls": %t
}`, host, srv.GetCertAuthorityTrust(), srv.GetCertFile(), srv.GetKeyFile(), srv.GetTls())

	// Create & open the Scylla-backed KV store (with retries built into storage_store)
	logger.Info("initializing scylla permissions store", "host", host, "keyspace", "rbac_permissions")
	srv.permissions = storage_store.NewScylla_store("", "", 1)
	if err := srv.permissions.Open(opts); err != nil {
		logger.Error("permissions store open failed - cannot start RBAC service", "err", err)
		os.Exit(1)
	}
	logger.Info("permissions store opened successfully", "backend", "scylla")

	// Initialize service
	logger.Info("initializing rbac service", "id", srv.Id, "domain", srv.Domain)
	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Debug("service initialized", "duration_ms", time.Since(start).Milliseconds())
	srv.MinioConfig = srv.loadMinioConfig()
	if srv.MinioConfig != nil {
		logger.Info("minio storage configured",
			"endpoint", srv.MinioConfig.Endpoint,
			"bucket", srv.MinioConfig.Bucket,
			"secure", srv.MinioConfig.Secure)
	}

	// Clear precomputed USED_SPACE keys on startup (ensures fresh computation)
	logger.Debug("clearing precomputed USED_SPACE cache keys")
	if idsRaw, err := srv.getItem("USED_SPACE"); err == nil {
		var ids []string
		if jsonErr := json.Unmarshal(idsRaw, &ids); jsonErr == nil {
			for _, k := range ids {
				_ = srv.removeItem(k)
			}
			logger.Debug("cleared used_space cache", "count", len(ids))
		}
	}

	// Register gRPC services
	logger.Debug("registering grpc services")
	rbacpb.RegisterRbacServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	// Service ready - log comprehensive startup info
	logger.Info("rbac service ready",
		"id", srv.Id,
		"version", srv.Version,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"address", srv.Address,
		"startup_ms", time.Since(start).Milliseconds())

	// Start gRPC server
	logger.Info("starting grpc server", "port", srv.Port)
	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

// printUsage prints comprehensive command-line usage information.
func printUsage() {
	fmt.Println("Globular RBAC Service")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  rbac-service [OPTIONS] [<id> [configPath]]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("POSITIONAL ARGUMENTS:")
	fmt.Println("  id          Service instance ID (optional, auto-generated if not provided)")
	fmt.Println("  configPath  Path to service configuration file (optional)")
	fmt.Println()
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("  GLOBULAR_DOMAIN       Override service domain")
	fmt.Println("  GLOBULAR_ADDRESS      Override service address")
	fmt.Println("  MINIO_ENDPOINT        MinIO/S3 endpoint for policy storage")
	fmt.Println("  MINIO_BUCKET          MinIO bucket name (default: globular)")
	fmt.Println("  MINIO_PREFIX          MinIO key prefix (default: /users)")
	fmt.Println("  MINIO_USE_SSL         Enable SSL for MinIO (true/false)")
	fmt.Println("  MINIO_ACCESS_KEY      MinIO access key")
	fmt.Println("  MINIO_SECRET_KEY      MinIO secret key")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with auto-generated ID and default config")
	fmt.Println("  rbac-service")
	fmt.Println()
	fmt.Println("  # Start with specific service ID")
	fmt.Println("  rbac-service my-rbac-service-id")
	fmt.Println()
	fmt.Println("  # Enable debug logging")
	fmt.Println("  rbac-service --debug")
	fmt.Println()
	fmt.Println("  # Print service metadata")
	fmt.Println("  rbac-service --describe")
	fmt.Println()
	fmt.Println("  # Check service health")
	fmt.Println("  rbac-service --health")
	fmt.Println()
}

// printVersion prints version information as JSON.
func printVersion() {
	info := map[string]string{
		"service":    "rbac",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}
