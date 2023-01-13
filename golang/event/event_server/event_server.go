package main

import (
	"context"
	"fmt"

	"log"
	"os"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	globular "github.com/globulario/services/golang/globular_service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// TODO take care of TLS/https
var (
	defaultPort  = 10050
	defaultProxy = 10051

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	// the default domain.
	domain string = "localhost"
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
	Mac             string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Domain          string
	Address         string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	LastError       string
	ModTime         int64
	State           string

	// event_server-signed X.509 public keys for distribution
	CertFile string
	// a private RSA key to sign and authenticate the public key
	KeyFile string
	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherId        string
	KeepUpToDate       bool
	Checksum           string
	Plaform            string
	KeepAlive          bool
	Permissions        []interface{} // contains the action permission for the services.
	Dependencies       []string      // The list of services needed by this services.

	// The grpc server.
	grpcServer *grpc.Server

	// Use to sync event channel manipulation.
	actions chan map[string]interface{}

	// stop the processing loop.
	exit chan bool
}

// The http address where the configuration can be found /config
func (svr *server) GetAddress() string {
	return svr.Address
}

func (svr *server) SetAddress(address string) {
	svr.Address = address
}

func (svr *server) GetProcess() int {
	return svr.Process
}

func (svr *server) SetProcess(pid int) {
	svr.Process = pid
}

func (svr *server) GetProxyProcess() int {
	return svr.ProxyProcess
}

func (svr *server) SetProxyProcess(pid int) {
	svr.ProxyProcess = pid
}

// The current service state
func (svr *server) GetState() string {
	return svr.State
}

func (svr *server) SetState(state string) {
	svr.State = state
}

// The last error
func (svr *server) GetLastError() string {
	return svr.LastError
}

func (svr *server) SetLastError(err string) {
	svr.LastError = err
}

// The modeTime
func (svr *server) SetModTime(modtime int64) {
	svr.ModTime = modtime
}
func (svr *server) GetModTime() int64 {
	return svr.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (event_server *server) GetId() string {
	return event_server.Id
}
func (event_server *server) SetId(id string) {
	event_server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (event_server *server) GetName() string {
	return event_server.Name
}
func (event_server *server) SetName(name string) {
	event_server.Name = name
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The description of the service
func (event_server *server) GetDescription() string {
	return event_server.Description
}
func (event_server *server) SetDescription(description string) {
	event_server.Description = description
}

// The list of keywords of the services.
func (event_server *server) GetKeywords() []string {
	return event_server.Keywords
}
func (event_server *server) SetKeywords(keywords []string) {
	event_server.Keywords = keywords
}

// Dist
func (event_server *server) Dist(path string) (string, error) {

	return globular.Dist(path, event_server)
}

func (server *server) GetDependencies() []string {

	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	return server.Dependencies
}

func (server *server) SetDependency(dependency string) {
	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	// Append the depency to the list.
	if !Utility.Contains(server.Dependencies, dependency) {
		server.Dependencies = append(server.Dependencies, dependency)
	}
}

func (svr *server) GetChecksum() string {

	return svr.Checksum
}

func (svr *server) SetChecksum(checksum string) {
	svr.Checksum = checksum
}

func (svr *server) GetPlatform() string {
	return svr.Plaform
}

func (svr *server) SetPlatform(platform string) {
	svr.Plaform = platform
}

// The path of the executable.
func (event_server *server) GetPath() string {
	return event_server.Path
}
func (event_server *server) SetPath(path string) {
	event_server.Path = path
}

// The path of the .proto file.
func (event_server *server) GetProto() string {
	return event_server.Proto
}
func (event_server *server) SetProto(proto string) {
	event_server.Proto = proto
}

// The gRpc port.
func (event_server *server) GetPort() int {
	return event_server.Port
}
func (event_server *server) SetPort(port int) {
	event_server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (event_server *server) GetProxy() int {
	return event_server.Proxy
}
func (event_server *server) SetProxy(proxy int) {
	event_server.Proxy = proxy
}

// Can be one of http/https/tls
func (event_server *server) GetProtocol() string {
	return event_server.Protocol
}
func (event_server *server) SetProtocol(protocol string) {
	event_server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (event_server *server) GetAllowAllOrigins() bool {
	return event_server.AllowAllOrigins
}
func (event_server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	event_server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (event_server *server) GetAllowedOrigins() string {
	return event_server.AllowedOrigins
}

func (event_server *server) SetAllowedOrigins(allowedOrigins string) {
	event_server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (event_server *server) GetDomain() string {
	return event_server.Domain
}
func (event_server *server) SetDomain(domain string) {
	event_server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (event_server *server) GetTls() bool {
	return event_server.TLS
}
func (event_server *server) SetTls(hasTls bool) {
	event_server.TLS = hasTls
}

// The certificate authority file
func (event_server *server) GetCertAuthorityTrust() string {
	return event_server.CertAuthorityTrust
}
func (event_server *server) SetCertAuthorityTrust(ca string) {
	event_server.CertAuthorityTrust = ca
}

// The certificate file.
func (event_server *server) GetCertFile() string {
	return event_server.CertFile
}
func (event_server *server) SetCertFile(certFile string) {
	event_server.CertFile = certFile
}

// The key file.
func (event_server *server) GetKeyFile() string {
	return event_server.KeyFile
}
func (event_server *server) SetKeyFile(keyFile string) {
	event_server.KeyFile = keyFile
}

// The service version
func (event_server *server) GetVersion() string {
	return event_server.Version
}
func (event_server *server) SetVersion(version string) {
	event_server.Version = version
}

// The publisher id.
func (event_server *server) GetPublisherId() string {
	return event_server.PublisherId
}
func (event_server *server) SetPublisherId(publisherId string) {
	event_server.PublisherId = publisherId
}

func (event_server *server) GetRepositories() []string {
	return event_server.Repositories
}
func (event_server *server) SetRepositories(repositories []string) {
	event_server.Repositories = repositories
}

func (event_server *server) GetDiscoveries() []string {
	return event_server.Discoveries
}
func (event_server *server) SetDiscoveries(discoveries []string) {
	event_server.Discoveries = discoveries
}

func (event_server *server) GetKeepUpToDate() bool {
	return event_server.KeepUpToDate
}
func (event_server *server) SetKeepUptoDate(val bool) {
	event_server.KeepUpToDate = val
}

func (event_server *server) GetKeepAlive() bool {
	return event_server.KeepAlive
}
func (event_server *server) SetKeepAlive(val bool) {
	event_server.KeepAlive = val
}

func (event_server *server) GetPermissions() []interface{} {
	return event_server.Permissions
}
func (event_server *server) SetPermissions(permissions []interface{}) {
	event_server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (event_server *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)

	err := globular.InitService(event_server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	event_server.grpcServer, err = globular.InitGrpcServer(event_server /*interceptors.ServerUnaryInterceptor*/, nil /*interceptors.ServerStreamInterceptor*/, nil)
	if err != nil {
		return err
	}

	event_server.exit = make(chan bool)

	return nil

}

// Save the configuration values.
func (event_server *server) Save() error {
	// Create the file...
	return globular.SaveService(event_server)
}

func (event_server *server) StartService() error {
	return globular.StartService(event_server, event_server.grpcServer)
}

func (event_server *server) StopService() error {
	event_server.grpcServer.Stop()
	return nil //globular.StopService(event_server, event_server.grpcServer)
}

func (event_server *server) Stop(context.Context, *eventpb.StopRequest) (*eventpb.StopResponse, error) {
	event_server.exit <- true
	fmt.Println(`Stop event server was called.`)
	return &eventpb.StopResponse{}, event_server.StopService()
}

//////////////////////////////////////////////////////////////////////////////
//	Services implementation.
//////////////////////////////////////////////////////////////////////////////

// That function process channel operation and run in it own go routine.
func (event_server *server) run() {

	channels := make(map[string][]string)
	streams := make(map[string]eventpb.EventService_OnEventServer)
	quits := make(map[string]chan bool)

	// Here will create the action channel.
	event_server.actions = make(chan map[string]interface{})

	for {
		select {
		case <-event_server.exit:
			break
		case a := <-event_server.actions:

			action := a["action"].(string)
			//fmt.Println("event server action received: ", action)

			if action == "onevent" {
				streams[a["uuid"].(string)] = a["stream"].(eventpb.EventService_OnEventServer)
				quits[a["uuid"].(string)] = a["quit"].(chan bool)
			} else if action == "subscribe" {

				if channels[a["name"].(string)] == nil {
					channels[a["name"].(string)] = make([]string, 0)
				}

				if !Utility.Contains(channels[a["name"].(string)], a["uuid"].(string)) {
					fmt.Println("subscribe: ", a["name"].(string), a["uuid"].(string))
					channels[a["name"].(string)] = append(channels[a["name"].(string)], a["uuid"].(string))
				}
			} else if action == "publish" {
				// Publish event only if the channel exist. The channel will be created when
				// the first subcriber is register and delete when the last subscriber unsubscribe.
				if channels[a["name"].(string)] != nil {
					toDelete := make([]string, 0)
					for i := 0; i < len(channels[a["name"].(string)]); i++ {
						uuid := channels[a["name"].(string)][i]
						stream := streams[uuid]
						if stream != nil {
							// Here I will send data to stream.
							err := stream.Send(&eventpb.OnEventResponse{
								Evt: &eventpb.Event{
									Name: a["name"].(string),
									Data: a["data"].([]byte),
								},
							})

							// In case of error I will remove the subscriber
							// from the list.
							if err != nil {
								// append to channle list to be close.
								toDelete = append(toDelete, uuid)
							}
						} else {
							log.Println("connection stream with ", uuid, "is nil!")
						}
					}

					// remove closed channel
					for i := 0; i < len(toDelete); i++ {
						uuid := toDelete[i]
						// remove uuid from all channels.
						for name, channel := range channels {
							uuids := make([]string, 0)
							for i := 0; i < len(channel); i++ {
								if uuid != channel[i] {
									uuids = append(uuids, channel[i])
								}
							}
							channels[name] = uuids
						}
						// return from OnEvent
						quits[uuid] <- true
						// remove the channel from the map.
						delete(quits, uuid)
					}
				}
			} else if action == "unsubscribe" {
				uuids := make([]string, 0)
				for i := 0; i < len(channels[a["name"].(string)]); i++ {
					if a["uuid"].(string) != channels[a["name"].(string)][i] {
						uuids = append(uuids, channels[a["name"].(string)][i])
					}
				}
				channels[a["name"].(string)] = uuids
			} else if action == "quit" {
				// remove uuid from all channels.
				for name, channel := range channels {
					uuids := make([]string, 0)
					for i := 0; i < len(channel); i++ {
						if a["uuid"].(string) != channel[i] {
							uuids = append(uuids, channel[i])
						}
					}
					channels[name] = uuids
				}
				// return from OnEvent
				quits[a["uuid"].(string)] <- true
				// remove the channel from the map.
				delete(quits, a["uuid"].(string))
			}
		}
	}
}

// Connect to an event channel or create it if it not already exist
// and stay in that function until UnSubscribe is call.
func (event_server *server) Quit(ctx context.Context, rqst *eventpb.QuitRequest) (*eventpb.QuitResponse, error) {
	quit := make(map[string]interface{})
	quit["action"] = "quit"
	quit["uuid"] = rqst.Uuid

	event_server.actions <- quit

	return &eventpb.QuitResponse{
		Result: true,
	}, nil
}

// Connect to an event channel or create it if it not already exist
// and stay in that function until UnSubscribe is call.
func (event_server *server) OnEvent(rqst *eventpb.OnEventRequest, stream eventpb.EventService_OnEventServer) error {

	onevent := make(map[string]interface{})
	onevent["action"] = "onevent"
	onevent["stream"] = stream
	onevent["uuid"] = rqst.Uuid
	onevent["quit"] = make(chan bool)

	event_server.actions <- onevent

	// wait util unsbscribe or connection is close.
	<-onevent["quit"].(chan bool)

	return nil
}

func (event_server *server) Subscribe(ctx context.Context, rqst *eventpb.SubscribeRequest) (*eventpb.SubscribeResponse, error) {
	subscribe := make(map[string]interface{})
	subscribe["action"] = "subscribe"
	subscribe["name"] = rqst.Name
	subscribe["uuid"] = rqst.Uuid

	event_server.actions <- subscribe

	return &eventpb.SubscribeResponse{
		Result: true,
	}, nil
}

// Disconnect to an event channel.(Return from Subscribe)
func (event_server *server) UnSubscribe(ctx context.Context, rqst *eventpb.UnSubscribeRequest) (*eventpb.UnSubscribeResponse, error) {
	unsubscribe := make(map[string]interface{})
	unsubscribe["action"] = "unsubscribe"
	unsubscribe["name"] = rqst.Name
	unsubscribe["uuid"] = rqst.Uuid

	event_server.actions <- unsubscribe

	return &eventpb.UnSubscribeResponse{
		Result: true,
	}, nil
}

// Publish event on channel.
func (event_server *server) Publish(ctx context.Context, rqst *eventpb.PublishRequest) (*eventpb.PublishResponse, error) {
	publish := make(map[string]interface{})
	publish["action"] = "publish"
	publish["name"] = rqst.Evt.Name
	publish["data"] = rqst.Evt.Data

	// publish the data.
	event_server.actions <- publish
	return &eventpb.PublishResponse{
		Result: true,
	}, nil

}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// The first argument must be the port number to listen to.

	// The actual server implementation.
	s_impl := new(server)
	s_impl.Name = string(eventpb.File_event_proto.Services().Get(0).FullName())
	s_impl.Proto = eventpb.File_event_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true

	// TODO set it from the program arguments...
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1]
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		fmt.Println("Fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
		return
	}

	// Register the echo services
	eventpb.RegisterEventServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Here I will make a signal hook to interrupt to exit cleanly.
	go s_impl.run()

	// Start the service.
	err = s_impl.StartService()

	if err != nil {
		fmt.Println("Fail to start service %s: %s", s_impl.Name, s_impl.Id)
		return
	}

}
