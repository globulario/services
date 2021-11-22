package process

import (
	//"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/struCoder/pidusage"
)

/**
 * Kill a given service instance.
 */
func KillServiceProcess(s map[string]interface{}) error {

	pid := -1
	if (s["Process"]) != nil {
		pid = Utility.ToInt(s["Process"])
	}

	proxyProcessPid := -1
	if (s["ProxyProcess"]) != nil {
		proxyProcessPid = Utility.ToInt(s["ProxyProcess"])
	}

	// nothing to do here...
	if proxyProcessPid == -1 && pid == -1 {
		return nil
	}

	// also kill it proxy process if exist in that case.
	proxyProcess, err := os.FindProcess(proxyProcessPid)
	if err == nil {
		proxyProcess.Kill()
		s["ProxyProcess"] = -1
	}

	fmt.Println("Kill process ", s["Name"], "with pid", pid, "and proxy", proxyProcessPid)

	// kill it in the name of...
	process, err := os.FindProcess(pid)
	s["State"] = "stopped"
	if err == nil {
		err := process.Kill()
		if err == nil {
			s["Process"] = -1
			s["State"] = "killed"
		} else {
			s["State"] = "failed"
			s["LastError"] = err.Error()
		}
	}

	// save the service configuration.
	return config.SaveServiceConfiguration(s)
}

var (
	log_client_ *log_client.Log_Client

	// Monitor the cpu usage of process.
	servicesCpuUsage    *prometheus.GaugeVec
	servicesMemoryUsage *prometheus.GaugeVec
)

/**
 * Get the log client.
 */
func getLogClient(domain string) (*log_client.Log_Client, error) {
	var err error
	if log_client_ == nil {
		log_client_, err = log_client.NewLogService_Client(domain, "log.LogService")
		if err != nil {
			return nil, err
		}

	}
	return log_client_, nil
}

func logInfo(name, domain, fileLine, functionName, message string, level logpb.LogLevel) {
	log_client_, err := getLogClient(domain)
	if err != nil {
		return
	}
	log_client_.Log(name, domain, functionName, level, message, fileLine, functionName)
}

func setServiceConfigurationError(err error, s map[string]interface{}) {
	s["State"] = "failed"
	s["LastError"] = err.Error()
	config.SaveServiceConfiguration(s)
	logInfo(s["Name"].(string)+":"+s["Id"].(string), s["Domain"].(string), Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
}

// Start a service process.
func StartServiceProcess(serviceId string, portsRange string) (int, error) {

	s, err := config.GetServiceConfigurationById(serviceId)
	if err != nil {
		return -1, err
	}

	// I will kill the process if is running...
	err = KillServiceProcess(s)
	if err != nil {
		return -1, err
	}

	// Get the next available port.
	port, err := config.GetNextAvailablePort(portsRange)
	if err != nil {
		setServiceConfigurationError(err, s)
		return -1, err
	}

	s["Port"] = port

	if !Utility.Exists(s["Path"].(string)) {
		// In that case I will try to
		if s["ConfigPath"] != nil {
			s["ConfigPath"] = strings.ReplaceAll(s["ConfigPath"].(string), "\\", "/")
			s["Path"] = s["ConfigPath"].(string)[0:strings.LastIndex(s["ConfigPath"].(string), "/")] + s["Path"].(string)
		}
	}

	err = os.Chmod(s["Path"].(string), 0755)
	if err != nil {
		setServiceConfigurationError(err, s)
		return -1, err
	}

	
	p := exec.Command(s["Path"].(string), Utility.ToString(port))
	stdout, err := p.StdoutPipe()
	if err != nil {
		setServiceConfigurationError(err, s)
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

	s["Path"] = strings.ReplaceAll(s["Path"].(string), "\\", "/")

	// Set the process dir.
	p.Dir = s["Path"].(string)[0:strings.LastIndex(s["Path"].(string), "/")]

	// Start reading the output
	go Utility.ReadOutput(output, stdout)
	err = p.Start()

	if err != nil {
		setServiceConfigurationError(err, s)
		stdout.Close()
		done <- true
		return -1, err
	}

	s["State"] = "running"
	s["Process"] = p.Process.Pid

	// so here I will start each service in it own go routine.
	go func(serviceId string) {

		// wait the process to finish
		err = p.Wait()

		s, _ := config.GetServiceConfigurationById(serviceId)

		if err != nil {
			setServiceConfigurationError(err, s)
			stdout.Close()
			done <- true
			return
		}

		// Close the output.
		stdout.Close()
		done <- true

		// kill it proxy process
		proxyProcessPid := Utility.ToInt(s["ProxyProcess"])
		if proxyProcessPid != -1 {
			proxyProcess, err := os.FindProcess(proxyProcessPid)
			if err == nil {
				proxyProcess.Kill()
				s["ProxyProcess"] = -1
			}
		}

		// Set the process to -1
		s["Process"] = -1
		s["State"] = "stopped"
		config.SaveServiceConfiguration(s)

	}(s["Id"].(string))

	// save the service configuration.
	return p.Process.Pid, config.SaveServiceConfiguration(s)
}

// Start a service process.
func StartServiceProxyProcess(serviceId, certificateAuthorityBundle, certificate, portsRange string, processPid int) (int, error) {

	s, err := config.GetServiceConfigurationById(serviceId)
	if err != nil {
		fmt.Println("error at line 232 ", err)
		return -1, err
	}

	servicePort := Utility.ToInt(s["Port"])
	pid := Utility.ToInt(s["ProxyProcess"])
	if pid != -1 {
		Utility.TerminateProcess(pid, 0)
	}

	dir, _ := os.Getwd()

	// Now I will start the proxy that will be use by javascript client.
	proxyPath := dir + "/bin/grpcwebproxy"
	if !strings.HasSuffix(proxyPath, ".exe") && runtime.GOOS == "windows" {
		proxyPath += ".exe" // in case of windows.
	}

	if !Utility.Exists(proxyPath) {
		fmt.Println("No grpcwebproxy found with pat" + proxyPath)
		return -1, errors.New("No grpcwebproxy found with pat" + proxyPath)
	}

	proxyBackendAddress := s["Domain"].(string) + ":" + strconv.Itoa(servicePort)
	proxyAllowAllOrgins := "true"
	proxyArgs := make([]string, 0)

	// Use in a local network or in test.
	proxyArgs = append(proxyArgs, "--backend_addr="+proxyBackendAddress)
	proxyArgs = append(proxyArgs, "--allow_all_origins="+proxyAllowAllOrgins)
	hasTls := s["TLS"].(bool)
	creds := config.GetConfigDir() + "/tls"

	// Test if the port is available.
	port, err := config.GetNextAvailablePort(portsRange)
	if err != nil {
		fmt.Println("fail to start proxy with error, ", err)
		return -1, err
	}

	s["Proxy"] = port
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
		proxyArgs = append(proxyArgs, "--server_http_tls_port="+strconv.Itoa(port))

		/* in case of public domain server files **/
		proxyArgs = append(proxyArgs, "--server_tls_key_file="+creds+"/server.pem")
		proxyArgs = append(proxyArgs, "--server_tls_client_ca_files="+creds+"/"+certificateAuthorityBundle)
		proxyArgs = append(proxyArgs, "--server_tls_cert_file="+creds+"/"+certificate)

	} else {
		// Now I will save the file with those new information in it.
		proxyArgs = append(proxyArgs, "--run_http_server=true")
		proxyArgs = append(proxyArgs, "--run_tls_server=false")
		proxyArgs = append(proxyArgs, "--server_http_debug_port="+strconv.Itoa(port))
		proxyArgs = append(proxyArgs, "--backend_tls=false")
	}

	// Keep connection open for longer exchange between client/service. Event Subscribe function
	// is a good example of long lasting connection. (48 hours) seam to be more than enought for
	// browser client connection maximum life.
	proxyArgs = append(proxyArgs, "--server_http_max_read_timeout=48h")
	proxyArgs = append(proxyArgs, "--server_http_max_write_timeout=48h")
	proxyArgs = append(proxyArgs, "--use_websockets=true")

	// start the proxy service one time
	//fmt.Println(proxyPath, proxyArgs)
	proxyProcess := exec.Command(proxyPath, proxyArgs...)
	proxyProcess.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err = proxyProcess.Start()
	if err != nil {
		fmt.Println("fail to start proxy with error, ", err)
		return -1, err
	}

	// save service configuration.
	s["ProxyProcess"] = proxyProcess.Process.Pid

	// Get the process id...
	go func() {
		err = proxyProcess.Wait()
		// refresh the configuration

		if err != nil {
			// if the attach process in running I will keep the proxy alive.
			if processPid != -1 {
				_, err := Utility.GetProcessRunningStatus(processPid)
				if err != nil {
					fmt.Println("proxy prcess fail with error:  ", err)
					StartServiceProxyProcess(serviceId, certificateAuthorityBundle, certificate, portsRange, processPid)
				}
			}
			return
		}
	}()

	fmt.Println("gRpc proxy start successfully with pid:", s["ProxyProcess"], "and name:", s["Name"])
	return proxyProcess.Process.Pid, config.SaveServiceConfiguration(s)
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

///////////////////////////// Monitoring //////////////////////////////////////////////////

/**
 * Start prometheus.
 */
func StartProcessMonitoring(httpPort int, exit chan bool) error {
	// Promometheus metrics for services.
	http.Handle("/metrics", promhttp.Handler())

	var err error

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
    - targets: ['localhost:9090']
  
  - job_name: 'globular_internal_services_metrics'
    scrape_interval: 5s
    static_configs:
    - targets: ['localhost:` + Utility.ToString(httpPort) + `']
    
  - job_name: 'node_exporter_metrics'
    scrape_interval: 5s
    static_configs:
    - targets: ['localhost:9100']
    
`
		err := ioutil.WriteFile(config.GetConfigDir()+"/prometheus.yml", []byte(config_), 0644)
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
  - url: 'http://127.0.0.1:5001/'
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

	prometheusCmd := exec.Command("prometheus", "--web.listen-address", "0.0.0.0:9090", "--config.file", config.GetConfigDir()+"/prometheus.yml", "--storage.tsdb.path", dataPath)
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
								//log.Println("---> set cpu for process ", services[i]["Name"], sysInfo.CPU)
								servicesCpuUsage.WithLabelValues(services[i]["Id"].(string), services[i]["Name"].(string)).Set(sysInfo.CPU)
								//log.Println("---> set memory for process ", services[i]["Name"], sysInfo.Memory)
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
