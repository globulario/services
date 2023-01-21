package globular_service

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"

	//"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"net/http"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/kardianos/osext"
	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"

	//"github.com/globulario/services/golang/config"
	"github.com/fsnotify/fsnotify"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Grpc Http1.1 / Websocket wrapper.
type WrapperedServerOptions struct {
	Addr                  string
	Cert                  string
	Key                   string
	AllowAllOrigins       bool
	AllowedOrigins        *[]string
	AllowedHeaders        *[]string
	UseWebSocket          bool
	WebsocketPingInterval time.Duration
}

func DefaultWrapperedServerOptions() WrapperedServerOptions {
	return WrapperedServerOptions{
		Addr:                  ":9090",
		Cert:                  "",
		Key:                   "",
		AllowAllOrigins:       true,
		AllowedHeaders:        &[]string{},
		AllowedOrigins:        &[]string{},
		UseWebSocket:          true,
		WebsocketPingInterval: 0,
	}
}

func NewWrapperedServerOptions(addr, cert, key string, websocket bool) WrapperedServerOptions {
	return WrapperedServerOptions{
		Addr:                  addr,
		Cert:                  cert,
		Key:                   key,
		AllowAllOrigins:       true,
		AllowedHeaders:        &[]string{},
		AllowedOrigins:        &[]string{},
		UseWebSocket:          true,
		WebsocketPingInterval: 0,
	}
}

type WrapperedGRPCWebServer struct {
	options    WrapperedServerOptions
	GRPCServer *grpc.Server
}

func NewWrapperedGRPCWebServer(options WrapperedServerOptions, s *grpc.Server) *WrapperedGRPCWebServer {
	return &WrapperedGRPCWebServer{
		options:    options,
		GRPCServer: s,
	}
}

type allowedOrigins struct {
	origins map[string]struct{}
}

func (a *allowedOrigins) IsAllowed(origin string) bool {
	_, ok := a.origins[origin]
	return ok
}

func makeAllowedOrigins(origins []string) *allowedOrigins {
	o := map[string]struct{}{}
	for _, allowedOrigin := range origins {
		o[allowedOrigin] = struct{}{}
	}
	return &allowedOrigins{
		origins: o,
	}
}

func (s *WrapperedGRPCWebServer) makeHTTPOriginFunc(allowedOrigins *allowedOrigins) func(origin string) bool {
	if s.options.AllowAllOrigins {
		return func(origin string) bool {
			return true
		}
	}
	return allowedOrigins.IsAllowed
}

func (s *WrapperedGRPCWebServer) makeWebsocketOriginFunc(allowedOrigins *allowedOrigins) func(req *http.Request) bool {
	if s.options.AllowAllOrigins {
		return func(req *http.Request) bool {
			return true
		}
	}
	return func(req *http.Request) bool {
		origin, err := grpcweb.WebsocketRequestOrigin(req)
		if err != nil {
			fmt.Println(err)
			return false
		}
		return allowedOrigins.IsAllowed(origin)
	}
}

func (s *WrapperedGRPCWebServer) Serve() error {
	addr := s.options.Addr

	if s.options.AllowAllOrigins && s.options.AllowedOrigins != nil && len(*s.options.AllowedOrigins) != 0 {
		fmt.Println("Ambiguous --allow_all_origins and --allow_origins configuration. Either set --allow_all_origins=true OR specify one or more origins to whitelist with --allow_origins, not both.")
	}

	allowedOrigins := makeAllowedOrigins(*s.options.AllowedOrigins)

	options := []grpcweb.Option{
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
		grpcweb.WithOriginFunc(s.makeHTTPOriginFunc(allowedOrigins)),
	}

	if s.options.UseWebSocket {
		fmt.Println("Using websockets")
		options = append(
			options,
			grpcweb.WithWebsockets(true),
			grpcweb.WithWebsocketOriginFunc(s.makeWebsocketOriginFunc(allowedOrigins)),
		)

		if s.options.WebsocketPingInterval >= time.Second {
			fmt.Println("websocket keepalive pinging enabled, the timeout interval is", s.options.WebsocketPingInterval.String())
			options = append(
				options,
				grpcweb.WithWebsocketPingInterval(s.options.WebsocketPingInterval),
			)
		}
	}

	if s.options.AllowedHeaders != nil && len(*s.options.AllowedHeaders) > 0 {
		options = append(
			options,
			grpcweb.WithAllowedRequestHeaders(*s.options.AllowedHeaders),
		)
	}

	wrappedServer := grpcweb.WrapServer(s.GRPCServer, options...)
	handler := func(resp http.ResponseWriter, req *http.Request) {
		wrappedServer.ServeHTTP(resp, req)
	}

	httpServer := http.Server{
		Addr:    addr,
		Handler: http.HandlerFunc(handler),
	}

	var listener net.Listener

	enableTLS := s.options.Cert != "" && s.options.Key != ""

	if enableTLS {
		cer, err := tls.LoadX509KeyPair(s.options.Cert, s.options.Key)
		if err != nil {
			log.Panicf("failed to load x509 key pair: %v", err)
			return err
		}
		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		tls, err := tls.Listen("tcp", addr, config)
		if err != nil {
			log.Panicf("failed to listen: tls %v", err)
			return err
		}
		listener = tls
	} else {
		tcp, err := net.Listen("tcp", addr)
		if err != nil {
			log.Panicf("failed to listen: tcp %v", err)
			return err
		}
		listener = tcp
	}

	fmt.Println("Starting gRPC/gRPC-Web combo server, bind:", addr, "with TLS: ", enableTLS)

	m := cmux.New(listener)
	grpcListener := m.Match(cmux.HTTP2())
	httpListener := m.Match(cmux.HTTP1Fast())
	g := new(errgroup.Group)
	g.Go(func() error { return s.GRPCServer.Serve(grpcListener) })
	g.Go(func() error { return httpServer.Serve(httpListener) })
	g.Go(m.Serve)
	fmt.Println("Run server: ", g.Wait())
	return nil
}

// The client service interface.
type Service interface {

	/** Getter/Setter **/

	// The id of a particular service instance.
	GetId() string
	SetId(string)

	// The name of a service, must be the gRpc Service name.
	GetName() string
	SetName(string)

	// Return the server mac address
	GetMac() string
	SetMac(string)

	// The address where the configuration can from http lnk
	GetAddress() string
	SetAddress(string)

	// The description of the services.
	GetDescription() string
	SetDescription(string)

	// The plaform of the services.
	SetPlatform(string)
	GetPlatform() string

	// The keywords.
	GetKeywords() []string
	SetKeywords([]string)

	// The path of the executable.
	GetPath() string
	SetPath(string)

	// The service state
	GetState() string
	SetState(string)

	// The path of the configuration.
	GetConfigurationPath() string
	SetConfigurationPath(string)

	// The last error
	GetLastError() string
	SetLastError(string)

	// The modeTime
	SetModTime(int64)
	GetModTime() int64

	// The path of the .proto file.
	GetProto() string
	SetProto(string)

	// The gRpc port.
	GetPort() int
	SetPort(int)

	// The reverse proxy port (use by gRpc Web)
	GetProxy() int
	SetProxy(int)

	GetProcess() int
	SetProcess(int)

	GetProxyProcess() int
	SetProxyProcess(int)

	// Can be one of http/https/tls
	GetProtocol() string
	SetProtocol(string)

	GetDiscoveries() []string
	SetDiscoveries([]string)

	GetRepositories() []string
	SetRepositories([]string)

	// Return true if all Origins are allowed to access the mircoservice.
	GetAllowAllOrigins() bool
	SetAllowAllOrigins(bool)

	// If AllowAllOrigins is false then AllowedOrigins will contain the
	// list of address that can reach the services.
	GetAllowedOrigins() string // comma separated string.
	SetAllowedOrigins(string)

	// Can be a ip address or domain name.
	GetDomain() string
	SetDomain(string)

	// This information is use to keep the exec update to the last avalaible exec.
	GetChecksum() string
	SetChecksum(string)

	// TLS section

	// If true the service run with TLS. The
	GetTls() bool
	SetTls(bool)

	// The certificate authority file
	GetCertAuthorityTrust() string
	SetCertAuthorityTrust(string)

	// The certificate file.
	GetCertFile() string
	SetCertFile(string)

	// The key file.
	GetKeyFile() string
	SetKeyFile(string)

	// The service version
	GetVersion() string
	SetVersion(string)

	// The publisher id.
	GetPublisherId() string
	SetPublisherId(string)

	GetKeepUpToDate() bool
	SetKeepUptoDate(bool)

	GetKeepAlive() bool
	SetKeepAlive(bool)

	GetPermissions() []interface{} // contains the action permission for the services.
	SetPermissions([]interface{})

	/** The list of requried service **/
	SetDependency(string)
	GetDependencies() []string

	/** Initialyse the service configuration **/
	Init() error

	/** Save the service configuration **/
	Save() error

	/** Stop the service **/
	StopService() error

	/** Start the service **/
	StartService() error

	/** That function create the dist folder of the services that can be use to
		to publish the services. The content of that folder must respect the structure,
		(path)/
	 **/
	Dist(path string) (string, error)
}

/**
 * Initialise a globular service.
 */
func InitService(s Service) error {

	// The exec path
	execPath, _ := osext.Executable()
	execPath = strings.ReplaceAll(execPath, "\\", "/")
	s.SetPath(execPath)

	if len(os.Args) == 3 {
		s.SetId(os.Args[1])
		s.SetConfigurationPath(strings.ReplaceAll(os.Args[2], "\\", "/"))
	} else if len(os.Args) == 2 {
		s.SetId(os.Args[1])
	} else if len(os.Args) == 1 {
		// Now I will set the path where the configuation will be save in that case.
		dir, _ := osext.ExecutableFolder()
		path := strings.ReplaceAll(dir, "\\", "/")
		s.SetConfigurationPath(path + "/config.json")
	}

	if len(s.GetConfigurationPath()) == 0 {
		fmt.Println("fail to retreive configuration for service " + s.GetId())
		return errors.New("fail to retreive configuration for service " + s.GetId())
	}

	// if the service configuration does not exist.
	if Utility.Exists(s.GetConfigurationPath()) {
		// Here I will get the configuration from the Configuration server...
		config_, err := config_client.GetServiceConfigurationById(s.GetConfigurationPath())
		if err != nil {
			fmt.Println("fail to retreive configuration at path ", s.GetConfigurationPath(), err)
			return err
		}

		// If no configuration was found from the configuration server i will get it from the configuration file.
		str, err := json.Marshal(config_)
		if err != nil {
			fmt.Println("fail to marshal configuration at path ", s.GetConfigurationPath(), err)
			return err
		}

		err = json.Unmarshal(str, &s)
		if err != nil {
			return err
		}
	} else {
		s.SetId(Utility.RandomUUID())
	}

	// Get the execname
	execName := filepath.Base(execPath)

	// set the proto path if is not found from it current configuration.
	if !Utility.Exists(s.GetProto()) {
		// The proto file name
		protoName := execName
		if strings.Contains(protoName, ".") {
			protoName = strings.Split(protoName, ".")[0]
		}

		if strings.Contains(protoName, "_server") {
			protoName = execName[0:strings.LastIndex(protoName, "_server")]
		}
		protoName = protoName + ".proto"

		protopath := execPath[0:strings.Index(execPath, "/services/")] + "/services"
		if s.GetProto() != protopath+"/"+protoName {
			if Utility.Exists(protopath) {
				// set the proto path.
				files, err := Utility.FindFileByName(protopath, protoName)
				if err == nil {
					if len(files) > 0 {
						s.SetProto(files[0])
					} else {
						protoName = s.GetName() + ".proto"
						files, err := Utility.FindFileByName(protopath, protoName)
						if err == nil {
							if len(files) > 0 {
								s.SetProto(files[0])
							} else {
								fmt.Println("459 no proto file found at path ", protopath, "with name", protoName)
							}
						} else {
							fmt.Println("462 no proto file found at path ", protopath, "with name", protoName)
						}
					}
				} else {
					// try with the service name instead...
					protoName = s.GetName() + ".proto"
					files, err := Utility.FindFileByName(protopath, protoName)
					if err == nil {
						if len(files) > 0 {
							s.SetProto(files[0])
						} else {
							fmt.Println("473 no proto file found at path ", protopath, "with name", protoName)
						}
					} else {
						fmt.Println("476 no proto file found at path ", protopath, "with name", protoName)
					}
				}
			}
		}
	}

	// set contextual values.
	address, _ := config.GetAddress()
	domain, _ := config.GetDomain()
	macAddress, _ := Utility.MyMacAddr(Utility.MyLocalIP())

	s.SetMac(macAddress)
	s.SetAddress(address)
	s.SetDomain(domain)

	// here the service is runing...
	s.SetState("running")
	s.SetProcess(os.Getpid())

	// Now the platform.
	s.SetPlatform(runtime.GOOS + "_" + runtime.GOARCH)
	s.SetChecksum(Utility.CreateFileChecksum(execPath))

	fmt.Println("Start service name: ", s.GetName()+":"+s.GetId())
	if len(os.Args) != 3{
		return SaveService(s)
	}
	
	return nil
}

/**
 * Save a globular service.
 */
func SaveService(s Service) error {
	// Set current process
	s.SetModTime(time.Now().Unix())
	config_, err := Utility.ToMap(s)
	if err != nil {
		return err
	}
	return config_client.SaveServiceConfiguration(config_)
}

/**
 * Generate the dist path with all necessary info in it.
 */
func Dist(distPath string, s Service) (string, error) {

	// Create the dist diectories...
	path := distPath + "/" + s.GetPublisherId() + "/" + s.GetName() + "/" + s.GetVersion() + "/" + s.GetId()
	Utility.CreateDirIfNotExist(path)

	// copy the proto file.
	err := Utility.Copy(s.GetProto(), distPath+"/"+s.GetPublisherId()+"/"+s.GetName()+"/"+s.GetVersion()+"/"+s.GetName()+".proto")
	if err != nil {
		return "", err
	}

	// copy the config file.
	config_path := s.GetPath()[0:strings.LastIndex(s.GetPath(), "/")] + "/config.json"
	if !Utility.Exists(config_path) {
		return "", errors.New("No config.json file was found for your service, run your service once to generate it configuration and try again.")
	}

	err = Utility.Copy(config_path, path+"/config.json")
	if err != nil {
		return "", err
	}

	exec_name := s.GetPath()[strings.LastIndex(s.GetPath(), "/")+1:]
	if !Utility.Exists(config_path) {
		return "", errors.New("No config.json file was found for your service, run your service once to generate it configuration and try again.")
	}

	err = Utility.Copy(s.GetPath(), path+"/"+exec_name)
	if err != nil {
		return "", err
	}

	return path, err
}

/** Create a service package **/
func CreateServicePackage(s Service, distPath string, platform string) (string, error) {

	// Take the information from the configuration...
	id := s.GetPublisherId() + "%" + s.GetName() + "%" + s.GetVersion() + "%" + s.GetId() + "%" + platform

	// So here I will create a directory and put file in it...
	path, err := Dist(distPath, s)
	if err != nil {
		return "", err
	}

	// tar + gzip
	var buf bytes.Buffer
	Utility.CompressDir(path, &buf)

	// write the .tar.gzip
	fileToWrite, err := os.OpenFile(os.TempDir()+string(os.PathSeparator)+id+".tar.gz", os.O_CREATE|os.O_RDWR, os.FileMode(0755))
	if err != nil {
		return "", err
	}

	defer fileToWrite.Close()

	if _, err := io.Copy(fileToWrite, &buf); err != nil {
		return "", err
	}

	// Remove the dir when the archive is created.
	err = os.RemoveAll(path)
	if err != nil {
		return "", err
	}

	return os.TempDir() + string(os.PathSeparator) + id + ".tar.gz", nil
}

/**
 * Return the OS and the arch
 */
func GetPlatform() string {
	platform := runtime.GOOS + "_" + runtime.GOARCH
	return platform
}

func GetTLSConfig(key string, cert string, ca string) *tls.Config {
	tlsCer, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		log.Fatalf("Failed to get credentials: %v", err)
	}

	certPool := x509.NewCertPool()
	clientCA, err := ioutil.ReadFile(ca)
	if err != nil {
		log.Fatalf("failed to read client ca cert: %s", err)
	}
	ok := certPool.AppendCertsFromPEM(clientCA)
	if !ok {
		log.Fatal("failed to append client certs")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCer},
		ClientAuth:   tls.RequireAnyClientCert,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			opts := x509.VerifyOptions{
				Roots:         certPool,
				CurrentTime:   time.Now(),
				Intermediates: x509.NewCertPool(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			}

			for _, cert := range rawCerts[1:] {
				opts.Intermediates.AppendCertsFromPEM(cert)
			}

			c, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return errors.New("tls: failed to verify client certificate: " + err.Error())
			}
			_, err = c.Verify(opts)
			if err != nil {
				return errors.New("tls: failed to verify client certificate: " + err.Error())
			}
			return nil
		},
	}
}

/**
 * Initilalsyse the grpc server that will run the service.
 * ** Here is an exemple how to use prometheus for monitoring and the websocket as proxy
 *    https://github.com/pion/ion-sfu/blob/master/cmd/signal/grpc/server/wrapped.go
 */
func InitGrpcServer(s Service, unaryInterceptor grpc.UnaryServerInterceptor, streamInterceptor grpc.StreamServerInterceptor) (*grpc.Server, error) {
	var server *grpc.Server

	if s.GetTls() {

		// Create the TLS credentials
		creds := credentials.NewTLS(GetTLSConfig(s.GetKeyFile(), s.GetCertFile(), s.GetCertAuthorityTrust()))

		// Create the gRPC server with the credentials
		opts := []grpc.ServerOption{grpc.Creds(creds)}
		if unaryInterceptor != nil {
			opts = append(opts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptor, grpc_prometheus.UnaryServerInterceptor)))
		}

		if streamInterceptor != nil {
			opts = append(opts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptor, grpc_prometheus.StreamServerInterceptor)))
		}

		server = grpc.NewServer(opts...)

	} else {
		if unaryInterceptor != nil && streamInterceptor != nil {
			server = grpc.NewServer(
				grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptor, grpc_prometheus.UnaryServerInterceptor)),
				grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptor, grpc_prometheus.StreamServerInterceptor)))
		} else {
			server = grpc.NewServer(grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor), grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor))
		}
	}

	// display server health info...
	grpc_health_v1.RegisterHealthServer(server, health.NewServer())
	grpc_prometheus.Register(server)

	return server, nil
}

var event_client_ *event_client.Event_Client

func getEventClient() (*event_client.Event_Client, error) {
	if event_client_ == nil {
		address, err := config.GetAddress()
		if err != nil {
			return nil, err
		}
		event_client_, err = event_client.NewEventService_Client(address, "event.EventService")
		if err != nil {
			return nil, err
		}
	}

	return event_client_, nil
}

func StartService(s Service, server *grpc.Server) error {

	// First of all I will create a listener.
	// Create the channel to listen on
	var lis net.Listener
	var err error

	lis, err = net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(s.GetPort()))
	if err != nil {
		err_ := errors.New("could not list at domain " + s.GetDomain() + err.Error())
		log.Print(err_)
		return err_
	}

	// Here I will make a signal hook to interrupt to exit cleanly.
	go func() {
		// no web-rpc server.
		fmt.Println("service name: "+s.GetName()+" id:"+s.GetId()+" is listening at gRPC port", s.GetPort(), "and process id is ", s.GetProcess())

		if err := server.Serve(lis); err != nil {
			fmt.Println("service has error ", err)
			return
		}

		// In case we want to use wrapped proxy...
		/*options := NewWrapperedServerOptions("0.0.0.0:" + strconv.Itoa(s.GetPort()), s.GetCertAuthorityTrust(), s.GetKeyFile(), false)
		s.SetProxy(s.GetPort())
		s.Save()

		wrapperedSrv := NewWrapperedGRPCWebServer(options, server)
		if err := wrapperedSrv.Serve(); err != nil {
			fmt.Println("wrappered grpc listening error: ", err)
			return //err
		}*/

	}()

	// Wait for signal to stop.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	/**
	Every breath you take
	And every change you make
	Every bond you break
	Every step you take
	I'll be watching you
	*/
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("NewWatcher failed: ", err)
	}
	defer watcher.Close()

	go func() {

		defer close(ch)

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op == fsnotify.Write {
					// reinit the service...
					data, err := os.ReadFile(s.GetConfigurationPath())
					err = json.Unmarshal(data, &s)
					if err == nil {
						// Publish the configuration change event.
						event_client_, err := getEventClient()
						if err == nil {
							event_client_.Publish("update_globular_service_configuration_evt", data)
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("error:", err)
			}
		}

	}()

	// watch for configuration change
	err = watcher.Add(s.GetConfigurationPath())
	if err != nil {
		log.Fatal("Add failed:", err)
	}

	<-ch

	fmt.Println("stop service name: ", s.GetName()+":"+s.GetId())
	server.Stop() // I kill it but not softly...

	s.SetState("stopped")
	s.SetProcess(-1)
	s.SetLastError("")
	if len(os.Args) < 3 {
		// managed by globular.
		return SaveService(s)
	}

	return nil
}

func StopService(s Service, server *grpc.Server) error {

	// Stop the service.
	server.Stop()
	return nil
}
