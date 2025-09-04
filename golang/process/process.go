package process

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

Side effects on the given service map (runtime only, persisted to etcd):
  - sets Process = -1 on success
  - sets State   = "killed" on success, or "failed" on error
  - clears ProxyProcess and stops the live lease
*/
func KillServiceProcess(s map[string]interface{}) error {
	pid := -1
	if s["Process"] != nil {
		pid = Utility.ToInt(s["Process"])
	}
	if pid == -1 {
		// ensure proxy is down & runtime cleaned
		_ = KillServiceProxyProcess(s)
		_ = config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
			"Process":      -1,
			"ProxyProcess": -1,
			"State":        "stopped",
			"LastError":    "",
		})
		config.StopLive(Utility.ToString(s["Id"]))
		return nil
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		// Process handle not found ⇒ treat as gone, clean state and proxy.
		slog.Warn("failed to find process to kill (treating as dead)", "pid", pid, "err", err)
		_ = KillServiceProxyProcess(s)
		_ = config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
			"Process":      -1,
			"ProxyProcess": -1,
			"State":        "stopped",
			"LastError":    "",
		})
		return nil
	}

	// 1) Try graceful stop first
	if runtime.GOOS == "windows" {
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
				_ = syscall.Kill(-id, syscall.SIGKILL)
				_ = syscall.Kill(id, syscall.SIGKILL)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// 4) Verify & update state
	if alive, _ := isAlive(pid); alive {
		err := fmt.Errorf("service pid %d did not exit after SIGTERM/SIGKILL", pid)
		_ = config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
			"LastError": err.Error(),
			"State":     "failed",
		})
		return err
	}

	// Mark stopped in runtime; stop liveness and proxy
	_ = config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
		"Process":      -1,
		"ProxyProcess": -1,
		"State":        "stopped",
		"LastError":    "",
	})
	config.StopLive(Utility.ToString(s["Id"]))
	_ = KillServiceProxyProcess(s)
	return nil
}

func isAlive(pid int) (bool, error) { return Utility.PidExists(pid) }
func isDead(pid int) (bool, error)  { alive, err := isAlive(pid); return !alive, err }

// --- public wrapper remains compatible ---
func StartServiceProcess(s map[string]interface{}, port int) (int, error) {
	return StartServiceProcessWithWriters(s, port, os.Stdout, os.Stderr)
}

// StartServiceProcessWithWriters starts the service, persists desired updates (Port/Proxy)
// to etcd, updates runtime via etcd, and supervises the child.
func StartServiceProcessWithWriters(
	s map[string]interface{},
	port int,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
) (int, error) {
	if stdoutWriter == nil { stdoutWriter = os.Stdout }
	if stderrWriter == nil { stderrWriter = os.Stderr }

	// Kill any previous instance
	if err := KillServiceProcess(s); err != nil {
		return -1, err
	}

	// Desired changes (persist to etcd BEFORE starting the child so it reads them)
	s["Port"] = port
	s["Proxy"] = port + 1
	s["State"] = "starting"
	s["Process"] = -1
	if err := config.SaveServiceConfiguration(s); err != nil {
		slog.Warn("failed to persist desired service config before start", "id", s["Id"], "err", err)
	}

	path := Utility.ToString(s["Path"])
	if path == "" || !Utility.Exists(path) {
		return -1, fmt.Errorf("service binary not found: %s", path)
	}

	// CHILD ARGUMENTS: only the Id. The service will fetch its config by Id from etcd.
	cmd := exec.Command(path, Utility.ToString(s["Id"]))
	cmd.Dir = filepath.Dir(path)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil { return -1, err }
	stderrR, err := cmd.StderrPipe()
	if err != nil { return -1, err }
	var startErrBuf bytes.Buffer

	doneStdout := make(chan struct{})
	go func() { crAwareCopy(stdoutWriter, stdout); close(doneStdout) }()

	doneStderr := make(chan struct{})
	go func() {
		tee := io.TeeReader(stderrR, &startErrBuf)
		r := bufio.NewReader(tee)
		for {
			line, e := r.ReadBytes('\n')
			if len(line) > 0 { _, _ = stderrWriter.Write(line) }
			if e != nil {
				if !errors.Is(e, io.EOF) {
					_, _ = stderrWriter.Write([]byte("\n[stderr stream error] " + e.Error() + "\n"))
				}
				break
			}
		}
		close(doneStderr)
	}()

	if err := cmd.Start(); err != nil {
		_ = stdout.Close(); _ = stderrR.Close()
		<-doneStdout; <-doneStderr
		slog.Error("failed to start service", "name", s["Name"], "id", s["Id"], "err", err,
			"stderr", strings.TrimSpace(startErrBuf.String()))
		return -1, err
	}

	pid := cmd.Process.Pid
	slog.Info("service process started", "name", s["Name"], "id", s["Id"], "pid", pid, "port", port, "proxy", port+1)

	// Update ONLY runtime in etcd
	_ = config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
		"Process":   pid,
		"State":     "running",
		"LastError": "",
	})

	// mark live
	if _, err := config.StartLive(Utility.ToString(s["Id"]), 15); err != nil {
		slog.Warn("failed to start etcd live lease", "id", s["Id"], "err", err)
	}

	// Supervise
	go func() {
		err := cmd.Wait()
		_ = stdout.Close(); _ = stderrR.Close()
		<-doneStdout; <-doneStderr

		state := "stopped"
		if err != nil {
			state = "failed"
			slog.Error("service process exited with error",
				"name", s["Name"], "id", s["Id"], "pid", pid, "err", err)
		} else {
			slog.Info("service process stopped", "name", s["Name"], "id", s["Id"], "pid", pid)
		}

		_ = config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
			"Process":   -1,
			"State":     state,
			"LastError": func() string { if err != nil { return err.Error() }; return "" }(),
		})

		config.StopLive(Utility.ToString(s["Id"]))
		// Optional: keepalive restart logic can read desired from etcd here.
	}()

	return pid, nil
}

/*
StartServiceProxyProcess launches the gRPC-Web proxy (grpcwebproxy) for a running service.
It reads TLS settings from local Globular certs and configures CORS/timeouts.

Returns the proxy PID. Runtime is updated in etcd (ProxyProcess/State).
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

	address := Utility.ToString(s["Address"])
	if address == "" {
		address, _ = config.GetAddress()
	}
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

	tlsEnabled := Utility.ToBool(s["TLS"])
	if tlsEnabled {
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
		proxy.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
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
				if err != nil { return -1, err }
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
		"tls", tlsEnabled)

	// Publish updated service configuration to listeners (event bus), best-effort
	str, _ := Utility.ToJson(s)
	wait := make(chan int, 1)

	go func() {
		address, _ := config.GetAddress()
		if ec, err := getEventClient(address); err == nil {
			_ = ec.Publish("update_globular_service_configuration_evt", []byte(str))
		}

		// hand back pid
		wait <- proxy.Process.Pid

		// Supervise proxy
		if err := proxy.Wait(); err != nil {
			slog.Error("grpcwebproxy terminated with error", "service", s["Name"], "id", s["Id"], "err", err)
		} else {
			slog.Info("grpcwebproxy stopped", "service", s["Name"], "id", s["Id"])
		}

		// Read current desired/runtime from etcd to decide on restart
		scEtcd, err := config.GetServiceConfigurationById(Utility.ToString(s["Id"]))
		if err != nil {
			slog.Warn("proxy exit: failed to read service cfg from etcd", "id", s["Id"], "err", err)
			return
		}
		procPid := Utility.ToInt(scEtcd["Process"])
		keepAlive := Utility.ToBool(scEtcd["KeepAlive"])
		if procPid != -1 && keepAlive {
			slog.Info("keepalive: restarting grpcwebproxy", "service", scEtcd["Name"], "id", scEtcd["Id"])
			_, _ = StartServiceProxyProcess(scEtcd, certificateAuthorityBundle, certificate)
		}
	}()

	proxyPid := <-wait

	// Update runtime: ProxyProcess + State
	_ = config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
		"ProxyProcess": proxyPid,
		"State":        "running",
	})
	return proxyPid, nil
}

// KillServiceProxyProcess terminates the grpcwebproxy for a service if present and updates etcd runtime.
func KillServiceProxyProcess(s map[string]interface{}) error {
	pid := Utility.ToInt(s["ProxyProcess"])
	if pid <= 0 { return nil }

	proc, _ := os.FindProcess(pid)

	// Graceful first
	if runtime.GOOS == "windows" { _ = proc.Kill() } else {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = proc.Signal(syscall.SIGTERM)
	}

	// Wait up to 5s
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := Utility.PidExists(pid); !ok { break }
		time.Sleep(150 * time.Millisecond)
	}

	// Escalate if needed
	if ok, _ := Utility.PidExists(pid); ok {
		if runtime.GOOS == "windows" { _ = proc.Kill() } else {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_ = proc.Kill()
		}
		// short final wait
		esc := time.Now().Add(2 * time.Second)
		for time.Now().Before(esc) {
			if ok, _ := Utility.PidExists(pid); !ok { break }
			time.Sleep(100 * time.Millisecond)
		}
	}

	state := "running"
	if Utility.ToInt(s["Process"]) == -1 { state = "stopped" }
	return config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
		"ProxyProcess": -1,
		"State":        state,
	})
}

/*
GetProcessRunningStatus checks if a given PID represents a currently running process.

Returns the *os.Process on success, or an error indicating whether the process
is not running or the operation is not permitted.
*/
func GetProcessRunningStatus(pid int) (*os.Process, error) {
	proc, err := os.FindProcess(pid)
	if err != nil { return nil, err }
	if err := proc.Signal(syscall.Signal(0)); err == nil { return proc, nil }
	if err == syscall.ESRCH { return nil, errors.New("process not running") }
	return nil, errors.New("process running but query operation not permitted")
}

// StartEtcdServer starts etcd in the background, waits until it's ready,
// then returns. It no longer blocks for the lifetime of etcd.
func StartEtcdServer() error {
	const readyTimeout = 15 * time.Second
	const probeEvery = 200 * time.Millisecond

	localConfig, err := config.GetLocalConfig(true)
	if err != nil { return err }
	protocol := Utility.ToString(localConfig["Protocol"])
	if protocol == "" { protocol = "http" }

	// host we advertise (no port)
	hostPort, _ := config.GetAddress()
	host := hostPort
	if i := strings.Index(hostPort, ":"); i > 0 { host = hostPort[:i] }

	name := Utility.ToString(localConfig["Name"])
	if name == "" { name, _ = config.GetName() }

	dataDir := config.GetDataDir() + "/etcd-data"
	_ = Utility.CreateDirIfNotExist(dataDir)
	cfgPath := config.GetConfigDir() + "/etcd.yml"

	// seed with any existing cfg
	nodeCfg := make(map[string]interface{})
	if Utility.Exists(cfgPath) {
		if data, err := os.ReadFile(cfgPath); err == nil { _ = yaml.Unmarshal(data, &nodeCfg) }
	}

	// Decide final scheme (may fall back to http if TLS files missing)
	scheme := protocol
	wantTLS := (protocol == "https")
	var certFile, keyFile, caFile string
	if wantTLS {
		domain, _ := config.GetDomain()
		certDir := config.GetConfigDir() + "/tls/" + name + "." + domain
		certFile = certDir + "/server.crt"
		keyFile = certDir + "/server.pem"
		caFile = certDir + "/ca.crt"
		if !(Utility.Exists(certFile) && Utility.Exists(keyFile) && Utility.Exists(caFile)) {
			slog.Warn("etcd TLS requested but missing cert/key/CA; falling back to HTTP",
				"cert", certFile, "key", keyFile, "ca", caFile)
			wantTLS = false
			scheme = "http"
		} else {
			nodeCfg["client-transport-security"] = map[string]interface{}{
				"cert-file":        certFile,
				"key-file":         keyFile,
				"client-cert-auth": true,
				"trusted-ca-file":  caFile,
			}
			nodeCfg["peer-transport-security"] = map[string]interface{}{
				"cert-file":        certFile,
				"key-file":         keyFile,
				"client-cert-auth": true,
				"trusted-ca-file":  caFile,
			}
		}
	}
	if !wantTLS { delete(nodeCfg, "client-transport-security"); delete(nodeCfg, "peer-transport-security") }

	// Listeners + advertised addresses
	listenClientURLs := scheme + "://0.0.0.0:2379"
	listenPeerURLs := scheme + "://0.0.0.0:2380"
	advertiseClientURLs := scheme + "://" + host + ":2379"
	initialAdvertisePeerURLs := scheme + "://" + host + ":2380"

	nodeCfg["name"] = name
	nodeCfg["data-dir"] = dataDir
	nodeCfg["listen-client-urls"] = listenClientURLs
	nodeCfg["listen-peer-urls"] = listenPeerURLs
	nodeCfg["advertise-client-urls"] = advertiseClientURLs
	nodeCfg["initial-advertise-peer-urls"] = initialAdvertisePeerURLs
	nodeCfg["initial-cluster-token"] = "etcd-cluster-1"

	// initial-cluster
	initialCluster := name + "=" + initialAdvertisePeerURLs
	if peers, ok := localConfig["Peers"].([]interface{}); ok {
		for _, raw := range peers {
			peer := raw.(map[string]interface{})
			ph := Utility.ToString(peer["Hostname"]) ; pd := Utility.ToString(peer["Domain"])
			if ph != "" && pd != "" {
				initialCluster += "," + ph + "=" + scheme + "://" + ph + "." + pd + ":2380"
			}
		}
	}
	nodeCfg["initial-cluster"] = initialCluster

	// cluster state
	if Utility.Exists(filepath.Join(dataDir, "member")) { nodeCfg["initial-cluster-state"] = "existing" } else { nodeCfg["initial-cluster-state"] = "new" }

	// write config
	out, err := yaml.Marshal(nodeCfg)
	if err != nil { return err }
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil { return err }

	etcdPath, err := findEtcdBinary()
	if err != nil { slog.Error("cannot start etcd", "err", err); return err }

	slog.Info("starting etcd", "config", cfgPath, "listen-client-urls", listenClientURLs, "advertise-client-urls", advertiseClientURLs)

	cmd := exec.Command(etcdPath, "--config-file", cfgPath)
	cmd.Dir = os.TempDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runtime.GOOS != "windows" { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM} }

	if err := cmd.Start(); err != nil { slog.Error("failed to start etcd", "err", err); return err }
	setEtcdCmd(cmd)

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() { sig := <-sigCh; slog.Info("received shutdown signal for etcd; terminating", "signal", sig); _ = StopEtcdServer() }()

	// Readiness probe
	deadline := time.Now().Add(readyTimeout)
	healthURL := scheme + "://127.0.0.1:2379/health"
	var httpClient *http.Client
	if scheme == "https" && wantTLS {
		cp := x509.NewCertPool()
		if pem, err := os.ReadFile(caFile); err == nil && cp.AppendCertsFromPEM(pem) {
			httpClient = &http.Client{ Timeout: 1 * time.Second, Transport: &http.Transport{ TLSClientConfig: &tls.Config{RootCAs: cp} } }
		} else {
			httpClient = &http.Client{ Timeout: 1 * time.Second, Transport: &http.Transport{ TLSClientConfig: &tls.Config{InsecureSkipVerify: true} } }
		}
	} else {
		httpClient = &http.Client{Timeout: 1 * time.Second}
	}

	for time.Now().Before(deadline) {
		select { case err := <-waitErr: if err == nil { return fmt.Errorf("etcd exited before becoming ready") } ; return fmt.Errorf("etcd exited early: %w", err) ; default: }
		if resp, err := httpClient.Get(healthURL); err == nil { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close(); if resp.StatusCode == http.StatusOK { slog.Info("etcd is ready"); return nil } }
		if conn, err := net.DialTimeout("tcp", "127.0.0.1:2379", 500*time.Millisecond); err == nil { _ = conn.Close(); slog.Info("etcd TCP port open; proceeding"); return nil }
		time.Sleep(probeEvery)
	}

	select { case err := <-waitErr: if err == nil { return fmt.Errorf("etcd exited before becoming ready") } ; return fmt.Errorf("etcd exited early: %w", err) ; default: }
	_ = StopEtcdServer()
	return fmt.Errorf("etcd did not become ready within %s", readyTimeout)
}

// ---------------- helpers ----------------

var etcdCmdMu sync.Mutex
var etcdCmd *exec.Cmd

func setEtcdCmd(c *exec.Cmd) { etcdCmdMu.Lock(); defer etcdCmdMu.Unlock(); etcdCmd = c }

// StopEtcdServer gracefully stops the etcd process started by StartEtcdServer.
func StopEtcdServer() error {
	etcdCmdMu.Lock(); c := etcdCmd; etcdCmd = nil; etcdCmdMu.Unlock()
	if c == nil || c.Process == nil { return nil }
	if runtime.GOOS == "windows" { _ = c.Process.Kill(); return nil }
	_ = syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
	_ = c.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{}, 1)
	go func() { _, _ = c.Process.Wait(); done <- struct{}{} }()
	select { case <-done: case <-time.After(5 * time.Second): _ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL); _ = c.Process.Kill() }
	return nil
}

func findEtcdBinary() (string, error) {
	if p, err := exec.LookPath("etcd"); err == nil { return p, nil }
	candidates := []string{"/usr/local/bin/etcd", "/usr/bin/etcd"}
	for _, p := range candidates { if Utility.Exists(p) { return p, nil } }
	return "", errors.New("etcd binary not found in PATH")
}

/*
StartEnvoyProxy starts Envoy with the config file at <config>/envoy.yml,
wiring stdout/stderr to the parent console and handling graceful shutdown.

It returns an error if the config file is missing or Envoy fails to start.
*/
func StartEnvoyProxy() error {
	cfgPath := config.GetConfigDir() + "/envoy.yml"
	if !Utility.Exists(cfgPath) { return errors.New("no envoy configuration file found at path " + cfgPath) }

	envoy := exec.Command("envoy", "-c", cfgPath, "-l", "warn")
	envoy.Dir = os.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() { select { case sig := <-sigCh: slog.Info("received shutdown signal for envoy; terminating", "signal", sig); cancel() ; case <-ctx.Done(): } }()

	envoy.Stdout = os.Stdout
	envoy.Stderr = os.Stderr

	slog.Info("starting envoy", "config", cfgPath)
	if err := envoy.Start(); err != nil { slog.Error("failed to start envoy", "err", err); return err }
	if err := envoy.Wait(); err != nil { slog.Error("envoy terminated with error", "err", err); return err }
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
		if err := os.WriteFile(promYml, []byte(cfg), 0644); err != nil { return err }
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
		if err := os.WriteFile(alertYml, []byte(cfg), 0644); err != nil { return err }
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
			if err := os.WriteFile(webCfg, []byte(cfg), 0644); err != nil { return err }
			slog.Info("generated prometheus TLS web config", "path", webCfg)
		}
		args = append(args, "--web.config.file", webCfg)
	}

	prom := exec.Command("prometheus", args...)
	prom.Dir = os.TempDir()
	prom.SysProcAttr = &syscall.SysProcAttr{}

	slog.Info("starting prometheus", "args", args)
	if err := prom.Start(); err != nil { slog.Error("failed to start prometheus", "err", err); return err }

	// Register metrics
	servicesCpuUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{ Name: "globular_services_cpu_usage_counter", Help: "Monitor the CPU usage of each service.", }, []string{"id", "name"})
	servicesMemoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{ Name: "globular_services_memory_usage_counter", Help: "Monitor the memory usage of each service.", }, []string{"id", "name"})
	prometheus.MustRegister(servicesCpuUsage, servicesMemoryUsage)

	// Feed metrics periodically
	ticker := time.NewTicker(1 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				execName := "Globular"
				if runtime.GOOS == "windows" { execName += ".exe" }
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
	if err := alert.Start(); err != nil { slog.Warn("failed to start alertmanager", "err", err) } else { slog.Info("alertmanager started") }

	nodeExp := exec.Command("node_exporter")
	nodeExp.Dir = os.TempDir()
	nodeExp.SysProcAttr = &syscall.SysProcAttr{}
	if err := nodeExp.Start(); err != nil { slog.Warn("failed to start node_exporter", "err", err) } else { slog.Info("node_exporter started") }

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
				_, _ = dst.Write([]byte("\n[stream error] " + err.Error() + "\n"))
			}
			return
		}
		if b == '\r' { _, _ = dst.Write([]byte("\r\033[K")); continue }
		_, _ = dst.Write([]byte{b})
	}
}

// Local event client helper (unchanged signature/behavior)
func getEventClient(address string) (*event_client.Event_Client, error) {
	if address == "" { address, _ = config.GetAddress() }
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil { return nil, err }
	return client.(*event_client.Event_Client), nil
}
