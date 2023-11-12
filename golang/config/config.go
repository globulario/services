package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"github.com/davecourtois/Utility"
	"github.com/emicklei/proto"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

//////////////////////////////////////////////////////////////////////////////////////
// Globular Configurations
// - Globular configuration are stored in /etc/globular/config/config.json
// - Globular configuration are stored in /etc/globular/config/services
// those configuration are stored in json format in config.json file. Configurations
// must be access in a synchronized way.
//////////////////////////////////////////////////////////////////////////////////////

/**
 * Return the local address.
 */
func GetAddress() (string, error) {

	// I need the local configuration to get info about the address.
	localConfig, err := GetConfig("", true)
	if err != nil {
		return "", err
	}

	domain := localConfig["Name"].(string)
	if len(localConfig["Domain"].(string)) > 0 {
		if len(domain) > 0 {
			domain += "."
		}
		domain += localConfig["Domain"].(string)
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

	localConfig, err := GetConfig("", true)
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

	// Retreive the domain from the local configuration.
	localConfig, err := GetConfig("", true)
	if err == nil {
		if len(localConfig["Domain"].(string)) != 0 {
			return strings.ToLower(localConfig["Domain"].(string)), nil
		} else {
			return "", errors.New("no domain was found in the local configuration")
		}
	} else {
		return "", err
	}
}

// Those function are use to get the correct
// directory where globular must be installed.
func GetRootDir() string {
	// Get the running exec dir as root instead of /var/local/share/globular...
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	dir = strings.ReplaceAll(dir, "\\", "/")
	return dir
}

/**
 * Return the location of globular executable.
 */
func GetGlobularExecPath() string {
	localConfig, err := GetConfig("", true)
	if err == nil {
		if localConfig["Path"] != nil {
			if len(localConfig["Path"].(string)) != 0 {
				return localConfig["Path"].(string)
			}
		}
	}
	return ""
}

// Return the list of public dir.
func GetPublicDirs() []string {

	public := make([]string, 0)
	// Retreive all configurations
	services, err := GetServicesConfigurationsByName("", "file.FileService")
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

	localConfig, err := GetConfig("", true)
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
		} else if Utility.Exists(dir[0:strings.LastIndex(dir, "/")] + "/services") {
			return dir[0:strings.LastIndex(dir, "/")] + "/services"
		} else {
			return GetConfigDir() + "/services"
		}

	} else {
		// test if service ServicesRoot is define, that will force to get services configurations
		// from a given directory

		if len(GetServicesRoot()) > 0 {
			return GetServicesRoot()
		} else if Utility.Exists(GetConfigDir() + "/services") {
			return GetConfigDir() + "/services"
		} else {

			if len(GetServicesDir()) > 0 {
				return GetServicesDir()
			}

			// Test if it's in the development environnement.
			_, filename, _, _ := runtime.Caller(0)
			fmt.Println("Current test filename: ", filename)
			if strings.Contains(filename, "/services/golang/config/") {
				return filename[0:strings.Index(filename, "/config/")]
			}

			// No service configuration was found
			return ""
		}
	}
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

// OrderDependencies orders the services based on their dependencies.
func OrderDependencies(services []map[string]interface{}) ([]string, error) {

	serviceMap := make(map[string]map[string]interface{})
	for _, service := range services {
		serviceMap[service["Name"].(string)] = service
	}

	var orderedServices []string
	visited := make(map[string]bool)
	var visit func(serviceName string) error

	visit = func(serviceName string) error {
		if visited[serviceName] {
			return nil
		}

		service, exists := serviceMap[serviceName]
		if !exists {
			return fmt.Errorf("service not found: %s", serviceName)
		}

		visited[serviceName] = true

		for _, dependency := range service["Dependencies"].([]interface{}) {
			if !visited[dependency.(string)] {
				if err := visit(dependency.(string)); err != nil {
					fmt.Println("fail to add dependency with error: ", err)
					return err
				}
			}
		}

		if !Utility.Contains(orderedServices, serviceName) {
			orderedServices = append(orderedServices, serviceName)
		}

		return nil
	}

	for _, service := range services {
		if !visited[service["Name"].(string)] {
			if err := visit(service["Name"].(string)); err != nil {
				return nil, err
			}
		}
	}

	return orderedServices, nil
}

// That function can be call by globular directly.
func GetOrderedServicesConfigurations(address string) ([]map[string]interface{}, error) {

	services, err := GetServicesConfigurations(address)
	if err != nil {
		return nil, err
	}

	// Order the services based on their dependencies.
	orderedServices, err := OrderDependencies(services)
	if err != nil {
		fmt.Println("fail to order services with error ", err)
		return nil, err
	}

	// Now I will order the services based on their dependencies.
	orderedServicesConfig := make([]map[string]interface{}, 0)
	for i := 0; i < len(orderedServices); i++ {
		for j := 0; j < len(services); j++ {
			if services[j]["Name"].(string) == orderedServices[i] {
				orderedServicesConfig = append(orderedServicesConfig, services[j])
				break
			}
		}
	}

	return orderedServicesConfig, nil
}

/**
 * Return the list of method's for a given service.
 */
func GetServiceMethods(mac string, name string, publisherId string, version string) ([]string, error) {

	// Here I will get the local configuration.
	if len(mac) == 0 {
		// Here I will get the address from the local configuration.
		local, err := Utility.MyMacAddr(Utility.MyLocalIP())
		if err != nil {
			return nil, err
		}

		// retreive the local configuration.
		mac = local
	}

	methods := make([]string, 0)
	configs, err := GetServicesConfigurationsByName(mac, name)
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
///////////////////////////////////////////////////////////////////////////////////////////////////////////////////


func lockFile(filename string) (*os.File, error) {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("cannot lock file: %v", err)
	}

	return file, nil
}

func unlockFile(file *os.File) {
	syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	file.Close()
}

/**
 * Test if etcd is running.
 */
func isEtcdRunning() bool {
	pids, err := Utility.GetProcessIdsByName("etcd")
	if err != nil {
		return false
	}

	if len(pids) == 0 {
		return false
	}

	return true
}

/**
 * Return the server local configuration if one exist.
 * if lazy is set to true service will not be set in the configuration.
 */
func GetConfig(mac string, lazy bool) (map[string]interface{}, error) {

	// Here I will get the local configuration.
	// Here I will get the address from the local configuration.
	local, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	if len(mac) == 0 {
		// set the mac address to the local one.
		mac = local
	}

	if isEtcdRunning() {
		// Here I will get the configuration from etcd.

	} else if local == mac {

		// display configuration value.
		ConfigPath := GetConfigDir() + "/config.json"

		if !Utility.Exists(ConfigPath) {
			err := errors.New("no local Globular configuration found with path " + ConfigPath)
			return nil, err
		}

		config := make(map[string]interface{})

		// Here I will lock the file.
		file, err := lockFile(ConfigPath)
		if err != nil {
			return nil, err
		}

		data, err := os.ReadFile(ConfigPath)
		if err != nil {
			unlockFile(file)
			return nil, err
		}

		// Here I will unlock the file.
		unlockFile(file)

		err = json.Unmarshal(data, &config)
		if err != nil {
			return nil, err
		}

		config["Mac"] = mac

		if lazy {
			return config, nil
		}

		// Now I will read the services configurations...
		config["Services"] = make(map[string]interface{})

		// use the ServicesRoot path if it set... or the Root (/usr/local/share/globular)
		services_config, err := GetServicesConfigurations(mac)
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

	return nil, errors.New("fail to retreive configuration for globule with mac address " + mac)
}

/**
 * Return the list of services all installed serverices on a server.
 */
func GetServicesConfigurations(mac string) ([]map[string]interface{}, error) {
	// Here I will get the address from the local configuration.
	local, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	if len(mac) == 0 {
		// set the mac address to the local one.
		mac = local
	}

	if isEtcdRunning() {
		// Here I will get the configuration from etcd.
	} else if local == mac {

		// Here I will get the configuration from the local file system.
		// I will start configuation processing...
		serviceConfigDir := GetServicesConfigDir()
		files, err := Utility.FindFileByName(serviceConfigDir, "config.json")
		services := make([]map[string]interface{}, 0)
		if err != nil {
			return services, err

		}

		for i := 0; i < len(files); i++ {
			// Here I will read the configuration file.

			file, err := lockFile(files[i])

			if err != nil {
				fmt.Println("fail to lock file ", files[i], " with error ", err)
				continue
			}

			data, err := os.ReadFile(files[i])
			if err != nil {
				unlockFile(file)
				fmt.Println("fail to read file ", files[i], " with error ", err)
				continue
			}

			// Here I will unlock the file.
			unlockFile(file)

			// Here I will parse the configuration file.
			config := make(map[string]interface{})
			err = json.Unmarshal(data, &config)
			if err != nil {
				fmt.Println("fail to parse file ", files[i], " with error ", err)
				continue
			}

			// Here I will add the configuration to the list.
			services = append(services, config)
		}

		return services, nil
	}

	return nil, errors.New("no service configuration found for mac address " + mac)
}

/**
 * Save a service configuration.
 */
func SaveServiceConfiguration(mac string, s map[string]interface{}) error {
	// Here I will get the address from the local configuration.
	local, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return err
	}

	if len(mac) == 0 {
		// set the mac address to the local one.
		mac = local
	}

	if isEtcdRunning() {
		// Here I will get the configuration from etcd.
	} else if local == mac {
		configPath := s["ConfigPath"].(string)
		if len(configPath) == 0 {
			return errors.New("no configuration path was set")
		}

		// Here I will save the configuration.
		data, err := Utility.ToJson(s)
		if err != nil {
			return err
		}
		// Here I will save the configuration.
		file, err := lockFile(configPath)
		if err != nil {
			return err
		}


		err = os.WriteFile(configPath, []byte(data), 0644)
		if err != nil {
			unlockFile(file)
			return err
		}

		// Here I will unlock the file.
		unlockFile(file)
	}

	return nil
}

/**
 * Return the list of service that match a given name.
 */
func GetServicesConfigurationsByName(mac string, name string) ([]map[string]interface{}, error) {
	// Here I will get the address from the local configuration.
	local, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	if len(mac) == 0 {
		// set the mac address to the local one.
		mac = local
	}


	if isEtcdRunning() {
		// Here I will get the configuration from etcd.
	} else if local == mac {

		services, err := GetServicesConfigurations(mac)
		if err != nil {
			return nil, err
		}

		// Here I will return the list of services that match the given name.
		servicesByName := make([]map[string]interface{}, 0)
		for i := 0; i < len(services); i++ {
			if services[i]["Name"] == name {
				servicesByName = append(servicesByName, services[i])
			}
		}

		return servicesByName, nil

	}

	return nil, errors.New("no service configuration found with name " + name + " for mac address " + mac)
}

/**
 * Return a service with a given configuration id, or if id is a name it will return the first service that match the name.
 */
func GetServiceConfigurationById(mac string, id string) (map[string]interface{}, error) {
	// Here I will get the address from the local configuration.
	local, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	if len(mac) == 0 {
		// set the mac address to the local one.
		mac = local
	}

	if !Utility.IsUuid(id) {
		services, err := GetServicesConfigurationsByName(mac, id)
		if err != nil {
			return nil, err
		}

		if len(services) == 0 {
			return nil, errors.New("no service configuration found with name " + id + " for mac address " + mac)
		}

		return services[0], nil
	}

	if isEtcdRunning() {
		// Here I will get the configuration from etcd.
	} else if local == mac {
		services, err := GetServicesConfigurations(mac)
		if err != nil {
			return nil, err
		}

		for i := 0; i < len(services); i++ {
			if services[i]["Id"] == id {
				return services[i], nil
			}
		}
	}

	return nil, errors.New("no service configuration found for with id " + id + " for mac address " + mac)
}
