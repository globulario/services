package globular_service

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"path/filepath"

	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"

	"bytes"
	"io"
	"log"
	"os"
	"os/signal"
	"time"

	"errors"
	"runtime"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/admin/admin_client"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

	// The description of the services.
	GetDescription() string
	SetDescription(string)

	// The plaform of the services.
	GetPlatform() string

	// The keywords.
	GetKeywords() []string
	SetKeywords([]string)

	// The path of the executable.
	GetPath() string
	SetPath(string)

	// The path of the .proto file.
	GetProto() string
	SetProto(string)

	// The gRpc port.
	GetPort() int
	SetPort(int)

	// The reverse proxy port (use by gRpc Web)
	GetProxy() int
	SetProxy(int)

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
 * Initialise a globular service from it configuration file.
 */
func InitService(path string, s Service) error {

	serviceRoot := os.Getenv("GLOBULAR_SERVICES_ROOT")

	if len(serviceRoot) == 0 {
		// Here I receive something like
		//  /usr/local/share/globular/services/globulario/mail.MailService/0.0.1/6364c9d4-3159-419b-85ac-4981bdc9c28d/config.json
		// the first part of that path is the path of executable and not the config... so I will change it
		path = strings.ReplaceAll(path, config.GetServicesDir(), config.GetServicesConfigDir())
	}

	// I Will set the executable path in the service config.
	execPath, _ := os.Executable()
	execPath = strings.ReplaceAll(execPath, "\\", "/")
	s.SetPath(execPath)

	fmt.Println("---------> read config from path ", path)

	// Here I will retreive the list of connections from file if there are some...
	file, err := config.ReadServiceConfigurationFile(path)

	if err == nil {
		// So Here I will
		return json.Unmarshal([]byte(file), s)

	} else {
		fmt.Println("create new configuration file...")
		// Generate an id if none exist in the given configuration.
		if len(s.GetId()) == 0 {
			// Generate random id for the server instance.
			s.SetId(Utility.RandomUUID())
		}
		s.SetMac(Utility.MyMacAddr())
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			log.Println(err)
			return err
		}

		// Here I will try to find the proto file...
		files, err := Utility.FindFileByName(dir, ".proto")
		if err == nil && len(files) > 0 {
			s.SetProto(files[0])
		} else if strings.Contains(execPath, serviceRoot) && len(serviceRoot) > 0 {
			s.SetProto(serviceRoot + s.GetProto())
		}

		// save the service configuation.
		return SaveService(path, s)
	}
}

/**
 * Save a globular service.
 */
func SaveService(path string, s Service) error {
	
	serviceRoot := os.Getenv("GLOBULAR_SERVICES_ROOT")

	if len(serviceRoot) == 0 {
		// Here I receive something like
		//  /usr/local/share/globular/services/globulario/mail.MailService/0.0.1/6364c9d4-3159-419b-85ac-4981bdc9c28d/config.json
		// the first part of that path is the path of executable and not the config... so I will change it
		path = strings.ReplaceAll(path, config.GetServicesDir(), config.GetServicesConfigDir())
	}

	config__, err := Utility.ToMap(s)
	if err != nil {
		return err
	}

	// So here before I save the configuration I will get values that are not part
	// of the services itself but use by globular.
	config_ := make(map[string]interface{})
	file, err := config.ReadServiceConfigurationFile(path)
	if err == nil {
		err = json.Unmarshal(file, &config_)
		if err != nil {
			return err
		}

		// Now I will set the values not found in the service object...
		if config_["Process"] != nil {
			config__["Process"] = config_["Process"]
		}

		if config_["ProxyProcess"] != nil {
			config__["ProxyProcess"] = config_["ProxyProcess"]
		}

		if config_["LastError"] != nil {
			config__["LastError"] = config_["LastError"]
		}

		if config_["ConfigPath"] != nil {
			config__["ConfigPath"] = path
		}

		if config_["Port"] != nil {
			config__["Port"] = config_["Port"]
		}

		if config_["Proxy"] != nil {
			config__["Proxy"] = config_["Proxy"]
		}
	}

	config.SaveServiceConfiguration(config__)
	if err != nil {
		return err
	}

	event_client_, _ := getEventClient(s.GetDomain())

	if err == nil {
		// Here I will publish the start service event
		str, _ := Utility.ToJson(config__)
		event_client_.Publish("update_globular_service_configuration_evt", []byte(str))
	}

	return err
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
		panic(err)
	}

	defer fileToWrite.Close()

	if _, err := io.Copy(fileToWrite, &buf); err != nil {
		panic(err)
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
 */
func InitGrpcServer(s Service, unaryInterceptor grpc.UnaryServerInterceptor, streamInterceptor grpc.StreamServerInterceptor) (*grpc.Server, error) {
	var server *grpc.Server

	if s.GetTls() {

		// Create the TLS credentials
		creds := credentials.NewTLS(GetTLSConfig(s.GetKeyFile(), s.GetCertFile(), s.GetCertAuthorityTrust()))

		// Create the gRPC server with the credentials

		opts := []grpc.ServerOption{grpc.Creds(creds)}
		if unaryInterceptor != nil {
			opts = append(opts, grpc.UnaryInterceptor(unaryInterceptor))
		}
		if streamInterceptor != nil {
			opts = append(opts, grpc.StreamInterceptor(streamInterceptor))
		}

		server = grpc.NewServer(opts...)

	} else {
		if unaryInterceptor != nil && streamInterceptor != nil {
			server = grpc.NewServer(
				grpc.UnaryInterceptor(unaryInterceptor),
				grpc.StreamInterceptor(streamInterceptor))
		} else {
			server = grpc.NewServer()
		}
	}

	return server, nil
}

var (
	admin_client_ *admin_client.Admin_Client
	event_client_ *event_client.Event_Client
)

/**
 * Get a the local resource client.
 */
func getAdminClient(domain string) (*admin_client.Admin_Client, error) {
	var err error
	if admin_client_ == nil {
		admin_client_, err = admin_client.NewAdminService_Client(domain, "admin.AdminService")
		if err != nil {
			return nil, err
		}
	}

	return admin_client_, nil
}

/**
 * Get local event client.
 */
func getEventClient(domain string) (*event_client.Event_Client, error) {
	var err error
	if event_client_ == nil {
		event_client_, err = event_client.NewEventService_Client(domain, "event.EventService")
		if err != nil {
			return nil, err
		}
	}

	return event_client_, nil
}

func StartService(s Service, server *grpc.Server) error {

	// First of all I will creat a listener.
	// Create the channel to listen on
	lis, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(s.GetPort()))
	if err != nil {
		err_ := errors.New("could not list at domain " + s.GetDomain() + err.Error())
		log.Print(err_)
		return err_
	}

	// Here I will make a signal hook to interrupt to exit cleanly.
	go func() {

		// no web-rpc server.
		fmt.Println("service name: " + s.GetName() + " id:" + s.GetId() + " started")
		if err := server.Serve(lis); err != nil {
			fmt.Println("service has error ", err)
			return
		}

	}()

	// Wait for signal to stop.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	server.Stop() // I kill it but not softly...
	return nil
}

func StopService(s Service, server *grpc.Server) error {

	// Stop the service.
	server.Stop()
	return nil
}
