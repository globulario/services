package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/emicklei/proto"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	service_manager_client "github.com/globulario/services/golang/services_manager/services_manager_client"
	"github.com/globulario/services/golang/services_manager/services_managerpb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/struCoder/pidusage"
	"google.golang.org/grpc"

	//"google.golang.org/grpc/grpclog"
	"encoding/json"
	ps "github.com/mitchellh/go-ps"
	"google.golang.org/grpc/reflection"
	"io/ioutil"
	"os/exec"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""

	domain string = "localhost"
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Name            string
	Domain          string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Version         string
	PublisherId     string
	KeepUpToDate    bool
	KeepAlive       bool
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string

	TLS bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	// The grpc server.
	grpcServer *grpc.Server

	// The list of install services.
	services *sync.Map

	// The list of (gRpc) method's supported by this server.
	methods []string

	// The server root...
	Root string

	// The path where tls certificates are located.
	Creds string

	// The configuration path
	ConfigPath string

	// The data path
	DataPath string

	// The porst Range
	PortsRange string

	// The list of port in use.
	portsInUse []int

	// https certificate path
	Certificate string

	// https certificate bundle path
	CertificateAuthorityBundle string

	// Update servirce watch delay in second
	WatchUpdateDelay int

	// Monitoring
	ServicesMetricsPort int
	PromethenusPort     int

	// The prometheus logging informations.
	methodsCounterLog *prometheus.CounterVec

	// Monitor the cpu usage of process.
	servicesCpuUsage    *prometheus.GaugeVec
	servicesMemoryUsage *prometheus.GaugeVec

	exit_ chan bool
}

// Globular services implementation...
// The id of a particular service instance.
func (svr *server) GetId() string {
	return svr.Id
}
func (svr *server) SetId(id string) {
	svr.Id = id
}

// The name of a service, must be the gRpc Service name.
func (svr *server) GetName() string {
	return svr.Name
}
func (svr *server) SetName(name string) {
	svr.Name = name
}

// The description of the service
func (svr *server) GetDescription() string {
	return svr.Description
}
func (svr *server) SetDescription(description string) {
	svr.Description = description
}

// The list of keywords of the services.
func (svr *server) GetKeywords() []string {
	return svr.Keywords
}
func (svr *server) SetKeywords(keywords []string) {
	svr.Keywords = keywords
}

func (svr *server) GetRepositories() []string {
	return svr.Repositories
}
func (svr *server) SetRepositories(repositories []string) {
	svr.Repositories = repositories
}

func (svr *server) GetDiscoveries() []string {
	return svr.Discoveries
}
func (svr *server) SetDiscoveries(discoveries []string) {
	svr.Discoveries = discoveries
}

// Dist
func (svr *server) Dist(path string) (string, error) {

	return globular.Dist(path, svr)
}

func (svr *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (svr *server) GetPath() string {
	return svr.Path
}
func (svr *server) SetPath(path string) {
	svr.Path = path
}

// The path of the .proto file.
func (svr *server) GetProto() string {
	return svr.Proto
}
func (svr *server) SetProto(proto string) {
	svr.Proto = proto
}

// The gRpc port.
func (svr *server) GetPort() int {
	return svr.Port
}
func (svr *server) SetPort(port int) {
	svr.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (svr *server) GetProxy() int {
	return svr.Proxy
}
func (svr *server) SetProxy(proxy int) {
	svr.Proxy = proxy
}

// Can be one of http/https/tls
func (svr *server) GetProtocol() string {
	return svr.Protocol
}
func (svr *server) SetProtocol(protocol string) {
	svr.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (svr *server) GetAllowAllOrigins() bool {
	return svr.AllowAllOrigins
}
func (svr *server) SetAllowAllOrigins(allowAllOrigins bool) {
	svr.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (svr *server) GetAllowedOrigins() string {
	return svr.AllowedOrigins
}

func (svr *server) SetAllowedOrigins(allowedOrigins string) {
	svr.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (svr *server) GetDomain() string {
	return svr.Domain
}
func (svr *server) SetDomain(domain string) {
	svr.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (svr *server) GetTls() bool {
	return svr.TLS
}
func (svr *server) SetTls(hasTls bool) {
	svr.TLS = hasTls
}

// The certificate authority file
func (svr *server) GetCertAuthorityTrust() string {
	return svr.CertAuthorityTrust
}
func (svr *server) SetCertAuthorityTrust(ca string) {
	svr.CertAuthorityTrust = ca
}

// The certificate file.
func (svr *server) GetCertFile() string {
	return svr.CertFile
}
func (svr *server) SetCertFile(certFile string) {
	svr.CertFile = certFile
}

// The key file.
func (svr *server) GetKeyFile() string {
	return svr.KeyFile
}
func (svr *server) SetKeyFile(keyFile string) {
	svr.KeyFile = keyFile
}

// The service version
func (svr *server) GetVersion() string {
	return svr.Version
}
func (svr *server) SetVersion(version string) {
	svr.Version = version
}

// The publisher id.
func (svr *server) GetPublisherId() string {
	return svr.PublisherId
}
func (svr *server) SetPublisherId(publisherId string) {
	svr.PublisherId = publisherId
}

func (svr *server) GetKeepUpToDate() bool {
	return svr.KeepUpToDate
}
func (svr *server) SetKeepUptoDate(val bool) {
	svr.KeepUpToDate = val
}

func (svr *server) GetKeepAlive() bool {
	return svr.KeepAlive
}
func (svr *server) SetKeepAlive(val bool) {
	svr.KeepAlive = val
}

func (svr *server) GetPermissions() []interface{} {
	return svr.Permissions
}
func (svr *server) SetPermissions(permissions []interface{}) {
	svr.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (svr *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewServicesManagerService_Client", service_manager_client.NewServicesManagerService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (svr *server) Save() error {
	// Create the file...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", svr)
}

func (svr *server) StartService() error {
	return globular.StartService(svr, svr.grpcServer)
}

func (svr *server) StopService() error {
	return globular.StopService(svr, svr.grpcServer)
}

/////////////////////// Utility functions /////////////////////////////////////////////////
// Little shortcut to get access to map value in one step.
func setValues(m *sync.Map, values map[string]interface{}) {
	if m == nil {
		m = new(sync.Map)
	}
	for k, v := range values {
		m.Store(k, v)
	}

}

func getStringVal(m *sync.Map, k string) string {
	v, ok := m.Load(k)
	if !ok {
		return ""
	}

	return Utility.ToString(v)
}

func getIntVal(m *sync.Map, k string) int {
	v, ok := m.Load(k)
	if !ok {
		return 0
	}

	return Utility.ToInt(v)
}

func getBoolVal(m *sync.Map, k string) bool {
	v, ok := m.Load(k)
	if !ok {
		return false
	}

	return Utility.ToBool(v)
}

func getVal(m *sync.Map, k string) interface{} {
	v, ok := m.Load(k)
	if !ok {
		return nil
	}
	return v
}

/////////////////////////////// Log service ///////////////////////////////////////////////

/////////////////////// Ressource manager function ////////////////////////////////////////
func (server *server) removeRolesAction(action string) error {

	return errors.New("Not implemented")
}

func (server *server) removeApplicationsAction(action string) error {

	return errors.New("Not implemented")
}

func (server *server) removePeersAction(action string) error {

	return errors.New("Not implemented")
}

func (server *server) setRoleActions(roleId string, actions []string) error {

	return errors.New("Not implemented")
}

///////////////////// event service functions ////////////////////////////////////
func (svr *server) publish(event string, data []byte) error {
	return errors.New("not implemented")
}

///////////////////// RBAC service function /////////////////////////////////////
func (svr *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	return errors.New("not implemented")
}

///////////////////////  Service Manager functions ////////////////////////////////////////////////
func (server *server) logServiceInfo(name string, infos string) {
	log.Println("-----------> INFO", name, ":", infos)
}

func (server *server) logServiceError(name string, infos string) {
	log.Println("-----------> ERROR", name, ":", infos)
}

func (server *server) initService(s *sync.Map) error {
	serviceId := getStringVal(s, "Id")
	serviceName := getStringVal(s, "Name")
	// TODO send message to service controller...
	hasTls := getBoolVal(s, "TLS")

	isLocal := true // TODO find a way to get information about service location.
	if isLocal {
		if hasTls {
			// Set TLS local services configuration here.
			s.Store("CertAuthorityTrust", server.Creds+"/ca.crt")
			s.Store("CertFile", server.Creds+"/server.crt")
			s.Store("KeyFile", server.Creds+"/server.pem")
		} else {
			// not secure services.
			s.Store("CertAuthorityTrust", "")
			s.Store("CertFile", "")
			s.Store("KeyFile", "")
		}
	}
	log.Println("Init service: ", serviceId, ":", serviceName)
	hasChange := server.saveServiceConfig(s)
	state := getStringVal(s, "State")
	if hasChange || state == "stopped" {

		// Always stop the service before restarting it...
		if state != "stopped" {
			server.stopService(s)
		}

		// TODO watch here wath to do if other conditio are set.
		// here the service will try to restart.
		if state != "terminated" && state != "deleted" {
			_, _, err := server.startService(s)
			if err != nil {
				s.Store("State", "failed")
				return err
			}
			server.setService(s)
		}

	} else if state == "deleted" {
		// be sure the process is no more there.
		server.deleteService(serviceId)
	}

	return nil
}

/**
 * Start prometheus.
 */
func (server *server) startMonitoring() error {

	var err error

	// Here I will start promethus.
	dataPath := server.DataPath + "/prometheus-data"
	Utility.CreateDirIfNotExist(dataPath)

	// Create the configuration the first time only...
	if !Utility.Exists(server.ConfigPath + "/prometheus.yml") {
		config := `# my global config
global:
  scrape_interval:     15s # Set the scrape interval to every 15 seconds. Default is every 1 minute.
  evaluation_interval: 15s # Evaluate rules every 15 seconds. The default is every 1 minute.
  # scrape_timeout is set to the global default (10s).

# Alertmanager configuration
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      # - alertmanager:9093

# Load rules once and periodically evaluate them according to the global 'evaluation_interval'.
rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

# A scrape configuration containing exactly one endpoint to scrape:
# Here it's Prometheus itself.
scrape_configs:
  - job_name: 'prometheus'

    # metrics_path defaults to '/metrics'
    # scheme defaults to 'http'.

    static_configs:
    - targets: ['localhost:9090']
  
  - job_name: 'globular_internal_services_metrics'
    scrape_interval: 5s
    static_configs:
    - targets: ['localhost:` + Utility.ToString(server.ServicesMetricsPort) + `']
    
  - job_name: 'node_exporter_metrics'
    scrape_interval: 5s
    static_configs:
    - targets: ['localhost:` + Utility.ToString(server.PromethenusPort) + `']
    
`
		err := ioutil.WriteFile(server.ConfigPath+"/prometheus.yml", []byte(config), 0644)
		if err != nil {
			return err
		}
	}

	if !Utility.Exists(server.ConfigPath + "/alertmanager.yml") {
		config := `global:
  resolve_timeout: 5m

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'
receivers:
- name: 'web.hook'
  webhook_configs:
  - url: 'http://127.0.0.1:5001/'
inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'dev', 'instance']
`
		err := ioutil.WriteFile(server.ConfigPath+"/alertmanager.yml", []byte(config), 0644)
		if err != nil {
			return err
		}
	}

	prometheusCmd := exec.Command("prometheus", "--web.listen-address", "0.0.0.0:"+Utility.ToString(server.PromethenusPort), "--config.file", server.ConfigPath+"/prometheus.yml", "--storage.tsdb.path", dataPath)
	err = prometheusCmd.Start()
	prometheusCmd.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	if err != nil {
		log.Println("fail to start prometheus ", err)
		return err
	}

	// Here I will register various metric that I would like to have for the dashboard.

	// Prometheus logging informations.
	server.methodsCounterLog = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "globular_methods_counter",
		Help: "Globular services methods usage.",
	},
		[]string{
			"application",
			"user",
			"method"},
	)
	prometheus.MustRegister(server.methodsCounterLog)

	// Here I will monitor the cpu usage of each services
	server.servicesCpuUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_services_cpu_usage_counter",
		Help: "Monitor the cpu usage of each services.",
	},
		[]string{
			"id",
			"name"},
	)

	server.servicesMemoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_services_memory_usage_counter",
		Help: "Monitor the memory usage of each services.",
	},
		[]string{
			"id",
			"name"},
	)

	// Set the function into prometheus.
	prometheus.MustRegister(server.servicesCpuUsage)
	prometheus.MustRegister(server.servicesMemoryUsage)

	// Start feeding the time series...
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				// Here I will manage the process...

				server.services.Range(func(key, s interface{}) bool {
					pids, err := Utility.GetProcessIdsByName("Globular")
					if err == nil {
						for i := 0; i < len(pids); i++ {
							sysInfo, err := pidusage.GetStat(pids[i])
							if err == nil {
								//log.Println("---> set cpu for process ", pid, getStringVal(s.(*sync.Map), "Name"), sysInfo.CPU)
								server.servicesCpuUsage.WithLabelValues("Globular", "Globular").Set(sysInfo.CPU)
								server.servicesMemoryUsage.WithLabelValues("Globular", "Globular").Set(sysInfo.Memory)
							}
						}
					}

					pid := getIntVal(s.(*sync.Map), "Process")
					if pid > 0 {
						sysInfo, err := pidusage.GetStat(pid)
						if err == nil {
							//log.Println("---> set cpu for process ", pid, getStringVal(s.(*sync.Map), "Name"), sysInfo.CPU)
							server.servicesCpuUsage.WithLabelValues(getStringVal(s.(*sync.Map), "Id"), getStringVal(s.(*sync.Map), "Name")).Set(sysInfo.CPU)
							server.servicesMemoryUsage.WithLabelValues(getStringVal(s.(*sync.Map), "Id"), getStringVal(s.(*sync.Map), "Name")).Set(sysInfo.Memory)
						}
					} else {
						path := getStringVal(s.(*sync.Map), "Path")
						if len(path) > 0 {
							server.servicesCpuUsage.WithLabelValues(getStringVal(s.(*sync.Map), "Id"), getStringVal(s.(*sync.Map), "Name")).Set(0)
							server.servicesMemoryUsage.WithLabelValues(getStringVal(s.(*sync.Map), "Id"), getStringVal(s.(*sync.Map), "Name")).Set(0)
							//log.Println("----> process is close for ", getStringVal(s.(*sync.Map), "Name"))
						}

					}
					return true
				})
			case <-server.exit_:
				return
			}
		}

	}()

	alertmanager := exec.Command("alertmanager", "--config.file", server.ConfigPath+"/alertmanager.yml")
	alertmanager.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err = alertmanager.Start()
	if err != nil {
		log.Println("fail to start prometheus alert manager", err)
		// do not return here in that case simply continue without node exporter metrics.
	}

	node_exporter := exec.Command("node_exporter")
	node_exporter.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err = node_exporter.Start()
	if err != nil {
		log.Println("fail to start prometheus node exporter", err)
		// do not return here in that case simply continue without node exporter metrics.
	}

	return nil
}

/**
 * Keep process found in configuration in line with one found on the server.
 */
func (server *server) manageProcess() {

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				// Here I will manage the process...
				services := server.getServices()
				runingProcess := make(map[string][]int)
				proxies := make([]int, 0)

				// Fist of all I will get all services process...
				for i := 0; i < len(services); i++ {
					s := services[i]
					pid := getIntVal(s, "Process")
					state := getStringVal(s, "State")
					p, err := ps.FindProcess(pid)
					if pid != -1 && err == nil && p != nil {
						name := p.Executable()
						if _, ok := runingProcess[name]; !ok {
							runingProcess[name] = make([]int, 0)
						}
						runingProcess[name] = append(runingProcess[name], getIntVal(s, "Process"))
					} else if pid == -1 || p == nil {
						if (state == "failed" || state == "stopped" || state == "running") && len(getStringVal(s, "Path")) > 1 {
							// make sure the process is no running...
							if getBoolVal(s, "KeepAlive") {
								server.killServiceProcess(s, pid)
								s.Store("State", "stopped")
								server.initService(s)
							}
						}
					} else if err != nil {
						log.Println(err)
					}

					proxies = append(proxies, getIntVal(s, "ProxyProcess"))
				}

				proxies_, _ := Utility.GetProcessIdsByName("grpcwebproxy")
				for i := 0; i < len(proxies_); i++ {
					proxy := proxies_[i]
					for j := 0; j < len(proxies); j++ {
						if proxy == proxies[j] {
							proxy = -1
							break
						}
					}
					if proxy != -1 {
						p, err := os.FindProcess(proxy)
						if err == nil {
							p.Kill()
						}
					}
				}

				// Now I will find process by name on the computer and kill process not found in the configuration file.
				for name, pids := range runingProcess {

					pids_, err := Utility.GetProcessIdsByName(name)
					if err == nil {
						for i := 0; i < len(pids_); i++ {
							pid := pids_[i]
							for j := 0; j < len(pids); j++ {
								if pid == pids[j] {
									pid = -1
									break
								}
							}

							if pid != -1 {
								p, err := os.FindProcess(pid)
								if err == nil {
									p.Kill()
								}
							}
						}
					}
				}
			}
		}
	}()
}

/**
 * Kill a given service instance.
 */
func (server *server) killServiceProcess(s *sync.Map, pid int) {

	proxyProcessPid := getIntVal(s, "ProxyProcess")
	log.Println("Kill process ", getStringVal(s, "Name"), ":", proxyProcessPid)

	// Here I will set a variable that tell globular to not keep the service alive...
	s.Store("State", "terminated")

	// also kill it proxy process if exist in that case.
	_, hasProxyProcess := s.Load("ProxyProcess")
	if hasProxyProcess {
		proxyProcess, err := os.FindProcess(proxyProcessPid)
		if err == nil {
			proxyProcess.Kill()
			s.Store("ProxyProcess", -1)
		}
	}

	// kill it in the name of...
	process, err := os.FindProcess(pid)
	if err == nil {
		err := process.Kill()
		if err == nil {
			s.Store("Process", -1)
			s.Store("State", "stopped")
		} else {
			s.Store("State", "failed")
		}
	}
}

/**
 * Return the path of the service executable.
 */
func (server *server) getServicePath(s *sync.Map) (string, error) {
	servicePath := getStringVal(s, "Path")
	if !Utility.Exists(servicePath) {
		log.Println("No executable path was found for path ", servicePath)

		// Here I will set various base on the standard dist directory structure.
		path := server.Root + "/services/" + getStringVal(s, "PublisherId") + "/" + getStringVal(s, "Name") + "/" + getStringVal(s, "Version") + "/" + getStringVal(s, "Id")
		execName := servicePath[strings.LastIndex(servicePath, "/")+1:]
		servicePath = path + "/" + execName

		if !Utility.Exists(servicePath) {
			// If the service is running...
			server.deleteService(getStringVal(s, "Id"))
			return "", errors.New("No executable was found for service " + getStringVal(s, "Name") + servicePath)
		}

		s.Store("Path", path+"/"+execName)
		_, exist := s.Load("Path")
		if !exist {
			return "", errors.New("Fail to retreive exe path " + servicePath)
		}

		// Try to get the prototype from the standard deployement path.
		path_ := server.Root + "/services/" + getStringVal(s, "PublisherId") + "/" + getStringVal(s, "Name") + "/" + getStringVal(s, "Version")
		files, err := Utility.FindFileByName(path_, ".proto")
		if err != nil {
			return "", errors.New("No prototype file was found for path '" + path_)
		}

		s.Store("Proto", files[0])
	}

	return servicePath, nil
}

/**
 * Return the next available port.
 **/
func (server *server) getNextAvailablePort() (int, error) {
	portRange := strings.Split(server.PortsRange, "-")
	start := Utility.ToInt(portRange[0]) + 1 // The first port of the range will be reserve to http configuration handler.
	end := Utility.ToInt(portRange[1])
	log.Println("get next available port form ", start, "to", end)
	for i := start; i < end; i++ {
		if server.isPortAvailable(i) {
			server.portsInUse = append(server.portsInUse, i)
			return i, nil
		}
	}

	return -1, errors.New("No port are available in the range " + server.PortsRange)
}

/**
 * Test if a process with a given pid is running
 */
func processIsRuning(pid int) bool {
	_, err := os.FindProcess(int(pid))
	return err == nil
}

/**
 * Get the list of port in Use
 */
func (server *server) getPortsInUse() []int {
	portsInUse := server.portsInUse

	// I will test if the port is already taken by e services.
	server.services.Range(func(key, value interface{}) bool {
		m := value.(*sync.Map)
		pid_, hasProcess := m.Load("Process")

		if hasProcess {
			pid := Utility.ToInt(pid_)
			if pid != -1 {
				if processIsRuning(pid) {
					p, _ := m.Load("Port")
					portsInUse = append(portsInUse, Utility.ToInt(p))
				}
			}
		}

		proxyPid_, hasProxyProcess := m.Load("ProxyProcess")
		if hasProxyProcess {
			proxyPid := Utility.ToInt(proxyPid_)
			if proxyPid != -1 {
				if processIsRuning(proxyPid) {
					p, _ := m.Load("ProxyProcess")
					portsInUse = append(portsInUse, Utility.ToInt(p))
				}
			}
		}
		return true
	})

	return portsInUse
}

/**
 * test if a given port is avalaible.
 */
func (server *server) isPortAvailable(port int) bool {
	portRange := strings.Split(server.PortsRange, "-")
	start := Utility.ToInt(portRange[0])
	end := Utility.ToInt(portRange[1])

	if port < start || port > end {
		return false
	}

	portsInUse := server.getPortsInUse()
	for i := 0; i < len(portsInUse); i++ {
		if portsInUse[i] == port {
			return false
		}
	}

	// wait before interogate the next port
	time.Sleep(100 * time.Millisecond)
	l, err := net.Listen("tcp", "0.0.0.0:"+Utility.ToString(port))
	if err == nil {
		defer l.Close()
		return true
	}

	return false
}

/**
 * Start the grpc proxy.
 */
func (server *server) startProxy(s *sync.Map, port int, proxy int) (int, error) {
	_, hasProxyProcess := s.Load("ProxyProcess")
	if !hasProxyProcess {
		s.Store("ProxyProcess", -1)
	}
	pid := getIntVal(s, "ProxyProcess")
	if pid != -1 {
		Utility.TerminateProcess(pid, 0)
	}

	// Now I will start the proxy that will be use by javascript client.
	proxyPath := "/bin/grpcwebproxy"
	if !strings.HasSuffix(proxyPath, ".exe") && runtime.GOOS == "windows" {
		proxyPath += ".exe" // in case of windows.
	}

	proxyBackendAddress := server.GetDomain() + ":" + strconv.Itoa(port)
	proxyAllowAllOrgins := "true"
	proxyArgs := make([]string, 0)

	// Use in a local network or in test.
	proxyArgs = append(proxyArgs, "--backend_addr="+proxyBackendAddress)
	proxyArgs = append(proxyArgs, "--allow_all_origins="+proxyAllowAllOrgins)
	hasTls := getBoolVal(s, "TLS")
	if hasTls {
		certAuthorityTrust := server.Creds + "/ca.crt"

		/* Services gRpc backend. */
		proxyArgs = append(proxyArgs, "--backend_tls=true")
		proxyArgs = append(proxyArgs, "--backend_tls_ca_files="+certAuthorityTrust)
		proxyArgs = append(proxyArgs, "--backend_client_tls_cert_file="+server.Creds+"/client.crt")
		proxyArgs = append(proxyArgs, "--backend_client_tls_key_file="+server.Creds+"/client.pem")

		/* http2 parameters between the browser and the proxy.*/
		proxyArgs = append(proxyArgs, "--run_http_server=false")
		proxyArgs = append(proxyArgs, "--run_tls_server=true")
		proxyArgs = append(proxyArgs, "--server_http_tls_port="+strconv.Itoa(proxy))

		/* in case of public domain server files **/
		proxyArgs = append(proxyArgs, "--server_tls_key_file="+server.Creds+"/server.pem")

		proxyArgs = append(proxyArgs, "--server_tls_client_ca_files="+server.Creds+"/"+server.CertificateAuthorityBundle)
		proxyArgs = append(proxyArgs, "--server_tls_cert_file="+server.Creds+"/"+server.Certificate)

	} else {
		// Now I will save the file with those new information in it.
		proxyArgs = append(proxyArgs, "--run_http_server=true")
		proxyArgs = append(proxyArgs, "--run_tls_server=false")
		proxyArgs = append(proxyArgs, "--server_http_debug_port="+strconv.Itoa(proxy))
		proxyArgs = append(proxyArgs, "--backend_tls=false")
	}

	// Keep connection open for longer exchange between client/service. Event Subscribe function
	// is a good example of long lasting connection. (48 hours) seam to be more than enought for
	// browser client connection maximum life.
	proxyArgs = append(proxyArgs, "--server_http_max_read_timeout=48h")
	proxyArgs = append(proxyArgs, "--server_http_max_write_timeout=48h")

	// start the proxy service one time
	proxyProcess := exec.Command(server.Root+proxyPath, proxyArgs...)
	proxyProcess.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err := proxyProcess.Start()

	if err != nil {
		return -1, err
	}

	// save service configuration.
	s.Store("ProxyProcess", proxyProcess.Process.Pid)

	return proxyProcess.Process.Pid, nil
}

/**
 * Start services define in the configuration.
 */
func (server *server) startService(s *sync.Map) (int, int, error) {

	serviceId := getStringVal(s, "Id")
	serviceName := getStringVal(s, "Name")
	pid := getIntVal(s, "Process")
	proxyPid := getIntVal(s, "ProxyProcess")

	isLocal := true // TODO find a way to get information about service location.
	if !isLocal {
		return -1, -1, errors.New("service " + serviceId + ":" + serviceName + "is not local")
	}

	// If a process already run for that service I will terminate it
	if pid != -1 {
		if runtime.GOOS == "windows" {
			// Program written with dotnet on window need this command to stop...
			kill := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(pid))
			kill.Stderr = os.Stderr
			kill.Stdout = os.Stdout
			kill.Run()
		} else {
			Utility.TerminateProcess(pid, 0)
		}
	}

	// Set back the value in the store.
	s.Store("Process", -1)

	// Get the service executable path.
	servicePath, err := server.getServicePath(s)

	if err != nil {
		return -1, -1, err
	}

	// Now I will set tls informations.
	hasTls := getBoolVal(s, "TLS")

	if hasTls {
		// Set TLS local services configuration here.
		s.Store("CertAuthorityTrust", server.Creds+"/ca.crt")
		s.Store("CertFile", server.Creds+"/server.crt")
		s.Store("KeyFile", server.Creds+"/server.pem")
	} else {
		// not secure services.
		s.Store("CertAuthorityTrust", "")
		s.Store("CertFile", "")
		s.Store("KeyFile", "")
	}

	// Reset the list of port in user.
	server.portsInUse = make([]int, 0)

	// Get the next available port.
	port := getIntVal(s, "Port")

	if !server.isPortAvailable(port) {
		port, err = server.getNextAvailablePort()
		if err != nil {
			return -1, -1, err
		}
		s.Store("Port", port)
		server.setService(s)

	}

	err = os.Chmod(servicePath, 0755)
	if err != nil {
		log.Println(err)
	}

	p := exec.Command(servicePath, Utility.ToString(port))
	var errb bytes.Buffer
	// The pipe will be use to display process console and error.
	pipe, _ := p.StdoutPipe()
	p.Stderr = &errb

	// Here I will set the command dir.
	p.Dir = servicePath[:strings.LastIndex(servicePath, "/")]
	p.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err = p.Start()
	if err != nil {
		s.Store("State", "fail")
		s.Store("Process", -1)
		log.Println("Fail to start service: ", getStringVal(s, "Name"), " at port ", port, " with error ", err)
		return -1, -1, err
	} else {
		pid = p.Process.Pid
		s.Store("Process", p.Process.Pid)
		log.Println("Service ", getStringVal(s, "Name")+":"+getStringVal(s, "Id"), "started with pid:", getIntVal(s, "Process"))
	}

	// save the services in the map.
	go func(s *sync.Map, p *exec.Cmd) {

		s.Store("State", "running")
		output := make(chan string)
		done := make(chan bool)

		// Process message util the command is done.
		go func() {
			for {
				select {
				case <-done:
					return

				case line := <-output:
					log.Println(line)
					server.logServiceInfo(getStringVal(s, "Name"), line)
				}
			}

		}()

		// Start reading the output
		go Utility.ReadOutput(output, pipe)

		// if the process is not define.
		err = p.Wait() // wait for the program to return

		// set default value
		s.Store("State", "stopped")
		done <- true

		pipe.Close()

		if err != nil {
			// I will log the program error into the admin logger.
			server.logServiceError(getStringVal(s, "Name"), err.Error())
		}

		// Print the error
		if len(errb.String()) > 0 {
			fmt.Println("service", getStringVal(s, "Name"), "err:", errb.String())
			server.logServiceError(getStringVal(s, "Name"), errb.String())
			s.Store("State", "failed")
		}

		s.Store("Process", -1)
		server.setService(s)
		server.logServiceInfo(getStringVal(s, "Name"), "Service stop.")

	}(s, p)

	// get another port.
	if proxyPid == -1 {
		proxy := getIntVal(s, "Proxy")
		if !server.isPortAvailable(proxy) {
			server.setService(s)
			proxy, err = server.getNextAvailablePort()
			if err != nil {
				s.Store("Proxy", -1)

				return -1, -1, err
			}
			// Set back the process
			s.Store("Proxy", proxy)
			server.setService(s)
		}

		// Start the proxy.
		proxyPid, err = server.startProxy(s, port, proxy)
		log.Println("-------------------> ", proxyPid, serviceName)
		if err != nil {
			return -1, -1, err
		}
	}

	proxy := getIntVal(s, "Proxy")
	log.Println("Service "+getStringVal(s, "Name")+":"+getStringVal(s, "Id")+" is up and running at port ", port, " and proxy ", proxy)

	// save service config.
	server.saveServiceConfig(s)

	return pid, proxyPid, nil
}

/**
 * Retunr the path of config.json for a given services.
 */
func (server *server) getServiceConfigPath(s *sync.Map) string {

	path := getStringVal(s, "Path")
	index := strings.LastIndex(path, "/")
	if index == -1 {
		return ""
	}

	path = path[0:index] + "/config.json"
	return path
}

// return true if the configuation has change.
func (server *server) saveServiceConfig(config *sync.Map) bool {

	// Here I will get the service configuration
	configPath := server.getServiceConfigPath(config)
	if len(configPath) == 0 {
		return false
	}

	// set the domain of the service.
	config.Store("Domain", getStringVal(config, "Domain"))

	// format the path's
	config.Store("Path", strings.ReplaceAll(getStringVal(config, "Path"), "\\", "/"))
	config.Store("Proto", strings.ReplaceAll(getStringVal(config, "Proto"), "\\", "/"))

	// so here I will get the previous information...
	f, err := os.Open(configPath)

	if err == nil {
		b, err := ioutil.ReadAll(f)
		if err == nil {
			// get previous configuration...
			config_ := make(map[string]interface{}, 0)
			json.Unmarshal(b, &config_)
			config__ := make(map[string]interface{}, 0)
			config.Range(func(k, v interface{}) bool {
				config__[k.(string)] = v
				return true
			})

			// Test if there change from the original value's
			if reflect.DeepEqual(config_, config__) {
				f.Close()
				// set back the path's info.
				return false
			}

			// sync the data/config file with the service file.
			jsonStr, _ := Utility.ToJson(config__)
			// here I will write the file
			err = ioutil.WriteFile(configPath, []byte(jsonStr), 0644)
			if err != nil {
				return false
			}

			err := server.publish("update_globular_service_configuration_evt", []byte(jsonStr))
			if err != nil {
				log.Println("fail to publish event with error: ", err)
			}
		}
	}
	f.Close()

	// Load the services permissions.
	// Here I will get the list of service permission and set it...
	permissions, hasPermissions := config.Load("Permissions")
	if hasPermissions {
		if permissions != nil {
			for i := 0; i < len(permissions.([]interface{})); i++ {
				permission := permissions.([]interface{})[i].(map[string]interface{})
				server.setActionResourcesPermissions(permission)
			}
		}
	}

	return true
}

/**
 * Return the list of services configurations
 */
func (server *server) getServicesConfig() []map[string]interface{} {

	// return the full service configuration.
	// Here I will give only the basic services informations and keep
	// all other infromation secret.
	configs := make([]map[string]interface{}, 0)
	services := server.getServices()
	for i := 0; i < len(services); i++ {
		service_config := services[i]

		// Keep only some basic services values.
		s := make(map[string]interface{}, 0)
		s["Domain"] = getStringVal(service_config, "Domain")
		s["Port"] = getIntVal(service_config, "Port")
		s["Proxy"] = getIntVal(service_config, "Proxy")
		s["TLS"] = getBoolVal(service_config, "TLS")
		s["Version"] = getStringVal(service_config, "Version")
		s["PublisherId"] = getStringVal(service_config, "PublisherId")
		s["KeepUpToDate"] = getBoolVal(service_config, "KeepUpToDate")
		s["KeepAlive"] = getBoolVal(service_config, "KeepAlive")
		s["Description"] = getStringVal(service_config, "Description")
		s["Keywords"] = getVal(service_config, "Keywords")
		s["Repositories"] = getVal(service_config, "Repositories")
		s["Discoveries"] = getVal(service_config, "Discoveries")
		s["State"] = getStringVal(service_config, "State")
		s["Id"] = getStringVal(service_config, "Id")
		s["Name"] = getStringVal(service_config, "Name")
		s["CertFile"] = getStringVal(service_config, "CertFile")
		s["KeyFile"] = getStringVal(service_config, "KeyFile")
		s["CertAuthorityTrust"] = getStringVal(service_config, "CertAuthorityTrust")

		configs = append(configs, s)
	}

	return configs

}

func (server *server) stopService(s *sync.Map) error {

	// Set keep alive to false...
	s.Store("State", "terminated")
	server.setService(s) // set in the map...

	_, hasProcessPid := s.Load("Process")
	if !hasProcessPid {
		s.Store("Process", -1)
	}

	pid := getIntVal(s, "Process")
	if pid != -1 {
		log.Println("stop service ", getStringVal(s, "Name"), "pid:", pid)
		if runtime.GOOS == "windows" {
			// Program written with dotnet on window need this command to stop...
			kill := exec.Command("TASKKILL", "/T", "/F", "/PID", strconv.Itoa(pid))
			kill.Stderr = os.Stderr
			kill.Stdout = os.Stdout
			kill.Run()
		} else {
			err := Utility.TerminateProcess(pid, 0)
			if err != nil {
				log.Println("fail to teminate process ", pid)
			}
		}
	}

	_, hasProxyProcessPid := s.Load("ProxyProcess")
	if !hasProxyProcessPid {
		s.Store("ProxyProcess", -1)
	}
	pid = getIntVal(s, "ProxyProcess")
	if pid != -1 {
		log.Println("terminate proxy process", pid)
		err := Utility.TerminateProcess(pid, 0)
		if err != nil {
			log.Println("fail to teminate proxy process ", pid)
		}
	}

	s.Store("Process", -1)
	s.Store("ProxyProcess", -1)
	s.Store("State", "stopped")

	// set the service back in the map.
	server.setService(s)

	config := make(map[string]interface{}, 0)
	s.Range(func(k, v interface{}) bool {
		config[k.(string)] = v
		return true
	})

	// sync the data/config file with the service file.
	jsonStr, _ := Utility.ToJson(config)

	// here I will write the file
	configPath := server.getServiceConfigPath(s)
	if len(configPath) > 0 {
		log.Println("save configuration at ", configPath)
		err := ioutil.WriteFile(configPath, []byte(jsonStr), 0644)
		if err != nil {
			return err
		}
	}

	server.logServiceInfo(getStringVal(s, "Name"), time.Now().String()+"Service "+getStringVal(s, "Name")+" was stopped!")
	return nil
}

func (server *server) getServices() []*sync.Map {
	_services_ := make([]*sync.Map, 0)
	//Append services into the array.
	server.services.Range(func(key, s interface{}) bool {
		// I will remove unfounded service from the map...
		servicePath := getStringVal(s.(*sync.Map), "Path")

		// Here I will set various base on the standard dist directory structure.
		path := server.Root + "/services/" + getStringVal(s.(*sync.Map), "PublisherId") + "/" + getStringVal(s.(*sync.Map), "Name") + "/" + getStringVal(s.(*sync.Map), "Version") + "/" + getStringVal(s.(*sync.Map), "Id")
		execName := servicePath[strings.LastIndex(servicePath, "/")+1:]
		servicePath = path + "/" + execName
		if Utility.Exists(servicePath) {
			s.(*sync.Map).Store("Path", servicePath)
			server.setService(s.(*sync.Map))
			_services_ = append(_services_, s.(*sync.Map))
		} else {
			log.Println("No executable path was found for path ", servicePath)
			server.deleteService(getStringVal(s.(*sync.Map), "Id"))
		}
		return true
	})

	return _services_

}

func (server *server) setService(s *sync.Map) {

	id, _ := s.Load("Id")
	// I will not set the services if it
	if getStringVal(s, "State") != "deleted" {
		server.services.Store(id.(string), s)
	}
}

func (server *server) getService(id string) *sync.Map {
	s, ok := server.services.Load(id)
	if ok {
		return s.(*sync.Map)
	} else {
		return nil
	}
}

func (server *server) deleteService(id string) error {
	// Remove the services from the map...
	s, exist := server.services.LoadAndDelete(id)

	// Kill process if not already deleted.
	if exist {
		err := server.stopService(s.(*sync.Map))
		if err != nil {
			return err
		}
	}

	if exist {
		log.Println("service", getStringVal(s.(*sync.Map), "Name"), getStringVal(s.(*sync.Map), "Id"), "was remove from the map!")
	}

	return nil
}

/**
 * Start microservices.
 */
func (server *server) startServices() error {

	// Initialyse the services.
	log.Println("Initialyse services")
	log.Println("local ip ", Utility.MyLocalIP())
	log.Println("external ip ", Utility.MyIP())

	// I will try to get configuration from services.
	filepath.Walk(server.Root, func(path string, info os.FileInfo, err error) error {
		path = strings.ReplaceAll(path, "\\", "/")
		if info == nil {
			return nil
		}

		if err == nil && info.Name() == "config.json" {
			// So here I will read the content of the file.
			s := make(map[string]interface{})
			config, err := ioutil.ReadFile(path)
			if err == nil {
				// Read the config file.
				err := json.Unmarshal(config, &s)
				if err == nil {
					if s["Protocol"] != nil {
						// If a configuration file exist It will be use to start services,
						// otherwise the service configuration file will be use.
						if s["Name"] == nil {
							log.Println("---> no 'Name' attribute found in service configuration in file config ", path)
						} else {

							// if no id was given I will generate a uuid.
							if s["Id"] == nil {
								s["Id"] = Utility.RandomUUID()
							}

							s_ := new(sync.Map)
							for k, v := range s {
								s_.Store(k, v)
							}

							server.setService(s_)
						}
					}
				} else {
					log.Println("fail to unmarshal configuration ", err)
				}
			} else {
				log.Println("Fail to read config file ", path, err)
			}
		}
		return nil
	})

	// Set service methods.
	filepath.Walk(server.Root, func(path string, info os.FileInfo, err error) error {
		path = strings.ReplaceAll(path, "\\", "/")
		if info == nil {
			return nil
		}
		if err == nil && strings.HasSuffix(info.Name(), ".proto") {
			name := info.Name()[0:strings.Index(info.Name(), ".")]
			server.setServiceMethods(name, path)
		}
		return nil
	})

	// Set the certificate keys...
	services := server.getServices()
	for _, s := range services {
		if getStringVal(s, "Protocol") == "grpc" {
			// The domain must be set in the sever configuration and not change after that.
			hasTls := getBoolVal(s, "TLS") // set the tls...
			if hasTls {
				// Set TLS local services configuration here.
				s.Store("CertAuthorityTrust", server.Creds+"/ca.crt")
				s.Store("CertFile", server.Creds+"/server.crt")
				s.Store("KeyFile", server.Creds+"/server.pem")
			} else {
				// not secure services.
				s.Store("CertAuthorityTrust", "")
				s.Store("CertFile", "")
				s.Store("KeyFile", "")
			}
		}
	}

	log.Println("Init services")
	// Initialyse service
	for _, s := range services {
		name := getStringVal(s, "Name")

		// Get existion process information.
		_, hasProcess := s.Load("Process")
		processPid := -1
		if hasProcess {
			processPid = getIntVal(s, "Process")
			// Now I will find if the process is running
			if processPid != -1 {
				_, err := os.FindProcess(processPid)
				log.Println("find running process for", name, ":", processPid)
				if err == nil {
					server.killServiceProcess(s, processPid)
					server.setService(s)
					processPid = -1
				}

			} else {
				_, hasProxyProcess := s.Load("ProxyProcess")
				if hasProxyProcess {
					proxyProcessPid := getIntVal(s, "ProxyProcess")
					proxyProcess, err := os.FindProcess(proxyProcessPid)
					if err == nil {
						log.Println("terminate proxy process", proxyProcessPid)
						proxyProcess.Kill()
					}
				}
			}
		}

		if processPid == -1 {
			// The service name.
			if name == "file.FileService" {
				s.Store("Root", server.DataPath+"/files")
			} else if name == "conversation.ConversationService" {
				s.Store("Root", server.DataPath)
			}
			err := server.initService(s)
			if err != nil {
				log.Println(err)
			}
		} else {
			log.Println("Process exist for service: ", name)
		}
	}

	return nil
}

/**
 * Stop external services.
 */
func (server *server) stopServices() {
	log.Println("stop services...")
	services := server.getServices()
	for i := 0; i < len(services); i++ {
		s := services[i]
		if s != nil {
			// I will also try to keep a client connection in to communicate with the service.
			log.Println("stop service: ", getStringVal(s, "Name"))
			server.stopService(s)
		}
	}

	// Double check that all process are terminated...
	for i := 0; i < len(services); i++ {
		s := services[i]
		processPid := getIntVal(s, "Process")
		if processPid != -1 {
			server.killServiceProcess(s, processPid)
			server.setService(s)
		}
	}

}

// Set admin method, guest role will be set in resource service directly because
// method are static.
func (server *server) registerMethods() error {

	// Here I will persit the sa role if it dosent already exist.
	err := server.setRoleActions("sa", server.methods)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Return the list of method's for a given service, the path is the path of the
 * proto file.
 */
func (server *server) getServiceMethods(name string, path string) []string {
	methods := make([]string, 0)

	// here I will parse the service defintion file to extract the
	// service difinition.
	reader, _ := os.Open(path)
	defer reader.Close()

	parser := proto.NewParser(reader)
	definition, _ := parser.Parse()

	// Stack values from walking tree
	stack := make([]interface{}, 0)

	handlePackage := func(stack *[]interface{}) func(*proto.Package) {
		return func(p *proto.Package) {
			*stack = append(*stack, p)
		}
	}(&stack)

	handleService := func(stack *[]interface{}) func(*proto.Service) {
		return func(s *proto.Service) {
			*stack = append(*stack, s)
		}
	}(&stack)

	handleRpc := func(stack *[]interface{}) func(*proto.RPC) {
		return func(r *proto.RPC) {
			*stack = append(*stack, r)
		}
	}(&stack)

	// Walk this way
	proto.Walk(definition,
		proto.WithPackage(handlePackage),
		proto.WithService(handleService),
		proto.WithRPC(handleRpc))

	var packageName string
	var serviceName string
	var methodName string

	for len(stack) > 0 {
		var x interface{}
		x, stack = stack[0], stack[1:]
		switch v := x.(type) {
		case *proto.Package:
			packageName = v.Name
		case *proto.Service:
			serviceName = v.Name
		case *proto.RPC:
			methodName = v.Name
			path := "/" + packageName + "." + serviceName + "/" + methodName
			// So here I will register the method into the backend.
			methods = append(methods, path)
		}
	}

	return methods
}

/**
 * Return the list of service configuaration with a given name.
 **/
func (server *server) getServiceConfigByName(name string) []map[string]interface{} {
	configs := make([]map[string]interface{}, 0)
	/*
		for _, config := range server.getConfig()["Services"].(map[string]interface{}) {
			if config.(map[string]interface{})["Name"].(string) == name {
				configs = append(configs, config.(map[string]interface{}))
			}
		}
	*/
	return configs
}

func (server *server) setServiceMethods(name string, path string) {

	// here I will parse the service defintion file to extract the
	// service difinition.
	reader, _ := os.Open(path)
	defer reader.Close()

	parser := proto.NewParser(reader)
	definition, _ := parser.Parse()

	// Stack values from walking tree
	stack := make([]interface{}, 0)

	handlePackage := func(stack *[]interface{}) func(*proto.Package) {
		return func(p *proto.Package) {
			*stack = append(*stack, p)
		}
	}(&stack)

	handleService := func(stack *[]interface{}) func(*proto.Service) {
		return func(s *proto.Service) {
			*stack = append(*stack, s)
		}
	}(&stack)

	handleRpc := func(stack *[]interface{}) func(*proto.RPC) {
		return func(r *proto.RPC) {
			*stack = append(*stack, r)
		}
	}(&stack)

	// Walk this way
	proto.Walk(definition,
		proto.WithPackage(handlePackage),
		proto.WithService(handleService),
		proto.WithRPC(handleRpc))

	var packageName string
	var serviceName string
	var methodName string

	for len(stack) > 0 {
		var x interface{}
		x, stack = stack[0], stack[1:]
		switch v := x.(type) {
		case *proto.Package:
			packageName = v.Name
		case *proto.Service:
			serviceName = v.Name
		case *proto.RPC:
			methodName = v.Name
			path := "/" + packageName + "." + serviceName + "/" + methodName
			//log.Println(path)
			// So here I will register the method into the backend.
			server.methods = append(server.methods, path)
		}
	}
}

// uninstall service
func (server *server) uninstallService(publisherId string, serviceId string, version string, deletePermissions bool) error {

	// log.Println("Uninstalling services ", publisherId, serviceId, "...")

	// First of all I will stop the running service(s) instance.
	for _, s := range server.getServices() {
		// Stop the instance of the service.
		id, ok := s.Load("Id")
		if ok {
			name := getStringVal(s, "Name")
			if getStringVal(s, "PublisherId") == publisherId && id == serviceId && getStringVal(s, "Version") == version {
				// First of all I will unsubcribe to the package event...
				log.Println("stop service ", name)
				server.stopService(s)

				log.Println("delete service ", name)
				server.deleteService(id.(string))

				// Get the list of method to remove from the list of actions.
				toDelete := server.getServiceMethods(name, getStringVal(s, "Proto"))
				methods := make([]string, 0)
				for i := 0; i < len(server.methods); i++ {
					if !Utility.Contains(toDelete, server.methods[i]) {
						methods = append(methods, server.methods[i])
					}
				}

				// Keep permissions use when we update a service.
				log.Println("delete service permissions")
				if deletePermissions {
					// Now I will remove action permissions
					for i := 0; i < len(toDelete); i++ {

						// Delete it from Role.
						server.removeRolesAction(toDelete[i])

						// Delete it from Application.
						server.removeApplicationsAction(toDelete[i])

						// Delete it from Peer.
						server.removePeersAction(toDelete[i])
					}
				}

				server.methods = methods
				server.registerMethods()

				// Test if the path exit.
				path := server.Root + "/services/" + publisherId + "/" + name + "/" + version + "/" + serviceId
				// Now I will remove the service.
				// Service are located into the packagespb...
				if Utility.Exists(path) {
					// remove directory and sub-directory.
					err := os.RemoveAll(path)
					if err != nil {
						return err
					}
				}

			}
		}
	}

	log.Println("services is now uninstalled")
	return nil

}

/**
 * Subscribe to Discoverie's and repositories to keep services up to date.
 */
func (server *server) keepServicesToDate() {

	ticker := time.NewTicker(time.Duration(server.WatchUpdateDelay) * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				// Connect to service update events...
				for _, s := range server.getServices() {
					if getIntVal(s, "Process") != -1 {
						// TODO implement the api where to validate the the actual service configuration and
						// get the new one as necessary.
						log.Println("Keep service up to date", getStringVal(s, "Name"), getStringVal(s, "Id"), getStringVal(s, "Version"))
					}
				}
			}
		}
	}()
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(services_managerpb.File_services_manager_proto.Services().Get(0).FullName())
	s_impl.Proto = services_managerpb.File_services_manager_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Mircoservice manager service"
	s_impl.Keywords = []string{"Manager", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Permissions = make([]interface{}, 0)

	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.WatchUpdateDelay = 60 * 60 // validate service version at each hours...

	// Create a new sync map.
	s_impl.services = new(sync.Map)
	s_impl.methods = make([]string, 0)
	s_impl.PortsRange = "10000-10100"
	s_impl.Root = "/usr/local/share/globular"
	s_impl.DataPath = "/var/globular/data"
	s_impl.ConfigPath = "/etc/globular/config"
	s_impl.Creds = "/etc/globular/config/tls"
	s_impl.exit_ = make(chan bool)

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id, err)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the echo services
	services_managerpb.RegisterServicesManagerServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start managed services.
	s_impl.startServices()

	// Start monitoring
	s_impl.startMonitoring()

	// Start watching from update
	s_impl.keepServicesToDate()

	// Register methods...
	s_impl.registerMethods()

	// Promometheus metrics for services.
	http.Handle("/metrics", promhttp.Handler())

	// Start the service.
	s_impl.StartService()

	// Stop services
	s_impl.stopServices()

	// stop monitoring...
	s_impl.exit_ <- true

	log.Println("service manager was stop...")

}
