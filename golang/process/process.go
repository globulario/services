package process

import (
	//"encoding/json"
	"errors"
	"fmt"
	//"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
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

func setServiceConfigurationError(err error, s map[string]interface{}){
	s["State"] = "failed"
	s["LastError"] = err.Error()
	config.SaveServiceConfiguration(s)
	logInfo(s["Name"].(string)+":"+s["Id"].(string), s["Domain"].(string), Utility.FileLine(), Utility.FunctionName(), err.Error(), logpb.LogLevel_ERROR_MESSAGE)
}

// Start a service process.
func StartServiceProcess(serviceId string, portsRange string) (int, error) {

	s, err := config.GetServicesConfigurationsById(serviceId)
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

	// Start reading the output
	go Utility.ReadOutput(output, stdout)
	err = p.Start()

	if err != nil {
		setServiceConfigurationError(err, s)
		stdout.Close()
		done <- true
		return -1,  err
	}

	s["State"] = "running"
	s["Process"] = p.Process.Pid

	// so here I will start each service in it own go routine.
	go func(serviceId string) {

		// wait the process to finish
		err = p.Wait()

		s, _ := config.GetServicesConfigurationsById(serviceId)

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
func StartServiceProxyProcess(serviceId, certificateAuthorityBundle, certificate, portsRange string, processPid int) error {

	s, err := config.GetServicesConfigurationsById(serviceId)
	if err != nil {
		fmt.Println("error at line 232 ", err)
		return err
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
		//CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
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