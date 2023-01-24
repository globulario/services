package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/emicklei/proto"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

//////////////////////////////////////////////////////////////////////////////////////
// Globular Configurations
//////////////////////////////////////////////////////////////////////////////////////

/**
 * Return the local address.
 */
func GetAddress() (string, error) {

	domain, _ := GetDomain()

	// I need the local configuration to get info about the address.
	localConfig, err := GetLocalConfig(true)
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
func GetName() (string, error) {
	localConfig, err := GetLocalConfig(true)
	if err == nil {
		if len(localConfig["Name"].(string)) != 0 {
			return strings.ToLower(localConfig["Name"].(string)), nil
		}
	}

	// Return the name
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
	localConfig, err := GetLocalConfig(true)
	if err == nil {

		domain := localConfig["Name"].(string)

		if len(localConfig["Domain"].(string)) > 0 {
			if len(domain) > 0 {
				domain += "."
			}
			domain += localConfig["Domain"].(string)
		}
		return strings.ToLower(domain), nil
	} else {
		fmt.Println("fail to retreive local configuration with error ", err)
	}

	// if not configuration already exist on the server I will return it hostname...
	return GetName()
}

// Those function are use to get the correct
// directory where globular must be installed.
func GetRootDir() string {
	// Get the running exec dir as root instead of /var/local/share/globular...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	dir = strings.ReplaceAll(dir, "\\", "/")

	// So here the root dir can be the client exec itself or globular.
	if runtime.GOOS == "darwin" {
		// Move service configuration to /etc/globular/config/services if not already there.
		if Utility.Exists(dir + "/etc/globular/config/services") {
			// keep existing service configurations...
			if !Utility.Exists("/etc/globular/config/services") {
				Utility.Move(dir+"/etc/globular/config/services", "/etc/globular/config")
			}
			os.RemoveAll(dir + "/etc")
		}
	}

	return dir
}

/**
 * Return the location of globular executable.
 */
func GetGlobularExecPath() string {
	localConfig, err := GetLocalConfig(true)
	if err == nil {
		if len(localConfig["Path"].(string)) != 0 {
			return localConfig["Path"].(string)
		}
	}
	return ""
}

// Return the list of public dir.
func GetPublicDirs() []string {

	public := make([]string, 0)
	// Retreive all configurations
	services, err := GetServicesConfigurationsByName("file.FileService")
	if err == nil {
		for i := 0; i < len(services); i++ {
			s := services[i]
			if s["Public"] != nil {
				for j := 0; j < len(s["Public"].([]interface{})); j++ {
					public = append(public, s["Public"].([]interface{})[j].(string))
				}
			}
		}
	}

	return public
}

func GetServicesDir() string {
	// if services are taken from development environnement.
	services_dir := GetServicesRoot()
	if len(services_dir) > 0 {
		return services_dir
	}

	// return the dir where the exec is
	root_dir := GetRootDir()

	// if a dir with /services exist it will be taken as services dir.
	if Utility.Exists(GetRootDir() + "/services") {
		return root_dir + "/services"
	}

	// in case the of the Globular(.exe) exec
	if Utility.Exists(root_dir[0:strings.LastIndex(root_dir, "/")] + "/services") {
		return root_dir[0:strings.LastIndex(root_dir, "/")] + "/services"
	}

	// in case of service exec
	if strings.Contains(root_dir, "/services/") {
		return root_dir[0:strings.LastIndex(root_dir, "/services/")] + "/services"
	}

	// so here we didint find nothing...
	var programFilePath string
	// fmt.Println("fail to find service configurations at at path ", serviceConfigDir, "with error ", err)
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "386" {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
			programFilePath += "/Globular"
		} else {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
			programFilePath += "/Globular"
		}
	} else {
		programFilePath = "/usr/local/share/globular"
	}

	if Utility.Exists(programFilePath + "/services") {
		return programFilePath + "/services"
	}

	return ""
}

// Force service to be read from a specific directory.
func GetServicesRoot() string {
	localConfig, err := GetLocalConfig(true)
	if err == nil {
		if localConfig["ServicesRoot"] != nil {
			return localConfig["ServicesRoot"].(string)
		}
	}
	return ""
}

/**
 * Return where service configuration can be found.
 */
func GetServicesConfigDir() string {

	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	dir = strings.ReplaceAll(dir, "\\", "/")

	// first I will get the exec name.
	execname := filepath.Base(os.Args[0])

	if strings.HasPrefix(execname, "Globular") {
		// Force to take service at a given location.
		if len(GetServicesRoot()) > 0 {
			return GetServicesRoot()
		}

		if Utility.Exists(dir[0:strings.LastIndex(dir, "/")] + "/services") {
			return dir[0:strings.LastIndex(dir, "/")] + "/services"
		}

	} else {

		// if config near the service has priority over config found from other location...
		if Utility.Exists(dir + "/config.json") {
			return GetServicesDir()
		}

	}

	// Use the default path
	return GetConfigDir() + "/services"
}

// Must be call from Globular exec...
func GetConfigDir() string {
	if runtime.GOOS == "windows" {
		// Here by default the configuration will
		if runtime.GOARCH == "386" {
			programFilePath, _ := Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
			return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/config" // "C:/Program Files (x86)/globular"
		} else {
			programFilePath, _ := Utility.GetEnvironmentVariable("PROGRAMFILES")
			return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/config" // "C:/Program Files/globular"
		}
	} else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		return "/etc/globular/config"
	}

	return ""
}

// Must be call from Globular exec...
func GetDataDir() string {
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "386" {
			programFilePath, _ := Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
			return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/data" // "C:/Program Files (x86)/globular"
		} else {
			programFilePath, _ := Utility.GetEnvironmentVariable("PROGRAMFILES")
			return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/data" // "C:/Program Files/globular"
		}
	} else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		return "/var/globular/data"
	}

	return "/globular/data"
}

// Must be call from Globular exec...
func GetWebRootDir() string {
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "386" {
			programFilePath, _ := Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
			return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/webroot" // "C:/Program Files (x86)/globular"
		} else {
			programFilePath, _ := Utility.GetEnvironmentVariable("PROGRAMFILES")
			return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/webroot" // "C:/Program Files/globular"
		}
	} else if runtime.GOOS == "linux" || runtime.GOOS == "freebsd" || runtime.GOOS == "darwin" {
		return "/var/globular/webroot"
	}
	return "/globular/webroot"
}

/**
 * Read token for a given domain.
 */
func GetToken(mac string) (string, error) {

	path := GetConfigDir() + "/tokens/" + strings.ReplaceAll(mac, ":", "_") + "_token"
	if !Utility.Exists(path) {
		return "", errors.New("no token found for domain " + mac + " at path " + path)
	}

	token, err := os.ReadFile(path)
	if err != nil {
		fmt.Println()
		return "", errors.New("fail to read token at path " + path + " with error: " + err.Error())
	}

	return string(token), nil
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

// That function can be call by globular directly.
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

	// Here I will try to put the configuration service as the first services.
	for i := 0; i < len(services); i++ {
		if services[i]["Name"] == "config.ConfigService" {
			configService := services[i]
			// remove it from the array
			services = append(services[:i], services[i+1:]...)
			// insert it at first element.
			services = append([]map[string]interface{}{configService}, services...)
			break
		}
	}

	return services, nil
}

/**
 * Get the remote client configuration, made use of http request to do so.
 */
func GetRemoteServiceConfig(address string, port int, id string) (map[string]interface{}, error) {

	if len(address) == 0 {
		return nil, errors.New("no address was given")
	}

	if len(id) == 0 {
		return nil, errors.New("no service id was given")
	}

	// Here I will get the configuration information from http...
	var resp *http.Response
	var err error
	// The default port address.
	if port == 0 {
		port = 80
	}

	// Try over
	resp, err = http.Get("http://" + address + ":" + Utility.ToString(port) + "/config")
	if err != nil {
		fmt.Println("fail to retreive remote config at url: ", "http://"+address+":"+Utility.ToString(port)+"/config", err)
		resp, err = http.Get("https://" + address + ":" + Utility.ToString(port) + "/config")
		if err != nil {
			fmt.Println("fail to retreive remote config at url: ", "https://"+address+":"+Utility.ToString(port)+"/config", err)
			return nil, err
		}

	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil && err.Error() != "EOF" {
		fmt.Println("fail to read the config content with err ", err)
		return nil, err
	}
	// set back the error to nil
	err = nil
	if strings.Contains(string(body), "Client sent an HTTP request to an HTTPS server.") {

		if port == 0 {
			port = 443
		}
		resp, err = http.Get("https://" + address + ":" + Utility.ToString(port) + "/config")
		if err != nil {
			return nil, err
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil && err.Error() != "EOF" {
			return nil, err
		}
		err = nil
	}

	var config map[string]interface{}

	err = json.Unmarshal(body, &config)
	if err != nil {
		return nil, err
	}

	if len(id) > 0 {
		// get service by id or by name... (take the first service with a given name in case of name.
		for _, s := range config["Services"].(map[string]interface{}) {
			if s.(map[string]interface{})["Name"].(string) == id || s.(map[string]interface{})["Id"].(string) == id {
				return s.(map[string]interface{}), nil
			}
		}
	}

	return config, nil
}

/**
 * Get the remote client configuration, made use of http request to do so.
 */
func GetRemoteConfig(address string, port int) (map[string]interface{}, error) {

	if len(address) == 0 {
		return nil, errors.New("no address was given")
	}

	// Here I will get the configuration information from http...
	var resp *http.Response
	var err error
	// The default port address.
	if port == 0 {
		port = 80
	}

	// Try over
	resp, err = http.Get("http://" + address + ":" + Utility.ToString(port) + "/config")
	if err != nil {
		fmt.Println("fail to retreive remote config at url: ", "http://"+address+":"+Utility.ToString(port)+"/config", err)
		resp, err = http.Get("https://" + address + ":" + Utility.ToString(port) + "/config")
		if err != nil {
			fmt.Println("fail to retreive remote config at url: ", "https://"+address+":"+Utility.ToString(port)+"/config", err)
			return nil, err
		}

	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil && err.Error() != "EOF" {
		fmt.Println("fail to read the config content with err ", err)
		return nil, err
	}
	// set back the error to nil
	err = nil
	if strings.Contains(string(body), "Client sent an HTTP request to an HTTPS server.") {

		if port == 0 {
			port = 443
		}
		resp, err = http.Get("https://" + address + ":" + Utility.ToString(port) + "/config")
		if err != nil {
			return nil, err
		}

		body, err = ioutil.ReadAll(resp.Body)
		if err != nil && err.Error() != "EOF" {
			return nil, err
		}
		err = nil
	}

	var config map[string]interface{}

	err = json.Unmarshal(body, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// keep the value in memory
var config_ map[string]interface{}

/**
 * Return the server local configuration if one exist.
 * if lazy is set to true service will not be set in the configuration.
 */
func GetLocalConfig(lazy bool) (map[string]interface{}, error) {
	if lazy && config_ != nil {
		return config_, nil
	}

	// display configuration value.
	ConfigPath := GetConfigDir() + "/config.json"

	if !Utility.Exists(ConfigPath) {
		err := errors.New("no local Globular configuration found with path " + ConfigPath)
		return nil, err
	}

	config := make(map[string]interface{})

	data, err := os.ReadFile(ConfigPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	// Set the mac address
	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	config["Mac"] = macAddress

	if lazy {
		config_ = config
		return config, nil
	}

	// Now I will read the services configurations...
	config["Services"] = make(map[string]interface{})

	// use the ServicesRoot path if it set... or the Root (/usr/local/share/globular)
	services_config, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(services_config); i++ {
		config["Services"].(map[string]interface{})[services_config[i]["Id"].(string)] = services_config[i]
	}

	// if the Globule name is not set I will use the name of the computer hostname itself.
	if len(config["Name"].(string)) == 0 {
		config["Name"], _ = GetName()
	}

	return config, nil
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

///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Synchronized access functions.
//////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// Init the service form the file.
func initServiceConfiguration(path, serviceDir string) (map[string]interface{}, error) {

	path = strings.ReplaceAll(path, "\\", "/")
	for isLocked(path) {
		time.Sleep(50 * time.Millisecond)
	}

	config, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(config) == 0 {
		return nil, errors.New("empty configuration found at path " + path)
	}

	s := make(map[string]interface{})
	err = json.Unmarshal(config, &s)
	if err != nil {
		return nil, err
	}

	info, _ := os.Stat(path)
	s["ModTime"] = info.ModTime().Unix()

	localConfig, err := GetLocalConfig(true)
	if err != nil {
		return nil, err
	}

	if s["Protocol"] != nil {
		// If a configuration file exist It will be use to start services,
		// otherwise the service configuration file will be use.
		if s["Name"] != nil {

			// if no id was given I will generate a uuid.
			if s["Id"] == nil {
				s["Id"] = Utility.RandomUUID()
			}

			// Now the exec path.
			if s["Path"] != nil {
				if !Utility.Exists(s["Path"].(string)) {
					execname := filepath.Base(s["Path"].(string))
					files, err := Utility.FindFileByName(serviceDir, execname)
					if err == nil {
						if len(files) > 0 {
							s["Path"] = files[0]
						}
					}
				}
			}

			// Keep the configuration path in the object...
			s["ConfigPath"] = strings.ReplaceAll(path, "\\", "/")

			if s["Root"] != nil {
				if s["Name"] == "file.FileService" {
					s["Root"] = GetDataDir() + "/files"
				} else {
					s["Root"] = GetDataDir()
				}
			}

			// Get the execname
			execPath := s["Path"].(string)
			execName := filepath.Base(execPath)

			if !Utility.Exists(s["Proto"].(string)) {
				// The proto file name
				protoName := execName
				if strings.Contains(protoName, ".") {
					protoName = strings.Split(protoName, ".")[0]
				}

				if strings.Contains(protoName, "_server") {
					protoName = execName[0:strings.LastIndex(protoName, "_server")]
				}

				protoName = protoName + ".proto"

				// set the proto path.
				protoPath := execPath[0:strings.Index(execPath, "/services/")] + "/services"
				if s["Proto"].(string) != protoPath+"/"+protoName {
					if Utility.Exists(protoPath) {
						files, err := Utility.FindFileByName(protoPath, protoName)
						if err == nil {
							if len(files) > 0 {
								s["Proto"] = files[0]
							} else {
								protoName = s["Name"].(string) + ".proto"
								files, err := Utility.FindFileByName(protoPath, protoName)
								if err == nil {
									if len(files) > 0 {
										s["Proto"] = files[0]
									} else {
										fmt.Println("no proto file found at path ", protoPath, "with name", protoName)
									}
								} else {
									fmt.Println("no proto file found at path ", protoPath, "with name", protoName)
								}
							}
						} else {
							// try with the service name instead...
							protoName = s["Name"].(string) + ".proto"
							files, err := Utility.FindFileByName(execPath[0:strings.Index(execPath, "/services/")]+"/services", protoName)
							if err == nil {
								if len(files) > 0 {
									s["Proto"] = files[0]
								} else {
									fmt.Println("no proto file found at path ", execPath[0:strings.Index(execPath, "/services/")]+"/services", "with name", protoName)
								}
							} else {
								fmt.Println("no proto file found at path ", execPath[0:strings.Index(execPath, "/services/")]+"/services", "with name", protoName)
							}
						}
					}
				}
			}

			// Set paremeter fro the globule itself.
			if len(localConfig["Certificate"].(string)) > 0 && localConfig["Protocol"].(string) == "https" {
				// set tls file...
				s["TLS"] = true
				s["KeyFile"] = GetConfigDir() + "/tls/server.pem"
				s["CertFile"] = GetConfigDir() + "/tls/server.crt"
				s["CertAuthorityTrust"] = GetConfigDir() + "/tls/ca.crt"

				if s["CertificateAuthorityBundle"] != nil {
					s["CertificateAuthorityBundle"] = localConfig["CertificateAuthorityBundle"]
				}

				if s["Certificate"] != nil {
					s["Certificate"] = localConfig["Certificate"]
				}
			} else {
				s["TLS"] = false
				s["KeyFile"] = ""
				s["CertFile"] = ""
				s["CertAuthorityTrust"] = ""
			}

			// Save back the values...
			s["Domain"], _ = GetDomain()
			s["Address"], _ = GetAddress()
			s["Mac"] = localConfig["Mac"]

			// Set the session timeout in minutes (resource has that values.)
			if s["SessionTimeout"] != nil {
				s["SessionTimeout"] = localConfig["SessionTimeout"]
			}
		}
	}

	return s, nil
}

var (
	// Help to sync file access.
	saveServiceConfigChan               = make(chan map[string]interface{})
	getServicesConfigChan               = make(chan map[string]interface{})
	getServiceConfigurationByIdChan     = make(chan map[string]interface{})
	getServicesConfigurationsByNameChan = make(chan map[string]interface{})
	exit                                = make(chan bool)

	// Help to determine if the process loop is running.
	isInit bool
)

func isLocked(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)
	return Utility.Exists(lock)
}

// keep lock paths in memory...

func lock(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)
	err := Utility.WriteStringToFile(lock, "")
	if err == nil {
		return true
	}
	return false
}

func unlock(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)

	for Utility.Exists(lock) {
		time.Sleep(10 * time.Millisecond)
		os.Remove(lock)
	}

	return true
}

// Remove all file lock.
func removeAllLocks() {
	locks, err := Utility.FindFileByName(GetServicesConfigDir(), "config.lock")
	if err == nil {
		for i := 0; i < len(locks); i++ {
			os.Remove(locks[i])
		}
	}

	locks, err = Utility.FindFileByName(GetConfigDir(), "config.lock")
	if err == nil {
		for i := 0; i < len(locks); i++ {
			os.Remove(locks[i])
		}
	}
}

/**
 * Read all existing configuration and keep it in memory...
 */
func initConfig() {
	if isInit {
		return
	}

	// get sure all files are unlock
	if strings.HasPrefix(filepath.Base(os.Args[0]), "Globular") {
		removeAllLocks()
	}

	// Create communication channels...
	isInit = true

	// I will start configuation processing...
	serviceConfigDir := GetServicesConfigDir()
	files, err := Utility.FindFileByName(serviceConfigDir, "config.json")

	if err != nil {

		// So here no service configuration was found at the given directory
		// I will try to find some at installation dir...
		var programFilePath string
		// fmt.Println("fail to find service configurations at at path ", serviceConfigDir, "with error ", err)
		if runtime.GOOS == "windows" {
			if runtime.GOARCH == "386" {
				programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
			} else {
				programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
			}
		} else {
			programFilePath = "/usr/local/share/globular"
		}

		serviceConfigDir = strings.ReplaceAll(programFilePath, "\\", "/") + "/Globular/config/services"
		fmt.Println("try to find services from ", serviceConfigDir)
		files, err = Utility.FindFileByName(serviceConfigDir, "config.json")
		if err != nil {
			return
		}

		if len(files) == 0 {
			return
		}
	}

	services := make([]map[string]interface{}, 0)

	// The service dir.
	serviceDir := GetServicesDir()

	execname := filepath.Base(os.Args[0])

	// I will try to get configuration from services.
	for i := 0; i < len(files); i++ {
		path := files[i]

		//fmt.Println("init service from config at path: ", path)
		s, err := initServiceConfiguration(path, serviceDir)
		if err != nil {
			fmt.Println("fail to initialyse service configuration from file "+path, "with error", err)
		} else {

			// save back the file...
			s["ConfigPath"] = strings.ReplaceAll(path, "\\", "/") // set the service configuration path.
			services = append(services, s)

			// If the execname is globular I will set the services path from exec found in that path...
			if strings.HasPrefix(execname, "Globular") {
				if !Utility.Exists(s["Path"].(string)) {
					service_name := filepath.Base(s["Path"].(string))
					files, err := Utility.FindFileByName(serviceDir, service_name)
					if err == nil {
						if len(files) > 0 {
							s["Path"] = files[0]
							// I will also save the configuration.
							jsonStr, err := Utility.ToJson(s)
							if err == nil {
								os.WriteFile(path, []byte(jsonStr), 0644)
							}
						}
					}
				}
			}
		}
	}

	// start the loop.
	go accesServiceConfigurationFile(services)

}

// Test if the service configuration has change and if so
// read it last values to update the service configuration in
// memory
func setServiceConfiguration(index int, services []map[string]interface{}) {
	s := services[index]
	path := s["ConfigPath"].(string)
	path = strings.ReplaceAll(path, "\\", "/")
	if s["ModTime"] == nil {
		s["ModTime"] = 0
	}
	if Utility.Exists(path) {
		info, _ := os.Stat(path)
		if Utility.ToInt(s["ModTime"]) < Utility.ToInt(info.ModTime().Unix()) {
			//fmt.Println(s["Name"], " actual modtime", s["ModTime"], info.ModTime().Unix())
			serviceDir := GetServicesDir()
			s_, err := initServiceConfiguration(path, serviceDir)
			if err == nil {
				s_["ModTime"] = info.ModTime().Unix()
				services[index] = s_
			}
		}
	}
}

// Main loop to read and write configuration.
func accesServiceConfigurationFile(services []map[string]interface{}) {

	for {
		select {
		case <-exit:
			break

		case infos := <-saveServiceConfigChan:
			s := infos["service_config"].(map[string]interface{})
			path := s["ConfigPath"].(string)
			return_chan := infos["return"].(chan error)

			// Save it config...
			jsonStr, err := Utility.ToJson(s)
			if err != nil {
				fmt.Println("fail to save service configuration", err)
				return_chan <- err
			} else if len(jsonStr) == 0 {
				return_chan <- errors.New("no configuration to save")
			} else {

				// so here I will test of the configuration has change before write it in the file.
				index := -1
				hasChange := true

				for i := 0; i < len(services); i++ {
					if services[i]["Id"] == s["Id"] {
						index = i
						break
					}
				}

				if index == -1 {
					index = len(services)
					services = append(services, s)
				} else {
					// so here the service is found in services list...
					s_ := services[index]
					jsonStr_, _ := Utility.ToJson(s_)
					hasChange = jsonStr_ != jsonStr
				}

				// if the configuration has change
				if hasChange {
					for isLocked(path) {
						time.Sleep(50 * time.Millisecond)
					}

					lock(path) // lock the file access
					err := os.WriteFile(path, []byte(jsonStr), 0644)
					unlock(path) // unlock the file access
					if err != nil {
						fmt.Println("fail to save service configuration.", err)
						infos["return"].(chan error) <- err
					}

					services[index]["ModTime"] = int64(0)

					// Set the service...
					setServiceConfiguration(index, services)
				}

				return_chan <- nil
			}

		case infos := <-getServicesConfigChan:
			services_ := make([]map[string]interface{}, 0)
			for index, _ := range services {
				setServiceConfiguration(index, services)
				// Here I will create a detach copy of the map...
				data, _ := json.Marshal(services[index])
				s := make(map[string]interface{})
				json.Unmarshal(data, &s)
				services_ = append(services_, s)
			}
			infos["return"].(chan map[string]interface{}) <- map[string]interface{}{"services": services_}

		case infos := <-getServiceConfigurationByIdChan:

			var s map[string]interface{}
			var err error
			id := infos["id"].(string) // can be id or configuration path...

			for i := 0; i < len(services); i++ {
				// Can be the id, the path or the name (return the first instance of a service with a given name in that case.)
				if services[i]["Id"].(string) == id || services[i]["Name"].(string) == id || strings.ReplaceAll(services[i]["ConfigPath"].(string), "\\", "/") == id {
					setServiceConfiguration(i, services)
					data, _ := json.Marshal(services[i])
					s = make(map[string]interface{})
					json.Unmarshal(data, &s)
					break
				}
			}

			if s == nil {
				fmt.Println("no service found with id " + id)
				err = errors.New("no service found with id " + id)
			}

			infos["return"].(chan map[string]interface{}) <- map[string]interface{}{"service": s, "error": err}

		case infos := <-getServicesConfigurationsByNameChan:
			name := infos["name"].(string)
			var err error
			services_ := make([]map[string]interface{}, 0)
			for i := 0; i < len(services); i++ {
				if services[i]["Name"] == name {
					setServiceConfiguration(i, services)
					data, _ := json.Marshal(services[i])
					s := make(map[string]interface{})
					json.Unmarshal(data, &s)

					services_ = append(services_, s)
				}
			}

			if len(services_) == 0 {
				err = errors.New("no services found with name " + name)
			}
			infos["return"].(chan map[string]interface{}) <- map[string]interface{}{"services": services_, "error": err}
		}

	}
}

/**
 * stop processing configurations
 */
func Exit() {
	exit <- true
}

/**
 * Return the list of services all installed serverices on a server.
 */
func GetServicesConfigurations() ([]map[string]interface{}, error) {

	initConfig()

	infos := make(map[string]interface{})
	infos["return"] = make(chan map[string]interface{})

	// Wait\
	getServicesConfigChan <- infos

	results_chan := infos["return"].(chan map[string]interface{})

	results := <-results_chan

	if results["error"] != nil {
		return nil, results["error"].(error)
	}

	return results["services"].([]map[string]interface{}), nil
}

/**
 * Save a service configuration.
 */
func SaveServiceConfiguration(s map[string]interface{}) error {

	initConfig()

	infos := make(map[string]interface{})
	data, _ := json.Marshal(s)
	s_ := make(map[string]interface{})
	json.Unmarshal(data, &s_)

	infos["service_config"] = s_
	infos["return"] = make(chan error)

	// set the info in the channel
	saveServiceConfigChan <- infos

	return <-infos["return"].(chan error)
}

/**
 * Return the list of service that match a given name.
 */
func GetServicesConfigurationsByName(name string) ([]map[string]interface{}, error) {

	initConfig()

	infos := make(map[string]interface{})
	infos["name"] = name
	infos["return"] = make(chan map[string]interface{})

	// Wait
	getServicesConfigurationsByNameChan <- infos

	results_chan := infos["return"].(chan map[string]interface{})
	results := <-results_chan

	if results["error"] != nil {
		return nil, results["error"].(error)
	}

	return results["services"].([]map[string]interface{}), nil
}

/**
 * Return a service with a given configuration id.
 */
func GetServiceConfigurationById(id string) (map[string]interface{}, error) {

	initConfig()

	infos := make(map[string]interface{})
	infos["id"] = id
	infos["return"] = make(chan map[string]interface{})

	// Wait
	getServiceConfigurationByIdChan <- infos
	results_chan := infos["return"].(chan map[string]interface{})
	results := <-results_chan
	if results["error"] != nil {
		return nil, results["error"].(error)
	}

	return results["service"].(map[string]interface{}), nil
}
