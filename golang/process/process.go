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
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/event/event_client"
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

	if pid != -1 {
		// kill it in the name of...
		process, err := os.FindProcess(pid)
		if err == nil {
			err := process.Kill()
			//err := syscall.Kill(process.Pid,syscall.SIGTERM)
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

// Start a service process.
func StartServiceProcess(s map[string]interface{}, portsRange string) (int, error) {

	// I will kill the process if is running...
	err := KillServiceProcess(s)
	if err != nil {
		return -1, err
	}

	// Get the next available port.
	port, err := config.GetNextAvailablePort(portsRange)
	if err != nil {
		return -1, err
	}

	s["Port"] = port
	if s["Path"] == nil {
		err := errors.New("no service path was found for service " + s["Name"].(string) + s["Id"].(string))
		fmt.Println(err)
		return -1, err
	}

	if !Utility.Exists(s["Path"].(string)) {
		log.Println("No service found at path ", s["Path"].(string))
		return -1, errors.New("No service found at path " + s["Path"].(string) + " be sure globular is install correctly, or the configuration at path " + s["ConfigPath"].(string) + " point at correct service path.")
	}

	err = os.Chmod(s["Path"].(string), 0755)
	if err != nil {
		return -1, err
	}

	p := exec.Command(s["Path"].(string), s["Id"].(string), s["ConfigPath"].(string))
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
	s["ProxyProcess"] = -1
	s["Process"] = -1

	// save the port and ProxyProcess
	err = config_client.SaveServiceConfiguration(s)
	if err != nil {
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
		return -1, err
	}

	waitUntilStart := make(chan int)

	// so here I will start each service in it own go routine.
	go func(serviceId string) {

		ok, err := Utility.PidExists(p.Process.Pid)
		for !ok {
			ok, err = Utility.PidExists(Utility.ToInt(s["Process"]))
			time.Sleep(10 * time.Millisecond)
		}

		// Set the pid...
		s["Process"] = p.Process.Pid
		s["State"] = "running"
		s["LastError"] = ""
		config_client.SaveServiceConfiguration(s)

		// give back the process id.
		waitUntilStart <- Utility.ToInt(s["Process"])

		err = p.Wait()
		// Here I will read the configuration to get more information about the process.
		if err != nil {
			fmt.Println("service "+s["Name"].(string)+" fail with error ", err)
			s["State"] = "failed"
		} else {
			s["State"] = "stopped"
		}

		// be sure the state is not nil and failed.
		if s["State"] != nil {
			// if the service fail
			if s["State"].(string) == "failed" || s["State"].(string) == "killed" {
				fmt.Println("the service ", s["Name"], "with process id", s["Process"], "has been terminate")
				if s["KeepAlive"].(bool) == true {

					// give ti some time to free resources like port files... etc.
					pid, err := StartServiceProcess(s, portsRange)

					// so here I need to restart it proxy process...
					proxyProcessPid := Utility.ToInt(s["ProxyProcess"])
					if proxyProcessPid != -1 {
						proxyProcess, err := os.FindProcess(proxyProcessPid)
						if err == nil {
							proxyProcess.Kill()
							s["ProxyProcess"] = -1
						}
					}

					if err == nil {
						localConfig, _ := config.GetLocalConfig(true)
						// Now I will restart it grpc service.
						StartServiceProxyProcess(s, localConfig["CertificateAuthorityBundle"].(string), localConfig["Certificate"].(string), localConfig["PortsRange"].(string), pid)
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

		s["Process"] = -1

		// kill it proxy process
		proxyProcessPid := Utility.ToInt(s["ProxyProcess"])
		if proxyProcessPid != -1 {
			proxyProcess, err := os.FindProcess(proxyProcessPid)
			if err == nil {
				proxyProcess.Kill()
				s["ProxyProcess"] = -1
			}
		}

		config_client.SaveServiceConfiguration(s)

	}(s["Id"].(string))

	pid := <-waitUntilStart

	// save the service configuration.
	return pid, nil
}

var (
	event_client_ *event_client.Event_Client
)

/**
 * Get local event client.
 */
func getEventClient(domain string) (*event_client.Event_Client, error) {
	var err error
	if event_client_ == nil {
		event_client_, err = event_client.NewEventService_Client(domain, "event.EventService")
		if err != nil {
			return nil, err
		}
	}

	return event_client_, nil
}

// Start a service process.
func StartServiceProxyProcess(s map[string]interface{}, certificateAuthorityBundle, certificate, portsRange string, processPid int) (int, error) {

	servicePort := Utility.ToInt(s["Port"])
	pid := Utility.ToInt(s["ProxyProcess"])
	if pid != -1 {
		// Utility.TerminateProcess(pid, 0)
		return -1, errors.New("proxy already exist for service " + s["Name"].(string))
	}

	// Now I will start the proxy that will be use by javascript client.
	cmd := "grpcwebproxy"
	if !strings.HasSuffix(cmd, ".exe") && runtime.GOOS == "windows" {
		cmd += ".exe" // in case of windows.
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
	proxyProcess := exec.Command(cmd, proxyArgs...)
	proxyProcess.SysProcAttr = &syscall.SysProcAttr{
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err = proxyProcess.Start()
	if err != nil {

		if err.Error() == `exec: "grpcwebproxy": executable file not found in $PATH` {
			if Utility.Exists(config.GetRootDir() + "/bin/" + cmd) {
				proxyProcess = exec.Command(config.GetRootDir()+"/bin/"+cmd, proxyArgs...)
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
					err = proxyProcess.Start()
					if err != nil {
						return -1, err
					}
				} else {
					return -1, err
				}
			}
		} else {
			return -1, err
		}
	}

	// save service configuration.

	// Get the process id...
	go func() {

		// finaly i will send event to give change of connected globule to update their service configurations with the new values...
		address, _ := config.GetAddress()
		event_client_, err := getEventClient(address)
		if err == nil {
			// Here I will publish the start service event
			str, _ := Utility.ToJson(s)
			event_client_.Publish("update_globular_service_configuration_evt", []byte(str))
		}

		// wait to proxy
		proxyProcess.Wait()
		fmt.Println("gRpc proxy with pid:", proxyProcess.Process.Pid, "service:", s["Name"].(string)+":"+s["Id"].(string), "stop successfully")

		if processPid != -1 {
			exist, err := Utility.PidExists(processPid)
			if err == nil && exist {
				StartServiceProxyProcess(s, certificateAuthorityBundle, certificate, portsRange, processPid)
			} else if err != nil {
				fmt.Println("proxy prcess fail with error:  ", err)
				return
			}
		}

		return

	}()

	// be sure the service
	s["State"] = "running"
	s["ProxyProcess"] = proxyProcess.Process.Pid
	s["Proxy"] = port

	fmt.Println("gRpc proxy start successfully with pid:", s["ProxyProcess"], " for service:", s["Name"].(string)+":"+s["Id"].(string), "pid:", s["Process"], "http port:", port, "grpc port", s["Port"])
	return proxyProcess.Process.Pid, config_client.SaveServiceConfiguration(s)

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
	// Be sure only one instance is running at time.
	ids, err := Utility.GetProcessIdsByName("prometheus")
	if err == nil {
		if len(ids) > 0 {
			return nil // nothing to do here...
		}
	}

	// Promometheus metrics for services.
	http.Handle("/metrics", promhttp.Handler())

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

				services, err := config_client.GetServicesConfigurations()
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
