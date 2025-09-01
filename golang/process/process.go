package process

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
KillServiceProcess terminates a running service process if its PID is set in
the service configuration map.

Side effects on the given service map:
  - sets s["Process"] = -1 on successful SIGTERM
  - sets s["State"]   = "killed" on success
  - sets s["State"]   = "failed" and s["LastError"] on error

Note: This function does NOT persist the configuration; callers may choose to save.
*/
func KillServiceProcess(s map[string]interface{}) error {
	pid := -1
	if s["Process"] != nil {
		pid = Utility.ToInt(s["Process"])
	}

	if pid == -1 {
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		slog.Warn("failed to find process to kill", "pid", pid, "err", err)
		return nil
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		s["State"] = "failed"
		s["LastError"] = err.Error()
		slog.Error("failed to terminate process", "pid", pid, "err", err)
		return nil
	}

	s["Process"] = -1
	s["State"] = "killed"
	slog.Info("service process terminated", "pid", pid)

	if err := KillServiceProxyProcess(s); err != nil {
		slog.Warn("failed to terminate proxy after service kill", "err", err)
	}
	
	return nil
}

/*
StartServiceProcess launches the service binary defined by s["Path"] with
arguments "<Id> <ConfigPath>". It:

  - injects s["Port"]=port and s["Proxy"]=port+1
  - starts the process
  - streams child stdout to the parent console with CR-aware copying
  - watches for exit in a goroutine and handles KeepAlive restarts
  - returns the child PID on success

The service map is persisted before and after startup, preserving the
original behavior.
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
		return -1, fmt.Errorf("missing service binary path for %s%s", s["Name"], s["Id"])
	}
	if !Utility.Exists(path) {
		return -1, fmt.Errorf("service binary not found at %s (config: %v)", path, s["ConfigPath"])
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
		_ = stdout.Close()
		<-doneCopy
		slog.Error("failed to start service", "name", s["Name"], "id", s["Id"], "err", err, "stderr", strings.TrimSpace(stderr.String()))
		return -1, err
	}

	slog.Info("service process started", "name", s["Name"], "id", s["Id"], "pid", cmd.Process.Pid, "port", port, "proxy", port+1)

	// Provide the PID immediately (matches original behavior)
	waitUntilStart := make(chan int, 1)
	waitUntilStart <- cmd.Process.Pid

	// Supervise in background
	go func() {
		err := cmd.Wait() // wait for process to exit

		if err != nil {
			slog.Error("service process exited with error", "name", s["Name"], "id", s["Id"], "pid", cmd.Process.Pid, "err", err, "stderr", strings.TrimSpace(stderr.String()))
			s["State"] = "failed"
		} else {
			// Reload fresh config state from disk (original behavior)
			if data, rerr := os.ReadFile(s["ConfigPath"].(string)); rerr == nil {
				_ = json.Unmarshal(data, &s)
			}
			s["State"] = "stopped"
			slog.Info("service process stopped", "name", s["Name"], "id", s["Id"], "pid", cmd.Process.Pid)
		}

		// Close stdout copier
		_ = stdout.Close()
		<-doneCopy

		// KeepAlive: auto-restart service and its proxy
		if st, ok := s["State"].(string); ok && (st == "failed" || st == "killed") && s["KeepAlive"].(bool) {
			slog.Info("keepalive: restarting service", "name", s["Name"], "id", s["Id"], "delay", "5s")
			time.Sleep(5 * time.Second) // give time to free ports/files

			if _, rerr := StartServiceProcess(s, port); rerr == nil {
				// restart proxy if needed
				localConf, _ := config.GetLocalConfig(true)
				proxyPid := Utility.ToInt(s["ProxyProcess"])
				if proxyPid != -1 {
					if _, perr := os.FindProcess(proxyPid); perr != nil {
						slog.Info("keepalive: restarting proxy (dead)", "name", s["Name"], "id", s["Id"])
						_, _ = StartServiceProxyProcess(s,
							localConf["CertificateAuthorityBundle"].(string),
							localConf["Certificate"].(string))
					}
				} else {
					slog.Info("keepalive: starting proxy", "name", s["Name"], "id", s["Id"])
					_, _ = StartServiceProxyProcess(s,
						localConf["CertificateAuthorityBundle"].(string),
						localConf["Certificate"].(string))
				}
			} else {
				slog.Error("keepalive: failed to restart service", "name", s["Name"], "id", s["Id"], "err", rerr)
			}
			return
		}

		if s["State"] == nil {
			// Align with prior behavior if state vanished
			slog.Info("service process terminated; resetting pid", "name", s["Name"], "id", s["Id"], "prevPid", s["Process"])
			s["Process"] = -1
			_ = config.SaveServiceConfiguration(s)
		}
	}()

	pid := <-waitUntilStart
	return pid, nil
}

/*
StartServiceProxyProcess launches the gRPC-Web proxy (grpcwebproxy) for a running service.
It reads TLS settings from local Globular certs and configures CORS/timeouts.

Returns the proxy PID. Side effects:
  - sets s["ProxyProcess"]
  - sets s["State"]="running"
  - persists configuration
*/
func StartServiceProxyProcess(s map[string]interface{}, certificateAuthorityBundle, certificate string) (int, error) {
	// The backend service must be running
	processPid := Utility.ToInt(s["Process"])
	if processPid == -1 {
		return -1, errors.New("service process pid must not be -1")
	}

	// Only one proxy per service instance
	if pid := Utility.ToInt(s["ProxyProcess"]); pid != -1 {
		return -1, fmt.Errorf("proxy already exists for service %s", s["Name"])
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
					return -1, errors.New("grpcwebproxy executable not found (system PATH or ./bin)")
				}
			}
		} else {
			return -1, startErr
		}
	}

	slog.Info("grpcwebproxy started",
		"service", s["Name"], "id", s["Id"],
		"backend", backend, "port", proxyPort,
		"tls", s["TLS"])

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

		wait <- proxy.Process.Pid

		// Wait for proxy to exit and optionally restart if backend still alive
		if err := proxy.Wait(); err != nil {
			slog.Error("grpcwebproxy terminated with error", "service", s["Name"], "id", s["Id"], "err", err)
		} else {
			slog.Info("grpcwebproxy stopped", "service", s["Name"], "id", s["Id"])
		}

		data, err := os.ReadFile(sc["ConfigPath"].(string))
		if err != nil {
			slog.Error("failed to read service config after proxy exit", "path", sc["ConfigPath"], "err", err)
			return
		}
		_ = json.Unmarshal(data, &sc)

		procPid := Utility.ToInt(sc["Process"])
		if procPid != -1 && sc["KeepAlive"].(bool) {
			if exist, err := Utility.PidExists(procPid); err == nil && exist {
				slog.Info("keepalive: restarting grpcwebproxy", "service", sc["Name"], "id", sc["Id"])
				_, _ = StartServiceProxyProcess(sc, certificateAuthorityBundle, certificate)
			} else if err != nil {
				slog.Error("keepalive: proxy restart check failed", "service", sc["Name"], "id", sc["Id"], "err", err)
			}
		}
	}()

	proxyPid := <-wait
	s["State"] = "running"
	s["ProxyProcess"] = proxyPid
	if err := config.SaveServiceConfiguration(s); err != nil {
		slog.Error("failed to save service configuration after proxy start", "service", s["Name"], "id", s["Id"], "err", err)
		return proxyPid, err
	}
	return proxyPid, nil
}

// KillServiceProxyProcess terminates the grpcwebproxy for a service if present.
func KillServiceProxyProcess(s map[string]interface{}) error {
    pid := Utility.ToInt(s["ProxyProcess"])
    if pid <= 0 {
        return nil
    }

    proc, err := os.FindProcess(pid)
    if err == nil {
        // Try graceful stop
        _ = proc.Signal(syscall.SIGTERM)
    }

    // Wait for exit
    deadline := time.Now().Add(5 * time.Second)
    for time.Now().Before(deadline) {
        if ok, _ := Utility.PidExists(pid); !ok {
            break
        }
        time.Sleep(200 * time.Millisecond)
    }

    // Force kill if still alive
    if ok, _ := Utility.PidExists(pid); ok {
        _ = proc.Kill()
    }

    // Important: mark proxy gone in config
    s["ProxyProcess"] = -1
    if Utility.ToInt(s["Process"]) == -1 {
        s["State"] = "stopped"
    }
    return config.SaveServiceConfiguration(s)
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
			slog.Info("received shutdown signal for etcd; terminating", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	etcd.Stdout = os.Stdout
	etcd.Stderr = os.Stderr

	slog.Info("starting etcd", "config", cfgPath)
	if err := etcd.Start(); err != nil {
		slog.Error("failed to start etcd", "err", err)
		return err
	}
	if err := etcd.Wait(); err != nil {
		slog.Error("etcd terminated with error", "err", err)
		return err
	}

	slog.Info("etcd exited")
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
			slog.Info("received shutdown signal for envoy; terminating", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	envoy.Stdout = os.Stdout
	envoy.Stderr = os.Stderr

	slog.Info("starting envoy", "config", cfgPath)
	if err := envoy.Start(); err != nil {
		slog.Error("failed to start envoy", "err", err)
		return err
	}
	if err := envoy.Wait(); err != nil {
		slog.Error("envoy terminated with error", "err", err)
		return err
	}

	slog.Info("envoy exited")
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
		slog.Info("prometheus already running; skipping start")
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
		slog.Info("generated prometheus config", "path", promYml)
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
		slog.Info("generated alertmanager config", "path", alertYml)
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
			slog.Info("generated prometheus TLS web config", "path", webCfg)
		}
		args = append(args, "--web.config.file", webCfg)
	}

	prom := exec.Command("prometheus", args...)
	prom.Dir = os.TempDir()
	prom.SysProcAttr = &syscall.SysProcAttr{}

	slog.Info("starting prometheus", "args", args)
	if err := prom.Start(); err != nil {
		slog.Error("failed to start prometheus", "err", err)
		return err
	}

	// Register metrics
	servicesCpuUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_services_cpu_usage_counter",
		Help: "Monitor the CPU usage of each service.",
	}, []string{"id", "name"})
	servicesMemoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "globular_services_memory_usage_counter",
		Help: "Monitor the memory usage of each service.",
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
				slog.Info("stopping process metrics feeder")
				return
			}
		}
	}()

	// Best-effort Alertmanager and node_exporter
	alert := exec.Command("alertmanager", "--config.file", alertYml)
	alert.Dir = os.TempDir()
	alert.SysProcAttr = &syscall.SysProcAttr{}
	if err := alert.Start(); err != nil {
		slog.Warn("failed to start alertmanager", "err", err)
	} else {
		slog.Info("alertmanager started")
	}

	nodeExp := exec.Command("node_exporter")
	nodeExp.Dir = os.TempDir()
	nodeExp.SysProcAttr = &syscall.SysProcAttr{}
	if err := nodeExp.Start(); err != nil {
		slog.Warn("failed to start node_exporter", "err", err)
	} else {
		slog.Info("node_exporter started")
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
  - other -> byte as-is
*/
func crAwareCopy(dst io.Writer, src io.Reader) {
	reader := bufio.NewReader(src)
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				// best-effort: surface the error once
				_, _ = dst.Write([]byte("\n[stream error] " + err.Error() + "\n"))
			}
			return
		}
		if b == '\r' {
			// return to start of line and clear it
			_, _ = dst.Write([]byte("\r\033[K"))
			continue
		}
		_, _ = dst.Write([]byte{b})
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
