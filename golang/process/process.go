package process

import (
	//"encoding/json"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/struCoder/pidusage"
	"gopkg.in/yaml.v3"
)

/**
 * Kill a given service instance.
 */
func KillServiceProcess(s map[string]interface{}) error {

	pid := -1
	if (s["Process"]) != nil {
		pid = Utility.ToInt(s["Process"])
	}

	if pid != -1 {
		// kill it in the name of...
		process, err := os.FindProcess(pid)
		if err == nil {
			err := process.Signal(syscall.SIGTERM)
			if err == nil {
				s["Process"] = -1
				s["State"] = "killed"
			} else {
				s["State"] = "failed"
				s["LastError"] = err.Error()
			}
		}
	}

	// save the service configuration.
	return nil
}

var (
	// Monitor the cpu usage of process.
	servicesCpuUsage    *prometheus.GaugeVec
	servicesMemoryUsage *prometheus.GaugeVec
)

// Start a service process.
func StartServiceProcess(s map[string]interface{}, port int) (int, error) {

	// I will kill the process if is running...
	err := KillServiceProcess(s)
	if err != nil {
		return -1, err
	}

	// I will start the service process.
	s["Port"] = port
	s["Proxy"] = port + 1
	
	if s["Path"] == nil {
		err := errors.New("no service path was found for service " + s["Name"].(string) + s["Id"].(string))
		fmt.Println(err)
		return -1, err
	}

	if !Utility.Exists(s["Path"].(string)) {
		log.Println("No service found at path ", s["Path"].(string))
		// before give up I will try to retreive the exec
		return -1, errors.New("No service found at path " + s["Path"].(string) + " be sure globular is install correctly, or the configuration at path " + s["ConfigPath"].(string) + " point at correct service path.")
	}

	p := exec.Command(s["Path"].(string), s["Id"].(string), s["ConfigPath"].(string))
	p.Dir = filepath.Dir(s["Path"].(string))

	var stderr bytes.Buffer
	p.Stderr = &stderr

	stdout, err := p.StdoutPipe()
	if err != nil {
		return -1, err
	}

	output := make(chan string)
	done := make(chan bool)

	// Process message util the command is done.
	go func() {
		for {
			select {
			case <-done:
				return

			case info := <-output:
				fmt.Println(info)
			}
		}

	}()

	// Set process values...
	s["Path"] = strings.ReplaceAll(s["Path"].(string), "\\", "/")
	s["Process"] = -1

	// save the port and ProxyProcess
	err = config.SaveServiceConfiguration(s)
	if err != nil {
		fmt.Println("fail to save service configuration", err)
		return -1, err
	}

	// Set the process dir.
	p.Dir = s["Path"].(string)[0:strings.LastIndex(s["Path"].(string), "/")]

	// Start reading the output
	go Utility.ReadOutput(output, stdout)

	// start the process.
	err = p.Start()
	if err != nil {
		stdout.Close()
		done <- true
		fmt.Println("fail to start service", s["Name"], err, "detail", stderr.String())
		return -1, err
	}

	waitUntilStart := make(chan int)

	// so here I will start each service in it own go routine.
	go func(serviceId string) {

		// The service is responsible to set the process id and the state.
		// and to save the configuration.
		// I will wait until the service is started.


		// give back the process id.
		waitUntilStart <- p.Process.Pid
		
		err = p.Wait() // wait util the process exit.

		// Here I will read the configuration to get more information about the process.
		if err != nil {
			fmt.Println("service "+s["Name"].(string)+" fail with error ", err, "detail", stderr.String())
			s["State"] = "failed"
		} else {

			// reload the config directly from the file...
			data, _ := os.ReadFile(s["ConfigPath"].(string))
			json.Unmarshal(data, &s)
			s["State"] = "stopped"
		}

		// be sure the state is not nil and failed.
		if s["State"] != nil {
			// if the service fail
			if s["State"].(string) == "failed" || s["State"].(string) == "killed" {
				fmt.Println("the service ", s["Name"], "with process id", s["Process"], "has been terminate")
				if s["KeepAlive"].(bool) {
					// give ti some time to free resources like port files... etc.
					time.Sleep(5 * time.Second)
					_, err := StartServiceProcess(s, port)
					if err != nil {
						return // fail to restart the process...
					}

					localConfig, _ := config.GetLocalConfig(true)

					// so here I need to restart it proxy process...
					proxyProcessPid := Utility.ToInt(s["ProxyProcess"])
					if proxyProcessPid != -1 {
						_, err = os.FindProcess(proxyProcessPid)
						if err != nil {
							StartServiceProxyProcess(s, localConfig["CertificateAuthorityBundle"].(string), localConfig["Certificate"].(string))
						}

					} else {
						// restart the proxy process.
						StartServiceProxyProcess(s, localConfig["CertificateAuthorityBundle"].(string), localConfig["Certificate"].(string))
					}
				}
			}

			stdout.Close()
			done <- true
			return
		}

		// Close the output.
		stdout.Close()
		done <- true

		fmt.Println("Process", s["Process"], "running", s["Name"], "has terminate and set back to -1")
		s["Process"] = -1

		config.SaveServiceConfiguration(s)

	}(s["Id"].(string))

	pid := <-waitUntilStart

	// save the service configuration.
	return pid, nil
}

// Start service proxy process.
func StartServiceProxyProcess(s map[string]interface{}, certificateAuthorityBundle, certificate string) (int, error) {

	// I will kill the process if is running...
	processPid := Utility.ToInt(s["Process"])

	if processPid == -1 {
		fmt.Println("process pid must no be -1")
		return -1, errors.New("process pid must no be -1")
	}

	// Get the service port.
	servicePort := Utility.ToInt(s["Port"])
	pid := Utility.ToInt(s["ProxyProcess"])
	if pid != -1 {
		fmt.Println("proxy already exist for service", s["Name"].(string))
		return -1, errors.New("proxy already exist for service " + s["Name"].(string))
	}

	// Now I will start the proxy that will be use by javascript client.
	cmd := "grpcwebproxy"
	if !strings.HasSuffix(cmd, ".exe") && runtime.GOOS == "windows" {
		cmd += ".exe" // in case of windows.
	}

	address := s["Address"].(string)
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}

	proxyBackendAddress := address + ":" + strconv.Itoa(servicePort)
	proxyAllowAllOrgins := "true"
	proxyArgs := make([]string, 0)

	// Use in a local network or in test.
	proxyArgs = append(proxyArgs, "--backend_addr="+proxyBackendAddress)
	proxyArgs = append(proxyArgs, "--allow_all_origins="+proxyAllowAllOrgins)
	hasTls := s["TLS"].(bool)


	name, _ :=  config.GetName()
	domain,_ := config.GetDomain()

	creds := config.GetConfigDir() + "/tls/" +name + "." + domain

	proxyPort := Utility.ToInt(s["Proxy"])

	if hasTls {
		certAuthorityTrust := creds + "/ca.crt"

		/* Services gRpc backend. */
		proxyArgs = append(proxyArgs, "--backend_tls=true")
		proxyArgs = append(proxyArgs, "--backend_tls_ca_files="+certAuthorityTrust)
		proxyArgs = append(proxyArgs, "--backend_client_tls_cert_file="+creds+"/client.crt")
		proxyArgs = append(proxyArgs, "--backend_client_tls_key_file="+creds+"/client.pem")

		
		/* http2 parameters between the browser and the proxy.*/
		proxyArgs = append(proxyArgs, "--run_http_server=false")
		proxyArgs = append(proxyArgs, "--run_tls_server=true")
		proxyArgs = append(proxyArgs, "--server_http_tls_port="+strconv.Itoa(proxyPort))

		/* in case of public domain server files **/
		proxyArgs = append(proxyArgs, "--server_tls_key_file="+creds+"/server.pem")
		proxyArgs = append(proxyArgs, "--server_tls_client_ca_files="+creds+"/"+certificateAuthorityBundle)
		proxyArgs = append(proxyArgs, "--server_tls_cert_file="+creds+"/"+certificate)

	} else {

		// Now I will save the file with those new information in it.
		proxyArgs = append(proxyArgs, "--run_http_server=true")
		proxyArgs = append(proxyArgs, "--run_tls_server=false")
		proxyArgs = append(proxyArgs, "--server_http_debug_port="+strconv.Itoa(proxyPort))
		proxyArgs = append(proxyArgs, "--backend_tls=false")
	}

	// Keep connection open for longer exchange between client/service. Event Subscribe function
	// is a good example of long lasting connection. (48 hours) seam to be more than enought for
	// browser client connection maximum life.
	proxyArgs = append(proxyArgs, "--server_http_max_read_timeout=48h")
	proxyArgs = append(proxyArgs, "--server_http_max_write_timeout=48h")
	proxyArgs = append(proxyArgs, "--use_websockets=false")

	// start the proxy service one time
	//fmt.Println(proxyPath, proxyArgs)
	proxyProcess := exec.Command(cmd, proxyArgs...)
	proxyProcess.Dir = filepath.Dir(cmd)
	proxyProcess.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	cmd_ := s["Name"].(string) + ": " + cmd + " "
	for i := 0; i < len(proxyArgs); i++ {
		cmd_ += proxyArgs[i] + " "
	}

	//fmt.Println(cmd_)
	err := proxyProcess.Start()
	if err != nil {

		fmt.Println("fail to start proxy", cmd_, err)
		if err.Error() == `exec: "grpcwebproxy": executable file not found in $PATH` || strings.Contains(err.Error(), "no such file or directory") {

			if Utility.Exists(config.GetGlobularExecPath() + "/bin/" + cmd) {

				proxyProcess = exec.Command(config.GetGlobularExecPath()+"/bin/"+cmd, proxyArgs...)
				proxyProcess.Dir = config.GetGlobularExecPath() + "/bin/"
				err = proxyProcess.Start()
				if err != nil {

					return -1, err
				}
			} else {

				ex, err := os.Executable()
				if err != nil {

					return -1, err
				}

				exPath := filepath.Dir(ex)
				if Utility.Exists(exPath + "/bin/" + cmd) {

					proxyProcess = exec.Command(exPath+"/bin/"+cmd, proxyArgs...)
					proxyProcess.Dir = exPath + "/bin/"
					err = proxyProcess.Start()
					if err != nil {
						return -1, err
					}
				} else {
					return -1, errors.New("the program grpcwebproxy is not install on the system")
				}
			}
		} else {
			return -1, err
		}
	}

	str, _ := Utility.ToJson(s)



	wait := make(chan int)

	// Get the process id...
	go func() {

		// finaly i will send event to give change of connected globule to update their service configurations with the new values...
		address, _ := config.GetAddress()
		event_client_, err := getEventClient(address)
		if err == nil {
			// Here I will publish the start service event
			//str, _ := Utility.ToJson(s)
			event_client_.Publish("update_globular_service_configuration_evt", []byte(str))
		}

		// create copy from the string...
		s := make(map[string]interface{})
		json.Unmarshal([]byte(str), &s)

		fmt.Println("Service", s["Name"].(string)+":"+s["Id"].(string), "is running pid", processPid, "and lisen at port", s["Port"], "with proxy pid", proxyProcess.Process.Pid, "lisen at port port", proxyPort)

		wait <- proxyProcess.Process.Pid // ok the proxy pid must be other than -1

		// wait to proxy
		proxyProcess.Wait()
	
		// reload the config directly from the file...
		data, err := os.ReadFile(s["ConfigPath"].(string))
		if err != nil {
			fmt.Println("proxy prcess fail with error:  ", err)
			return
		}

		json.Unmarshal(data, &s)
		processPid = Utility.ToInt(s["Process"])
		if processPid != -1 && s["KeepAlive"].(bool) {
			exist, err := Utility.PidExists(processPid)
			if err == nil && exist {
				StartServiceProxyProcess(s, certificateAuthorityBundle, certificate)
			} else if err != nil {
				fmt.Println("proxy prcess fail with error:  ", err)
				return
			}
		}

	}()

	// wait for proxy to start...
	proxyProcessPid := <-wait

	// be sure the service
	s["State"] = "running"
	s["ProxyProcess"] = proxyProcessPid

	
	return proxyProcessPid, config.SaveServiceConfiguration(s)

}

/**
 * Get local event client.
 */
func getEventClient(address string) (*event_client.Event_Client, error) {

	if address == "" {
		address, _ = config.GetAddress()
	}

	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// check if the process is actually running
// However, on Unix systems, os.FindProcess always succeeds and returns
// a Process for the given pid...regardless of whether the process exists
// or not.
func GetProcessRunningStatus(pid int) (*os.Process, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	//double check if process is running and alive
	//by sending a signal 0
	//NOTE : syscall.Signal is not available in Windows
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return proc, nil
	}

	if err == syscall.ESRCH {
		return nil, errors.New("process not running")
	}

	// default
	return nil, errors.New("process running but query operation not permitted")
}

// /////////////////////////// Etcd ditributed Key-value store ///////////////////////////////
func StartEtcdServer() error {

	local_config, err := config.GetLocalConfig(true)
	if err != nil {
		return err
	}

	protocol := local_config["Protocol"].(string)

	// Here I will create the config file for etcd.
	node_config := make(map[string]interface{})

	configPath := config.GetConfigDir() + "/etcd.yml"

	// if the config file already exist I will read it.
	if Utility.Exists(configPath) {
		// Here I will read the config file.
		data, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}

		err = yaml.Unmarshal(data, &node_config)
		if err != nil {
			return err
		}

		if protocol == "https" {	
			if node_config["tls"] != nil {
				return nil // nothing to do here...
			}
		}
	}

	// set the node config.
	address, _ :=  config.GetAddress()
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}

	node_config["name"] = local_config["Name"]
	node_config["data-dir"] = config.GetDataDir() + "/etcd-data"
	node_config["listen-peer-urls"] = protocol + "://" + address + ":2380"
	node_config["listen-client-urls"] = protocol + "://" + address + ":2379"
	node_config["advertise-client-urls"] = protocol + "://" + address + ":2379"
	node_config["initial-advertise-peer-urls"] = protocol + "://" + address + ":2380"
	node_config["initial-cluster"] =local_config["Name"].(string) + "=" + protocol + "://" + address + ":2380"
	node_config["initial-cluster-token"] = "etcd-cluster-1"
	node_config["initial-cluster-state"] = "new"

	// Here I will add the tls configuration.
	if protocol == "https" {
		node_config["tls"] = make(map[string]interface{})
		certificatePath := config.GetConfigDir() + "/tls/" + strings.Split(address, ":")[0]
		node_config["tls"].(map[string]interface{})["cert-file"] = certificatePath + "/server.crt"
		node_config["tls"].(map[string]interface{})["key-file"] = certificatePath + "/server.pem"
		node_config["tls"].(map[string]interface{})["client-cert-auth"] = true
		node_config["tls"].(map[string]interface{})["trusted-ca-file"] = certificatePath + "/ca.crt"
	}

	// Now I will append the other nodes.
	if local_config["Peers"] != nil {
		peers := local_config["Peers"].([]interface{})
		for i := 0; i < len(peers); i++ {
			peer := peers[i].(map[string]interface{})
			node_config["initial-cluster"] = node_config["initial-cluster"].(string) + "," +  peer["Hostname"].(string)+"="+protocol+"://"+peer["Hostname"].(string)+"."+peer["Domain"].(string)+":2380"
		}
	}

	// I will save the config file.
	str, err := yaml.Marshal(node_config)
	if err != nil {
		return err
	}

	err = os.WriteFile(configPath, str, 0644)
	if err != nil {
		return err
	}

	// Here I will start etcd.
	etcd := exec.Command("etcd", "--config-file", configPath)
	etcd.Dir = os.TempDir()

	// Set up a context to allow for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up a signal handler to gracefully stop etcd on interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigCh:
			fmt.Printf("Received signal %v. Shutting down gracefully...\n", sig)
			cancel() // Cancel the context to signal graceful shutdown
		}
	}()

	etcd.Stdout = os.Stdout
	etcd.Stderr = os.Stderr

	err = etcd.Start()
	if err != nil {
		log.Println("fail to start etcd", err)
		return err
	}

	// Wait for the etcd process to finish or for the context to be canceled
	err = etcd.Wait()
	if err != nil {
		log.Println("etcd process terminated with error:", err)
		return err
	}

	fmt.Println("etcd has been shut down gracefully.")

	return nil
}

// /////////////////////////// Envoy proxy /////////////////////////////////////////////////
func StartEnvoyProxy() error {

	configPath := config.GetConfigDir() + "/envoy.yml"
	if !Utility.Exists(configPath) {
	  return errors.New("no envoy configuration file found at path " + configPath)
	}

	// Here I will start envoy proxy.
	envoy := exec.Command("envoy", "-c", configPath, "-l", "warn")
	envoy.Dir = os.TempDir()

	// Set up a context to allow for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up a signal handler to gracefully stop envoy on interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigCh:
			fmt.Printf("Received signal %v. Shutting down envoy gracefully...\n", sig)
			cancel() // Cancel the context to signal graceful shutdown
		}
	}()

	envoy.Stdout = os.Stdout
	envoy.Stderr = os.Stderr

	err := envoy.Start()
	if err != nil {
		log.Println("fail to start envoy proxy", err)
		return err
	}

	// Wait for the envoy process to finish or for the context to be canceled
	err = envoy.Wait()
	if err != nil {
		log.Println("envoy process terminated with error:", err)
		return err
	}

	fmt.Println("envoy proxy has been shut down gracefully.")

	return nil
}

///////////////////////////// Monitoring //////////////////////////////////////////////////

/**
 * Start prometheus.
 */
func StartProcessMonitoring(protocol string, httpPort int, exit chan bool) error {
	// Be sure only one instance is running at time.
	ids, err := Utility.GetProcessIdsByName("prometheus")
	if err == nil {
		if len(ids) > 0 {
			return nil // nothing to do here...
		}
	}

	// Promometheus metrics for services.
	http.Handle("/metrics", promhttp.Handler())
	

	domain, _ := config.GetAddress()
	domain = strings.Split(domain, ":")[0]

	// Here I will start promethus.
	dataPath := config.GetDataDir() + "/prometheus-data"
	Utility.CreateDirIfNotExist(dataPath)
	if !Utility.Exists(config.GetConfigDir() + "/prometheus.yml") {
		config_ := `# my global config
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
    - targets: ['0.0.0.0:9090']

  - job_name: 'globular_internal_services_metrics'
    scrape_interval: 5s
    static_configs:
    - targets: ['0.0.0.0:` + Utility.ToString(httpPort) + `']

  - job_name: "envoy"
    scrape_interval: 1s
    metrics_path: /stats
    params:
      format: ['prometheus']
    static_configs:
    - targets: ['0.0.0.0:9901']

  - job_name: 'node_exporter_metrics'
    scrape_interval: 5s
    static_configs:
    - targets: ['0.0.0.0:9100']
`
		logServiceConfig, err := config.GetServiceConfigurationById("log.LogService")
		if err == nil {
			config_ += `
  - job_name: 'log_entries_metrics'
    scrape_interval: 5s
    static_configs:
    - targets: ['0.0.0.0:` + Utility.ToString(logServiceConfig["Monitoring_Port"]) + `']
    metrics_path: /metrics
    scheme: http
`
		}

		err = ioutil.WriteFile(config.GetConfigDir()+"/prometheus.yml", []byte(config_), 0644)
		if err != nil {
			return err
		}
	}

	if !Utility.Exists(config.GetConfigDir() + "/alertmanager.yml") {
		config_ := `global:
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
  - url: 'http://0.0.0.0:5001/'
inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'dev', 'instance']
`
		err := ioutil.WriteFile(config.GetConfigDir()+"/alertmanager.yml", []byte(config_), 0644)
		if err != nil {
			return err
		}
	}

	args := []string{"--web.listen-address", "0.0.0.0:9090", "--config.file", config.GetConfigDir() + "/prometheus.yml", "--storage.tsdb.path", dataPath}
	if protocol == "https" {
		args = append(args, "--web.config.file")

		// Here I will create the config file for prometheus.
		if !Utility.Exists(config.GetConfigDir() + "/prometheus_tls.yml") {

			config_ := `tls_server_config:
 cert_file: ` + config.GetConfigDir()+ "/tls/" + domain + "/" + config.GetLocalCertificate() + `
 key_file: ` + config.GetConfigDir()+ "/tls/" + domain + "/" +  config.GetLocalServerCerificateKeyPath() 

			err := ioutil.WriteFile(config.GetConfigDir()+"/prometheus_tls.yml", []byte(config_), 0644)
			if err != nil {
				return err
			}

		}

		// add the tls config file.
		args = append(args, config.GetConfigDir()+"/prometheus_tls.yml")
	}

	prometheusCmd := exec.Command("prometheus", args...)

	prometheusCmd.Dir = os.TempDir()

	err = prometheusCmd.Start()
	prometheusCmd.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	if err != nil {
		log.Println("fail to start prometheus ", err)
		return err
	}

	// Here I will monitor the cpu usage of each services
	servicesCpuUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_services_cpu_usage_counter",
		Help: "Monitor the cpu usage of each services.",
	},
		[]string{
			"id",
			"name"},
	)

	servicesMemoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_services_memory_usage_counter",
		Help: "Monitor the memory usage of each services.",
	},
		[]string{
			"id",
			"name"},
	)

	// Set the function into prometheus.
	prometheus.MustRegister(servicesCpuUsage)
	prometheus.MustRegister(servicesMemoryUsage)

	// Start feeding the time series...
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {

			case <-ticker.C:

				execName := "Globular"
				if runtime.GOOS == "windows" {
					execName += ".exe" // in case of windows
				}

				// Monitor globular itserf...
				pids, err := Utility.GetProcessIdsByName(execName)
				if err == nil {
					for i := 0; i < len(pids); i++ {
						sysInfo, err := pidusage.GetStat(pids[i])
						if err == nil {
							//log.Println("---> set cpu for process ", pids[i], "Globular", sysInfo.CPU)
							servicesCpuUsage.WithLabelValues("Globular", "Globular").Set(sysInfo.CPU)
							//log.Println("---> set memory for process ", pids[i], "Globular", sysInfo.Memory)
							servicesMemoryUsage.WithLabelValues("Globular", "Globular").Set(sysInfo.Memory)
						}
					}
				}

				services, err := config.GetServicesConfigurations()
				if err == nil {
					for i := 0; i < len(services); i++ {

						pid := Utility.ToInt(services[i]["Process"])

						if pid > 0 {
							sysInfo, err := pidusage.GetStat(pid)
							if err == nil {
								servicesCpuUsage.WithLabelValues(services[i]["Id"].(string), services[i]["Name"].(string)).Set(sysInfo.CPU)
								servicesMemoryUsage.WithLabelValues(services[i]["Id"].(string), services[i]["Name"].(string)).Set(sysInfo.Memory)
							}
						}
					}
				}
			case <-exit:
				break
			}
		}

	}()

	alertmanager := exec.Command("alertmanager", "--config.file", config.GetConfigDir()+"/alertmanager.yml")
	alertmanager.Dir = os.TempDir()

	alertmanager.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err = alertmanager.Start()
	if err != nil {
		log.Println("fail to start prometheus alert manager", err)
		// do not return here in that case simply continue without node exporter metrics.
	}

	node_exporter := exec.Command("node_exporter")
	node_exporter.Dir = os.TempDir()

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
