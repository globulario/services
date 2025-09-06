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
	"net/url"
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

// =====================================================================================
// Prometheus metrics
// =====================================================================================

var (
	servicesCpuUsage    *prometheus.GaugeVec
	servicesMemoryUsage *prometheus.GaugeVec
)

// =====================================================================================
// Process lifecycle (services + proxy)
// =====================================================================================

/*
KillServiceProcess terminates a running service process if its PID is set on the
service configuration map.

Side effects (runtime in etcd):
  - sets Process      = -1
  - sets ProxyProcess = -1
  - sets State        = "stopped" (or "failed" when SIGKILL escalation still leaves it alive)
  - clears LastError
  - stops the live lease
*/
func KillServiceProcess(s map[string]interface{}) error {
	pid := -1
	if s["Process"] != nil {
		pid = Utility.ToInt(s["Process"])
	}
	if pid == -1 {
		// Nothing to kill; ensure proxy/runtime are consistent.
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
		// Can't even obtain a handle. Treat as gone and clean up state.
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
		// Prefer group-terminate if we started service with Setpgid=true.
		// (negative pid sends the signal to the process group).
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

/*
StartServiceProcessWithWriters starts the service, ensures dependencies are up,
persists desired updates (Port/Proxy) to etcd, updates runtime via etcd, and
supervises the child process.

It is cycle-safe and will not infinitely recurse if A depends on B and B depends
on A.

Dependency formats supported under s["Dependencies"]:
  - []string{"rbac.RbacService", "log.LogService"}
  - []any (values ToString-able)
  - "rbac.RbacService,log.LogService"
  - map[string]bool or map[string]any (truthy values mean "required")

Readiness for dependencies: simply TCP dial on Address:Port with backoff,
bounded timeout.
*/
func StartServiceProcessWithWriters(
	s map[string]interface{},
	port int,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
) (int, error) {
	return startServiceProcessWithWritersInternal(s, port, stdoutWriter, stderrWriter, make(map[string]struct{}))
}

// depShortName extracts the leading token before the first dot (e.g. "log.LogService" -> "log").
func depShortName(dep string) string {
	if dep == "" {
		return ""
	}
	parts := strings.Split(dep, ".")
	if len(parts) == 0 {
		return strings.ToLower(dep)
	}
	return strings.ToLower(parts[0])
}

// findFreePort binds to 127.0.0.1:0 and returns the chosen port.
func findFreePort(ownerID string) (int, error) {
	allocator, err := config.NewDefaultPortAllocator()
	if err != nil {
		return 0, err
	}

	return allocator.Next(ownerID)
}

func parseDependencies(raw any) []string {
	out := []string{}
	switch v := raw.(type) {
	case nil:
		return out
	case []string:
		for _, s := range v {
			if ss := strings.TrimSpace(s); ss != "" {
				out = append(out, ss)
			}
		}
	case []any:
		for _, x := range v {
			s := strings.TrimSpace(Utility.ToString(x))
			if s != "" {
				out = append(out, s)
			}
		}
	case string:
		for _, part := range strings.Split(v, ",") {
			if s := strings.TrimSpace(part); s != "" {
				out = append(out, s)
			}
		}
	case map[string]bool:
		for k, ok := range v {
			if ok {
				if s := strings.TrimSpace(k); s != "" {
					out = append(out, s)
				}
			}
		}
	case map[string]any:
		for k, val := range v {
			// treat truthy as required
			truthy := false
			switch vv := val.(type) {
			case bool:
				truthy = vv
			default:
				truthy = Utility.ToString(vv) != "" && Utility.ToString(vv) != "false" && Utility.ToString(vv) != "0"
			}
			if truthy {
				if s := strings.TrimSpace(k); s != "" {
					out = append(out, s)
				}
			}
		}
	default:
		// best-effort stringification
		if s := strings.TrimSpace(Utility.ToString(v)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ensureDependenciesRunning parses s["Dependencies"], starts missing deps (recursively),
// and waits for their TCP port to accept connections (bounded, with backoff).
// If a dependency has no configuration in etcd, it will attempt to discover a local
// binary in config.GetServicesRoot(), create a minimal desired configuration, save it,
// and start it. Cycle-safe via 'seen'.
func ensureDependenciesRunning(s map[string]interface{}, seen map[string]struct{}) error {
	deps := parseDependencies(s["Dependencies"])
	if len(deps) == 0 {
		return nil
	}

	var errs []string
	for _, dep := range deps {
		if dep == "" {
			continue
		}

		// 1) Try to get existing config by Id or by Name (best candidate)
		cfg, _ := config.GetServiceConfigurationById(dep)
		if cfg == nil {
			cfg, _ = config.GetServiceConfigurationsByName(dep)
		}

		// 2) If still nil, discover a local binary and seed a minimal config.
		if cfg == nil {
			root := config.GetServicesRoot()
			if strings.TrimSpace(root) == "" {
				errs = append(errs, fmt.Sprintf("%s: ServicesRoot is empty; cannot seed", dep))
				continue
			}

			bin, err := config.FindServiceBinary(root, depShortName(dep))
			if err != nil || strings.TrimSpace(bin) == "" {
				errs = append(errs, fmt.Sprintf("%s: no configuration found and discovery failed: %v", dep, err))
				continue
			}

			addr, _ := config.GetAddress()
			if addr == "" {
				addr = "127.0.0.1"
			}

			port, err := findFreePort(dep)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: could not allocate port: %v", dep, err))
				continue
			}

			cfg = map[string]interface{}{
				"Id":           dep,
				"Name":         dep,
				"Path":         bin,
				"Address":      addr,
				"Port":         port,
				"Proxy":        port + 1,
				"State":        "stopped",
				"Process":      -1,
				"ProxyProcess": -1,
			}

			if err := config.SaveServiceConfiguration(cfg); err != nil {
				errs = append(errs, fmt.Sprintf("%s: failed to save seeded config: %v", dep, err))
				continue
			}

			slog.Info("seeded dependency configuration from services root",
				"dep", dep, "path", Utility.ToString(cfg["Path"]), "port", Utility.ToInt(cfg["Port"]))
		}

		depID := Utility.ToString(cfg["Id"])
		depName := Utility.ToString(cfg["Name"])
		state := strings.ToLower(Utility.ToString(cfg["State"]))
		addr := Utility.ToString(cfg["Address"])
		if addr == "" {
			addr = "127.0.0.1"
		}
		port := Utility.ToInt(cfg["Port"])

		// If there's still no port, allocate one and persist.
		if port <= 0 {
			fp, err := findFreePort(depID)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s (id=%s): could not allocate port: %v", depName, depID, err))
				continue
			}
			cfg["Port"] = fp
			cfg["Proxy"] = fp + 1
			if err := config.SaveServiceConfiguration(cfg); err != nil {
				errs = append(errs, fmt.Sprintf("%s (id=%s): failed to save port assignment: %v", depName, depID, err))
				continue
			}
			port = fp
		}

		// 3) If not running, try to start it.
		if state != "running" {
			slog.Info("starting dependency", "dep", firstNonEmpty(depID, depName), "port", port)
			if _, err := startServiceProcessWithWritersInternal(cfg, port, nil, nil, seen); err != nil {
				errs = append(errs, fmt.Sprintf("%s: start failed: %v", firstNonEmpty(depID, depName), err))
				continue
			}
		}

		slog.Info("dependency ready", "dep", firstNonEmpty(depID, depName))
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func startServiceProcessWithWritersInternal(
	s map[string]interface{},
	port int,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	seen map[string]struct{}, // cycle guard across recursive starts
) (int, error) {
	if stdoutWriter == nil {
		stdoutWriter = os.Stdout
	}
	if stderrWriter == nil {
		stderrWriter = os.Stderr
	}

	id := Utility.ToString(s["Id"])
	name := Utility.ToString(s["Name"])
	if id == "" && name == "" {
		return -1, fmt.Errorf("invalid service config: missing Id and Name")
	}
	key := id
	if key == "" {
		key = name
	}
	if _, already := seen[key]; already {
		slog.Warn("dependency cycle detected (skipping)", "service", key)
	} else {
		seen[key] = struct{}{} // mark before to guard recursion
		if err := ensureDependenciesRunning(s, seen); err != nil {
			return -1, fmt.Errorf("dependencies not satisfied for %s: %w", key, err)
		}
	}

	// Kill any previous instance (best-effort)
	if err := KillServiceProcess(s); err != nil {
		return -1, err
	}

	// Persist desired before spawn so child can read it
	s["Port"] = port
	s["Proxy"] = port + 1
	s["State"] = "starting"
	s["Process"] = -1
	if err := config.SaveServiceConfiguration(s); err != nil {
		slog.Warn("failed to persist desired service config before start", "id", s["Id"], "name", s["Name"], "err", err)
	}

	path := Utility.ToString(s["Path"])
	if path == "" || !Utility.Exists(path) {
		return -1, fmt.Errorf("service binary not found: %s (service=%s id=%s)", path, name, id)
	}

	// Ensure executable bit on Unix
	if fixed, err := config.ResolveServiceExecutable(Utility.ToString(s["Path"])); err == nil {
		s["Path"] = fixed
	} else {
		slog.Warn("service path not executable", "id", id, "name", name, "path", s["Path"], "err", err)
		return -1, fmt.Errorf("service path not executable: %s (service=%s id=%s): %w", path, name, id, err)
	}

	path = Utility.ToString(s["Path"])

	// CHILD ARGUMENTS: only the Id. The service will fetch its config by Id from etcd.
	cmd := exec.Command(path, id)
	cmd.Dir = filepath.Dir(path)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderrR, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}
	var startErrBuf bytes.Buffer

	doneStdout := make(chan struct{})
	go func() { crAwareCopy(stdoutWriter, stdout); close(doneStdout) }()

	doneStderr := make(chan struct{})
	go func() {
		tee := io.TeeReader(stderrR, &startErrBuf)
		r := bufio.NewReader(tee)
		for {
			line, e := r.ReadBytes('\n')
			if len(line) > 0 {
				_, _ = stderrWriter.Write(line)
			}
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
		_ = stdout.Close()
		_ = stderrR.Close()
		<-doneStdout
		<-doneStderr
		slog.Error("failed to start service", "name", name, "id", id, "err", err,
			"stderr", strings.TrimSpace(startErrBuf.String()))
		return -1, err
	}

	pid := cmd.Process.Pid
	slog.Info("service process started", "name", name, "id", id, "pid", pid, "port", port, "proxy", port+1)

	// Update ONLY runtime in etcd
	_ = config.PutRuntime(id, map[string]interface{}{
		"Process":   pid,
		"State":     "running",
		"LastError": "",
	})

	// mark live
	if _, err := config.StartLive(id, 15); err != nil {
		slog.Warn("failed to start etcd live lease", "id", id, "err", err)
	}

	// Supervise
	go func() {
		err := cmd.Wait()
		_ = stdout.Close()
		_ = stderrR.Close()
		<-doneStdout
		<-doneStderr

		state := "stopped"
		if err != nil {
			state = "failed"
			slog.Error("service process exited with error",
				"name", name, "id", id, "pid", pid, "err", err)
		} else {
			slog.Info("service process stopped", "name", name, "id", id, "pid", pid)
		}

		_ = config.PutRuntime(id, map[string]interface{}{
			"Process": -1,
			"State":   state,
			"LastError": func() string {
				if err != nil {
					return err.Error()
				}
				return ""
			}(),
		})

		config.StopLive(id)
		// Optional: restart policy could re-read desired here.
	}()

	return pid, nil
}

// -------------------------------------------------------------------------------------
// PROXY MANAGEMENT
// -------------------------------------------------------------------------------------

/*
StartServiceProxyProcess launches the gRPC-Web proxy (grpcwebproxy) for a running service.

Fixes vs. previous version:
  • If ProxyProcess is set but that PID is not actually running, we clear it and try to start again
    instead of failing with "proxy already exists". (This is the root cause behind your logs.)
  • We actively probe the backend gRPC port before starting the proxy to avoid immediate exits.
  • Improved binary resolution (PATH, <globular>/bin, <executable-dir>/bin) with clearer logs.
  • More robust supervision + KeepAlive restart when the backend is still up.
  • Runtime state is kept accurate even if the proxy exits unexpectedly.

Arguments:
  - s: service configuration map (must have Process != -1 and Port set)
  - certificateAuthorityBundle, certificate: filenames located under the TLS dir
    (<config>/tls/<name>.<domain>/) to use when TLS is enabled.

Returns:
  - proxy PID (int) on success
  - error on failure (and runtime updated accordingly)
*/
func StartServiceProxyProcess(s map[string]interface{}, certificateAuthorityBundle, certificate string) (int, error) {
	id := Utility.ToString(s["Id"])
	name := Utility.ToString(s["Name"])

	// The backend service must be running
	servicePid := Utility.ToInt(s["Process"])
	if servicePid == -1 {
		return -1, errors.New("service process pid must not be -1")
	}

	// If we THINK a proxy is already running, verify the PID before refusing to start.
	if pid := Utility.ToInt(s["ProxyProcess"]); pid != -1 {
		if alive, _ := Utility.PidExists(pid); alive {
			return -1, fmt.Errorf("proxy already exists for service %s (pid %d)", name, pid)
		}
		// Stale PID ⇒ clear it in runtime and continue to start a new proxy.
		_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
		slog.Warn("found stale proxy pid; cleared and retrying start", "service", name, "id", id, "stale_pid", pid)
	}

	// Resolve service address:port the proxy should talk to.
	servicePort := Utility.ToInt(s["Port"])
	address := Utility.ToString(s["Address"])
	if address == "" {
		address, _ = config.GetAddress()
	}
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}
	backend := net.JoinHostPort(address, strconv.Itoa(servicePort))

	// Wait briefly for the backend port to be open (avoid races).
	if ok := waitTcpOpen(backend, 3*time.Second); !ok {
		slog.Warn("starting proxy but backend is not listening yet (will try anyway)", "backend", backend)
	}

	// Build proxy args
	cmdName := "grpcwebproxy"
	if !strings.HasSuffix(cmdName, ".exe") && runtime.GOOS == "windows" {
		cmdName += ".exe"
	}

	nameCfg, _ := config.GetName()
	domain, _ := config.GetDomain()
	credsDir := filepath.Join(config.GetConfigDir(), "tls", nameCfg+"."+domain)
	proxyPort := Utility.ToInt(s["Proxy"])
	tlsEnabled := Utility.ToBool(s["TLS"])

	args := []string{
		"--backend_addr=" + backend,
		"--allow_all_origins=true",
		"--use_websockets=false",
		"--server_http_max_read_timeout=48h",
		"--server_http_max_write_timeout=48h",
	}

	if tlsEnabled {
		caTrust := filepath.Join(credsDir, "ca.crt")
		args = append(args,
			"--backend_tls=true",
			"--backend_tls_ca_files="+caTrust,
			"--backend_client_tls_cert_file="+filepath.Join(credsDir, "client.crt"),
			"--backend_client_tls_key_file="+filepath.Join(credsDir, "client.pem"),
			"--run_http_server=false",
			"--run_tls_server=true",
			"--server_http_tls_port="+strconv.Itoa(proxyPort),
			"--server_tls_key_file="+filepath.Join(credsDir, "server.pem"),
			"--server_tls_client_ca_files="+filepath.Join(credsDir, certificateAuthorityBundle),
			"--server_tls_cert_file="+filepath.Join(credsDir, certificate),
		)
	} else {
		args = append(args,
			"--run_http_server=true",
			"--run_tls_server=false",
			"--server_http_debug_port="+strconv.Itoa(proxyPort),
			"--backend_tls=false",
		)
	}

	// Resolve grpcwebproxy binary
	cmdPath, cmdDir, err := resolveGrpcWebProxy(cmdName)
	if err != nil {
		return -1, err
	}

	proxy := exec.Command(cmdPath, args...)
	proxy.Dir = cmdDir
	if runtime.GOOS != "windows" {
		proxy.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	}

	// Pipe through to parent logs (noisy is better than silent here).
	proxy.Stdout = os.Stdout
	proxy.Stderr = os.Stderr

	if err := proxy.Start(); err != nil {
		return -1, err
	}
	proxyPid := proxy.Process.Pid

	slog.Info("grpcwebproxy started",
		"service", name, "id", id,
		"backend", backend, "port", proxyPort,
		"tls", tlsEnabled, "pid", proxyPid, "path", cmdPath)

	// Update runtime: ProxyProcess + State
	_ = config.PutRuntime(id, map[string]interface{}{
		"ProxyProcess": proxyPid,
		"State":        "running",
	})

	// Publish updated service configuration to listeners (event bus), best-effort
	if str, _ := Utility.ToJson(s); str != "" {
		if address, _ := config.GetAddress(); address != "" {
			if ec, err := getEventClient(address); err == nil {
				_ = ec.Publish("update_globular_service_configuration_evt", []byte(str))
			}
		}
	}

	// Supervise proxy
	go func() {
		err := proxy.Wait()
		if err != nil {
			slog.Error("grpcwebproxy terminated with error", "service", name, "id", id, "pid", proxyPid, "err", err)
		} else {
			slog.Info("grpcwebproxy stopped", "service", name, "id", id, "pid", proxyPid)
		}

		// Clear runtime proxy pid if it still points to us
		_ = config.PutRuntime(id, map[string]interface{}{
			"ProxyProcess": -1,
		})

		// KeepAlive restart if backend is up
		scEtcd, errCfg := config.GetServiceConfigurationById(id)
		if errCfg != nil || scEtcd == nil {
			return
		}
		backendPid := Utility.ToInt(scEtcd["Process"])
		keepAlive := Utility.ToBool(scEtcd["KeepAlive"])
		if backendPid != -1 && keepAlive {
			slog.Info("keepalive: restarting grpcwebproxy", "service", scEtcd["Name"], "id", scEtcd["Id"])
			_, _ = StartServiceProxyProcess(scEtcd, certificateAuthorityBundle, certificate)
		}
	}()

	return proxyPid, nil
}

// KillServiceProxyProcess terminates the grpcwebproxy for a service if present and updates etcd runtime.
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

	state := "running"
	if Utility.ToInt(s["Process"]) == -1 {
		state = "stopped"
	}
	return config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
		"ProxyProcess": -1,
		"State":        state,
	})
}

// resolveGrpcWebProxy looks for the grpcwebproxy binary in a few common places and returns (path, dir).
func resolveGrpcWebProxy(cmdName string) (string, string, error) {
	// 1) PATH
	if p, err := exec.LookPath(cmdName); err == nil {
		return p, filepath.Dir(p), nil
	}

	// 2) <globular exec>/bin
	if dir := filepath.Join(config.GetGlobularExecPath(), "bin"); Utility.Exists(filepath.Join(dir, cmdName)) {
		return filepath.Join(dir, cmdName), dir, nil
	}

	// 3) <current executable dir>/bin
	if ex, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(ex), "bin")
		if Utility.Exists(filepath.Join(dir, cmdName)) {
			return filepath.Join(dir, cmdName), dir, nil
		}
	}

	return "", "", errors.New("grpcwebproxy executable not found (system PATH or ./bin)")
}

// waitTcpOpen returns true if tcp://hostPort becomes reachable within the timeout.
func waitTcpOpen(hostPort string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", hostPort, 250*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

// =====================================================================================
// Process helpers / status
// =====================================================================================

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

// =====================================================================================
// ETCD
// =====================================================================================

// sanitizeAdvertiseHost makes sure we don't advertise wildcards or empty hosts.
// Falls back to 127.0.0.1 for local dev.
func sanitizeAdvertiseHost(h string) string {
	h = strings.TrimSpace(h)
	switch strings.ToLower(h) {
	case "", "0.0.0.0", "::", "[::]":
		return "127.0.0.1"
	case "localhost":
		return "127.0.0.1"
	default:
		return h
	}
}

// StartEtcdServer starts etcd in the background, waits until it's ready,
// then returns. It no longer blocks for the lifetime of etcd.
func StartEtcdServer() error {
	const readyTimeout = 60 * time.Second // more forgiving than 15s
	const probeEvery = 200 * time.Millisecond

	localConfig, err := config.GetLocalConfig(true)
	if err != nil {
		return err
	}
	protocol := Utility.ToString(localConfig["Protocol"])
	if protocol == "" {
		protocol = "http"
	}

	// host we advertise (no port)
	hostPort, _ := config.GetAddress()
	host := hostPort
	if i := strings.Index(hostPort, ":"); i > 0 {
		host = hostPort[:i]
	}
	host = sanitizeAdvertiseHost(host) // avoid advertising 0.0.0.0 / blank / "::"

	name := Utility.ToString(localConfig["Name"])
	if name == "" {
		name, _ = config.GetName()
	}

	dataDir := config.GetDataDir() + "/etcd-data"
	_ = Utility.CreateDirIfNotExist(dataDir)
	cfgPath := config.GetConfigDir() + "/etcd.yml"

	// seed with any existing cfg
	nodeCfg := make(map[string]interface{})
	if Utility.Exists(cfgPath) {
		if data, err := os.ReadFile(cfgPath); err == nil {
			_ = yaml.Unmarshal(data, &nodeCfg)
		}
	}

	// Decide final scheme (may fall back to http if TLS files missing)
	scheme := protocol
	wantTLS := (protocol == "https")
	var certFile, keyFile, caFile string
	var clientCertFile, clientKeyFile string

	if wantTLS {
		domain, _ := config.GetDomain()
		certDir := config.GetConfigDir() + "/tls/" + name + "." + domain
		certFile = certDir + "/server.crt"
		keyFile = certDir + "/server.pem"
		caFile = certDir + "/ca.crt"
		clientCertFile = certDir + "/client.crt"
		clientKeyFile = certDir + "/client.pem"

		if !(Utility.Exists(certFile) && Utility.Exists(keyFile) && Utility.Exists(caFile)) {
			slog.Warn("etcd TLS requested but missing cert/key/CA; falling back to HTTP",
				"cert", certFile, "key", keyFile, "ca", caFile)
			wantTLS = false
			scheme = "http"
		} else {
			// Only enable client-cert-auth if we actually have a client cert we can use in probes.
			clientCertAuth := Utility.Exists(clientCertFile) && Utility.Exists(clientKeyFile)

			nodeCfg["client-transport-security"] = map[string]interface{}{
				"cert-file":        certFile,
				"key-file":         keyFile,
				"client-cert-auth": clientCertAuth,
				"trusted-ca-file":  caFile,
			}
			nodeCfg["peer-transport-security"] = map[string]interface{}{
				"cert-file":        certFile,
				"key-file":         keyFile,
				"client-cert-auth": true, // peers should mutually auth
				"trusted-ca-file":  caFile,
			}
		}
	}
	if !wantTLS {
		delete(nodeCfg, "client-transport-security")
		delete(nodeCfg, "peer-transport-security")
	}

	// Listeners + advertised addresses
	listenClientURLs := scheme + "://" + host + ":2379"
	listenPeerURLs := scheme + "://" + host + ":2380"
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
			ph := Utility.ToString(peer["Hostname"])
			pd := Utility.ToString(peer["Domain"])
			if ph != "" && pd != "" {
				initialCluster += "," + ph + "=" + scheme + "://" + ph + "." + pd + ":2380"
			}
		}
	}
	nodeCfg["initial-cluster"] = initialCluster

	// cluster state
	if Utility.Exists(filepath.Join(dataDir, "member")) {
		nodeCfg["initial-cluster-state"] = "existing"
	} else {
		nodeCfg["initial-cluster-state"] = "new"
	}

	// write config
	out, err := yaml.Marshal(nodeCfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, out, 0o644); err != nil {
		return err
	}

	etcdPath, err := findEtcdBinary()
	if err != nil {
		slog.Error("cannot start etcd", "err", err)
		return err
	}

	slog.Info("starting etcd",
		"config", cfgPath,
		"listen-client-urls", listenClientURLs,
		"advertise-client-urls", advertiseClientURLs)

	cmd := exec.Command(etcdPath, "--config-file", cfgPath)
	cmd.Dir = os.TempDir()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	}

	if err := cmd.Start(); err != nil {
		slog.Error("failed to start etcd", "err", err)
		return err
	}
	setEtcdCmd(cmd)

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	// Graceful shutdown on SIGINT/SIGTERM
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		slog.Info("received shutdown signal for etcd; terminating", "signal", sig)
		_ = StopEtcdServer()
	}()

	// -------- Readiness probe (multi-endpoint) ----------
	deadline := time.Now().Add(readyTimeout)

	// Build candidates to probe: 127.0.0.1:2379, <host>:2379, plus any host:port from advertise/listen URLs.
	type endpoint struct {
		HostPort string // "10.0.0.63:2379"
		URL      string // "http://10.0.0.63:2379/health"
	}
	dedup := make(map[string]struct{})
	add := func(hp string) {
		if hp == "" {
			return
		}
		if _, ok := dedup[hp]; ok {
			return
		}
		dedup[hp] = struct{}{}
	}

	// Always try loopback first
	add(net.JoinHostPort("127.0.0.1", "2379"))
	add(net.JoinHostPort(host, "2379"))

	parseClientURLs := func(csv string) {
		for _, raw := range strings.Split(csv, ",") {
			u := strings.TrimSpace(raw)
			if u == "" {
				continue
			}
			uu, err := url.Parse(u)
			if err != nil {
				continue
			}
			hp := uu.Host
			if !strings.Contains(hp, ":") {
				hp = net.JoinHostPort(hp, "2379")
			}
			add(hp)
		}
	}
	parseClientURLs(advertiseClientURLs)
	parseClientURLs(listenClientURLs)

	var candidates []endpoint
	for hp := range dedup {
		candidates = append(candidates, endpoint{
			HostPort: hp,
			URL:      scheme + "://" + hp + "/health",
		})
	}

	// Prepare HTTP client (TLS or not) once.
	var httpClient *http.Client
	if scheme == "https" && wantTLS {
		tlsCfg := &tls.Config{}
		// Trust the server CA if present, otherwise (dev) skip verify.
		if b, err := os.ReadFile(caFile); err == nil {
			cp := x509.NewCertPool()
			if cp.AppendCertsFromPEM(b) {
				tlsCfg.RootCAs = cp
			}
		}
		// If etcd was configured with client-cert-auth, present client cert if available.
		if Utility.Exists(clientCertFile) && Utility.Exists(clientKeyFile) {
			if cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile); err == nil {
				tlsCfg.Certificates = []tls.Certificate{cert}
			}
		} else if tlsCfg.RootCAs == nil {
			// Last resort for dev: allow handshake if no CA loaded.
			tlsCfg.InsecureSkipVerify = true //nolint:gosec
		}
		httpClient = &http.Client{
			Timeout:   1 * time.Second,
			Transport: &http.Transport{TLSClientConfig: tlsCfg},
		}
	} else {
		httpClient = &http.Client{Timeout: 1 * time.Second}
	}

	probeOne := func(ep endpoint) bool {
		// First try HTTP /health
		if resp, err := httpClient.Get(ep.URL); err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				slog.Info("etcd is healthy", "endpoint", ep.URL)
				return true
			}
		}
		// Fallback: check TCP open
		if conn, err := net.DialTimeout("tcp", ep.HostPort, 500*time.Millisecond); err == nil {
			conn.Close()
			slog.Info("etcd TCP port open; proceeding", "endpoint", ep.HostPort)
			return true
		}
		return false
	}

	for time.Now().Before(deadline) {
		select {
		case err := <-waitErr:
			if err == nil {
				return fmt.Errorf("etcd exited before becoming ready")
			}
			return fmt.Errorf("etcd exited early: %w", err)
		default:
		}
		for _, ep := range candidates {
			if probeOne(ep) {
				slog.Info("etcd is ready")
				return nil
			}
		}
		time.Sleep(probeEvery)
	}

	select {
	case err := <-waitErr:
		if err == nil {
			return fmt.Errorf("etcd exited before becoming ready")
		}
		return fmt.Errorf("etcd exited early: %w", err)
	default:
	}
	_ = StopEtcdServer()
	return fmt.Errorf("etcd did not become ready within %s", readyTimeout)
}

// ---------------- helpers ----------------

var etcdCmdMu sync.Mutex
var etcdCmd *exec.Cmd

func setEtcdCmd(c *exec.Cmd) { etcdCmdMu.Lock(); defer etcdCmdMu.Unlock(); etcdCmd = c }

// StopEtcdServer gracefully stops the etcd process started by StartEtcdServer.
func StopEtcdServer() error {
	etcdCmdMu.Lock()
	c := etcdCmd
	etcdCmd = nil
	etcdCmdMu.Unlock()
	if c == nil || c.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		_ = c.Process.Kill()
		return nil
	}
	_ = syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
	_ = c.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{}, 1)
	go func() { _, _ = c.Process.Wait(); done <- struct{}{} }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		_ = c.Process.Kill()
	}
	return nil
}

func findEtcdBinary() (string, error) {
	if p, err := exec.LookPath("etcd"); err == nil {
		return p, nil
	}
	candidates := []string{"/usr/local/bin/etcd", "/usr/bin/etcd"}
	for _, p := range candidates {
		if Utility.Exists(p) {
			return p, nil
		}
	}
	return "", errors.New("etcd binary not found in PATH")
}

// =====================================================================================
// Envoy
// =====================================================================================

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

// =====================================================================================
// Monitoring (Prometheus / node_exporter / alertmanager)
// =====================================================================================

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
	servicesCpuUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "globular_services_cpu_usage_counter", Help: "Monitor the CPU usage of each service."}, []string{"id", "name"})
	servicesMemoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "globular_services_memory_usage_counter", Help: "Monitor the memory usage of each service."}, []string{"id", "name"})
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

// =====================================================================================
// Internals
// =====================================================================================

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
		if b == '\r' {
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