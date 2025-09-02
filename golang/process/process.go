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
// KillServiceProcess sends SIGTERM (or Kill on Windows), waits for exit,
// escalates to SIGKILL after a short timeout, and only returns once the
// process is really gone. Then it clears state and kills the proxy.
func KillServiceProcess(s map[string]interface{}) error {
	pid := -1
	if s["Process"] != nil {
		pid = Utility.ToInt(s["Process"])
	}
	if pid == -1 {
		// ensure proxy is down & runtime cleaned
		_ = KillServiceProxyProcess(s)
		s["State"] = "stopped"
		s["Process"] = -1
		_ = config.SaveServiceConfiguration(s)
		config.StopLive(Utility.ToString(s["Id"]))
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		// Process handle not found ⇒ treat as gone, clean state and proxy.
		slog.Warn("failed to find process to kill (treating as dead)", "pid", pid, "err", err)
		s["Process"] = -1
		s["State"] = "stopped"
		_ = config.SaveServiceConfiguration(s)
		if err := KillServiceProxyProcess(s); err != nil {
			slog.Warn("failed to terminate proxy after missing process", "err", err)
		}
		return nil
	}

	// 1) Try graceful stop first
	if runtime.GOOS == "windows" {
		// No SIGTERM on Windows; Kill does the job
		_ = proc.Kill()
	} else {
		// Prefer group-terminate if you started service with Setpgid: true
		// (negative pid sends to process group). Fall back to single PID.
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}

	// 2) Wait up to 5s for the process to die
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		dead, _ := isDead(pid)
		if dead {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	// 3) If still alive, escalate to hard kill
	if alive, _ := isAlive(pid); alive && runtime.GOOS != "windows" {
		bin := filepath.Base(Utility.ToString(s["Path"]))
		if ids, _ := Utility.GetProcessIdsByName(bin); len(ids) > 0 {
			for _, id := range ids {
				// try group then pid
				_ = syscall.Kill(-id, syscall.SIGKILL)
				_ = syscall.Kill(id, syscall.SIGKILL)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// 4) Verify and update state
	// Verify & update state
	if alive, _ := isAlive(pid); alive {
		err := fmt.Errorf("service pid %d did not exit after SIGTERM/SIGKILL", pid)
		s["LastError"] = err.Error()
		s["State"] = "failed"
		_ = config.SaveServiceConfiguration(s)
		return err
	}

	// Mark stopped in runtime; stop liveness
	s["Process"] = -1
	s["State"] = "stopped"
	_ = config.SaveServiceConfiguration(s)
	config.StopLive(Utility.ToString(s["Id"]))

	// Clean proxy afterwards
	_ = KillServiceProxyProcess(s)
	return nil
}

func isAlive(pid int) (bool, error) {
	return Utility.PidExists(pid) // your helper already returns (bool, error)
}

func isDead(pid int) (bool, error) {
	alive, err := isAlive(pid)
	return !alive, err
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
// --- NEW: public wrapper remains compatible ---
func StartServiceProcess(s map[string]interface{}, port int) (int, error) {
	return StartServiceProcessWithWriters(s, port, os.Stdout, os.Stderr)
}

// --- NEW: leveled writers variant ---
func StartServiceProcessWithWriters(
	s map[string]interface{},
	port int,
	stdoutWriter io.Writer, // nil => os.Stdout
	stderrWriter io.Writer, // nil => os.Stderr
) (int, error) {
	if stdoutWriter == nil {
		stdoutWriter = os.Stdout
	}
	if stderrWriter == nil {
		stderrWriter = os.Stderr
	}

	// Kill any previous instance
	if err := KillServiceProcess(s); err != nil {
		return -1, err
	}

	// Inject ports
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
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	}

	// stdout pipe -> CR-aware copier into provided writer
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}

	// stderr pipe -> line copier into provided writer (keep buffer for start error details)
	stderrR, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}
	var startErrBuf bytes.Buffer // only for Start() failure path

	// Normalize stored path, reset pid; persist config (unchanged)
	s["Path"] = strings.ReplaceAll(path, "\\", "/")
	s["Process"] = -1
	if err := config.SaveServiceConfiguration(s); err != nil {
		return -1, fmt.Errorf("save service config: %w", err)
	}

	// Start streaming BEFORE starting the process
	doneStdout := make(chan struct{})
	go func() {
		crAwareCopy(stdoutWriter, stdout)
		close(doneStdout)
	}()

	doneStderr := make(chan struct{})
	go func() {
		// We don’t need CR handling for stderr; line-wise copy is good.
		// But keep a tee into startErrBuf for Start() failure reporting.
		tee := io.TeeReader(stderrR, &startErrBuf)
		reader := bufio.NewReader(tee)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				_, _ = stderrWriter.Write(line)
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					_, _ = stderrWriter.Write([]byte("\n[stderr stream error] " + err.Error() + "\n"))
				}
				break
			}
		}
		close(doneStderr)
	}()

	if err := cmd.Start(); err != nil {
		// close and drain goroutines
		_ = stdout.Close()
		_ = stderrR.Close()
		<-doneStdout
		<-doneStderr
		slog.Error("failed to start service",
			"name", s["Name"], "id", s["Id"], "err", err,
			"stderr", strings.TrimSpace(startErrBuf.String()))
		return -1, err
	}

	slog.Info("service process started", "name", s["Name"], "id", s["Id"], "pid", cmd.Process.Pid, "port", port, "proxy", port+1)

	// Hand back PID immediately
	pid := cmd.Process.Pid
	s["Process"] = pid
	s["State"] = "running"
	if err := config.SaveServiceConfiguration(s); err != nil {
		slog.Warn("failed to persist service pid/state after start",
			"name", s["Name"], "id", s["Id"], "pid", pid, "err", err)
	}

	if _, err := config.StartLive(Utility.ToString(s["Id"]), 15); err != nil {
		slog.Warn("failed to start etcd live lease", "id", s["Id"], "err", err)
	}

	// supervise in background (mostly unchanged)
	go func() {
		err := cmd.Wait()

		// close pipes and join copy goroutines
		_ = stdout.Close()
		_ = stderrR.Close()
		<-doneStdout
		<-doneStderr

		if err != nil {
			s["State"] = "failed"
			s["LastError"] = err.Error()
			slog.Error("service process exited with error", "name", s["Name"], "id", s["Id"], "pid", pid, "err", err)
		} else {
			s["State"] = "stopped"
			slog.Info("service process stopped", "name", s["Name"], "id", s["Id"], "pid", pid)
		}
		s["Process"] = -1
		_ = config.SaveServiceConfiguration(s)
		config.StopLive(Utility.ToString(s["Id"]))

		// (optional) keepalive restart logic here, reading *desired* from etcd
	}()

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

	if runtime.GOOS != "windows" {
		proxy.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM} // Linux: child gets SIGTERM if parent exits
	}

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

	// Update runtime: ProxyProcess + State
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

	proc, _ := os.FindProcess(pid)

	// Graceful first
	if runtime.GOOS == "windows" {
		_ = proc.Kill()
	} else {

		// if we started with Setpgid=true, kill the whole group
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = proc.Signal(syscall.SIGTERM)
	}

	// Wait up to 5s
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := Utility.PidExists(pid); !ok {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	// Escalate if needed
	if ok, _ := Utility.PidExists(pid); ok {
		if runtime.GOOS == "windows" {
			_ = proc.Kill()
		} else {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_ = proc.Kill()
		}
		// short final wait
		esc := time.Now().Add(2 * time.Second)
		for time.Now().Before(esc) {
			if ok, _ := Utility.PidExists(pid); !ok {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

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
	// ---- load local settings safely
	localConfig, err := config.GetLocalConfig(true)
	if err != nil {
		return err
	}
	protocol := Utility.ToString(localConfig["Protocol"])
	if protocol == "" {
		protocol = "http"
	}

	// host we’ll bind/advertise (strip port)
	hostPort, _ := config.GetAddress()
	host := hostPort
	if i := strings.Index(hostPort, ":"); i > 0 {
		host = hostPort[:i]
	}

	name := Utility.ToString(localConfig["Name"])
	if name == "" {
		name, _ = config.GetName()
	}

	dataDir := config.GetDataDir() + "/etcd-data"
	_ = Utility.CreateDirIfNotExist(dataDir)

	cfgPath := config.GetConfigDir() + "/etcd.yml"

	// ---- seed with any existing cfg
	nodeCfg := make(map[string]interface{})
	if Utility.Exists(cfgPath) {
		if data, err := os.ReadFile(cfgPath); err == nil {
			_ = yaml.Unmarshal(data, &nodeCfg)
		}
	}

	// ---- base cluster settings
	nodeCfg["name"] = name
	nodeCfg["data-dir"] = dataDir
	nodeCfg["listen-peer-urls"] = protocol + "://" + host + ":2380"
	nodeCfg["listen-client-urls"] = protocol + "://" + host + ":2379"
	nodeCfg["advertise-client-urls"] = protocol + "://" + host + ":2379"
	nodeCfg["initial-advertise-peer-urls"] = protocol + "://" + host + ":2380"
	nodeCfg["initial-cluster-token"] = "etcd-cluster-1"

	// Build initial-cluster
	initialCluster := name + "=" + protocol + "://" + host + ":2380"
	if peers, ok := localConfig["Peers"].([]interface{}); ok {
		for _, raw := range peers {
			peer := raw.(map[string]interface{})
			ph := Utility.ToString(peer["Hostname"])
			pd := Utility.ToString(peer["Domain"])
			if ph != "" && pd != "" {
				initialCluster += "," + ph + "=" + protocol + "://" + ph + "." + pd + ":2380"
			}
		}
	}
	nodeCfg["initial-cluster"] = initialCluster

	// initial-cluster-state: detect from data dir
	state := "new"
	if Utility.Exists(filepath.Join(dataDir, "member")) {
		state = "existing"
	}
	nodeCfg["initial-cluster-state"] = state

	// ---- TLS (both client & peer) when protocol == https
	if protocol == "https" {
		// certs layout: /etc/globular/config/tls/<name>.<domain>/{server.crt,server.pem,ca.crt}
		domain, _ := config.GetDomain()
		certDir := config.GetConfigDir() + "/tls/" + name + "." + domain

		// sanity check (optional but helpful)
		for _, p := range []string{certDir + "/server.crt", certDir + "/server.pem", certDir + "/ca.crt"} {
			if !Utility.Exists(p) {
				return fmt.Errorf("missing TLS file for etcd: %s", p)
			}
		}

		// These keys match etcd’s YAML schema.
		nodeCfg["client-transport-security"] = map[string]interface{}{
			"cert-file":        certDir + "/server.crt",
			"key-file":         certDir + "/server.pem",
			"client-cert-auth": true,
			"trusted-ca-file":  certDir + "/ca.crt",
		}
		nodeCfg["peer-transport-security"] = map[string]interface{}{
			"cert-file":        certDir + "/server.crt", // reuse, or point to dedicated peer certs if you have them
			"key-file":         certDir + "/server.pem",
			"client-cert-auth": true,                    // require peer client certs
			"trusted-ca-file":  certDir + "/ca.crt",
		}
	}

	// ---- write config
	out, err := yaml.Marshal(nodeCfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		return err
	}

	// ---- start etcd
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "etcd", "--config-file", cfgPath)
	cmd.Dir = os.TempDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received shutdown signal for etcd; terminating", "signal", sig)
		// Try graceful first
		if cmd.Process != nil {
			if runtime.GOOS == "windows" {
				_ = cmd.Process.Kill() // best-effort on Windows
			} else {
				_ = cmd.Process.Signal(syscall.SIGTERM)
				// give it a moment, then hard kill if needed
				timer := time.NewTimer(5 * time.Second)
				defer timer.Stop()
				done := make(chan struct{}, 1)
				go func() {
					_ = cmd.Wait()
					done <- struct{}{}
				}()
				select {
				case <-done:
				case <-timer.C:
					_ = cmd.Process.Kill()
				}
			}
		}
		cancel()
	}()

	slog.Info("starting etcd", "config", cfgPath)
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start etcd", "err", err)
		return err
	}
	err = cmd.Wait()
	if err != nil {
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
