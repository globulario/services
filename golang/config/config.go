package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
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

// Return the path of Globular executable file
func GetGlobularExecPath() string {
	localConfig, err := GetLocalConfig(true)
	if err == nil {
		if len(localConfig["Path"].(string)) != 0 {
			return strings.ToLower(localConfig["Path"].(string))
		}
	}

	return ""
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
	return dir
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

// Return the list of public dir.
func GetPublicDirs() []string {
	public := make([]string, 0)
	
	// Retreive all configurations
	services, err := GetServicesConfigurations()
	if err == nil {
		for i := 0; i < len(services); i++ {
			s := services[i]
			if s["Name"] == "file.FileService" {
				if s["Public"] != nil {
					for j := 0; j < len(s["Public"].([]interface{})); j++ {
						public = append(public, s["Public"].([]interface{})[j].(string))
					}
				}
			}
		}
	}

	return public
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

/**
 * Return the server local configuration if one exist.
 * if lazy is set to true service will not be set in the configuration.
 */
func GetLocalConfig(lazy bool) (map[string]interface{}, error) {
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

// ////////////////////////////////////// Port ////////////////////////////////////////////
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
			exist, err := Utility.PidExists(pid)
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

///////////////////////////////////////////////////////////////////////////////////////////
// Services configurations
///////////////////////////////////////////////////////////////////////////////////////////

/**
 * Return the list of services all installed serverices on a server.
 */
func GetServicesConfigurations() ([]map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

/**
 * Save a service configuration.
 */
func SaveServiceConfiguration(s map[string]interface{}) error {
	return errors.New("not implemented")
}

/**
 * Return the list of service that match a given name.
 */
func GetServicesConfigurationsByName(name string) ([]map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

/**
 * Return a service with a given configuration id.
 */
func GetServiceConfigurationById(id string) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}
