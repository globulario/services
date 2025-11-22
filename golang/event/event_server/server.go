package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Default runtime options.
// NOTE: TLS/HTTPS is TODO.
var (
	defaultPort        = 10050
	defaultProxy       = 10051
	allow_all_origins  = true // by default, allow all origins
	allowed_origins    = ""   // comma-separated origins; used only if allow_all_origins == false
	errNotInitialized  = errors.New("event service: not initialized")
	errServerNil       = errors.New("event service: grpc server is nil")
	errMissingStream   = errors.New("event service: missing stream")
	errMissingUUID     = errors.New("event service: missing uuid")
	errMissingChanName = errors.New("event service: missing channel name")
)

// server holds service metadata and runtime state.
type server struct {
	// Globular service metadata
	Id              string
	Name            string
	Mac             string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string
	Protocol        string
	Domain          string
	Address         string
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

	// TLS
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherID        string
	KeepUpToDate       bool
	Checksum           string
	Plaform            string
	KeepAlive          bool
	Permissions        []interface{} // action permissions for the service
	Dependencies       []string      // required services

	// runtime
	grpcServer *grpc.Server
	actions    chan map[string]interface{} // serialized control channel
	exit       chan bool                   // termination signal

	// logger (not part of public API)
	logger *slog.Logger
}

///////////////////////////////////////////////////////////////////////////////
// Globular configuration getters/setters (public) - documented
///////////////////////////////////////////////////////////////////////////////

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
func (srv *server) GetDescription() string            { return srv.Description }
func (srv *server) SetDescription(description string) { srv.Description = description }
func (srv *server) GetKeywords() []string             { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)     { srv.Keywords = keywords }
func (srv *server) Dist(path string) (string, error)  { return globular.Dist(path, srv) }
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
func (srv *server) GetRepositories() []string                { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string)    { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string                 { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)      { srv.Discoveries = discoveries }
func (srv *server) GetKeepUpToDate() bool                    { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                 { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                       { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                    { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}            { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	reader := resourcepb.Role{
		Id:          "role:event.reader",
		Name:        "Event Reader",
		Domain:      domain,
		Description: "Subscribe to channels and receive events.",
		Actions: []string{
			"/event.EventService/OnEvent",
			"/event.EventService/Quit",
			"/event.EventService/Subscribe",
			"/event.EventService/UnSubscribe",
		},
		TypeName: "resource.Role",
	}

	publisher := resourcepb.Role{
		Id:          "role:event.publisher",
		Name:        "Event Publisher",
		Domain:      domain,
		Description: "Publish to channels (and read if allowed).",
		Actions: []string{
			"/event.EventService/Publish",
			// commonly include read-side, too:
			"/event.EventService/OnEvent",
			"/event.EventService/Quit",
			"/event.EventService/Subscribe",
			"/event.EventService/UnSubscribe",
		},
		TypeName: "resource.Role",
	}

	admin := resourcepb.Role{
		Id:          "role:event.admin",
		Name:        "Event Admin",
		Domain:      domain,
		Description: "Full control of the event service.",
		Actions: []string{
			"/event.EventService/Stop",
			"/event.EventService/OnEvent",
			"/event.EventService/Quit",
			"/event.EventService/Subscribe",
			"/event.EventService/UnSubscribe",
			"/event.EventService/Publish",
		},
		TypeName: "resource.Role",
	}

	return []resourcepb.Role{reader, publisher, admin}
}

///////////////////////////////////////////////////////////////////////////////
// Lifecycle
///////////////////////////////////////////////////////////////////////////////

func (srv *server) Init() error {
	if srv.logger == nil {
		srv.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	// Ensure control channels are initialized BEFORE any RPCs can hit them.
	if srv.actions == nil {
		// Small buffer so RPC handlers don't immediately block if the event loop
		// is briefly busy. Size can be tuned if needed.
		srv.actions = make(chan map[string]interface{}, 1024)
	}
	if srv.exit == nil {
		srv.exit = make(chan bool)
	}

	if err := globular.InitService(srv); err != nil {
		srv.logger.Error("init service failed", "service", srv.Name, "id", srv.Id, "err", err)
		return err
	}
	grpcSrv, err := globular.InitGrpcServer(srv)
	if err != nil {
		srv.logger.Error("init grpc server failed", "service", srv.Name, "id", srv.Id, "err", err)
		return err
	}
	srv.grpcServer = grpcSrv
	srv.logger.Info("service initialized", "service", srv.Name, "id", srv.Id)
	return nil
}

func (srv *server) Save() error {
	if srv == nil {
		return errNotInitialized
	}
	return globular.SaveService(srv)
}
func (srv *server) StartService() error {
	if srv.grpcServer == nil {
		return errServerNil
	}
	srv.logger.Info("starting service", "service", srv.Name, "id", srv.Id, "port", srv.Port, "proxy", srv.Proxy, "protocol", srv.Protocol)
	return globular.StartService(srv, srv.grpcServer)
}
func (srv *server) StopService() error {
	if srv.grpcServer == nil {
		return nil
	}
	srv.grpcServer.Stop()
	srv.logger.Info("service stopped", "service", srv.Name, "id", srv.Id)
	return nil
}
func (srv *server) Stop(ctx context.Context, _ *eventpb.StopRequest) (*eventpb.StopResponse, error) {
	srv.logger.Info("stop requested", "service", srv.Name, "id", srv.Id)
	srv.exit <- true
	return &eventpb.StopResponse{}, srv.StopService()
}

///////////////////////////////////////////////////////////////////////////////
// Event subsystem
///////////////////////////////////////////////////////////////////////////////

func (srv *server) run() {
	if srv.logger == nil {
		srv.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	// Safety: Init() is supposed to set this; if it's still nil something
	// is badly wrong, so log and block rather than panic on send.
	if srv.actions == nil {
		srv.logger.Error("event loop starting with nil actions channel; Init() may have failed")
		// fall back to a sane default to avoid nil-channel deadlocks
		srv.actions = make(chan map[string]interface{}, 1024)
	}

	channels := make(map[string][]string)                          // channel -> uuids
	streams := make(map[string]eventpb.EventService_OnEventServer) // uuid -> stream
	quits := make(map[string]chan bool)                            // uuid -> quit
	ka := make(chan *eventpb.KeepAlive)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				ka <- &eventpb.KeepAlive{}
			}
		}
	}()

	srv.logger.Info("event loop started", "service", srv.Name, "id", srv.Id)

	for {
		select {
		case <-srv.exit:
			close(done)
			for uuid, q := range quits {
				select {
				case q <- true:
				default:
				}
				delete(quits, uuid)
				delete(streams, uuid)
			}
			srv.logger.Info("event loop stopped",
				"service", srv.Name,
				"id", srv.Id,
				"channels", len(channels),
				"streams", len(streams))
			return

		case ka_ := <-ka:
			var toDelete []string
			for uuid, stream := range streams {
				if stream == nil {
					toDelete = append(toDelete, uuid)
					continue
				}
				if err := stream.Send(&eventpb.OnEventResponse{Data: &eventpb.OnEventResponse_Ka{Ka: ka_}}); err != nil {
					srv.logger.Warn("keepalive send failed; will drop stream", "uuid", uuid, "err", err)
					toDelete = append(toDelete, uuid)
				}
			}
			if len(toDelete) > 0 {
				srv.cleanupSubscribers(toDelete, channels, quits, streams)
			}

		case a := <-srv.actions:
			action, _ := a["action"].(string)
			switch action {
			case "onevent":
				stream, _ := a["stream"].(eventpb.EventService_OnEventServer)
				uuid, _ := a["uuid"].(string)
				qc, ok := a["quit"].(chan bool)
				if stream == nil || uuid == "" || !ok {
					srv.logger.Error("invalid onevent request", "uuid", uuid, "has_stream", stream != nil, "has_quit", ok)
					continue
				}
				streams[uuid] = stream
				quits[uuid] = qc
				srv.logger.Info("stream registered", "uuid", uuid)

			case "subscribe":
				name, _ := a["name"].(string)
				uuid, _ := a["uuid"].(string)
				if name == "" || uuid == "" {
					srv.logger.Error("invalid subscribe request", "name", name, "uuid", uuid)
					continue
				}
				if channels[name] == nil {
					channels[name] = make([]string, 0)
				}
				if !Utility.Contains(channels[name], uuid) {
					channels[name] = append(channels[name], uuid)
					srv.logger.Info("subscribed", "channel", name, "uuid", uuid, "subscribers", len(channels[name]))
				}

			case "publish":
				name, _ := a["name"].(string)
				data, _ := a["data"].([]byte)
				if name == "" {
					srv.logger.Error("invalid publish request: missing channel name")
					continue
				}
				uuids := channels[name]
				if uuids == nil {
					// No subscribers is not an error; just nothing to do.
					continue
				}
				var toDelete []string
				for _, uuid := range uuids {
					stream := streams[uuid]
					if stream == nil {
						toDelete = append(toDelete, uuid)
						continue
					}
					err := stream.Send(&eventpb.OnEventResponse{
						Data: &eventpb.OnEventResponse_Evt{
							Evt: &eventpb.Event{Name: name, Data: data},
						},
					})
					if err != nil {
						srv.logger.Warn("event send failed; will drop subscriber", "channel", name, "uuid", uuid, "err", err)
						toDelete = append(toDelete, uuid)
					}
				}
				if len(toDelete) > 0 {
					srv.cleanupSubscribers(toDelete, channels, quits, streams)
				}

			case "unsubscribe":
				name, _ := a["name"].(string)
				uuid, _ := a["uuid"].(string)
				if name == "" || uuid == "" {
					srv.logger.Error("invalid unsubscribe request", "name", name, "uuid", uuid)
					continue
				}
				uuids := make([]string, 0, len(channels[name]))
				for _, id := range channels[name] {
					if id != uuid {
						uuids = append(uuids, id)
					}
				}
				if len(uuids) == 0 {
					delete(channels, name)
				} else {
					channels[name] = uuids
				}
				srv.logger.Info("unsubscribed", "channel", name, "uuid", uuid, "remaining", len(channels[name]))

			case "quit":
				uuid, _ := a["uuid"].(string)
				if uuid == "" {
					srv.logger.Error("invalid quit request: missing uuid")
					continue
				}
				srv.cleanupSubscribers([]string{uuid}, channels, quits, streams)
				srv.logger.Info("stream quit", "uuid", uuid)

			default:
				srv.logger.Warn("unknown action", "action", action)
			}
		}
	}
}

func (srv *server) cleanupSubscribers(
	toDelete []string,
	channels map[string][]string,
	quits map[string]chan bool,
	streams map[string]eventpb.EventService_OnEventServer,
) {
	for _, uuid := range toDelete {
		// Remove from all channels
		for name, ch := range channels {
			uuids := make([]string, 0, len(ch))
			for _, id := range ch {
				if id != uuid {
					uuids = append(uuids, id)
				}
			}
			if len(uuids) == 0 {
				delete(channels, name)
			} else {
				channels[name] = uuids
			}
		}

		// Signal quit (if any) and remove
		if q, ok := quits[uuid]; ok {
			select {
			case q <- true:
			default:
			}
			delete(quits, uuid)
		}

		// Remove stream entry so future keepalives don't see it
		if _, ok := streams[uuid]; ok {
			delete(streams, uuid)
		}

		srv.logger.Info("subscriber cleanup", "uuid", uuid)
	}
}

///////////////////////////////////////////////////////////////////////////////
// RPCs (public prototypes preserved)
///////////////////////////////////////////////////////////////////////////////

func (srv *server) Quit(ctx context.Context, rqst *eventpb.QuitRequest) (*eventpb.QuitResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("Quit: invalid request", "err", errMissingUUID)
		return &eventpb.QuitResponse{Result: false}, errMissingUUID
	}
	msg := map[string]interface{}{"action": "quit", "uuid": rqst.Uuid}
	srv.actions <- msg
	srv.logger.Info("Quit: ok", "uuid", rqst.Uuid)
	return &eventpb.QuitResponse{Result: true}, nil
}

func (srv *server) OnEvent(rqst *eventpb.OnEventRequest, stream eventpb.EventService_OnEventServer) error {
	if stream == nil {
		srv.logger.Error("OnEvent: missing stream", "err", errMissingStream)
		return errMissingStream
	}
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("OnEvent: invalid request", "err", errMissingUUID)
		return errMissingUUID
	}

	onevent := map[string]interface{}{
		"action": "onevent",
		"stream": stream,
		"uuid":   rqst.Uuid,
		"quit":   make(chan bool),
	}
	srv.actions <- onevent
	srv.logger.Info("OnEvent: registered", "uuid", rqst.Uuid)

	<-onevent["quit"].(chan bool)
	srv.logger.Info("OnEvent: stream ended", "uuid", rqst.Uuid)
	return nil
}

func (srv *server) Subscribe(ctx context.Context, rqst *eventpb.SubscribeRequest) (*eventpb.SubscribeResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("Subscribe: invalid request", "err", errMissingUUID)
		return &eventpb.SubscribeResponse{Result: false}, errMissingUUID
	}
	if rqst.Name == "" {
		srv.logger.Error("Subscribe: invalid request", "err", errMissingChanName)
		return &eventpb.SubscribeResponse{Result: false}, errMissingChanName
	}
	subscribe := map[string]interface{}{"action": "subscribe", "name": rqst.Name, "uuid": rqst.Uuid}
	srv.actions <- subscribe
	srv.logger.Info("Subscribe: ok", "channel", rqst.Name, "uuid", rqst.Uuid)
	return &eventpb.SubscribeResponse{Result: true}, nil
}

func (srv *server) UnSubscribe(ctx context.Context, rqst *eventpb.UnSubscribeRequest) (*eventpb.UnSubscribeResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("UnSubscribe: invalid request", "err", errMissingUUID)
		return &eventpb.UnSubscribeResponse{Result: false}, errMissingUUID
	}
	if rqst.Name == "" {
		srv.logger.Error("UnSubscribe: invalid request", "err", errMissingChanName)
		return &eventpb.UnSubscribeResponse{Result: false}, errMissingChanName
	}
	unsubscribe := map[string]interface{}{"action": "unsubscribe", "name": rqst.Name, "uuid": rqst.Uuid}
	srv.actions <- unsubscribe
	srv.logger.Info("UnSubscribe: ok", "channel", rqst.Name, "uuid", rqst.Uuid)
	return &eventpb.UnSubscribeResponse{Result: true}, nil
}

func (srv *server) Publish(ctx context.Context, rqst *eventpb.PublishRequest) (*eventpb.PublishResponse, error) {
	if rqst == nil || rqst.Evt == nil || rqst.Evt.Name == "" {
		srv.logger.Error("Publish: invalid request", "err", errMissingChanName)
		return &eventpb.PublishResponse{Result: false}, errMissingChanName
	}
	publish := map[string]interface{}{"action": "publish", "name": rqst.Evt.Name, "data": rqst.Evt.Data}
	srv.actions <- publish
	return &eventpb.PublishResponse{Result: true}, nil
}

///////////////////////////////////////////////////////////////////////////////
// CLI helpers
///////////////////////////////////////////////////////////////////////////////

func printUsage() {
	fmt.Fprintf(os.Stdout, `
Usage: %s [options] <id> [configPath]

Options:
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

Arguments:
  <id>            Service instance ID
  [configPath]    Optional path to configuration file

`, filepath.Base(os.Args[0]))
}

///////////////////////////////////////////////////////////////////////////////
// main
///////////////////////////////////////////////////////////////////////////////

func main() {
	// Structured logger to stdout
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Build skeleton (no etcd/config yet)
	srv := new(server)
	srv.logger = logger
	srv.Name = string(eventpb.File_event_proto.Services().Get(0).FullName())
	srv.Proto = eventpb.File_event_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "Event service"
	srv.Keywords = []string{"Event", "PubSub", "Subscribe", "Publish"}
	srv.Repositories = []string{}
	srv.Discoveries = []string{}
	srv.Dependencies = []string{}
	srv.Permissions = []interface{}{} // none required by default; fill here if you have RBAC rules
	srv.Process = -1
	srv.ProxyProcess = -1
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.AllowAllOrigins = allow_all_origins
	srv.AllowedOrigins = allowed_origins

	// e.g., right after you build `srv` in main() — before Describe/Init.
	{
		res := func(field, perm string) map[string]interface{} {
			// `field` supports dot-paths for nested payloads (e.g., "Evt.Name").
			return map[string]interface{}{"index": 0, "field": field, "permission": perm}
		}
		rule := func(action, perm string, r ...map[string]interface{}) map[string]interface{} {
			m := map[string]interface{}{"action": action, "permission": perm}
			if len(r) > 0 {
				rs := make([]any, 0, len(r))
				for _, x := range r {
					rs = append(rs, x)
				}
				m["resources"] = rs
			}
			return m
		}

		srv.Permissions = []any{
			// stream lifecycle (per-stream UUID)
			rule("/event.EventService/OnEvent", "read", res("Uuid", "read")),
			rule("/event.EventService/Quit", "read", res("Uuid", "read")),

			// channel membership (per channel name)
			rule("/event.EventService/Subscribe", "read", res("Name", "read")),
			rule("/event.EventService/UnSubscribe", "read", res("Name", "read")),

			// publishing (per channel name; nested field)
			rule("/event.EventService/Publish", "write", res("Evt.Name", "write")),

			// admin
			rule("/event.EventService/Stop", "write"),
		}
	}

	// CLI flags BEFORE touching config
	args := os.Args[1:]
	if len(args) == 0 {
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
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			srv.Process = os.Getpid()
			srv.State = "starting"
			// fill domain/address from env with sane defaults
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
			// IMPORTANT: Permissions already initialized above, so DescribeJSON will include them.
			b, err := globular.DescribeJSON(srv)
			if err != nil {
				logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if srv.Port == 0 || srv.Name == "" {
				logger.Error("health error: uninitialized", "service", srv.Name, "port", srv.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--version", "-v":
			fmt.Fprintf(os.Stdout, "%s\n", srv.Version)
			return
		}
	}

	// Optional positional args: <id> [configPath]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Safe to touch config now
	if d, err := config.GetDomain(); err == nil {
		srv.Domain = d
	} else {
		srv.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	// Register client constructor for dynamic routing
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register RPCs
	eventpb.RegisterEventServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	// Start event manager
	go srv.run()

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"address", srv.Address,
		"listen_ms", time.Since(start).Milliseconds())

	// Start gRPC
	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}
