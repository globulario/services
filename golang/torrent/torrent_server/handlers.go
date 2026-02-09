package main

import (
	"github.com/anacrolix/torrent"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"github.com/minio/minio-go/v7"
	"google.golang.org/grpc"
)

// server implements Globular plumbing + Torrent runtime fields.
type server struct {
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

	// TLS
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// Runtime
	grpcServer *grpc.Server

	// Torrent-specific runtime
	DownloadDir     string // where files are downloaded before being copied
	Seed            bool   // keep seeding after completion
	torrent_client_ *torrent.Client
	actions         chan map[string]interface{} // internal action bus
	done            chan bool                   // shutdown signal

	// Optional MinIO storage backend (mirrors media/file services)
	UseMinio       bool
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioPrefix    string
	MinioUseSSL    bool

	minioClient *minio.Client
}

// -----------------------------------------------------------------------------\n// Globular service contract (getters/setters)\n// -----------------------------------------------------------------------------

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
func (srv *server) GetDescription() string                { return srv.Description }
func (srv *server) SetDescription(description string)     { srv.Description = description }
func (srv *server) GetMac() string                        { return srv.Mac }
func (srv *server) SetMac(mac string)                     { srv.Mac = mac }
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
func (srv *server) GetChecksum() string                      { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)              { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                      { return srv.Plaform }
func (srv *server) SetPlatform(platform string)              { srv.Plaform = platform }
func (srv *server) GetPath() string                          { return srv.Path }
func (srv *server) SetPath(path string)                      { srv.Path = path }
func (srv *server) GetProto() string                         { return srv.Proto }
func (srv *server) SetProto(proto string)                    { srv.Proto = proto }
func (srv *server) GetPort() int                             { return srv.Port }
func (srv *server) SetPort(port int)                         { srv.Port = port }
func (srv *server) GetProxy() int                            { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                       { srv.Proxy = proxy }
func (srv *server) GetProtocol() string                      { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)              { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool                 { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool)  { srv.AllowAllOrigins = allowAllOrigins }
func (srv *server) GetAllowedOrigins() string                { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(allowedOrigins string)  { srv.AllowedOrigins = allowedOrigins }
func (srv *server) GetDomain() string                        { return srv.Domain }
func (srv *server) SetDomain(domain string)                  { srv.Domain = domain }
func (srv *server) GetTls() bool                             { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                       { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string            { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)          { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string                      { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)              { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                       { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)                { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                       { return srv.Version }
func (srv *server) SetVersion(version string)                { srv.Version = version }
func (srv *server) GetPublisherID() string                   { return srv.PublisherID }
func (srv *server) SetPublisherID(PublisherID string)        { srv.PublisherID = PublisherID }
func (srv *server) GetKeepUpToDate() bool                    { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                 { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                       { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                    { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}            { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// Default roles for the Torrent service.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	view := []string{
		"/torrent.TorrentService/GetTorrentInfos",
		"/torrent.TorrentService/GetTorrentLnks",
	}

	write := append([]string{
		"/torrent.TorrentService/DownloadTorrent",
		"/torrent.TorrentService/DropTorrent",
	}, view...) // writers can also view

	admin := append([]string{}, write...)

	return []resourcepb.Role{
		{
			Id:          "role:torrent.viewer",
			Name:        "Torrent Viewer",
			Domain:      domain,
			Description: "Read-only: list saved links and stream torrent progress.",
			Actions:     view,
			TypeName:    "resource.Role",
		},
		{
			Id:          "role:torrent.user",
			Name:        "Torrent User",
			Domain:      domain,
			Description: "Start downloads/seeding, drop torrents, and view progress.",
			Actions:     write,
			TypeName:    "resource.Role",
		},
		{
			Id:          "role:torrent.admin",
			Name:        "Torrent Admin",
			Domain:      domain,
			Description: "Full control over torrent operations.",
			Actions:     admin,
			TypeName:    "resource.Role",
		},
	}
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
	// Ensure download directory exists
	if err := Utility.CreateDirIfNotExist(srv.DownloadDir); err != nil {
		return err
	}

	// Channels and permissions mirror previous behavior
	srv.actions = make(chan map[string]interface{})
	srv.done = make(chan bool)
	srv.Permissions = append(srv.Permissions, map[string]interface{}{
		"action":    "/torrent.TorrentService/DownloadTorrentRequest",
		"resources": []interface{}{map[string]interface{}{"index": 1, "permission": "write"}},
	})

	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = srv.DownloadDir
	cfg.Seed = srv.Seed
	client, err := torrent.NewClient(cfg)
	if err != nil {
		return err
	}
	srv.torrent_client_ = client

	// Start processing loop
	srv.processTorrent()

	// Resume saved links asynchronously
	go func() {
		lnks, err := srv.readTorrentLnks()
		if err != nil {
			logger.Warn("read saved torrent links failed", "err", err)
			return
		}
		for _, lnk := range lnks {
			logger.Info("resuming torrent", "name", lnk.Name)
			if err := srv.downloadTorrent(lnk.Lnk, lnk.Dir, lnk.Seed, lnk.Owner); err != nil {
				logger.Error("resume torrent failed", "name", lnk.Name, "err", err)
			}
		}
	}()

	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }

// getRbacClient returns a connected RBAC client.
func getRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(token, path, subject, resourceType string, subjectType rbacpb.SubjectType) error {
	rbacClient, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(token, path, subject, resourceType, subjectType)
}

func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*event_client.Event_Client), nil
}
