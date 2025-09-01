package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/conversation/conversation_client"
	"github.com/globulario/services/golang/conversation/conversationpb"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/search/search_engine"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	defaultPort       = 10029
	defaultProxy      = 10030
	allow_all_origins = true
	allowed_origins   = ""

	// package-level structured logger
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
)

// server implements Globular’s service contract and acts as
// the Conversation microservice runtime container.
// It carries basic service identity, configuration, gRPC server,
// and runtime state (actions, store, search engine, etc.).
type server struct {
	// Globular fields
	Id              string
	Name            string
	Mac             string
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
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64

	// Conversation service config
	Root    string // base path for data
	PortSFU int
	TLS     bool

	// TLS files
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	Permissions  []interface{}
	Dependencies []string

	grpcServer *grpc.Server

	// runtime internals
	actions chan map[string]interface{}
	exit    chan bool

	search_engine *search_engine.BleveSearchEngine
	store         storage_store.Store
	conversations *syncMap // wrapper defined below to avoid importing sync in this file
}

// minimal wrapper so server.go doesn’t import sync
type syncMap struct{ m *storage_store.Badger_store }

func (s *syncMap) Delete(dbPath string) {
	if s.m != nil {
		_ = s.m.Close()
		s.m = nil
	}
}

func (s *syncMap) Store(dbPath string, conn *storage_store.Badger_store) {
	if s.m != nil {
		// Close previous store if open
		_ = s.m.Close()
	}
	s.m = conn
}

func (s *syncMap) Load(dbPath string) (any, any) {
	if s.m == nil {
		return nil, false
	}
	return s.m, true
}

////////////////////////////////////////////////////////////////////////////////
// Globular interface & lifecycle
////////////////////////////////////////////////////////////////////////////////

// GetConfigurationPath returns the path to the service configuration file.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path to the service configuration file.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the network address (host:port) of this service.
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the network address (host:port) of this service.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the PID of the service process (or -1 if not started).
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the PID of the service process. When set to -1, it closes the store.
func (srv *server) SetProcess(pid int) {
	if pid == -1 && srv.store != nil {
		_ = srv.store.Close()
	}
	srv.Process = pid
}

// GetProxyProcess returns the PID of the proxy process (if any).
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the PID of the proxy process.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the service state string.
func (srv *server) GetState() string { return srv.State }

// SetState sets the service state string.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error string captured by the service.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError sets the last error string captured by the service.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification timestamp.
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the last modification timestamp.
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the service ID.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the service ID.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetMac returns the MAC address associated with the service.
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address associated with the service.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetKeywords returns the service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns repositories attached to the service.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets repositories attached to the service.
func (srv *server) SetRepositories(repos []string) { srv.Repositories = repos }

// GetDiscoveries returns discovery endpoints.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets discovery endpoints.
func (srv *server) SetDiscoveries(disc []string) { srv.Discoveries = disc }

// Dist resolves and returns a distribution path for the given relative path.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of service dependencies.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	return srv.Dependencies
}

// SetDependency appends a dependency if it is not already present.
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	if !Utility.Contains(srv.Dependencies, dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
	}
}

// GetChecksum returns the package checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the package checksum.
func (srv *server) SetChecksum(sum string) { srv.Checksum = sum }

// GetPlatform returns the target platform.
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the target platform.
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the service binary path.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the service binary path.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the service proto path.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the service proto path.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the service port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the service port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the proxy port.
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the proxy port.
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the transport protocol ("grpc").
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the transport protocol.
func (srv *server) SetProtocol(p string) { srv.Protocol = p }

// GetAllowAllOrigins returns whether CORS allows all origins.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether CORS allows all origins.
func (srv *server) SetAllowAllOrigins(v bool) { srv.AllowAllOrigins = v }

// GetAllowedOrigins returns the allowed CORS origins list.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the allowed CORS origins list.
func (srv *server) SetAllowedOrigins(v string) { srv.AllowedOrigins = v }

// GetDomain returns the configured domain.
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the configured domain.
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns whether TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls sets whether TLS is enabled.
func (srv *server) SetTls(v bool) { srv.TLS = v }

// GetCertAuthorityTrust returns the CA bundle path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA bundle path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the certificate path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the certificate path.
func (srv *server) SetCertFile(cert string) { srv.CertFile = cert }

// GetKeyFile returns the certificate key path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the certificate key path.
func (srv *server) SetKeyFile(key string) { srv.KeyFile = key }

// GetVersion returns the package version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the package version.
func (srv *server) SetVersion(v string) { srv.Version = v }

// GetPublisherID returns the publisher ID.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher ID.
func (srv *server) SetPublisherID(id string) { srv.PublisherID = id }

// GetKeepUpToDate returns whether the service should auto-update.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate sets whether the service should auto-update.
func (srv *server) SetKeepUptoDate(v bool) { srv.KeepUpToDate = v }

// GetKeepAlive returns whether the service should be kept alive.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive sets whether the service should be kept alive.
func (srv *server) SetKeepAlive(v bool) { srv.KeepAlive = v }

// GetPermissions returns the service permissions list.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the service permissions list.
func (srv *server) SetPermissions(p []interface{}) { srv.Permissions = p }

// Init initializes the service (config, gRPC server, store, search engine).
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		logger.Error("InitService failed", "err", err)
		return err
	}
	var err error
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		logger.Error("InitGrpcServer failed", "err", err)
		return err
	}

	// search engine & store
	srv.search_engine = new(search_engine.BleveSearchEngine)
	srv.store = storage_store.NewBadger_store()
	if err := srv.store.Open(`{"path":"` + srv.Root + `", "name":"conversations"}`); err != nil {
		logger.Error("opening root store failed", "path", srv.Root, "err", err)
		return err
	}
	logger.Info("service initialized", "name", srv.Name, "id", srv.Id)
	return nil
}

// Save persists the service configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the gRPC service.
func (srv *server) StartService() error {
	logger.Info("starting gRPC service", "addr", srv.Address, "port", srv.Port)
	return globular.StartService(srv, srv.grpcServer)
}

// StopService stops the gRPC service and signals the run loop to exit.
func (srv *server) StopService() error {
	if srv.exit != nil {
		srv.exit <- true
	}
	logger.Info("stopping gRPC service", "addr", srv.Address, "port", srv.Port)
	return globular.StopService(srv, srv.grpcServer)
}

// Stop stops the service via RPC.
func (srv *server) Stop(ctx context.Context, _ *conversationpb.StopRequest) (*conversationpb.StopResponse, error) {
	srv.exit <- true
	return &conversationpb.StopResponse{}, srv.StopService()
}

////////////////////////////////////////////////////////////////////////////////
// Event helpers
////////////////////////////////////////////////////////////////////////////////

// getEventClient returns a connected Event service client.
func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		logger.Error("getEventClient failed", "err", err)
		return nil, err
	}
	return c.(*event_client.Event_Client), nil
}

// publish publishes an event with the given name and payload.
func (srv *server) publish(event string, data []byte) error {
	c, err := srv.getEventClient()
	if err != nil {
		return err
	}
	logger.Info("publishing event", "event", event, "size", len(data))
	return c.Publish(event, data)
}

// subscribe subscribes this service to the given event and listener.
func (srv *server) subscribe(evt string, listener func(evt *eventpb.Event)) error {
	c, err := srv.getEventClient()
	if err != nil {
		return err
	}
	logger.Info("subscribing to event", "event", evt)
	return c.Subscribe(evt, srv.Name, listener)
}

////////////////////////////////////////////////////////////////////////////////
// RBAC helpers
////////////////////////////////////////////////////////////////////////////////

// GetRbacClient returns a connected RBAC service client.
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	c, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		logger.Error("GetRbacClient failed", "err", err)
		return nil, err
	}
	return c.(*rbac_client.Rbac_Client), nil
}

// deleteResourcePermissions removes all permissions attached to a resource path.
func (srv *server) deleteResourcePermissions(path string) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	logger.Info("deleting resource permissions", "path", path)
	return c.DeleteResourcePermissions(path)
}

// validateAccess verifies a subject has the named permission on a resource path.
func (srv *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return false, false, err
	}
	return c.ValidateAccess(subject, subjectType, name, path)
}

// addResourceOwner makes subject the owner of a resource path.
func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	logger.Info("adding resource owner", "path", path, "type", resourceType, "subject", subject)
	return c.AddResourceOwner(path, resourceType, subject, subjectType)
}

// setActionResourcesPermissions sets action/resource permissions in RBAC.
func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return c.SetActionResourcesPermissions(permissions)
}

////////////////////////////////////////////////////////////////////////////////
// run loop & listeners
////////////////////////////////////////////////////////////////////////////////

// run is the in-memory broker for connections, channel membership, and fan-out.
func (srv *server) run() {
	logger.Info("conversation service run loop started", "service", srv.Name)
	channels := make(map[string][]string)
	clientIds := make(map[string]string)
	streams := make(map[string]conversationpb.ConversationService_ConnectServer)
	quits := make(map[string]chan bool)

	srv.actions = make(chan map[string]interface{})
	srv.exit = make(chan bool)

	for {
		select {
		case <-srv.exit:
			logger.Info("conversation service run loop exiting")
			return

		case a := <-srv.actions:
			switch a["action"] {
			case "connect":
				streams[a["uuid"].(string)] = a["stream"].(conversationpb.ConversationService_ConnectServer)
				clientIds[a["uuid"].(string)] = a["clientId"].(string)
				quits[a["uuid"].(string)] = a["quit"].(chan bool)
				logger.Info("client connected", "uuid", a["uuid"], "clientId", a["clientId"])

			case "join":
				name := a["name"].(string)
				uuid := a["uuid"].(string)
				if channels[name] == nil {
					channels[name] = []string{}
				}
				if !Utility.Contains(channels[name], uuid) {
					channels[name] = append(channels[name], uuid)
				}
				logger.Info("client joined conversation", "conversation", name, "uuid", uuid)

			case "send_message":
				name := a["name"].(string)
				msg := a["message"].(*conversationpb.Message)
				logger.Info("fanout message", "conversation", name, "messageId", msg.Uuid, "author", msg.Author)

				if channels[name] != nil {
					toDelete := []string{}
					for _, uuid := range channels[name] {
						stream := streams[uuid]
						if stream != nil {
							if err := stream.Send(&conversationpb.ConnectResponse{Msg: msg}); err != nil {
								logger.Warn("stream send failed, marking for cleanup", "uuid", uuid, "err", err)
								toDelete = append(toDelete, uuid)
							}
						}
					}
					for _, uuid := range toDelete {
						clientId := clientIds[uuid]
						for n, list := range channels {
							keep := []string{}
							for _, u := range list {
								if u != uuid {
									keep = append(keep, u)
								}
							}
							channels[n] = keep
							_ = srv.removeConversationParticipant(clientId, uuid)
						}
						if q, ok := quits[uuid]; ok {
							q <- true
						}
						delete(quits, uuid)
						logger.Info("disconnected stale client", "uuid", uuid)
					}
				}

			case "leave":
				name := a["name"].(string)
				uuid := a["uuid"].(string)
				keep := []string{}
				for _, u := range channels[name] {
					if u != uuid {
						keep = append(keep, u)
					}
				}
				channels[name] = keep
				logger.Info("client left conversation", "conversation", name, "uuid", uuid)

			case "disconnect":
				uuid := a["uuid"].(string)
				for n, list := range channels {
					keep := []string{}
					for _, u := range list {
						if u != uuid {
							keep = append(keep, u)
						}
					}
					channels[n] = keep
				}
				if q, ok := quits[uuid]; ok {
					q <- true
				}
				delete(quits, uuid)
				logger.Info("client disconnected", "uuid", uuid)
			}
		}
	}
}

// deleteAccountListener handles account deletion events by cleaning up
// that account’s conversations and related artifacts.
func (srv *server) deleteAccountListener(evt *eventpb.Event) {
	accountId := string(evt.Data)
	logger.Info("delete account event received", "accountId", accountId)
	conversations, err := srv.getConversations(accountId)
	if err == nil {
		for _, c := range conversations.GetConversations() {
			_ = srv.deleteConversation(accountId, c)
		}
	} else {
		logger.Error("failed to list conversations for deleted account", "accountId", accountId, "err", err)
	}
}

////////////////////////////////////////////////////////////////////////////////
// main
////////////////////////////////////////////////////////////////////////////////

func main() {
	s := new(server)
	s.Name = string(conversationpb.File_conversation_proto.Services().Get(0).FullName())
	s.Proto = conversationpb.File_conversation_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Domain, _ = config.GetDomain()
	s.Address, _ = config.GetAddress()
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "A way to communicate with other members of an organization"
	s.Keywords = []string{"Conversation", "Chat", "Messenger"}
	s.Repositories = []string{}
	s.Discoveries = []string{}
	s.Dependencies = []string{"rbac.RbacService"}
	s.Permissions = make([]interface{}, 2)
	s.Process = -1
	s.ProxyProcess = -1
	s.PortSFU = 5551
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allow_all_origins
	s.AllowedOrigins = allowed_origins

	Utility.RegisterFunction("NewConversationService_Client", conversation_client.NewConversationService_Client)

	if len(s.Root) == 0 {
		s.Root = os.TempDir()
	}
	if len(os.Args) == 2 {
		s.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s.Id = os.Args[1]
		s.ConfigPath = os.Args[2]
	}

	// permissions
	s.Permissions[0] = map[string]interface{}{
		"action": "/conversation.ConversationService/DeleteConversation",
		"resources": []interface{}{
			map[string]interface{}{"index": 0, "permission": "owner"},
		},
	}
	s.Permissions[1] = map[string]interface{}{
		"action": "/conversation.ConversationService/KickoutFromConversation",
		"resources": []interface{}{
			map[string]interface{}{"index": 0, "permission": "owner"},
		},
	}

	if err := s.Init(); err != nil {
		logger.Error("service initialization failed", "name", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	if s.Address == "" {
		s.Address, _ = config.GetAddress()
	}

	// init runtime containers
	s.search_engine = new(search_engine.BleveSearchEngine)
	s.conversations = &syncMap{m: storage_store.NewBadger_store()}

	Utility.CreateDirIfNotExist(s.Root + "/conversations")
	_ = s.store.Open(`{"path":"` + s.Root + "/conversations" + `", "name":"index"}`)

	// register service
	conversationpb.RegisterConversationServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	// subscribe & run
	go func() {
		if err := s.subscribe("delete_account_evt", s.deleteAccountListener); err != nil {
			logger.Error("subscribe failed", "event", "delete_account_evt", "err", err)
		}
	}()
	go s.run()

	if err := s.StartService(); err != nil {
		logger.Error("service failed to start", "err", err)
		os.Exit(1)
	}
}
