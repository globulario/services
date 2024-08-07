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
	"runtime"
	"syscall"

	//"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/fsnotify/fsnotify"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/kardianos/osext"
	"google.golang.org/grpc/keepalive"

	//"github.com/globulario/services/golang/config"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)


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
		servicesDir := config.GetServicesDir()
		dir, _ := osext.ExecutableFolder()
		path := strings.ReplaceAll(dir, "\\", "/")

		if !strings.HasPrefix(path, servicesDir) {
			// this will create a new configuration config.json beside the exec if no configuration file
			// already exist. Mostly use by development environnement.
			s.SetConfigurationPath(path + "/config.json")
		} else {
			servicesConfigDir := config.GetServicesConfigDir()
			configPath := strings.Replace(path, servicesDir, servicesConfigDir, -1)
			if Utility.Exists(configPath + "/config.json") {
				s.SetConfigurationPath(configPath + "/config.json")
			} else {
				// so here no configuration exist at default configuration path and the
				// service was started without argument. In that case I will create the configuration
				// file beside the executable file.
				s.SetConfigurationPath(path + "/config.json")
			}
		}
	}

	if len(s.GetConfigurationPath()) == 0 {
		fmt.Println("fail to retreive configuration for service " + s.GetId())
		return errors.New("fail to retreive configuration for service " + s.GetId())
	}

	// if the service configuration does not exist.
	if Utility.Exists(s.GetConfigurationPath()) {
		// Here I will get the configuration from the Configuration srv...
		if len(s.GetId()) > 0 {
			config_, err := config.GetServiceConfigurationById(s.GetId())
			if err != nil {
				fmt.Println("fail to retreive configuration at path ", s.GetConfigurationPath(), err)
				return err
			}

			// If no configuration was found from the configuration server i will get it from the configuration file.
			str, err := Utility.ToJson(config_)
			if err != nil {
				fmt.Println("fail to marshal configuration at path ", s.GetConfigurationPath(), err)
				return err
			}

			err = json.Unmarshal([]byte(str), &s)
			if err != nil {
				return err
			}
		} else {
			// Here I will simply get the configuration from the configuration file directly.
			data, err := os.ReadFile(s.GetConfigurationPath())
			if err != nil {
				fmt.Println("fail read ", s.GetConfigurationPath(), "with error", err)
				return err
			}

			err = json.Unmarshal(data, &s)
			if err != nil {
				return err
			}
		}

	} else {
		s.SetId(Utility.RandomUUID())
	}

	// set contextual values.
	address, _ := config.GetAddress()
	domain, _ := config.GetDomain()
	macAddress, _ := config.GetMacAddress()

	s.SetMac(macAddress)
	s.SetAddress(address)
	s.SetDomain(domain)

	// here the service is runing...
	s.SetState("starting")
	s.SetProcess(os.Getpid())

	// Now the platform.
	s.SetPlatform(runtime.GOOS + "_" + runtime.GOARCH)
	s.SetChecksum(Utility.CreateFileChecksum(execPath))

	return SaveService(s)
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

	return config.SaveServiceConfiguration(config_)
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

// Here I will initilise the grpc server.
const (
	grpcKeepaliveTime        = 30 * time.Second
	grpcKeepaliveTimeout     = 5 * time.Second
	grpcKeepaliveMinTime     = 30 * time.Second
	grpcMaxConcurrentStreams = 1000000
)

/**
 * Initilalsyse the grpc server that will run the service.
 * ** Here is an exemple how to use prometheus for monitoring and the websocket as proxy
 *    https://github.com/pion/ion-sfu/blob/master/cmd/signal/grpc/server/wrapped.go
 */
func InitGrpcServer(s Service, unaryInterceptor grpc.UnaryServerInterceptor, streamInterceptor grpc.StreamServerInterceptor) (*grpc.Server, error) {
	var server *grpc.Server
	var opts []grpc.ServerOption

	// Connection management options
	opts = append(opts,
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    grpcKeepaliveTime,
			Timeout: grpcKeepaliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcKeepaliveMinTime,
			PermitWithoutStream: true,
		}),
	)

	if s.GetTls() {
		// Create the TLS credentials
		creds := credentials.NewTLS(GetTLSConfig(s.GetKeyFile(), s.GetCertFile(), s.GetCertAuthorityTrust()))

		// Create the gRPC server with the credentials
		opts = append(opts, grpc.Creds(creds))
		if unaryInterceptor != nil {
			opts = append(opts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptor, grpc_prometheus.UnaryServerInterceptor)))
		}

		if streamInterceptor != nil {
			opts = append(opts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptor, grpc_prometheus.StreamServerInterceptor)))
		}

	} else {
		if unaryInterceptor != nil && streamInterceptor != nil {
			opts = append(opts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptor, grpc_prometheus.UnaryServerInterceptor)))
			opts = append(opts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptor, grpc_prometheus.StreamServerInterceptor)))

		} else {
			opts = append(opts, grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor))
			opts = append(opts, grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor))

		}
	}

	// Here I will create the server.
	server = grpc.NewServer(opts...)

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

func StartService(s Service, srv *grpc.Server) error {

	// First of all I will create a listener.
	// Create the channel to listen on
	var lis net.Listener
	var err error
	address := "0.0.0.0"

	lis, err = net.Listen("tcp", address+":"+strconv.Itoa(s.GetPort()))
	if err != nil {
		err_ := errors.New("could not listen at domain " + s.GetDomain() + err.Error())
		fmt.Println("service", s.GetName(), "fail to lisent at port", s.GetPort(), "with error", err)

		s.SetLastError(err_.Error())
		StopService(s, srv)

		return err_
	}

	// Here I will make a signal hook to interrupt to exit cleanly.
	go func() {
		// no web-rpc srv.
		if err := srv.Serve(lis); err != nil {
			fmt.Println("service", s.GetName(), "exit with error", err)

			s.SetLastError(err.Error())

			// Stop the service.
			StopService(s, srv)

			return
		}
	}()

/*
	// Now I will start the proxy...
	options := NewWrapperedServerOptions("0.0.0.0:"+Utility.ToString(s.GetPort()), s.GetCertFile(), s.GetKeyFile(), true)

	// Create the wrappered gRPC-Web server
	wrapperedServer := NewWrapperedGRPCWebServer(options, srv)

	// Start the server
	if err := wrapperedServer.Serve(); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
*/

	// Wait for signal to stop.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

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
					config, err := config.GetServiceConfigurationById(s.GetId())
					if err != nil {
						data, err := json.Marshal(config)
						if err == nil {
							err = json.Unmarshal(data, &s)
							if err == nil {
								// Publish the configuration change event.
								event_client_, err := getEventClient()
								if err == nil {
									event_client_.Publish("update_globular_service_configuration_evt", data)
								}
							}

							if s.GetState() == "stopped" {
								// Stop the service.
								StopService(s, srv)

								// exit program.
								os.Exit(0)
							}
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

	// wait for the service to start.
	time.Sleep(1 * time.Second) // wait for the service to start.

	// Here I will set the service state to running.
	s.SetState("running")
	s.SetLastError("")
	s.SetProcess(os.Getpid())

	// managed by globular.
	SaveService(s)

	<-ch

	// Stop the service.
	StopService(s, srv)

	// managed by globular.
	return SaveService(s)

}

func StopService(s Service, srv *grpc.Server) error {

	s.SetState("stopped")
	s.SetProcess(-1)
	s.SetLastError("")

	// Stop the service.
	srv.Stop()
	return nil
}
