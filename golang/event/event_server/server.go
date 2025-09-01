package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	globular "github.com/globulario/services/golang/globular_service"
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
	CertFile            string
	KeyFile             string
	CertAuthorityTrust  string
	TLS                 bool
	Version             string
	PublisherID         string
	KeepUpToDate        bool
	Checksum            string
	Plaform             string
	KeepAlive           bool
	Permissions         []interface{} // action permissions for the service
	Dependencies        []string      // required services

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

// GetConfigurationPath returns the path to the service configuration file.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path to the service configuration file.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP(S) address where configuration is available (e.g., /config).
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP(S) address where configuration is available.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the PID of this service process (or -1 if not started).
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the PID of this service process.
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the PID of the reverse proxy process (or -1 if not started).
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the PID of the reverse proxy process.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state string.
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state string.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message (if any).
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError sets the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification time (unix seconds).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the last modification time (unix seconds).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the unique ID of this service instance.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the unique ID of this service instance.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetMac returns the MAC address associated with this service (if any).
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address associated with this service.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetKeywords returns the keyword list associated with this service.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the keyword list associated with this service.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// Dist packages the service for distribution into path and returns the resulting artifact path.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the service dependencies.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency adds a service dependency if not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the service binary checksum string.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the service binary checksum string.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the target platform string.
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the target platform string.
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the path to the executable.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the path to the executable.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the path to the .proto file used by the service.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the path to the .proto file used by the service.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse proxy port (gRPC-Web).
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse proxy port (gRPC-Web).
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the transport protocol ("grpc", "http", "https", or "tls").
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the transport protocol ("grpc", "http", "https", or "tls").
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins reports whether all CORS origins are allowed.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether all CORS origins are allowed.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated list of allowed CORS origins.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed CORS origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the service domain (hostname).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the service domain (hostname).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls reports whether TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA trust file path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA trust file path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the TLS certificate file path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the TLS certificate file path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the TLS private key file path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the TLS private key file path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version string.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version string.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher ID.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher ID.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetRepositories returns discovery repositories for this service.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets discovery repositories for this service.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns discovery endpoints for this service.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets discovery endpoints for this service.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// GetKeepUpToDate reports whether the service should auto-update.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate sets whether the service should auto-update.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive reports whether the service should be relaunched if stopped.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive sets whether the service should be relaunched if stopped.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns action permissions for this service.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets action permissions for this service.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

///////////////////////////////////////////////////////////////////////////////
// Lifecycle
///////////////////////////////////////////////////////////////////////////////

// Init initializes service configuration and gRPC server.
func (srv *server) Init() error {
	if srv.logger == nil {
		// Fallback logger if not injected; text to stdout at INFO.
		srv.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	if err := globular.InitService(srv); err != nil {
		srv.logger.Error("init service failed", "service", srv.Name, "id", srv.Id, "err", err)
		return err
	}

	grpcSrv, err := globular.InitGrpcServer(srv, nil, nil)
	if err != nil {
		srv.logger.Error("init grpc server failed", "service", srv.Name, "id", srv.Id, "err", err)
		return err
	}
	srv.grpcServer = grpcSrv
	srv.exit = make(chan bool)
	srv.logger.Info("service initialized", "service", srv.Name, "id", srv.Id)
	return nil
}

// Save persists the current service configuration to disk.
func (srv *server) Save() error {
	if srv == nil {
		return errNotInitialized
	}
	return globular.SaveService(srv)
}

// StartService starts the gRPC server and reverse proxy as needed.
func (srv *server) StartService() error {
	if srv.grpcServer == nil {
		return errServerNil
	}
	srv.logger.Info("starting service", "service", srv.Name, "id", srv.Id, "port", srv.Port, "proxy", srv.Proxy, "protocol", srv.Protocol)
	return globular.StartService(srv, srv.grpcServer)
}

// StopService stops the gRPC server only (globular will manage proxy lifecycle).
func (srv *server) StopService() error {
	if srv.grpcServer == nil {
		return nil
	}
	srv.grpcServer.Stop()
	srv.logger.Info("service stopped", "service", srv.Name, "id", srv.Id)
	return nil
}

// Stop implements eventpb.EventServiceServer Stop RPC, signaling the service to quit.
func (srv *server) Stop(ctx context.Context, _ *eventpb.StopRequest) (*eventpb.StopResponse, error) {
	srv.logger.Info("stop requested", "service", srv.Name, "id", srv.Id)
	srv.exit <- true
	return &eventpb.StopResponse{}, srv.StopService()
}

///////////////////////////////////////////////////////////////////////////////
// Event subsystem
///////////////////////////////////////////////////////////////////////////////

// run manages subscriptions and publications via a serialized action channel.
func (srv *server) run() {
	if srv.logger == nil {
		srv.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}

	channels := make(map[string][]string)                          // channel name -> list of subscriber UUIDs
	streams := make(map[string]eventpb.EventService_OnEventServer) // UUID -> stream
	quits := make(map[string]chan bool)                            // UUID -> quit chan
	ka := make(chan *eventpb.KeepAlive)

	srv.actions = make(chan map[string]interface{})

	// Heartbeat (keep stream alive)
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
			// Drain and close
			close(done)
			// Close all streams cleanly
			for uuid, q := range quits {
				select {
				case q <- true:
				default:
				}
				delete(quits, uuid)
			}
			srv.logger.Info("event loop stopped", "service", srv.Name, "id", srv.Id)
			return

		case ka_ := <-ka:
			// Broadcast keepalive; drop dead streams
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
				srv.cleanupSubscribers(toDelete, channels, quits)
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
					srv.logger.Info("subscribed", "channel", name, "uuid", uuid)
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
					// no subscribers; silently ignore
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
					srv.cleanupSubscribers(toDelete, channels, quits)
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
				channels[name] = uuids
				srv.logger.Info("unsubscribed", "channel", name, "uuid", uuid)

			case "quit":
				uuid, _ := a["uuid"].(string)
				if uuid == "" {
					srv.logger.Error("invalid quit request: missing uuid")
					continue
				}
				// remove uuid from all channels
				for name, ch := range channels {
					uuids := make([]string, 0, len(ch))
					for _, id := range ch {
						if id != uuid {
							uuids = append(uuids, id)
						}
					}
					channels[name] = uuids
				}
				// signal the stream goroutine to return
				if q, ok := quits[uuid]; ok {
					select {
					case q <- true:
					default:
					}
					delete(quits, uuid)
				}
				srv.logger.Info("stream quit", "uuid", uuid)

			default:
				srv.logger.Warn("unknown action", "action", action)
			}
		}
	}
}

// cleanupSubscribers removes the provided UUIDs from all channels and signals their quit chans.
func (srv *server) cleanupSubscribers(toDelete []string, channels map[string][]string, quits map[string]chan bool) {
	for _, uuid := range toDelete {
		// remove uuid from all channels
		for name, ch := range channels {
			uuids := make([]string, 0, len(ch))
			for _, id := range ch {
				if id != uuid {
					uuids = append(uuids, id)
				}
			}
			channels[name] = uuids
		}
		// signal quit (non-blocking)
		if q, ok := quits[uuid]; ok {
			select {
			case q <- true:
			default:
			}
			delete(quits, uuid)
		}
		delete(quits, uuid)
	}
}

///////////////////////////////////////////////////////////////////////////////
// RPCs (public prototypes preserved)
///////////////////////////////////////////////////////////////////////////////

// Quit disconnects a stream by UUID, removing it from all channel subscriptions.
func (srv *server) Quit(ctx context.Context, rqst *eventpb.QuitRequest) (*eventpb.QuitResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("Quit: invalid request", "err", errMissingUUID)
		return &eventpb.QuitResponse{Result: false}, errMissingUUID
	}

	msg := map[string]interface{}{
		"action": "quit",
		"uuid":   rqst.Uuid,
	}
	srv.actions <- msg
	srv.logger.Info("Quit: ok", "uuid", rqst.Uuid)
	return &eventpb.QuitResponse{Result: true}, nil
}

// OnEvent registers a server-side stream associated with rqst.Uuid.
// The call blocks until the client unsubscribes or the connection is closed.
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

	// block until asked to quit
	<-onevent["quit"].(chan bool)
	srv.logger.Info("OnEvent: stream ended", "uuid", rqst.Uuid)
	return nil
}

// Subscribe registers rqst.Uuid to receive events for rqst.Name.
func (srv *server) Subscribe(ctx context.Context, rqst *eventpb.SubscribeRequest) (*eventpb.SubscribeResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("Subscribe: invalid request", "err", errMissingUUID)
		return &eventpb.SubscribeResponse{Result: false}, errMissingUUID
	}
	if rqst.Name == "" {
		srv.logger.Error("Subscribe: invalid request", "err", errMissingChanName)
		return &eventpb.SubscribeResponse{Result: false}, errMissingChanName
	}

	subscribe := map[string]interface{}{
		"action": "subscribe",
		"name":   rqst.Name,
		"uuid":   rqst.Uuid,
	}
	srv.actions <- subscribe
	srv.logger.Info("Subscribe: ok", "channel", rqst.Name, "uuid", rqst.Uuid)
	return &eventpb.SubscribeResponse{Result: true}, nil
}

// UnSubscribe removes rqst.Uuid from the subscribers of rqst.Name.
func (srv *server) UnSubscribe(ctx context.Context, rqst *eventpb.UnSubscribeRequest) (*eventpb.UnSubscribeResponse, error) {
	if rqst == nil || rqst.Uuid == "" {
		srv.logger.Error("UnSubscribe: invalid request", "err", errMissingUUID)
		return &eventpb.UnSubscribeResponse{Result: false}, errMissingUUID
	}
	if rqst.Name == "" {
		srv.logger.Error("UnSubscribe: invalid request", "err", errMissingChanName)
		return &eventpb.UnSubscribeResponse{Result: false}, errMissingChanName
	}

	unsubscribe := map[string]interface{}{
		"action": "unsubscribe",
		"name":   rqst.Name,
		"uuid":   rqst.Uuid,
	}
	srv.actions <- unsubscribe
	srv.logger.Info("UnSubscribe: ok", "channel", rqst.Name, "uuid", rqst.Uuid)
	return &eventpb.UnSubscribeResponse{Result: true}, nil
}

// Publish broadcasts an event to all subscribers of the event channel name.
func (srv *server) Publish(ctx context.Context, rqst *eventpb.PublishRequest) (*eventpb.PublishResponse, error) {
	if rqst == nil || rqst.Evt == nil || rqst.Evt.Name == "" {
		srv.logger.Error("Publish: invalid request", "err", errMissingChanName)
		return &eventpb.PublishResponse{Result: false}, errMissingChanName
	}

	publish := map[string]interface{}{
		"action": "publish",
		"name":   rqst.Evt.Name,
		"data":   rqst.Evt.Data,
	}
	srv.actions <- publish
	srv.logger.Info("Publish: ok", "channel", rqst.Evt.Name, "size", len(rqst.Evt.Data))
	return &eventpb.PublishResponse{Result: true}, nil
}

///////////////////////////////////////////////////////////////////////////////
// main
///////////////////////////////////////////////////////////////////////////////

// main boots and runs the Event service.
func main() {
	// Structured logger to stdout
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Instantiate server
	srv := new(server)
	srv.logger = logger
	srv.Name = string(eventpb.File_event_proto.Services().Get(0).FullName())
	srv.Proto = eventpb.File_event_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Domain, _ = config.GetDomain()
	srv.Address, _ = config.GetAddress()
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Permissions = make([]interface{}, 0)
	srv.Keywords = make([]string, 0)
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)
	srv.Process = -1
	srv.ProxyProcess = -1
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.AllowAllOrigins = allow_all_origins
	srv.AllowedOrigins = allowed_origins

	// Parse optional id/configPath args
	switch len(os.Args) {
	case 2:
		srv.Id = os.Args[1]
	case 3:
		srv.Id = os.Args[1]
		srv.ConfigPath = os.Args[2]
	default:
		// id may also come from config; warn if absent
		if srv.Id == "" {
			logger.Warn("no Id provided on command line; will rely on config file")
		}
	}

	// Register client constructor for dynamic routing
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)

	// Init
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Ensure address is set (can be overridden by config)
	if srv.Address == "" {
		if addr, err := config.GetAddress(); err == nil {
			srv.Address = addr
		}
	}

	// Register RPCs
	eventpb.RegisterEventServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	// Start event manager
	go srv.run()

	// Start gRPC
	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	logger.Info("service started", "service", srv.Name, "id", srv.Id, "port", srv.Port, "proxy", srv.Proxy, "domain", srv.Domain, "address", srv.Address)
}
