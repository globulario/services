package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
	"runtime"

	"github.com/davecourtois/Utility"
	"github.com/emicklei/proto"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

// Those function are use to get the correct
// directory where globular must be installed.
func GetRootDir() string{
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "386" {
			return "/Program Files (x86)/globular"
		}else{
			return "/Program Files/globular"
		}
	}else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin"{
		return "/usr/local/share/globular"
	}

	return "/globular"
}

func GetServicesDir() string{
	return GetRootDir() + "/services"
}

func GetConfigDir() string{
	if runtime.GOOS == "windows" {
		return GetRootDir() + "/config"
	}else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin"{
		return "/etc/globular/config"
	}

	return "/globular/config"
}

func GetDataDir() string{
	if runtime.GOOS == "windows" {
		return GetRootDir() + "/data"
	}else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin"{
		return "/var/globular/data"
	}

	return "/globular/data"
}

func GetWebRootDir() string{
	if runtime.GOOS == "windows" {
		return GetRootDir() + "/webroot"
	}else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin"{
		return "/var/globular/webroot"
	}

	return "/globular/webroot"
}

/**
 * Return the list of services all installed serverices on a server.
 */
func GetServicesConfigurations() ([]map[string]interface{}, error) {

	services := make([]map[string]interface{}, 0)

	// admin, resource, ca and services.
	serviceDir := os.Getenv("GLOBULAR_SERVICES_ROOT")
	if len(serviceDir) == 0 {
		serviceDir = GetServicesDir()
	}

	// I will try to get configuration from services.
	filepath.Walk(serviceDir, func(path string, info os.FileInfo, err error) error {
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
						if s["Name"] != nil {

							// if no id was given I will generate a uuid.
							if s["Id"] == nil {
								s["Id"] = Utility.RandomUUID()
							}

							// Here I will set the proto file path.
							if !Utility.Exists(s["Proto"].(string)) {
								s["Proto"] = serviceDir + "/proto/" + strings.Split(s["Name"].(string), ".")[0] + ".proto"
							}

							// Now the exec path.
							if !Utility.Exists(s["Path"].(string)) {
								s["Path"] = path[0:strings.LastIndex(path, "/")] + "/" + s["Path"].(string)[strings.LastIndex(s["Path"].(string), "/"):]
							}

							// Keep the configuration path in the object...
							s["configPath"] = path

							if s["Root"] != nil {
								if s["Name"] == "file.FileService" {
									s["Root"] = GetDataDir() + "/files"
								} else if s["Name"] == "conversation.ConversationService" {
									s["Root"] = GetDataDir()
								}
							}

							services = append(services, s)
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

	// return the services configuration.
	return services, nil
}

/**
 * Return the list of service that match a given name.
 */
func GetServicesConfigurationsByName(name string) ([]map[string]interface{}, error) {
	services_ := make([]map[string]interface{}, 0)

	services, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(services); i++ {
		if services[i]["Name"] == name {
			services_ = append(services_, services[i])
		}
	}

	return services_, nil
}

/**
 * Return a service with a given configuration id.
 */
func GetServicesConfigurationsById(id string) (map[string]interface{}, error) {
	// if no configuration found.
	services, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(services); i++ {
		if services[i]["Id"] == id {
			return services[i], nil
		}
	}

	return nil, errors.New("no service found with id " + id)
}

/**
 * Save a service configuration.
 */
func SaveServiceConfiguration(s map[string]interface{}) error {
	// Save it config...
	jsonStr, _ := Utility.ToJson(s)

	return ioutil.WriteFile(s["configPath"].(string), []byte(jsonStr), 0644)
}

/**
 * Return the list of method's for a given service.
 */
func GetServiceMethods(name string, publisherId string, version string) ([]string, error) {
	methods := make([]string, 0)

	configs, err := GetServicesConfigurationsByName(name)
	if err != nil {
		return nil, err
	}

	path := ""
	for i := 0; i < len(configs); i++ {
		if configs[i]["PublisherId"] == publisherId && configs[i]["Version"] == version {
			path = configs[i]["configPath"].(string)
			break
		}
	}

	// if no service exist for the given input informations.
	if len(path) == 0 {
		return nil, errors.New("no service found with name " + name + " version " + version + " and publisher id " + publisherId)
	}

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

	return methods, nil
}

//////////////////////////////////////// Port ////////////////////////////////////////////
// The list of port in use.
var (
	portsInUse = make([]int, 0)
)

/**
 * Return the next available port.
 **/
func GetNextAvailablePort(portRange_ string) (int, error) {
	
	portRange := strings.Split(portRange_, "-")
	start := Utility.ToInt(portRange[0]) + 1 // The first port of the range will be reserve to http configuration handler.
	end := Utility.ToInt(portRange[1])

	for i := start; i < end; i++ {
		if IsPortAvailable(i, portRange_) {
			portsInUse = append(portsInUse, i)
			return i, nil
		}
	}

	return -1, errors.New("No port are available in the range " + portRange_)
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
func getPortsInUse() []int {
	services, _ := GetServicesConfigurations()
	_portsInUse_ := portsInUse

	// I will test if the port is already taken by e services.
	for i := 0; i < len(services); i++ {
		s := services[i]
		pid := -1
		if s["Process"] != nil {
			s["Process"] = Utility.ToInt(pid)
		}
		
		if pid != -1 {
			if processIsRuning(pid) {
				port := Utility.ToInt(s["Port"])
				_portsInUse_ = append(_portsInUse_, port)
			}
		}

		proxyPid_ := -1
		if s["ProxyProcess"] != nil {
			s["ProxyProcess"] = Utility.ToInt(pid)
		}
		
		if proxyPid_ != -1 {
			if processIsRuning(proxyPid_) {
				port := Utility.ToInt(s["Proxy"])
				_portsInUse_ = append(_portsInUse_, port)
			}
		}
	}
	return _portsInUse_
}

/**
 * test if a given port is avalaible.
 */
func IsPortAvailable(port int, portRange_ string) bool {
	portRange := strings.Split(portRange_, "-")
	start := Utility.ToInt(portRange[0])
	end := Utility.ToInt(portRange[1])

	if port < start || port > end {
		return false
	}

	portsInUse := getPortsInUse()
	for i := 0; i < len(portsInUse); i++ {
		if portsInUse[i] == port {
			return false
		}
	}

	time.Sleep(50 * time.Millisecond)
	l, err := net.Listen("tcp", "0.0.0.0:"+Utility.ToString(port))
	if err == nil {
		defer l.Close()
		return true
	}

	return false
}
