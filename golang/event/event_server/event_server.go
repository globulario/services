package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"os"

	Utility "github.com/davecourtois/!utility"
	"github.com/globulario/services/golang/config"
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
	ConfigPath      string
	LastError       string
	ModTime         int64
	State           string

	// srv-signed X.509 public keys for distribution
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

// The path of the configuration.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
}

// The http address where the configuration can be found /config
func (srv *server) GetAddress() string {
	return srv.Address
}

func (srv *server) SetAddress(address string) {
	srv.Address = address
}

func (srv *server) GetProcess() int {
	return srv.Process
}

func (srv *server) SetProcess(pid int) {
	srv.Process = pid
}

func (srv *server) GetProxyProcess() int {
	return srv.ProxyProcess
}

func (srv *server) SetProxyProcess(pid int) {
	srv.ProxyProcess = pid
}

// The current service state
func (srv *server) GetState() string {
	return srv.State
}

func (srv *server) SetState(state string) {
	srv.State = state
}

// The last error
func (srv *server) GetLastError() string {
	return srv.LastError
}

func (srv *server) SetLastError(err string) {
	srv.LastError = err
}

// The modeTime
func (srv *server) SetModTime(modtime int64) {
	srv.ModTime = modtime
}
func (srv *server) GetModTime() int64 {
	return srv.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (srv *server) GetId() string {
	return srv.Id
}
func (srv *server) SetId(id string) {
	srv.Id = id
}

// The name of a service, must be the gRpc Service name.
func (srv *server) GetName() string {
	return srv.Name
}
func (srv *server) SetName(name string) {
	srv.Name = name
}

func (srv *server) GetMac() string {
	return srv.Mac
}

func (srv *server) SetMac(mac string) {
	srv.Mac = mac
}

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}
func (srv *server) SetDescription(description string) {
	srv.Description = description
}

// The list of keywords of the services.
func (srv *server) GetKeywords() []string {
	return srv.Keywords
}
func (srv *server) SetKeywords(keywords []string) {
	srv.Keywords = keywords
}

// Dist
func (srv *server) Dist(path string) (string, error) {

	return globular.Dist(path, srv)
}

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

	// Append the depency to the list.
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

func (srv *server) GetChecksum() string {

	return srv.Checksum
}

func (srv *server) SetChecksum(checksum string) {
	srv.Checksum = checksum
}

func (srv *server) GetPlatform() string {
	return srv.Plaform
}

func (srv *server) SetPlatform(platform string) {
	srv.Plaform = platform
}

// The path of the executable.
func (srv *server) GetPath() string {
	return srv.Path
}
func (srv *server) SetPath(path string) {
	srv.Path = path
}

// The path of the .proto file.
func (srv *server) GetProto() string {
	return srv.Proto
}
func (srv *server) SetProto(proto string) {
	srv.Proto = proto
}

// The gRpc port.
func (srv *server) GetPort() int {
	return srv.Port
}
func (srv *server) SetPort(port int) {
	srv.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (srv *server) GetProxy() int {
	return srv.Proxy
}
func (srv *server) SetProxy(proxy int) {
	srv.Proxy = proxy
}

// Can be one of http/https/tls
func (srv *server) GetProtocol() string {
	return srv.Protocol
}
func (srv *server) SetProtocol(protocol string) {
	srv.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (srv *server) GetAllowAllOrigins() bool {
	return srv.AllowAllOrigins
}
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) {
	srv.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (srv *server) GetAllowedOrigins() string {
	return srv.AllowedOrigins
}

func (srv *server) SetAllowedOrigins(allowedOrigins string) {
	srv.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (srv *server) GetDomain() string {
	return srv.Domain
}
func (srv *server) SetDomain(domain string) {
	srv.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (srv *server) GetTls() bool {
	return srv.TLS
}
func (srv *server) SetTls(hasTls bool) {
	srv.TLS = hasTls
}

// The certificate authority file
func (srv *server) GetCertAuthorityTrust() string {
	return srv.CertAuthorityTrust
}
func (srv *server) SetCertAuthorityTrust(ca string) {
	srv.CertAuthorityTrust = ca
}

// The certificate file.
func (srv *server) GetCertFile() string {
	return srv.CertFile
}
func (srv *server) SetCertFile(certFile string) {
	srv.CertFile = certFile
}

// The key file.
func (srv *server) GetKeyFile() string {
	return srv.KeyFile
}
func (srv *server) SetKeyFile(keyFile string) {
	srv.KeyFile = keyFile
}

// The service version
func (srv *server) GetVersion() string {
	return srv.Version
}
func (srv *server) SetVersion(version string) {
	srv.Version = version
}

// The publisher id.
func (srv *server) GetPublisherId() string {
	return srv.PublisherId
}
func (srv *server) SetPublisherId(publisherId string) {
	srv.PublisherId = publisherId
}

func (srv *server) GetRepositories() []string {
	return srv.Repositories
}
func (srv *server) SetRepositories(repositories []string) {
	srv.Repositories = repositories
}

func (srv *server) GetDiscoveries() []string {
	return srv.Discoveries
}
func (srv *server) SetDiscoveries(discoveries []string) {
	srv.Discoveries = discoveries
}

func (srv *server) GetKeepUpToDate() bool {
	return srv.KeepUpToDate
}
func (srv *server) SetKeepUptoDate(val bool) {
	srv.KeepUpToDate = val
}

func (srv *server) GetKeepAlive() bool {
	return srv.KeepAlive
}
func (srv *server) SetKeepAlive(val bool) {
	srv.KeepAlive = val
}

func (srv *server) GetPermissions() []interface{} {
	return srv.Permissions
}
func (srv *server) SetPermissions(permissions []interface{}) {
	srv.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (srv *server) Init() error {

	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv /*interceptors.ServerUnaryInterceptor*/, nil /*interceptors.ServerStreamInterceptor*/, nil)
	if err != nil {
		return err
	}

	srv.exit = make(chan bool)

	return nil

}

// Save the configuration values.
func (srv *server) Save() error {
	// Create the file...
	return globular.SaveService(srv)
}

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	srv.grpcServer.Stop()
	return nil //globular.StopService(srv, srv.grpcServer)
}

func (srv *server) Stop(context.Context, *eventpb.StopRequest) (*eventpb.StopResponse, error) {
	srv.exit <- true
	fmt.Println(`Stop event server was called.`)
	return &eventpb.StopResponse{}, srv.StopService()
}

//////////////////////////////////////////////////////////////////////////////
//	Services implementation.
//////////////////////////////////////////////////////////////////////////////

// That function process channel operation and run in it own go routine.
func (srv *server) run() {

	channels := make(map[string][]string)
	streams := make(map[string]eventpb.EventService_OnEventServer)
	quits := make(map[string]chan bool)
	ka := make(chan *eventpb.KeepAlive)

	// Here will create the action channel.
	srv.actions = make(chan map[string]interface{})

	// validate stream at interval of 5 second
	// it will prevent the stream to be close by grpc...
	// and non responding stream will be remove from the list of listener.
	ticker := time.NewTicker(15 * time.Second)
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

	for {
		select {
		case <-srv.exit:
			break

		case ka_ := <-ka:
			for uuid, stream := range streams {
				err := stream.Send(&eventpb.OnEventResponse{
					Data: &eventpb.OnEventResponse_Ka{
						Ka: ka_,
					},
				})

				if err != nil {
					// invalidate the stream in that case.
					go func(quit chan bool) {
						quit <- true
					}(quits[uuid])

					// remove the channel from the map.
					delete(quits, uuid)
				}
			}

		case a := <-srv.actions:

			action := a["action"].(string)

			if action == "onevent" {
				streams[a["uuid"].(string)] = a["stream"].(eventpb.EventService_OnEventServer)
				quits[a["uuid"].(string)] = a["quit"].(chan bool)
			} else if action == "subscribe" {
				if channels[a["name"].(string)] == nil {
					channels[a["name"].(string)] = make([]string, 0)
				}

				if !Utility.Contains(channels[a["name"].(string)], a["uuid"].(string)) {
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
								Data: &eventpb.OnEventResponse_Evt{
									Evt: &eventpb.Event{
										Name: a["name"].(string),
										Data: a["data"].([]byte),
									},
								},
							})

							// In case of error I will remove the subscriber
							// from the list.
							if err != nil {
								// append to channle list to be close.
								fmt.Println("fail to send message over stream ", uuid, " with error ", err)
								toDelete = append(toDelete, uuid)
							}
						} else {
							fmt.Println("remove stream ", uuid)
							toDelete = append(toDelete, uuid)
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
						go func(quit chan bool) {
							quit <- true
						}(quits[uuid])

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
func (srv *server) Quit(ctx context.Context, rqst *eventpb.QuitRequest) (*eventpb.QuitResponse, error) {
	quit := make(map[string]interface{})
	quit["action"] = "quit"
	quit["uuid"] = rqst.Uuid

	srv.actions <- quit

	return &eventpb.QuitResponse{
		Result: true,
	}, nil
}

// Connect to an event channel or create it if it not already exist
// and stay in that function until UnSubscribe is call.
func (srv *server) OnEvent(rqst *eventpb.OnEventRequest, stream eventpb.EventService_OnEventServer) error {

	onevent := make(map[string]interface{})
	onevent["action"] = "onevent"
	onevent["stream"] = stream
	onevent["uuid"] = rqst.Uuid
	onevent["quit"] = make(chan bool)

	srv.actions <- onevent

	// wait util unsbscribe or connection is close.
	<-onevent["quit"].(chan bool)

	fmt.Println("lister ", rqst.Uuid, "quit")

	return nil
}

func (srv *server) Subscribe(ctx context.Context, rqst *eventpb.SubscribeRequest) (*eventpb.SubscribeResponse, error) {
	subscribe := make(map[string]interface{})
	subscribe["action"] = "subscribe"
	subscribe["name"] = rqst.Name
	subscribe["uuid"] = rqst.Uuid

	srv.actions <- subscribe

	return &eventpb.SubscribeResponse{
		Result: true,
	}, nil
}

// Disconnect to an event channel.(Return from Subscribe)
func (srv *server) UnSubscribe(ctx context.Context, rqst *eventpb.UnSubscribeRequest) (*eventpb.UnSubscribeResponse, error) {
	unsubscribe := make(map[string]interface{})
	unsubscribe["action"] = "unsubscribe"
	unsubscribe["name"] = rqst.Name
	unsubscribe["uuid"] = rqst.Uuid

	srv.actions <- unsubscribe

	return &eventpb.UnSubscribeResponse{
		Result: true,
	}, nil
}

// Publish event on channel.
func (srv *server) Publish(ctx context.Context, rqst *eventpb.PublishRequest) (*eventpb.PublishResponse, error) {

	publish := make(map[string]interface{})
	publish["action"] = "publish"
	publish["name"] = rqst.Evt.Name
	publish["data"] = rqst.Evt.Data

	// publish the data.
	srv.actions <- publish
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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "localhost"
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

	// Register the client function, so it can be use for dynamic routing, (ex: ["GetFile", "round-robin"])
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)

	// TODO set it from the program arguments...
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s with error %s", s_impl.Name, s_impl.Id, err.Error())
		return
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Register the echo services
	eventpb.RegisterEventServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Here I will make a signal hook to interrupt to exit cleanly.
	go s_impl.run()

	// Start the service.
	err = s_impl.StartService()

	if err != nil {
		fmt.Printf("Fail to start service %s: %s\n", s_impl.Name, s_impl.Id)
		return
	}

}
