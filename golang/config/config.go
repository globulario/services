package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/emicklei/proto"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

// I will keep the service configuation in a sync map.
var (
	// Use a sync map to limit excessive file reading.
	configs *sync.Map

	// keep list of public location accessibles...
	public []string
)

/**
 * Return the local address.
 */
func GetAddress() (string, error) {
	domain, _ := GetDomain()

	// I need the local configuration to get info about the address.
	localConfig, err := GetLocalConfig()
	if err != nil {
		return "", err
	}

	// Return the address where to grab the configuration.
	address := domain
	if Utility.ToString(localConfig["Protocol"]) == "https" {
		address += ":" + Utility.ToString(localConfig["PortHttps"])
	} else {
		address += ":" + Utility.ToString(localConfig["PortHttp"])
	}

	return strings.ToLower(address), nil
}

/**
 * Return the computer name.
 */
func GetHostName() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return strings.ToLower(hostname), nil
}

/**
 * Return the Domain.
 */
func GetDomain() (string, error) {
	localConfig, err := GetLocalConfig()
	if err == nil {
		domain := localConfig["Name"].(string)
		if len(localConfig["Domain"].(string)) > 0 {
			if len(domain) > 0 {
				domain += "."
			}
			domain += localConfig["Domain"].(string)
		}
		return strings.ToLower(domain), nil
	}

	// if not configuration already exist on the server I will return it hostname...
	return GetHostName()
}

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

func GetPublicDirs() []string {
	if public == nil {
		public = make([]string, 0)
	}
	return public
}

func GetServicesDir() string {
	return GetRootDir() + "/services"
}

func GetServicesConfigDir() string {
	// That variable is use in development to set services from diffrent location...
	serviceRoot := os.Getenv("GLOBULAR_SERVICES_ROOT")
	if len(serviceRoot) > 0 {
		return serviceRoot
	}

	if Utility.Exists(GetConfigDir() + "/services") {
		return GetConfigDir() + "/services"
	}

	// Look in the service dir directly...
	return GetServicesDir()
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
 * Get the remote client configuration.
 */
func GetRemoteConfig(address string, port int) (map[string]interface{}, error) {

	// Here I will get the configuration information from http...
	var resp *http.Response
	var err error
	var configAddress = "http://" + address + ":" + Utility.ToString(port) + "/config"
	fmt.Println("get remote configuration from address ", configAddress)
	resp, err = http.Get(configAddress)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var config map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

/**
 * Return the server local configuration if one exist.
 */
func GetLocalConfig() (map[string]interface{}, error) {
	ConfigPath := GetConfigDir() + "/config.json"
	if !Utility.Exists(ConfigPath) {
		err := errors.New("no local Globular configuration found")
		fmt.Println(err)
		return nil, err
	}

	config := make(map[string]interface{})
	data, err := ReadServiceConfigurationFile(ConfigPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Now I will read the services configurations...
	config["Services"] = make(map[string]interface{})

	// use the GLOBULAR_SERVICES_ROOT path if it set... or the Root (/usr/local/share/globular)
	services_config, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(services_config); i++ {
		config["Services"].(map[string]interface{})[services_config[i]["Id"].(string)] = services_config[i]
	}

	// if the Globule name is not set I will use the name of the computer itself.
	if len(config["Name"].(string)) == 0 {
		config["Name"], _ = GetHostName()
	}

	return config, nil
}

func initServiceConfiguration(path, serviceDir string) (map[string]interface{}, error) {

	config, err := ReadServiceConfigurationFile(path)
	if err != nil {
		return nil, err
	}

	if len(config) == 0 {
		return nil, errors.New("empty configuration found at path " + path)
	}

	s := make(map[string]interface{})
	err = json.Unmarshal(config, &s)
	if err != nil {
		log.Println("fail to unmarshal configuration path:", path, err)
		return nil, err
	}

	info, _ := os.Stat(path)
	s["modtime"] = info.ModTime().Unix()

	if s["Protocol"] != nil {
		// If a configuration file exist It will be use to start services,
		// otherwise the service configuration file will be use.
		if s["Name"] != nil {

			// if no id was given I will generate a uuid.
			if s["Id"] == nil {
				s["Id"] = Utility.RandomUUID()
			}

			// Here I will set the proto file path.
			if s["Proto"] != nil {
				if !Utility.Exists(s["Proto"].(string)) {
					s["Proto"] = serviceDir + "/proto/" + strings.Split(s["Name"].(string), ".")[0] + ".proto"
				}
			}

			// Now the exec path.
			if s["Path"] != nil {
				if !Utility.Exists(s["Path"].(string)) {
					s["Path"] = path[0:strings.LastIndex(path, "/")+1] + s["Path"].(string)[strings.LastIndex(s["Path"].(string), "/"):]
				}
			}

			// Keep the configuration path in the object...
			s["ConfigPath"] = path

			if s["Root"] != nil {
				if s["Name"] == "file.FileService" {
					s["Root"] = GetDataDir() + "/files"
					// append public path from file services accessible to configuration client...
					if s["Public"] != nil {
						for i := 0; i < len(s["Public"].([]interface{})); i++ {
							path := s["Public"].([]interface{})[i].(string)
							if Utility.Exists(path) {
								if !Utility.Contains(GetPublicDirs(), path) {
									public = append(GetPublicDirs(), path)
								}
							}
						}
					}
				} else {
					s["Root"] = GetDataDir()
				}
			}

			// Create the sync map.
			if configs == nil {
				configs = new(sync.Map)
			}

			// keep in the sync map.
			getConfigs().Store(s["Id"].(string), s)
		}
	}

	return s, nil
}

// Singleton that initalyse and keep in sync map all services configurations.
func getConfigs() *sync.Map {
	if configs == nil {
		serviceDir := GetServicesConfigDir()
		configs = new(sync.Map)
		serviceDir = strings.ReplaceAll(serviceDir, "\\", "/")

		files, err := Utility.FindFileByName(serviceDir, "config.json")
		if err != nil {
			fmt.Println("fail to find service configurations at at path ", serviceDir)
			return nil
		}

		// I will try to get configuration from services.
		for i := 0; i < len(files); i++ {
			path := files[i]
			Unlock(path) // be sure no service configuration file are lock
			_, err = initServiceConfiguration(path, serviceDir)
			if err != nil {
				fmt.Println("fail to initialyse service configuration from file " + path)
			}
		}
	}
	return configs
}

/**
 * Return the list of services all installed serverices on a server.
 */
func GetServicesConfigurations() ([]map[string]interface{}, error) {

	services := make([]map[string]interface{}, 0)
	// I will get the services configuations from the config.json files.
	serviceDir := GetServicesConfigDir()

	// I will get the services from the sync map.
	getConfigs().Range(func(key, value interface{}) bool {
		// Here I will create a detach copy of the map...
		data, _ := json.Marshal(value)
		s := make(map[string]interface{})
		json.Unmarshal(data, &s)

		// Here I will validate the service configuration has not change...
		path := s["ConfigPath"].(string)
		info, _ := os.Stat(path)
		modtime := int64(0)
		if s["modtime"] != nil {
			modtime = int64(s["modtime"].(float64))
		}

		if modtime != info.ModTime().Unix() {
			// The value from the configuration file may have change...
			s, err := initServiceConfiguration(path, serviceDir)
			if err == nil {
				services = append(services, s)
			} else {
				fmt.Println("fail to get service configuration ", path, " with error: ", err)
			}
		} else {
			services = append(services, s)
		}

		return true
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
func GetServiceConfigurationById(id string) (map[string]interface{}, error) {

	// if no configuration found.
	services, err := GetServicesConfigurations()
	if err != nil {
		fmt.Println("fail to retreive service configurations ", err)
		return nil, err
	}

	for i := 0; i < len(services); i++ {
		if services[i]["Id"].(string) == id {
			return services[i], nil
		}
	}
	err = errors.New("no service found with id " + id)
	return nil, err
}

func SetServiceConfiguration(s map[string]interface{}) {
	// set the config in the map.
	getConfigs().Store(s["Id"].(string), s)
}

var (
	// Help to sync file access.
	saveFileChan chan map[string]interface{}
	readFileChan chan map[string]interface{}
)

func isLocked(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)
	return Utility.Exists(lock)
}

func Lock(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)
	err := Utility.WriteStringToFile(lock, "")
	if err == nil {
		return true
	}
	return false
}

func Unlock(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)
	err := os.Remove(lock)
	if err == nil {
		return true
	}
	return false
}

// Remove all file lock.
func RemoveAllLocks() {
	serviceDir := GetConfigDir()
	locks, err := Utility.FindFileByName(serviceDir, "config.lock")
	if err == nil {
		for i := 0; i < len(locks); i++ {
			os.Remove(locks[i])
		}
	}
}

// Create a save entry point to access configuration file. Because
// many process can access the same configuration file can be corrupted.
func accesServiceConfigurationFile() {
	for {
		select {
		case infos := <-saveFileChan:

			s := infos["service_config"].(map[string]interface{})
			path := s["ConfigPath"].(string)

			return_chan := infos["return"].(chan error)

			// Save it config...
			jsonStr, err := Utility.ToJson(s)

			if err != nil {
				return_chan <- err
			} else if len(jsonStr) == 0 {
				return_chan <- errors.New("no configuration to save")
			} else {
				// wait util the file is unlocked...
				for isLocked(path) {
					time.Sleep(500 * time.Millisecond)
				}

				Lock(path) // lock the file access
				return_chan <- ioutil.WriteFile(path, []byte(jsonStr), 0644)
				Unlock(path) // unlock the file access
			}

		case infos := <-readFileChan:
			path := infos["path"].(string)
			// wait util the file is unlocked...
			for isLocked(path) {
				time.Sleep(500 * time.Millisecond)
			}
			// fmt.Println(" read -----------------------> 551", path)
			Lock(path) // lock the file access
			data, err := ioutil.ReadFile(path)
			Unlock(path) // unlock the file access
			return_chan := infos["return"].(chan map[string]interface{})
			return_chan <- map[string]interface{}{"error": err, "data": data}
		}
	}
}

func ReadServiceConfigurationFile(path string) ([]byte, error) {
	if saveFileChan == nil && readFileChan == nil {
		saveFileChan = make(chan map[string]interface{})
		readFileChan = make(chan map[string]interface{})

		// start the loop.
		go accesServiceConfigurationFile()
	}
	infos := make(map[string]interface{})
	infos["path"] = path
	infos["return"] = make(chan map[string]interface{})

	// Wait
	readFileChan <- infos

	results_chan := infos["return"].(chan map[string]interface{})
	results := <-results_chan

	if results["error"] != nil {
		return nil, results["error"].(error)
	}

	return results["data"].([]byte), nil
}

/**
 * Save a service configuration.
 */
func SaveServiceConfiguration(s map[string]interface{}) error {
	if saveFileChan == nil && readFileChan == nil {
		// Create the sync map.
		saveFileChan = make(chan map[string]interface{})
		readFileChan = make(chan map[string]interface{})

		// start the loop.
		go accesServiceConfigurationFile()
	}

	// set the config in the map.
	getConfigs().Store(s["Id"].(string), s)

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
			exist, err:= Utility.PidExists(pid)
			if exist && err == nil {
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
