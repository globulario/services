package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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

	// Here I will set a variable that tell globular to not keep the service alive...
	s["State"] = "terminated"

	if pid != -1 {
		
		// kill it in the name of...
		process, err := os.FindProcess(pid)
		s["State"] = "stopped"
		if err == nil {
			fmt.Println("Kill process ", s["Name"], "with pid", pid)
			err := process.Kill()
			if err == nil {
				s["Process"] = -1
				s["State"] = "killed"
			} else {
				s["State"] = "failed"
				s["LastError"] = err.Error()
			}
		}else{
			s["Process"] = -1
			s["State"] = "stopped"
		}
	}else{
		s["State"] = "stopped"
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

		service_config["Process"] = -1
		service_config["State"] = "stopped"

		config.SaveServiceConfiguration(service_config)
	}(s["Id"].(string))

	s["State"] = "starting"
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
