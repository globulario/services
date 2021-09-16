package process

import (
	"errors"
	"fmt"
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
	ps "github.com/mitchellh/go-ps"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	// Here I will set a variable that tell globular to not keep the service alive...
	s["State"] = "terminated"

	// nothing to do here...
	if proxyProcessPid == -1 && pid == -1 {
		config.SaveServiceConfiguration(s)
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
func StartServiceProcess(serviceId string, portsRange string) error {

	s, err := config.GetServicesConfigurationsById(serviceId)
	if err != nil {
		return err
	}

	// I will kill the process if is running...
	err = KillServiceProcess(s)
	if err != nil {
		return err
	}

	// Get the next available port.
	port, err := config.GetNextAvailablePort(portsRange)
	if err != nil {
		s["State"] = "failed"
		s["LastError"] = err.Error()
		config.SaveServiceConfiguration(s)
		logInfo(s["Name"].(string)+":"+s["Id"].(string), s["Domain"].(string), Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
		return err
	}

	s["Port"] = port
	err = os.Chmod(s["Path"].(string), 0755)
	if err != nil {
		s["State"] = "failed"
		s["LastError"] = err.Error()
		config.SaveServiceConfiguration(s)
		logInfo(s["Name"].(string)+":"+s["Id"].(string), s["Domain"].(string), Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
		return err
	}

	p := exec.Command(s["Path"].(string), Utility.ToString(port))
	stdout, err := p.StdoutPipe()
	if err != nil {
		s["State"] = "failed"
		s["LastError"] = err.Error()
		logInfo(s["Name"].(string)+":"+s["Id"].(string), s["Domain"].(string), Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
		config.SaveServiceConfiguration(s)

		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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

	// so here I will start each service in it own go routine.
	go func(serviceId string) {

		service_config, _ := config.GetServicesConfigurationsById(serviceId)

		// Start reading the output
		go Utility.ReadOutput(output, stdout)
		err := p.Start()

		if err != nil {
			service_config["State"] = "failed"
			service_config["Process"] = -1
			service_config["LastError"] = err.Error()
			logInfo(service_config["Name"].(string)+":"+service_config["Id"].(string), service_config["Domain"].(string), Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
			config.SaveServiceConfiguration(s)
			stdout.Close()
			done <- true
			return
		}

		service_config["State"] = "running"
		service_config["Process"] = p.Process.Pid
		config.SaveServiceConfiguration(service_config)

		// wait the process to finish
		err = p.Wait()

		if err != nil {
			service_config["State"] = "failed"
			service_config["LastError"] = err.Error()
			service_config["Process"] = -1
			logInfo(service_config["Name"].(string)+":"+service_config["Id"].(string), service_config["Domain"].(string), Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
			stdout.Close()
			done <- true
			return
		}

		// Close the output.
		stdout.Close()
		done <- true

		// kill it proxy process
		proxyProcessPid := Utility.ToInt(service_config["Process"])
		if proxyProcessPid != -1 {
			proxyProcess, err := os.FindProcess(proxyProcessPid)
			if err == nil {
				proxyProcess.Kill()
				service_config["ProxyProcess"] = -1
			}
		}

		// Set the process to -1
		service_config["ProxyProcess"] = -1
		service_config["Process"] = -1
		service_config["State"] = "stopped"

		config.SaveServiceConfiguration(service_config)
	}(s["Id"].(string))

	s["State"] = "starting"
	return config.SaveServiceConfiguration(s)
}

// Start a service process.
func StartServiceProxyProcess(serviceId, certificateAuthorityBundle, certificate, portsRange string) error {

	s, err := config.GetServicesConfigurationsById(serviceId)
	if err != nil {
		fmt.Println("error at line 232 ", err)
		return err
	}

	servicePort := Utility.ToInt(s["Port"])
	pid := Utility.ToInt(s["ProxyProcess"])
	if pid != -1 {
		KillServiceProcess(s)
	}

	dir, _ := os.Getwd()

	// Now I will start the proxy that will be use by javascript client.
	proxyPath := dir + "/bin/grpcwebproxy"
	if !strings.HasSuffix(proxyPath, ".exe") && runtime.GOOS == "windows" {
		proxyPath += ".exe" // in case of windows.
	}

	if !Utility.Exists(proxyPath) {
		fmt.Println("No grpcwebproxy found with pat" + proxyPath)
		return errors.New("No grpcwebproxy found with pat" + proxyPath)
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
		return err
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
	proxyProcess := exec.Command(proxyPath, proxyArgs...)
	proxyProcess.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	err = proxyProcess.Start()
	if err != nil {
		fmt.Println("fail to start proxy with error, ", err)
		return err
	}

	// save service configuration.
	s["ProxyProcess"] = proxyProcess.Process.Pid

	// Get the process id...
	go func() {
		err = proxyProcess.Wait()
		if err != nil {
			// if the attach process in running I will keep the proxy alive.
			_, err := Utility.GetProcessRunningStatus(Utility.ToInt(s["Process"]))
			if err != nil {
				StartServiceProxyProcess(serviceId, certificateAuthorityBundle, certificate, portsRange)
			}
			return
		}
	}()

	fmt.Println("gRpc proxy start successfully with pid:", s["ProxyProcess"], "and name:", s["Name"])

	return config.SaveServiceConfiguration(s)
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

/**
 * Keep process found in configuration in line with one found on the server.
 */
func ManageServicesProcess(exit chan bool) {

	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-exit:
				return // stop processing...
			case <-ticker.C:
				// Here I will manage the process...
				services, err := config.GetServicesConfigurations()
				if err != nil {
					return
				}

				runingProcess := make(map[string][]int)
				proxies := make([]int, 0)

				// Fist of all I will get all services process...
				for i := 0; i < len(services); i++ {
					s := services[i]
					if s["Name"] != nil {
						pid := -1
						if s["Process"] != nil {
							Utility.ToInt(s["Process"])
						}

						state := "stopped"
						if s["State"] != nil {
							state = s["State"].(string)
						}

						p, err := ps.FindProcess(pid)
						if pid != -1 && err == nil && p != nil {
							name := p.Executable()
							if _, ok := runingProcess[name]; !ok {
								runingProcess[name] = make([]int, 0)
							}
							runingProcess[name] = append(runingProcess[name], Utility.ToInt(s["Process"]))
						} else if pid == -1 || p == nil {
							if state == "killed" || state == "failed" || state == "stopped" || state == "running" {
								// make sure the process is no running...
								if s["KeepAlive"].(bool) {
									KillServiceProcess(s)
								}
							}
						}
						if s["ProxyProcess"] != nil {
							proxies = append(proxies, Utility.ToInt(s["ProxyProcess"]))
						}
					}
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
							fmt.Println("try to kill proxy process pid ", p.Pid)
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
									fmt.Println("try to kill process pid ", p.Pid)
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
