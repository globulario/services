package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/conversation/conversation_client"
	"github.com/globulario/services/golang/conversation/conversationpb"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/search/search_engine"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// -----------------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------------

var (
	defaultPort       = 10029
	defaultProxy      = 10030
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// STDERR logger so --describe/--health JSON stays clean on STDOUT
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// -----------------------------------------------------------------------------
// Service implementation
// -----------------------------------------------------------------------------

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
	conversations sync.Map // map[accountId]map[conversationName]*conversationpb.Conversation
}

////////////////////////////////////////////////////////////////////////////////
// Globular interface & lifecycle
////////////////////////////////////////////////////////////////////////////////

func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }
func (srv *server) GetAddress() string               { return srv.Address }
func (srv *server) SetAddress(address string)        { srv.Address = address }
func (srv *server) GetProcess() int                  { return srv.Process }
func (srv *server) SetProcess(pid int) {
	if pid == -1 && srv.store != nil {
		_ = srv.store.Close()
	}
	srv.Process = pid
}
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
func (srv *server) GetDescription() string            { return srv.Description }
func (srv *server) SetDescription(description string) { srv.Description = description }
func (srv *server) GetKeywords() []string             { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)     { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string         { return srv.Repositories }
func (srv *server) SetRepositories(repos []string)    { srv.Repositories = repos }
func (srv *server) GetDiscoveries() []string          { return srv.Discoveries }
func (srv *server) SetDiscoveries(disc []string)      { srv.Discoveries = disc }
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
func (srv *server) GetChecksum() string             { return srv.Checksum }
func (srv *server) SetChecksum(sum string)          { srv.Checksum = sum }
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
func (srv *server) SetProtocol(p string)            { srv.Protocol = p }
func (srv *server) GetAllowAllOrigins() bool        { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)       { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string       { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)      { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string               { return srv.Domain }
func (srv *server) SetDomain(domain string)         { srv.Domain = domain }
func (srv *server) GetTls() bool                    { return srv.TLS }
func (srv *server) SetTls(v bool)                   { srv.TLS = v }
func (srv *server) GetCertAuthorityTrust() string   { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string             { return srv.CertFile }
func (srv *server) SetCertFile(cert string)         { srv.CertFile = cert }
func (srv *server) GetKeyFile() string              { return srv.KeyFile }
func (srv *server) SetKeyFile(key string)           { srv.KeyFile = key }
func (srv *server) GetVersion() string              { return srv.Version }
func (srv *server) SetVersion(v string)             { srv.Version = v }
func (srv *server) GetPublisherID() string          { return srv.PublisherID }
func (srv *server) SetPublisherID(id string)        { srv.PublisherID = id }
func (srv *server) GetKeepUpToDate() bool           { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(v bool)          { srv.KeepUpToDate = v }
func (srv *server) GetKeepAlive() bool              { return srv.KeepAlive }
func (srv *server) SetKeepAlive(v bool)             { srv.KeepAlive = v }
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})  { srv.Permissions = p }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	reader := resourcepb.Role{
		Id:          "role:conversation.reader",
		Name:        "Conversation Reader",
		Domain:      domain,
		Description: "Read conversations, connect, and browse.",
		Actions: []string{
			"/conversation.ConversationService/Connect",
			"/conversation.ConversationService/Disconnect",
			"/conversation.ConversationService/FindConversations",
			"/conversation.ConversationService/JoinConversation",
			"/conversation.ConversationService/GetConversation",
			"/conversation.ConversationService/GetConversations",
			"/conversation.ConversationService/GetReceivedInvitations",
			"/conversation.ConversationService/GetSentInvitations",
			"/conversation.ConversationService/FindMessages",
		},
		TypeName: "resource.Role",
	}

	member := resourcepb.Role{
		Id:          "role:conversation.member",
		Name:        "Conversation Member",
		Domain:      domain,
		Description: "Participate in conversations you have access to.",
		Actions: append([]string{},
			reader.Actions...,
		),
		TypeName: "resource.Role",
	}
	member.Actions = append(member.Actions,
		"/conversation.ConversationService/SendMessage",
		"/conversation.ConversationService/LikeMessage",
		"/conversation.ConversationService/DislikeMessage",
		"/conversation.ConversationService/SetMessageRead",
		"/conversation.ConversationService/LeaveConversation",
		"/conversation.ConversationService/AcceptInvitation",
		"/conversation.ConversationService/DeclineInvitation",
	)

	moderator := resourcepb.Role{
		Id:          "role:conversation.moderator",
		Name:        "Conversation Moderator",
		Domain:      domain,
		Description: "Moderate content in conversations.",
		Actions: []string{
			"/conversation.ConversationService/DeleteMessage",
		},
		TypeName: "resource.Role",
	}

	owner := resourcepb.Role{
		Id:          "role:conversation.owner",
		Name:        "Conversation Owner",
		Domain:      domain,
		Description: "Manage membership and delete the conversation.",
		Actions: append([]string{},
			member.Actions...,
		),
		TypeName: "resource.Role",
	}
	owner.Actions = append(owner.Actions,
		"/conversation.ConversationService/SendInvitation",
		"/conversation.ConversationService/RevokeInvitation",
		"/conversation.ConversationService/KickoutFromConversation",
		"/conversation.ConversationService/DeleteConversation",
	)

	admin := resourcepb.Role{
		Id:          "role:conversation.admin",
		Name:        "Conversation Admin",
		Domain:      domain,
		Description: "Full control over conversations and service lifecycle.",
		Actions: append(append(append(reader.Actions, member.Actions...), moderator.Actions...),
			"/conversation.ConversationService/SendInvitation",
			"/conversation.ConversationService/RevokeInvitation",
			"/conversation.ConversationService/KickoutFromConversation",
			"/conversation.ConversationService/DeleteConversation",
			"/conversation.ConversationService/Stop",
		),
		TypeName: "resource.Role",
	}

	return []resourcepb.Role{reader, member, moderator, owner, admin}
}

func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		logger.Error("InitService failed", "err", err)
		return err
	}
	gs, err := globular.InitGrpcServer(srv) // interceptors wired internally (auth-template style)
	if err != nil {
		logger.Error("InitGrpcServer failed", "err", err)
		return err
	}
	srv.grpcServer = gs

	// search engine & root store (single open here)
	if srv.Root == "" {
		srv.Root = os.TempDir()
	}
	srv.search_engine = new(search_engine.BleveSearchEngine)
	srv.store = storage_store.NewBadger_store()
	if err := srv.store.Open(`{"path":"` + srv.Root + `", "name":"conversations"}`); err != nil {
		logger.Error("opening root store failed", "path", srv.Root, "err", err)
		return err
	}
	logger.Info("service initialized", "name", srv.Name, "id", srv.Id)
	return nil
}

func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error {
	if srv.exit != nil {
		srv.exit <- true
	}
	return globular.StopService(srv, srv.grpcServer)
}

// Stop RPC
func (srv *server) Stop(ctx context.Context, _ *conversationpb.StopRequest) (*conversationpb.StopResponse, error) {
	if srv.exit != nil {
		srv.exit <- true
	}
	return &conversationpb.StopResponse{}, srv.StopService()
}

////////////////////////////////////////////////////////////////////////////////
// Event helpers
////////////////////////////////////////////////////////////////////////////////

func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		logger.Error("getEventClient failed", "err", err)
		return nil, err
	}
	return c.(*event_client.Event_Client), nil
}
func (srv *server) publish(event string, data []byte) error {
	c, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return c.Publish(event, data)
}
func (srv *server) subscribe(evt string, listener func(evt *eventpb.Event)) error {
	c, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return c.Subscribe(evt, srv.Name, listener)
}

////////////////////////////////////////////////////////////////////////////////
// RBAC helpers
////////////////////////////////////////////////////////////////////////////////

func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	c, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		logger.Error("GetRbacClient failed", "err", err)
		return nil, err
	}
	return c.(*rbac_client.Rbac_Client), nil
}
func (srv *server) deleteResourcePermissions(token, path string) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return c.DeleteResourcePermissions(token, path)
}
func (srv *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return false, false, err
	}
	return c.ValidateAccess(subject, subjectType, name, path)
}
func (srv *server) addResourceOwner(token, path, subject, resourceType string, subjectType rbacpb.SubjectType) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return c.AddResourceOwner(token, path, subject, resourceType, subjectType)
}
func (srv *server) setActionResourcesPermissions(token string, permissions map[string]interface{}) error {
	c, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return c.SetActionResourcesPermissions(token, permissions)
}

////////////////////////////////////////////////////////////////////////////////
// run loop & listeners
////////////////////////////////////////////////////////////////////////////////

func (srv *server) run() {
	logger.Info("conversation service run loop started", "service", srv.Name)

	channels := make(map[string][]string)
	clientIds := make(map[string]string)
	streams := make(map[string]conversationpb.ConversationService_ConnectServer)
	quits := make(map[string]chan bool)

	srv.actions = make(chan map[string]interface{})
	srv.exit = make(chan bool)

	srv.conversations = sync.Map{} // map[accountId]map[conversationName]*conversationpb.Conversation

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

func (srv *server) deleteAccountListener(evt *eventpb.Event) {
	accountId := string(evt.Data)
	token, _ := security.GetLocalToken(srv.Mac)
	logger.Info("delete account event received", "accountId", accountId)
	conversations, err := srv.getConversations(accountId)
	if err == nil {
		for _, c := range conversations.GetConversations() {
			_ = srv.deleteConversation(token, accountId, c)
		}
	} else {
		logger.Error("failed to list conversations for deleted account", "accountId", accountId, "err", err)
	}
}

////////////////////////////////////////////////////////////////////////////////
// Usage
////////////////////////////////////////////////////////////////////////////////

func printUsage() {
	exe := filepath.Base(os.Args[0])
	os.Stdout.WriteString(`
` + exe + ` - Conversation service with real-time messaging and WebRTC support

USAGE:
  ` + exe + ` [OPTIONS] [<id>] [<configPath>]

OPTIONS:
  --debug         Enable debug logging
  --version       Print version information as JSON and exit
  --help          Show this usage information and exit
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

ARGUMENTS:
  <id>            Service instance ID (optional)
  <configPath>    Optional path to configuration file

FEATURES:
  • Real-time messaging with WebSocket connections
  • Chat rooms and private conversations
  • WebRTC video and audio support
  • Message history and search
  • Typing indicators and presence
  • RBAC permissions per conversation
  • Integration with Event service for notifications
  • Full-text search with Bleve

ENVIRONMENT:
  GLOBULAR_DOMAIN   Set the service domain (default: localhost)
  GLOBULAR_ADDRESS  Set the service address (default: localhost:<port>)

EXAMPLES:
  ` + exe + ` --version
  ` + exe + ` --describe
  ` + exe + ` --debug conversation-1
  ` + exe + ` conversation-1 /etc/globular/conversation/config.json

`)
}

func printVersion() {
	data := map[string]string{
		"service":    "conversation",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

////////////////////////////////////////////////////////////////////////////////
// main
////////////////////////////////////////////////////////////////////////////////

func main() {
	// Build a skeleton service (no etcd/config yet)
	s := new(server)
	s.Name = string(conversationpb.File_conversation_proto.Services().Get(0).FullName())
	s.Proto = conversationpb.File_conversation_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = Version
	s.PublisherID = "localhost"
	s.Description = "Conversation service with real-time messaging, chat rooms, and WebRTC support"
	s.Keywords = []string{"conversation", "chat", "messaging", "webrtc", "video", "audio", "rooms"}
	s.Repositories = []string{}
	s.Discoveries = []string{}
	s.Dependencies = []string{"rbac.RbacService"}
	// Default RBAC permissions for ConversationService.
	// Generic verbs: read / write / delete.
	// For resource binding, we point to request fields by index and (optionally) field path.
	s.Permissions = []interface{}{

		// ---- Service control
		map[string]interface{}{
			"action":     "/conversation.ConversationService/Stop",
			"resources":  []interface{}{}, // privileged op
			"permission": "write",
		},

		// ---- Connections (control channel)
		map[string]interface{}{
			"action":     "/conversation.ConversationService/Connect",
			"resources":  []interface{}{}, // requires valid JWT; no concrete resource id
			"permission": "read",
		},
		map[string]interface{}{
			"action":     "/conversation.ConversationService/Disconnect",
			"resources":  []interface{}{},
			"permission": "read",
		},

		// ---- Conversation lifecycle
		map[string]interface{}{
			"action":     "/conversation.ConversationService/CreateConversation",
			"resources":  []interface{}{}, // creates a new resource
			"permission": "write",
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/DeleteConversation",
			"resources": []interface{}{
				// DeleteConversationRequest.conversation_uuid
				map[string]interface{}{"index": 0, "field": "ConversationUuid", "permission": "delete"},
			},
		},

		// ---- Conversation discovery & access
		map[string]interface{}{
			"action":     "/conversation.ConversationService/FindConversations",
			"resources":  []interface{}{},
			"permission": "read",
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/JoinConversation",
			"resources": []interface{}{
				// JoinConversationRequest.conversation_uuid
				map[string]interface{}{"index": 0, "field": "ConversationUuid", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/LeaveConversation",
			"resources": []interface{}{
				// LeaveConversationRequest.conversation_uuid
				map[string]interface{}{"index": 0, "field": "ConversationUuid", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/GetConversation",
			"resources": []interface{}{
				// GetConversationRequest.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/conversation.ConversationService/GetConversations",
			"resources":  []interface{}{}, // list by creator (server enforces identity)
			"permission": "read",
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/KickoutFromConversation",
			"resources": []interface{}{
				// KickoutFromConversationRequest.conversation_uuid
				map[string]interface{}{"index": 0, "field": "ConversationUuid", "permission": "write"},
			},
		},

		// ---- Invitations
		map[string]interface{}{
			"action": "/conversation.ConversationService/SendInvitation",
			"resources": []interface{}{
				// SendInvitationRequest.invitation.conversation
				map[string]interface{}{"index": 0, "field": "Invitation.Conversation", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/AcceptInvitation",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Invitation.Conversation", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/DeclineInvitation",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Invitation.Conversation", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/RevokeInvitation",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Invitation.Conversation", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/GetReceivedInvitations",
			"resources": []interface{}{
				// GetReceivedInvitationsRequest.account
				map[string]interface{}{"index": 0, "field": "Account", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/GetSentInvitations",
			"resources": []interface{}{
				// GetSentInvitationsRequest.account
				map[string]interface{}{"index": 0, "field": "Account", "permission": "read"},
			},
		},

		// ---- Messages
		map[string]interface{}{
			"action": "/conversation.ConversationService/SendMessage",
			"resources": []interface{}{
				// SendMessageRequest.msg.conversation
				map[string]interface{}{"index": 0, "field": "Msg.Conversation", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/DeleteMessage",
			"resources": []interface{}{
				// DeleteMessageRequest.conversation
				map[string]interface{}{"index": 0, "field": "Conversation", "permission": "delete"},
			},
		},
		map[string]interface{}{
			"action":     "/conversation.ConversationService/FindMessages",
			"resources":  []interface{}{},
			"permission": "read",
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/LikeMessage",
			"resources": []interface{}{
				// LikeMessageRqst.conversation
				map[string]interface{}{"index": 0, "field": "Conversation", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/DislikeMessage",
			"resources": []interface{}{
				// DislikeMessageRqst.conversation
				map[string]interface{}{"index": 0, "field": "Conversation", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action": "/conversation.ConversationService/SetMessageRead",
			"resources": []interface{}{
				// SetMessageReadRqst.conversation
				map[string]interface{}{"index": 0, "field": "Conversation", "permission": "write"},
			},
		},
	}

	s.Process = -1
	s.ProxyProcess = -1
	s.PortSFU = 5551
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr

	// Dynamic client registration
	Utility.RegisterFunction("NewConversationService_Client", conversation_client.NewConversationService_Client)

	// CLI flags BEFORE touching config
	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
	)

	flag.Usage = printUsage
	flag.Parse()

	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	if *showVersion {
		printVersion()
		return
	}

	if *showHelp {
		printUsage()
		return
	}

	if *showDescribe {
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
	}

	if *showHealth {
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
	}

	args := flag.Args()

	// Custom port allocation if no positional args
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

	// Optional positional args: <id> [configPath]
	if len(args) == 1 {
		s.Id = args[0]
	} else if len(args) == 2 {
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
	if s.Root == "" {
		s.Root = os.TempDir()
	}

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service initialization failed", "name", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// Ensure on-disk area for conversation artifacts / indices
	_ = Utility.CreateDirIfNotExist(filepath.Join(s.Root, "conversations"))

	// Register service
	conversationpb.RegisterConversationServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	// Subscribe & run
	go func() {
		if err := s.subscribe("delete_account_evt", s.deleteAccountListener); err != nil {
			logger.Error("subscribe failed", "event", "delete_account_evt", "err", err)
		}
	}()
	go s.run()

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := s.StartService(); err != nil {
		logger.Error("service failed to start", "err", err)
		os.Exit(1)
	}
}
