package process

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	Utility "github.com/globulario/utility"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/struCoder/pidusage"
	"gopkg.in/yaml.v3"
)

var (
	// Prometheus metrics
	servicesCpuUsage    *prometheus.GaugeVec
	servicesMemoryUsage *prometheus.GaugeVec
)

/*
KillServiceProcess terminates a running service process if its PID is set in the
service configuration map. It sets:

  - s["Process"] = -1
  - s["State"]   = "killed" (or "failed" with s["LastError"])

NOTE: This function does NOT persist the configuration to disk; the caller
is responsible for saving it if needed.
*/
func KillServiceProcess(s map[string]interface{}) error {
	pid := -1
	if s["Process"] != nil {
		pid = Utility.ToInt(s["Process"])
	}

	if pid != -1 {
		proc, err := os.FindProcess(pid)
		if err == nil {
			if err := proc.Signal(syscall.SIGTERM); err == nil {
				s["Process"] = -1
				s["State"] = "killed"
			} else {
				s["State"] = "failed"
				s["LastError"] = err.Error()
			}
		}
	}
	return nil
}

/*
StartServiceProcess launches the service binary defined by s["Path"] with the
arguments: <id> <configPath>. It:

  - Assigns service ports (s["Port"] = port, s["Proxy"] = port+1)
  - Starts the process
  - Streams child stdout to the parent console using a *carriage-return aware*
    copier so progress bars (using '\r') don’t flood the terminal.
  - Waits for process exit in a goroutine and handles KeepAlive restarts
  - Returns the child PID on success

It keeps the existing s fields and persists the configuration before/after
startup, preserving the original behavior.
*/
func StartServiceProcess(s map[string]interface{}, port int) (int, error) {
	// Kill any previous instance tracked in config.
	if err := KillServiceProcess(s); err != nil {
		return -1, err
	}

	// Ports to inject in config for the service to read.
	s["Port"] = port
	s["Proxy"] = port + 1

	path, _ := s["Path"].(string)
	if path == "" {
		return -1, fmt.Errorf("no service path for %s%s", s["Name"], s["Id"])
	}
	if !Utility.Exists(path) {
		return -1, fmt.Errorf("no service found at path %s; check install or ConfigPath %s", path, s["ConfigPath"])
	}

	cmd := exec.Command(path, s["Id"].(string), s["ConfigPath"].(string))
	cmd.Dir = filepath.Dir(path)

	// --- stdout: CR-aware to avoid terminal spam ---
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	// --- stderr: keep buffered for error messages (original behavior) ---
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Normalize stored path, reset pid
	s["Path"] = strings.ReplaceAll(path, "\\", "/")
	s["Process"] = -1

	// Persist initial config (with Port/Proxy)
	if err := config.SaveServiceConfiguration(s); err != nil {
		return -1, fmt.Errorf("save service config: %w", err)
	}

	// Ensure working directory is the folder of the binary
	cmd.Dir = s["Path"].(string)[:strings.LastIndex(s["Path"].(string), "/")]

	// Start streaming stdout before starting the process to avoid pipe races
	doneCopy := make(chan struct{})
	go func() {
		crAwareCopy(os.Stdout, stdout)
		close(doneCopy)
	}()

	// Start the process
	if err := cmd.Start(); err != nil {
		// Close pipe reader goroutine
		_ = stdout.Close()
		<-doneCopy
		fmt.Println("fail to start service", s["Name"], err, "detail", stderr.String())
		return -1, err
	}

	// Provide the PID immediately (matches original behavior)
	waitUntilStart := make(chan int, 1)
	waitUntilStart <- cmd.Process.Pid

	// Supervise in background
	go func(serviceId string) {
		err := cmd.Wait() // wait for process to exit

		if err != nil {
			fmt.Println("service "+s["Name"].(string)+" fail with error ", err, "detail", stderr.String())
			s["State"] = "failed"
		} else {
			// Reload fresh config state from disk (original behavior)
			if data, rerr := os.ReadFile(s["ConfigPath"].(string)); rerr == nil {
				_ = json.Unmarshal(data, &s)
			}
			s["State"] = "stopped"
		}

		// Close stdout copier
		_ = stdout.Close()
		<-doneCopy

		// KeepAlive: auto-restart service and its proxy
		if st, ok := s["State"].(string); ok && (st == "failed" || st == "killed") && s["KeepAlive"].(bool) {
			time.Sleep(5 * time.Second) // give time to free ports/files
			if _, rerr := StartServiceProcess(s, port); rerr == nil {
				// restart proxy if needed
				localConf, _ := config.GetLocalConfig(true)
				proxyPid := Utility.ToInt(s["ProxyProcess"])
				if proxyPid != -1 {
					if _, perr := os.FindProcess(proxyPid); perr != nil {
						_, _ = StartServiceProxyProcess(s,
							localConf["CertificateAuthorityBundle"].(string),
							localConf["Certificate"].(string))
					}
				} else {
					_, _ = StartServiceProxyProcess(s,
						localConf["CertificateAuthorityBundle"].(string),
						localConf["Certificate"].(string))
				}
			}
			return
		}

		if s["State"] == nil {
			fmt.Println("Process", s["Process"], "running", s["Name"], "has terminate and set back to -1")
			s["Process"] = -1
			_ = config.SaveServiceConfiguration(s)
		}
	}(s["Id"].(string))

	pid := <-waitUntilStart
	return pid, nil
}

/*
StartServiceProxyProcess launches the gRPC-Web proxy (grpcwebproxy) for a running service.
It reads TLS settings from the local Globular certs and configures CORS/timeouts.

Returns the proxy PID and persists s["ProxyProcess"] and s["State"]="running".
*/
func StartServiceProxyProcess(s map[string]interface{}, certificateAuthorityBundle, certificate string) (int, error) {
	// The backend service must be running
	processPid := Utility.ToInt(s["Process"])
	if processPid == -1 {
		return -1, errors.New("process pid must no be -1")
	}

	// Only one proxy per service instance
	if pid := Utility.ToInt(s["ProxyProcess"]); pid != -1 {
		return -1, fmt.Errorf("proxy already exist for service %s", s["Name"])
	}

	servicePort := Utility.ToInt(s["Port"])
	cmdName := "grpcwebproxy"
	if !strings.HasSuffix(cmdName, ".exe") && runtime.GOOS == "windows" {
		cmdName += ".exe"
	}

	address := s["Address"].(string)
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}

	backend := address + ":" + strconv.Itoa(servicePort)
	args := []string{
		"--backend_addr=" + backend,
		"--allow_all_origins=true",
		"--use_websockets=false",
		"--server_http_max_read_timeout=48h",
		"--server_http_max_write_timeout=48h",
	}

	name, _ := config.GetName()
	domain, _ := config.GetDomain()
	creds := config.GetConfigDir() + "/tls/" + name + "." + domain
	proxyPort := Utility.ToInt(s["Proxy"])

	if s["TLS"].(bool) {
		caTrust := creds + "/ca.crt"
		args = append(args,
			"--backend_tls=true",
			"--backend_tls_ca_files="+caTrust,
			"--backend_client_tls_cert_file="+creds+"/client.crt",
			"--backend_client_tls_key_file="+creds+"/client.pem",
			"--run_http_server=false",
			"--run_tls_server=true",
			"--server_http_tls_port="+strconv.Itoa(proxyPort),
			"--server_tls_key_file="+creds+"/server.pem",
			"--server_tls_client_ca_files="+creds+"/"+certificateAuthorityBundle,
			"--server_tls_cert_file="+creds+"/"+certificate,
		)
	} else {
		args = append(args,
			"--run_http_server=true",
			"--run_tls_server=false",
			"--server_http_debug_port="+strconv.Itoa(proxyPort),
			"--backend_tls=false",
		)
	}

	proxy := exec.Command(cmdName, args...)
	proxy.Dir = filepath.Dir(cmdName)
	proxy.SysProcAttr = &syscall.SysProcAttr{}

	startErr := proxy.Start()
	if startErr != nil {
		// Try fallback locations (preserve original behavior)
		if startErr.Error() == `exec: "grpcwebproxy": executable file not found in $PATH` || strings.Contains(startErr.Error(), "no such file or directory") {
			if Utility.Exists(config.GetGlobularExecPath() + "/bin/" + cmdName) {
				proxy = exec.Command(config.GetGlobularExecPath()+"/bin/"+cmdName, args...)
				proxy.Dir = config.GetGlobularExecPath() + "/bin/"
				if err := proxy.Start(); err != nil {
					return -1, err
				}
			} else {
				ex, err := os.Executable()
				if err != nil {
					return -1, err
				}
				exPath := filepath.Dir(ex)
				if Utility.Exists(exPath + "/bin/" + cmdName) {
					proxy = exec.Command(exPath+"/bin/"+cmdName, args...)
					proxy.Dir = exPath + "/bin/"
					if err := proxy.Start(); err != nil {
						return -1, err
					}
				} else {
					return -1, errors.New("the program grpcwebproxy is not install on the system")
				}
			}
		} else {
			return -1, startErr
		}
	}

	// Publish updated service configuration to listeners
	str, _ := Utility.ToJson(s)
	wait := make(chan int, 1)

	go func() {
		address, _ := config.GetAddress()
		if ec, err := getEventClient(address); err == nil {
			_ = ec.Publish("update_globular_service_configuration_evt", []byte(str))
		}

		var sc map[string]interface{}
		_ = json.Unmarshal([]byte(str), &sc)

		fmt.Println("Service", sc["Name"].(string)+":"+sc["Id"].(string),
			"is running pid", processPid, "and lisen at port", sc["Port"],
			"with proxy pid", proxy.Process.Pid, "lisen at port port", proxyPort)

		wait <- proxy.Process.Pid

		// Wait for proxy to exit and optionally restart if backend still alive
		_ = proxy.Wait()

		data, err := os.ReadFile(sc["ConfigPath"].(string))
		if err != nil {
			fmt.Println("proxy process fail with error:", err)
			return
		}
		_ = json.Unmarshal(data, &sc)

		procPid := Utility.ToInt(sc["Process"])
		if procPid != -1 && sc["KeepAlive"].(bool) {
			if exist, err := Utility.PidExists(procPid); err == nil && exist {
				_, _ = StartServiceProxyProcess(sc, certificateAuthorityBundle, certificate)
			} else if err != nil {
				fmt.Println("proxy process fail with error:", err)
			}
		}
	}()

	proxyPid := <-wait
	s["State"] = "running"
	s["ProxyProcess"] = proxyPid
	return proxyPid, config.SaveServiceConfiguration(s)
}

/*
GetProcessRunningStatus checks if a given PID represents a currently running process.

Returns the *os.Process on success, or an error indicating whether the process
is not running or the operation is not permitted.
*/
func GetProcessRunningStatus(pid int) (*os.Process, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}
	if err := proc.Signal(syscall.Signal(0)); err == nil {
		return proc, nil
	}
	if err == syscall.ESRCH {
		return nil, errors.New("process not running")
	}
	return nil, errors.New("process running but query operation not permitted")
}

/*
StartEtcdServer writes an etcd configuration (if needed) and starts the etcd
process in the foreground, wiring stdout/stderr to the parent console. It also
handles graceful shutdown on SIGINT/SIGTERM.
*/
func StartEtcdServer() error {
	localConfig, err := config.GetLocalConfig(true)
	if err != nil {
		return err
	}
	protocol := localConfig["Protocol"].(string)

	nodeCfg := make(map[string]interface{})
	cfgPath := config.GetConfigDir() + "/etcd.yml"

	if Utility.Exists(cfgPath) {
		if data, err := os.ReadFile(cfgPath); err == nil {
			_ = yaml.Unmarshal(data, &nodeCfg)
			if protocol == "https" && nodeCfg["tls"] != nil {
				// already TLS-configured
			}
		}
	}

	address, _ := config.GetAddress()
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}

	nodeCfg["name"] = localConfig["Name"]
	nodeCfg["data-dir"] = config.GetDataDir() + "/etcd-data"
	nodeCfg["listen-peer-urls"] = protocol + "://" + address + ":2380"
	nodeCfg["listen-client-urls"] = protocol + "://" + address + ":2379"
	nodeCfg["advertise-client-urls"] = protocol + "://" + address + ":2379"
	nodeCfg["initial-advertise-peer-urls"] = protocol + "://" + address + ":2380"
	nodeCfg["initial-cluster"] = localConfig["Name"].(string) + "=" + protocol + "://" + address + ":2380"
	nodeCfg["initial-cluster-token"] = "etcd-cluster-1"
	nodeCfg["initial-cluster-state"] = "new"

	if protocol == "https" {
		certDir := config.GetConfigDir() + "/tls/" + address
		nodeCfg["tls"] = map[string]interface{}{
			"cert-file":        certDir + "/server.crt",
			"key-file":         certDir + "/server.pem",
			"client-cert-auth": true,
			"trusted-ca-file":  certDir + "/ca.crt",
		}
	}

	if localConfig["Peers"] != nil {
		for _, p := range localConfig["Peers"].([]interface{}) {
			peer := p.(map[string]interface{})
			nodeCfg["initial-cluster"] = nodeCfg["initial-cluster"].(string) + "," +
				peer["Hostname"].(string) + "=" + protocol + "://" + peer["Hostname"].(string) + "." + peer["Domain"].(string) + ":2380"
		}
	}

	out, err := yaml.Marshal(nodeCfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, out, 0644); err != nil {
		return err
	}

	etcd := exec.Command("etcd", "--config-file", cfgPath)
	etcd.Dir = os.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigCh:
			fmt.Printf("Received signal %v. Shutting down gracefully...\n", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	etcd.Stdout = os.Stdout
	etcd.Stderr = os.Stderr

	if err := etcd.Start(); err != nil {
		log.Println("fail to start etcd", err)
		return err
	}
	if err := etcd.Wait(); err != nil {
		log.Println("etcd process terminated with error:", err)
		return err
	}
	fmt.Println("etcd has been shut down gracefully.")
	return nil
}

/*
StartEnvoyProxy starts Envoy with the config file at <config>/envoy.yml,
wiring stdout/stderr to the parent console and handling graceful shutdown.

It returns an error if the config file is missing or Envoy fails to start.
*/
func StartEnvoyProxy() error {
	cfgPath := config.GetConfigDir() + "/envoy.yml"
	if !Utility.Exists(cfgPath) {
		return errors.New("no envoy configuration file found at path " + cfgPath)
	}

	envoy := exec.Command("envoy", "-c", cfgPath, "-l", "warn")
	envoy.Dir = os.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigCh:
			fmt.Printf("Received signal %v. Shutting down envoy gracefully...\n", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	envoy.Stdout = os.Stdout
	envoy.Stderr = os.Stderr

	if err := envoy.Start(); err != nil {
		log.Println("fail to start envoy proxy", err)
		return err
	}
	if err := envoy.Wait(); err != nil {
		log.Println("envoy process terminated with error:", err)
		return err
	}
	fmt.Println("envoy proxy has been shut down gracefully.")
	return nil
}

/*
StartProcessMonitoring starts Prometheus, Alertmanager, and node_exporter (best-effort),
exposes internal metrics at /metrics, and periodically feeds per-process CPU/memory
metrics for Globular and known services.

Arguments:
  - protocol: "http" or "https" to configure Prometheus web UI
  - httpPort: the Globular HTTP port to scrape internal metrics
  - exit:     a channel to signal termination of the metrics feeder loop
*/
func StartProcessMonitoring(protocol string, httpPort int, exit chan bool) error {
	// If Prometheus is already running, do nothing
	if ids, err := Utility.GetProcessIdsByName("prometheus"); err == nil && len(ids) > 0 {
		return nil
	}

	// Expose /metrics
	http.Handle("/metrics", promhttp.Handler())

	domain, _ := config.GetAddress()
	domain = strings.Split(domain, ":")[0]

	// Prepare data/config files
	dataPath := config.GetDataDir() + "/prometheus-data"
	_ = Utility.CreateDirIfNotExist(dataPath)

	promYml := config.GetConfigDir() + "/prometheus.yml"
	if !Utility.Exists(promYml) {
		cfg := `# my global config
global:
  scrape_interval:     15s
  evaluation_interval: 15s
alerting:
  alertmanagers:
  - static_configs:
    - targets: []
rule_files: []
scrape_configs:
  - job_name: 'prometheus'
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
		if logCfg, err := config.GetServiceConfigurationById("log.LogService"); err == nil {
			cfg += `
  - job_name: 'log_entries_metrics'
    scrape_interval: 5s
    static_configs:
    - targets: ['0.0.0.0:` + Utility.ToString(logCfg["Monitoring_Port"]) + `']
    metrics_path: /metrics
    scheme: http
`
		}
		if err := os.WriteFile(promYml, []byte(cfg), 0644); err != nil {
			return err
		}
	}

	alertYml := config.GetConfigDir() + "/alertmanager.yml"
	if !Utility.Exists(alertYml) {
		cfg := `global:
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
		if err := os.WriteFile(alertYml, []byte(cfg), 0644); err != nil {
			return err
		}
	}

	args := []string{
		"--web.listen-address", "0.0.0.0:9090",
		"--config.file", promYml,
		"--storage.tsdb.path", dataPath,
	}
	if protocol == "https" {
		webCfg := config.GetConfigDir() + "/prometheus_tls.yml"
		if !Utility.Exists(webCfg) {
			cfg := "tls_server_config:\n" +
				" cert_file: " + config.GetConfigDir() + "/tls/" + domain + "/" + config.GetLocalCertificate() + "\n" +
				" key_file: " + config.GetConfigDir() + "/tls/" + domain + "/" + config.GetLocalServerCerificateKeyPath() + "\n"
			if err := os.WriteFile(webCfg, []byte(cfg), 0644); err != nil {
				return err
			}
		}
		args = append(args, "--web.config.file", webCfg)
	}

	prom := exec.Command("prometheus", args...)
	prom.Dir = os.TempDir()
	prom.SysProcAttr = &syscall.SysProcAttr{}

	if err := prom.Start(); err != nil {
		log.Println("fail to start prometheus", err)
		return err
	}

	// Register metrics
	servicesCpuUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_services_cpu_usage_counter",
		Help: "Monitor the cpu usage of each services.",
	}, []string{"id", "name"})
	servicesMemoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_services_memory_usage_counter",
		Help: "Monitor the memory usage of each services.",
	}, []string{"id", "name"})
	prometheus.MustRegister(servicesCpuUsage, servicesMemoryUsage)

	// Feed metrics periodically
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				execName := "Globular"
				if runtime.GOOS == "windows" {
					execName += ".exe"
				}
				if pids, err := Utility.GetProcessIdsByName(execName); err == nil {
					for _, pid := range pids {
						if stat, err := pidusage.GetStat(pid); err == nil {
							servicesCpuUsage.WithLabelValues("Globular", "Globular").Set(stat.CPU)
							servicesMemoryUsage.WithLabelValues("Globular", "Globular").Set(stat.Memory)
						}
					}
				}
				if services, err := config.GetServicesConfigurations(); err == nil {
					for _, svc := range services {
						pid := Utility.ToInt(svc["Process"])
						if pid > 0 {
							if stat, err := pidusage.GetStat(pid); err == nil {
								servicesCpuUsage.WithLabelValues(svc["Id"].(string), svc["Name"].(string)).Set(stat.CPU)
								servicesMemoryUsage.WithLabelValues(svc["Id"].(string), svc["Name"].(string)).Set(stat.Memory)
							}
						}
					}
				}
			case <-exit:
				return
			}
		}
	}()

	// Best-effort Alertmanager and node_exporter
	alert := exec.Command("alertmanager", "--config.file", alertYml)
	alert.Dir = os.TempDir()
	alert.SysProcAttr = &syscall.SysProcAttr{}
	if err := alert.Start(); err != nil {
		log.Println("fail to start prometheus alert manager", err)
	}

	nodeExp := exec.Command("node_exporter")
	nodeExp.Dir = os.TempDir()
	nodeExp.SysProcAttr = &syscall.SysProcAttr{}
	if err := nodeExp.Start(); err != nil {
		log.Println("fail to start prometheus node exporter", err)
	}

	return nil
}

// ---------- internals ----------

/*
crAwareCopy copies from src to dst while treating '\r' (carriage return) as
"in-place update" instead of a new line. This prevents tools that render
progress bars using '\r' from flooding the parent console with thousands
of lines when stdout is piped.

It writes:
  - '\r'  -> "\r\033[K" (CR + clear line)
  - '\n'  -> '\n'
  - other -> byte as-is
*/
func crAwareCopy(dst io.Writer, src io.Reader) {
	reader := bufio.NewReader(src)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// best-effort: surface the error once
				fmt.Fprintln(dst, "\n[stream err]", err.Error())
			}
			return
		}
		switch b {
		case '\r':
			// return to start of line and clear it
			_, _ = dst.Write([]byte("\r\033[K"))
		default:
			_, _ = dst.Write([]byte{b})
		}
	}
}

// Local event client helper (unchanged signature/behavior)
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
