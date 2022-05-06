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

	//"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/kardianos/osext"

	//"github.com/globulario/services/golang/config"
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

	// The address where the configuration can from http lnk
	GetAddress() string
	SetAddress(string)

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
		dir, _ := osext.ExecutableFolder()
		serviceRoot := os.Getenv("GLOBULAR_SERVICES_ROOT")
		path := strings.ReplaceAll(dir, "\\", "/")

		if len(serviceRoot) > 0 {
			fmt.Println("red config from ", path+"/config.json")
			s.SetConfigurationPath(path + "/config.json")
		} else {

			// In that case the path will be create from the service properties.
			var serviceDir = config.GetServicesConfigDir() + "/"
			if len(s.GetPublisherId()) == 0 {
				serviceDir += s.GetDomain() + "/" + s.GetName() + "/" + s.GetVersion()
			} else {
				serviceDir += s.GetPublisherId() + "/" + s.GetName() + "/" + s.GetVersion()
			}

			// Here I will get the existing uuid...
			values := strings.Split(execPath, "/")
			uuid := values[len(values)-2] // the path must be at /uuid/name_server.exe

			if Utility.IsUuid(uuid) {
				fmt.Println(221)
				s.SetId(uuid)
				configPath := serviceDir + "/" + uuid + "/config.json"
				// set the service dir.
				s.SetConfigurationPath(configPath)

			} else {
				// Set the configuration dir...
				uuid = Utility.RandomUUID()
				Utility.CreateDirIfNotExist(serviceDir + "/" + uuid)
				configPath := serviceDir + "/" + uuid + "/config.json"
				s.SetConfigurationPath(configPath)
			}

		}
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
			fmt.Println("fail to unmarshal configuration at path ", s.GetConfigurationPath(), err)
			return err
		}
	} else {
		s.SetId(Utility.RandomUUID())
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

	fmt.Println("Start service name: ", s.GetName()+":"+s.GetId())
	if len(os.Args) < 3 {
		SaveService(s)
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
		//fmt.Println("--------------------> fail to save service")
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

func StartService(s Service, server *grpc.Server) error {

	// First of all I will creat a listener.
	// Create the channel to listen on
	lis, err := net.Listen("tcp", "0.0.0.0:"+strconv.Itoa(s.GetPort()))
	if err != nil {
		err_ := errors.New("could not list at domain " + s.GetDomain() + err.Error())
		log.Print(err_)
		return err_
	}
	/*
		profileFileName := strings.ReplaceAll(s.GetPath(), ".exe", "") + ".pprof"
		f, err := os.Create(profileFileName)

		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
	*/
	// Here I will make a signal hook to interrupt to exit cleanly.
	go func() {
		// no web-rpc server.
		fmt.Println("service name: "+s.GetName()+" id:"+s.GetId()+" is listening at gRPC port", s.GetPort(), "and process id is ", s.GetProcess())
		if err := server.Serve(lis); err != nil {
			fmt.Println("service has error ", err)
			return
		}
	}()

	// Wait for signal to stop.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	fmt.Println("stop service name: ", s.GetName()+":"+s.GetId())
	server.Stop() // I kill it but not softly...

	//	pprof.StopCPUProfile()
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
