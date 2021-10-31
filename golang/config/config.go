package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"sync"

	"github.com/davecourtois/Utility"
	"github.com/emicklei/proto"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

// I will keep the service configuation in a sync map.
var (
	// Use a sync map to limit excessive file reading.
	configs *sync.Map
)

// Those function are use to get the correct
// directory where globular must be installed.
func GetRootDir() string {
	if runtime.GOOS == "windows" {

		if runtime.GOARCH == "386" {
			programFilePath, _ := Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
			return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular" // "C:/Program Files (x86)/globular"
		} else {
			programFilePath, _ := Utility.GetEnvironmentVariable("PROGRAMFILES")
			return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular" // "C:/Program Files/globular"
		}
	} else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		return "/usr/local/share/globular"
	}

	return "/globular"
}

func GetServicesDir() string {
	return GetRootDir() + "/services"
}

func GetConfigDir() string {
	if runtime.GOOS == "windows" {
		return GetRootDir() + "/config"
	} else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		return "/etc/globular/config"
	}

	return "/globular/config"
}

func GetDataDir() string {
	if runtime.GOOS == "windows" {
		return GetRootDir() + "/data"
	} else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		return "/var/globular/data"
	}

	return "/globular/data"
}

func GetWebRootDir() string {
	if runtime.GOOS == "windows" {
		return GetRootDir() + "/webroot"
	} else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		return "/var/globular/webroot"
	}

	return "/globular/webroot"
}

/**
 * Insert an object to an array at a given index
 */
func insertObject(array []map[string]interface{}, value map[string]interface{}, index int) []map[string]interface{} {
	return append(array[:index], append([]map[string]interface{}{value}, array[index:]...)...)
}

func removeObject(array []map[string]interface{}, index int) []map[string]interface{} {
	return append(array[:index], array[index+1:]...)
}

func moveObject(array []map[string]interface{}, srcIndex int, dstIndex int) []map[string]interface{} {
	value := array[srcIndex]
	return insertObject(removeObject(array, srcIndex), value, dstIndex)
}

/**
 * Return the services index in a slice.
 */
func getObjectIndex(value, name string, objects []map[string]interface{}) int {
	for i := 0; i < len(objects); i++ {
		if objects[i][name].(string) == value {
			return i
		}
	}
	return -1
}

func GetOrderedServicesConfigurations() ([]map[string]interface{}, error) {
	services, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}

	// Now I will order the services in a way required service will start first...
	servicesNames := make([]string, len(services))
	for i := 0; i < len(services); i++ {
		servicesNames[i] = services[i]["Name"].(string)
	}

	// Now I will move the services below all it dependencie in the array...
	for i := 0; i < len(servicesNames); i++ {
		index := getObjectIndex(servicesNames[i], "Name", services)
		if services[index]["Dependencies"] != nil {
			dependencies := services[index]["Dependencies"].([]interface{})

			for j := 0; j < len(dependencies); j++ {
				index_ := getObjectIndex(dependencies[j].(string), "Name", services)
				if index_ != -1 {
					if index < index_ {
						// move the services in the array...
						services = moveObject(services, index_, index)
					}
				}
			}
		}
	}

	return services, nil
}

/**
 * Return the list of services all installed serverices on a server.
 */
func GetServicesConfigurations() ([]map[string]interface{}, error) {

	services := make([]map[string]interface{}, 0)
	if configs == nil {
		// I will get the services configuations from the config.json files.
		serviceDir := os.Getenv("GLOBULAR_SERVICES_ROOT")
		if len(serviceDir) == 0 {
			serviceDir = GetServicesDir()
		}

		serviceDir = strings.ReplaceAll(serviceDir, "\\", "/")

		files, err := Utility.FindFileByName(serviceDir, "config.json")
		if err != nil{
			return nil,  err
		}

		// I will try to get configuration from services.
		for i:=0; i < len(files); i++ {
			s := make(map[string]interface{})
				path := files[i]
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
									s["Path"] = path[0:strings.LastIndex(path, "/")+1] + s["Path"].(string)[strings.LastIndex(s["Path"].(string), "/"):]
								}

								// Keep the configuration path in the object...
								s["ConfigPath"] = path

								if s["Root"] != nil {
									if s["Name"] == "file.FileService" {
										s["Root"] = GetDataDir() + "/files"
									} else {
										s["Root"] = GetDataDir()
									}
								}

								// Create the sync map.
								if configs == nil {
									configs = new(sync.Map)
								}
								// keep in the sync map.
								configs.Store(s["Id"].(string), s)

								services = append(services, s)
							}
						}
					} else {
						log.Println("fail to unmarshal configuration path:", path, err)

					}
				} else {
					log.Println("Fail to read config file path:", path, err)
				}
		}
	} else {
		// I will get the services from the sync map.
		configs.Range(func(key, value interface{}) bool {
			// Here I will create a detach copy of the map...
			data, _ := json.Marshal(value)
			s := make(map[string]interface{})
			json.Unmarshal(data, &s)
			services = append(services, s)
			return true
		})
	}

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
func GetServiceConfigurationById(id string) (map[string]interface{}, error) {
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

func SetServiceConfiguration(s map[string]interface{}) {
	// set the config in the map.
	configs.Store(s["Id"].(string), s)
}

var (
	// Help to sync file access.
	saveFileChan chan map[string]interface{}
)

func saveServiceConfiguration() {
	for {
		select {
		case infos := <-saveFileChan:
			s := infos["service_config"].(map[string]interface{})
			return_chan := infos["return"].(chan error)

			// Save it config...
			jsonStr, err := Utility.ToJson(s)

			if err != nil {
				return_chan <- err
			} else if len(jsonStr) == 0 {
				return_chan <- errors.New("no configuration to save")
			} else {
				// return the
				return_chan <- ioutil.WriteFile(s["ConfigPath"].(string), []byte(jsonStr), 0644)
			}
		}
	}
}

/**
 * Save a service configuration.
 */
func SaveServiceConfiguration(s map[string]interface{}) error {
	if saveFileChan == nil {
		saveFileChan = make(chan map[string]interface{})
		// start the loop.
		go saveServiceConfiguration()
	}

	// set the config in the map.
	SetServiceConfiguration(s)

	infos := make(map[string]interface{})
	infos["service_config"] = s
	infos["return"] = make(chan error)

	// set the info in the channel
	saveFileChan <- infos

	return <-infos["return"].(chan error)
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
			path = configs[i]["ConfigPath"].(string)
			break
		}
	}

	// if no service exist for the given input informations.
	if len(path) == 0 {
		return nil, errors.New("no service found with name " + name + " version " + version + " and publisher id " + publisherId)
	}

	// here I will parse the service defintion file to extract the
	// service difinition.
	reader, err := os.Open(path)
	if err != nil {
		return methods, err
	}

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
