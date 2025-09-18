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
	"go.etcd.io/etcd/client/v3/concurrency"
	"gopkg.in/yaml.v3"
)

//
// =====================================================================================
// Prometheus metrics
// =====================================================================================
//

var (
	servicesCpuUsage    *prometheus.GaugeVec
	servicesMemoryUsage *prometheus.GaugeVec
)

//
// =====================================================================================
/*
   RUNTIME SYNC

   Helpers to reconcile an in-memory service map `s` with authoritative runtime
   info stored in etcd (via config.{Get,Put}Runtime). This reduces races and
   fixes "duplicate start" problems caused by stale PIDs in config maps.
*/
// =====================================================================================
//

// syncFromRuntime copies Process/ProxyProcess (and optionally Port/State) from runtime into s.
func syncFromRuntime(s map[string]interface{}) {
	id := Utility.ToString(s["Id"])
	if id == "" {
		return
	}
	if rt, _ := config.GetRuntime(id); rt != nil {
		if v := rt["Process"]; v != nil {
			s["Process"] = v
		}
		if v := rt["ProxyProcess"]; v != nil {
			s["ProxyProcess"] = v
		}
		if v := rt["State"]; v != nil {
			s["State"] = v
		}
		if v := rt["Port"]; v != nil {
			s["Port"] = v
		}
	}
}

//
// =====================================================================================
// Process lifecycle (services + proxy)
// =====================================================================================
//

/*
KillServiceProcess terminates a running service process. It:

  - syncs PIDs from etcd runtime (defensive against stale maps)
  - sends SIGTERM to the process group (Unix), falls back to SIGKILL if needed
  - stops the live lease and proxy
  - updates etcd runtime to a stopped/failed state
*/
func KillServiceProcess(s map[string]interface{}) error {
	// Reconcile from runtime first to avoid acting on stale PIDs.
	syncFromRuntime(s)

	pid := -1
	if s["Process"] != nil {
		pid = Utility.ToInt(s["Process"])
	}

	// No PID? Clean up runtime + proxy and exit gracefully.
	if pid == -1 || pid == 0 {
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
		// Treat as gone; clean up and move on.
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

	// 1) Try graceful stop first.
	if runtime.GOOS == "windows" {
		_ = proc.Kill()
	} else {
		// Negative PID => signal the whole process group (if our child was started with Setpgid).
		if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
			_ = proc.Signal(syscall.SIGTERM)
		}
	}

	// 2) Wait up to 5s for the process to die.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		dead, _ := isDead(pid)
		if dead {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	// 3) Escalate to SIGKILL if still alive (Unix).
	if alive, _ := isAlive(pid); alive && runtime.GOOS != "windows" {
		// Best-effort: kill any sibling by basename (some services may fork).
		bin := filepath.Base(Utility.ToString(s["Path"]))
		if ids, _ := Utility.GetProcessIdsByName(bin); len(ids) > 0 {
			for _, id := range ids {
				_ = syscall.Kill(-id, syscall.SIGKILL)
				_ = syscall.Kill(id, syscall.SIGKILL)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// 4) Verify & update state.
	if alive, _ := isAlive(pid); alive {
		err := fmt.Errorf("service pid %d did not exit after SIGTERM/SIGKILL", pid)
		_ = config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
			"LastError": err.Error(),
			"State":     "failed",
		})
		return err
	}

	// Mark stopped; stop liveness; stop proxy.
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

// acquireStartLock returns an etcd session and a mutex for the service start.
// Caller must call mu.Unlock(ctx) and sess.Close() when done.
func acquireStartLock(ctx context.Context, serviceKey string) (*concurrency.Session, *concurrency.Mutex, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, nil, err
	}
	// Short TTL so abandoned locks free themselves quickly.
	sess, err := concurrency.NewSession(cli, concurrency.WithTTL(10))
	if err != nil {
		return nil, nil, err
	}
	m := concurrency.NewMutex(sess, "/globular/locks/start/"+serviceKey)
	if err := m.Lock(ctx); err != nil {
		_ = sess.Close()
		return nil, nil, err
	}
	return sess, m, nil
}

/*
StartServiceProcessWithWriters starts the service and supervises it.

Key behavior:
  - Syncs with etcd runtime to avoid acting on stale PIDs.
  - Short-circuits if the service is already RUNNING and the PID is alive.
  - Uses a distributed lock to serialize concurrent starts.
  - Marks state STARTING early; flips to RUNNING after spawn succeeds.
  - Supervises the child process; updates runtime on exit.
  - Does a best-effort KillServiceProcess only when needed (no duplicate starts).
*/
func StartServiceProcessWithWriters(
	s map[string]interface{},
	port int,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
) (int, error) {
	return startServiceProcessWithWritersInternal(s, port, stdoutWriter, stderrWriter, make(map[string]struct{}))
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
	lockKey := id
	if lockKey == "" {
		lockKey = name
	}

	// Acquire a short-lived distributed lock per service.
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	sess, mu, err := acquireStartLock(ctx, lockKey)
	if err != nil {
		return -1, fmt.Errorf("failed to acquire start lock for %s: %w", lockKey, err)
	}
	defer func() {
		_ = mu.Unlock(context.Background())
		_ = sess.Close()
	}()

	// Sync from runtime before deciding anything.
	syncFromRuntime(s)

	// ---- Fast path: if already running, skip duplicate start ----
	if rt, _ := config.GetRuntime(id); rt != nil {
		if rt["State"] != nil && rt["Process"] != nil {
			state := strings.ToLower(Utility.ToString(rt["State"]))
			pid := Utility.ToInt(rt["Process"])
			if state == "starting" {
				return -1, fmt.Errorf("service %s is already starting", id)
			}
			if state == "running" && pid > 0 {
				if ok, _ := Utility.PidExists(pid); ok {
					slog.Info("service already running; skipping duplicate start", "id", id, "pid", pid)
					return pid, nil
				}
			}
		}
	}

	// Kill any previous instance (best-effort). This uses up-to-date PID due to syncFromRuntime().
	if err := KillServiceProcess(s); err != nil {
		return -1, err
	}

	// Persist desired settings BEFORE spawn so the child reads them.
	s["Port"] = port
	s["Proxy"] = port + 1
	s["Process"] = -1
	if err := config.SaveServiceConfiguration(s); err != nil {
		slog.Warn("failed to persist desired service config before start", "id", s["Id"], "name", s["Name"], "err", err)
	}

	// Mark "starting" early so other contenders back off.
	_ = config.PutRuntime(id, map[string]interface{}{
		"Process":   -1,
		"State":     "starting",
		"LastError": "",
	})

	// Resolve & verify service binary path.
	path := Utility.ToString(s["Path"])
	if path == "" || !Utility.Exists(path) {
		return -1, fmt.Errorf("service binary not found: %s (service=%s id=%s)", path, name, id)
	}
	if fixed, err := config.ResolveServiceExecutable(path); err == nil {
		s["Path"] = fixed
	} else {
		slog.Warn("service path not executable", "id", id, "name", name, "path", s["Path"], "err", err)
		return -1, fmt.Errorf("service path not executable: %s (service=%s id=%s): %w", path, name, id, err)
	}
	path = Utility.ToString(s["Path"])

	// Spawn: only the Id as arg; service fetches its config from etcd.
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
		slog.Error("failed to start service", "name", name, "id", id, "err", err, "stderr", strings.TrimSpace(startErrBuf.String()))
		_ = config.PutRuntime(id, map[string]interface{}{
			"Process":   -1,
			"State":     "failed",
			"LastError": err.Error(),
		})
		return -1, err
	}

	pid := cmd.Process.Pid
	slog.Info("service process started", "name", name, "id", id, "pid", pid, "port", port, "proxy", port+1)

	// Update runtime to RUNNING.
	_ = config.PutRuntime(id, map[string]interface{}{
		"Process":   pid,
		"State":     "running",
		"LastError": "",
	})

	// Start liveness lease.
	if _, err := config.StartLive(id, 15); err != nil {
		slog.Warn("failed to start etcd live lease", "id", id, "err", err)
	}

	// Supervise: capture exit and update runtime.
	go func() {
		err := cmd.Wait()
		_ = stdout.Close()
		_ = stderrR.Close()
		<-doneStdout
		<-doneStderr

		state := "stopped"
		if err != nil {
			state = "failed"
			slog.Error("service process exited with error", "name", name, "id", id, "pid", pid, "err", err)
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
	}()

	return pid, nil
}

//
// -------------------------------------------------------------------------------------
// PROXY MANAGEMENT
// -------------------------------------------------------------------------------------
//

/*
StartServiceProxyProcess launches grpcwebproxy for a running service.

Defenses:
  - Sync runtime first (avoid stale ProxyProcess PIDs).
  - If proxy port is already bound, assume another instance is serving and skip.
  - If ProxyProcess is set but dead, clear it and continue.
  - Try-lock to avoid concurrent proxy starts.
  - Supervise and clear runtime ProxyProcess on exit.
*/
// StartServiceProxyProcess launches grpcwebproxy for a running service.
//
// Changes vs previous version:
// - Removed the optimistic "port already bound; skip" branch.
// - Always attempt to start the proxy after runtime/PID sanity checks.
// - After starting, verify the proxy port actually becomes reachable within a short timeout.
//   If it doesn't, log+return a concrete error instead of silently skipping.
// - Still guards against double-start via etcd TryLock and clears stale ProxyProcess PIDs.
func StartServiceProxyProcess(s map[string]interface{}, certificateAuthorityBundle, certificate string) (int, error) {
	// Reconcile from runtime; needed for correct Process/ProxyProcess decisions.
	syncFromRuntime(s)

	id := Utility.ToString(s["Id"])
	name := Utility.ToString(s["Name"])

	// Backend service must be running.
	servicePid := Utility.ToInt(s["Process"])
	if servicePid == -1 || servicePid == 0 {
		return -1, errors.New("service process pid must not be -1")
	}

	// Per-service try-lock to avoid double proxy start.
	if cli, err := config.GetEtcdClient(); err == nil && cli != nil {
		if sess, e := concurrency.NewSession(cli, concurrency.WithTTL(8)); e == nil {
			m := concurrency.NewMutex(sess, "/globular/locks/proxy/"+id)
			ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			defer cancel()
			if e := m.TryLock(ctx); e != nil {
				_ = sess.Close()
				return -1, fmt.Errorf("proxy start in progress for %s", id)
			}
			defer func() {
				_ = m.Unlock(context.Background())
				_ = sess.Close()
			}()
		}
	}

	// If we think a proxy is running, verify its PID.
	if pid := Utility.ToInt(s["ProxyProcess"]); pid > 0 {
		if alive, _ := Utility.PidExists(pid); alive {
			return -1, fmt.Errorf("proxy already exists for service %s (pid %d)", name, pid)
		}
		// Stale PID ⇒ clear it in runtime and continue to start a new proxy.
		_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
		slog.Warn("found stale proxy pid; cleared and retrying start", "service", name, "id", id, "stale_pid", pid)
	}

	// Resolve backend address:port.
	servicePort := Utility.ToInt(s["Port"])
	address := Utility.ToString(s["Address"])
	if address == "" {
		address, _ = config.GetAddress()
	}
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}
	backend := net.JoinHostPort(address, strconv.Itoa(servicePort))

	// Briefly wait for backend to listen (avoid races).
	if ok := waitTcpOpen(backend, 3*time.Second); !ok {
		slog.Warn("starting proxy but backend is not listening yet (will try anyway)", "backend", backend)
	}

	// Build proxy args.
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
			"--server_tls_key_file="+filepath.Join(credsDir, "server.key"),
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

	// Resolve grpcwebproxy binary.
	cmdPath, cmdDir, err := resolveGrpcWebProxy(cmdName)
	if err != nil {
		return -1, err
	}

	proxy := exec.Command(cmdPath, args...)
	proxy.Dir = cmdDir
	if runtime.GOOS != "windows" {
		proxy.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	}
	proxy.Stdout = os.Stdout
	proxy.Stderr = os.Stderr

	if err := proxy.Start(); err != nil {
		return -1, err
	}
	proxyPid := proxy.Process.Pid

	slog.Info("grpcwebproxy started (verifying reachability)", "service", name, "id", id, "backend", backend, "port", proxyPort, "tls", tlsEnabled, "pid", proxyPid, "path", cmdPath)

	// Update runtime immediately with the PID (we’ll clear it if startup fails).
	_ = config.PutRuntime(id, map[string]interface{}{
		"ProxyProcess": proxyPid,
		"State":        "running",
	})

	// Quick readiness: give the proxy a moment to bind; then confirm the port is open.
	ready := make(chan error, 1)
	go func() { ready <- proxy.Wait() }() // if it crashes instantly, we’ll learn here

	// Wait up to ~2s for the port to open; if the process already died, surface that error.
	checkDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(checkDeadline) {
		select {
		case err := <-ready:
			// Process exited early — startup failed (very often "address already in use").
			_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
			if err != nil {
				slog.Error("grpcwebproxy terminated during startup", "service", name, "id", id, "pid", proxyPid, "err", err)
				return -1, fmt.Errorf("grpcwebproxy failed to start: %w", err)
			}
			slog.Info("grpcwebproxy exited immediately without error (unexpected)", "service", name, "id", id, "pid", proxyPid)
			return -1, errors.New("grpcwebproxy exited immediately")
		default:
			// Try both IPv4 and IPv6 localhost to confirm the bind.
			hp4 := net.JoinHostPort("127.0.0.1", strconv.Itoa(proxyPort))
			hp6 := net.JoinHostPort("::1", strconv.Itoa(proxyPort))
			if portOpen := waitTcpOpen(hp4, 150*time.Millisecond) || waitTcpOpen(hp6, 150*time.Millisecond); portOpen {
				// Looks good — keep supervising below and return success.
				goto SUPERVISE
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Not reachable; assume bad start. Try to collect exit state if it died just after.
	select {
	case err := <-ready:
		_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
		if err != nil {
			slog.Error("grpcwebproxy terminated during startup", "service", name, "id", id, "pid", proxyPid, "err", err)
			return -1, fmt.Errorf("grpcwebproxy failed to start: %w", err)
		}
	default:
	}
	// Still alive but not listening — kill it and report error.
	_ = syscall.Kill(-proxyPid, syscall.SIGTERM)
	_ = proxy.Process.Signal(syscall.SIGTERM)
	time.Sleep(200 * time.Millisecond)
	_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
	return -1, fmt.Errorf("grpcwebproxy did not become reachable on port %d", proxyPort)

SUPERVISE:
	// Supervise proxy (clear runtime PID on exit).
	go func() {
		err := <-ready
		if err != nil {
			slog.Error("grpcwebproxy terminated with error", "service", name, "id", id, "pid", proxyPid, "err", err)
		} else {
			slog.Info("grpcwebproxy stopped", "service", name, "id", id, "pid", proxyPid)
		}
		_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
	}()

	return proxyPid, nil
}

/*
KillServiceProxyProcess terminates grpcwebproxy for a service if present and updates etcd runtime.
*/
func KillServiceProxyProcess(s map[string]interface{}) error {
	// Reconcile from runtime to avoid stale PIDs.
	syncFromRuntime(s)

	pid := Utility.ToInt(s["ProxyProcess"])
	if pid <= 0 {
		return nil
	}

	proc, _ := os.FindProcess(pid)

	// Graceful first.
	if runtime.GOOS == "windows" {
		_ = proc.Kill()
	} else {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = proc.Signal(syscall.SIGTERM)
	}

	// Wait up to 5s.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := Utility.PidExists(pid); !ok {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	// Escalate if needed.
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
	if Utility.ToInt(s["Process"]) <= 0 {
		state = "stopped"
	}
	return config.PutRuntime(Utility.ToString(s["Id"]), map[string]interface{}{
		"ProxyProcess": -1,
		"State":        state,
	})
}

//
// -------------------------------------------------------------------------------------
// grpcwebproxy binary resolution
// -------------------------------------------------------------------------------------
//

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

//
// =====================================================================================
// Process helpers / status
// =====================================================================================
//

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

//
// =====================================================================================
// ETCD (single-node bootstrap with DNS-first advertising)
// =====================================================================================

/*
StartEtcdServer launches etcd using a generated config file. It:

  - Builds listen/advertise URLs (DNS-first) and optional TLS sections.
  - Skips start if a healthy etcd is already reachable at the advertised URL.
  - Refuses to start if the listen ports are in use but the endpoint is unhealthy.
  - Waits for readiness (HTTP /health or TCP).
  - Handles SIGTERM for graceful shutdown.

The generated config is written to <config>/etcd.yml and data to <data>/etcd-data.
*/
func StartEtcdServer() error {
	const readyTimeout = 60 * time.Second
	const probeEvery = 200 * time.Millisecond

	localConfig, err := config.GetLocalConfig(true)
	if err != nil {
		return err
	}

	protocol := strings.ToLower(Utility.ToString(localConfig["Protocol"]))
	if protocol == "" {
		protocol = "http"
	}

	// Service identity.
	name := Utility.ToString(localConfig["Name"])
	if name == "" {
		name, _ = config.GetName()
	}
	domain, _ := config.GetDomain()

	// The advertised host is the stable DNS name that clients will use.
	advHost := name
	if domain != "" && !strings.Contains(name, ".") {
		advHost = name + "." + domain
	}
	advHost = strings.TrimSpace(advHost)

	if advHost == "" {
		return errors.New("cannot determine etcd advertised hostname")
	}

	if advHost == "127.0.0.1" || advHost == "0.0.0.0" {
		slog.Warn("etcd advertised hostname resolves to localhost; this is not suitable for clustering")
		advHost = "localhost"
	}

	dataDir := config.GetDataDir() + "/etcd-data"
	_ = Utility.CreateDirIfNotExist(dataDir)
	cfgPath := config.GetConfigDir() + "/etcd.yml"

	// Seed from existing cfg if present.
	nodeCfg := make(map[string]interface{})
	if Utility.Exists(cfgPath) {
		if data, err := os.ReadFile(cfgPath); err == nil {
			_ = yaml.Unmarshal(data, &nodeCfg)
		}
	}

	// TLS
	scheme := protocol
	wantTLS := (protocol == "https")
	var certFile, keyFile, caFile string
	var clientCertFile, clientKeyFile string

	if wantTLS {

		certDir := filepath.Join(config.GetConfigDir(), "tls", advHost)
		certFile = filepath.Join(certDir, "server.crt")
		keyFile = filepath.Join(certDir, "server.pem")
		caFile = filepath.Join(certDir, "ca.crt")
		clientCertFile = filepath.Join(certDir, "client.crt")
		clientKeyFile = filepath.Join(certDir, "client.pem")

		if !(Utility.Exists(certFile) && Utility.Exists(keyFile) && Utility.Exists(caFile)) {
			slog.Warn("etcd TLS requested but missing cert/key/CA; falling back to HTTP",
				"cert", certFile, "key", keyFile, "ca", caFile)
			wantTLS = false
			scheme = "http"
		} else {
			clientCertAuth := false
			if Utility.Exists(clientCertFile) && Utility.Exists(clientKeyFile) {
				if crt, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile); err == nil && isClientAuthCert(crt) {
					clientCertAuth = true
				}
			}
			nodeCfg["client-transport-security"] = map[string]interface{}{
				"cert-file":        certFile,
				"key-file":         keyFile,
				"client-cert-auth": clientCertAuth,
			}
			if clientCertAuth {
				nodeCfg["client-transport-security"].(map[string]interface{})["trusted-ca-file"] = caFile
			}
			nodeCfg["peer-transport-security"] = map[string]interface{}{
				"cert-file":        certFile,
				"key-file":         keyFile,
				"client-cert-auth": true,
				"trusted-ca-file":  caFile,
			}
		}
	}
	if !wantTLS {
		delete(nodeCfg, "client-transport-security")
		delete(nodeCfg, "peer-transport-security")
	}

	// DNS-first URLs: bind on all interfaces; advertise the DNS name.
	const clientPort = "2379"
	const peerPort = "2380"

	
	listenAddress :=  config.GetLocalIP()
	listenClientURLs := scheme + "://" + net.JoinHostPort(listenAddress, clientPort)
	listenPeerURLs := scheme + "://" + net.JoinHostPort(listenAddress, peerPort)

	advertiseClientURLs := scheme + "://" + net.JoinHostPort(advHost, clientPort)
	initialAdvertisePeerURLs := scheme + "://" + net.JoinHostPort(advHost, peerPort)

	nodeCfg["name"] = name
	nodeCfg["data-dir"] = dataDir
	nodeCfg["listen-client-urls"] = "http://127.0.0.1:" + clientPort + "," + listenClientURLs
	nodeCfg["listen-peer-urls"] = listenPeerURLs
	nodeCfg["advertise-client-urls"] = advertiseClientURLs
	nodeCfg["initial-advertise-peer-urls"] = initialAdvertisePeerURLs

	// Initial cluster (use advertised DNS hosts).
	initialCluster := name + "=" + initialAdvertisePeerURLs
	if peers, ok := localConfig["Peers"].([]interface{}); ok {
		for _, raw := range peers {
			peer, _ := raw.(map[string]interface{})
			ph := strings.TrimSpace(Utility.ToString(peer["Hostname"]))
			pd := strings.TrimSpace(Utility.ToString(peer["Domain"]))
			if ph == "" {
				continue
			}
			adv := ph
			if pd != "" && !strings.Contains(ph, ".") {
				adv = ph + "." + pd
			}
			initialCluster += "," + Utility.ToString(peer["Name"]) + "=" + scheme + "://" + net.JoinHostPort(adv, peerPort)
		}
	}

	nodeCfg["initial-cluster"] = initialCluster

	// Cluster state: "existing" if data dir already has a member.
	if Utility.Exists(filepath.Join(dataDir, "member")) {
		nodeCfg["initial-cluster-state"] = "existing"
	} else {
		nodeCfg["initial-cluster-state"] = "new"
	}

	// If etcd already healthy at advertised endpoint, do NOT start another.
	if alreadyHealthy(scheme, caFile, clientCertFile, clientKeyFile) {
		slog.Info("etcd already running and healthy; skipping start", "advertise-client-urls", advertiseClientURLs)
		return nil
	}

	// If ports are bound, refuse to start a second server unless endpoint is healthy.
	if portInUse(net.JoinHostPort(listenAddress, peerPort)) || portInUse(net.JoinHostPort(listenAddress, clientPort)) {
		return fmt.Errorf("etcd ports already in use but endpoint not healthy (peer=%s, client=%s)",
			net.JoinHostPort(listenAddress, peerPort), net.JoinHostPort(listenAddress, clientPort))
	}

	// Write config.
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

	slog.Info("starting etcd", "config", cfgPath, "listen-client-urls", listenClientURLs, "advertise-client-urls", advertiseClientURLs)

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

	// Readiness probe (DNS-first).
	deadline := time.Now().Add(readyTimeout)
	httpClient := healthHTTPClient(wantTLS, caFile, clientCertFile, clientKeyFile)

	probeOne := func(u string) bool {
		raw := u

		// Ensure scheme for HTTP probe
		if !strings.Contains(u, "://") {
			// etcd’s /health is HTTP when you’re in insecure mode; use https if your httpClient has TLS.
			if wantTLS {
				u = "https://" + u
			} else {
				u = "http://" + u
			}
		}

		// HTTP /health (short timeout recommended)
		req, _ := http.NewRequest("GET", u+"/health", nil)
		ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
		defer cancel()
		req = req.WithContext(ctx)

		if resp, err := httpClient.Do(req); err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				slog.Info("etcd is healthy", "endpoint", u)
				return true
			}
		}

		// TCP fallback: extract host:port even if original had no scheme
		uu, _ := url.Parse(u)
		hp := uu.Host
		if hp == "" {
			// If Host is empty (because we added scheme above, this normally won't happen),
			// recover from the original raw input.
			hp = raw
		}
		if !strings.Contains(hp, ":") {
			hp = net.JoinHostPort(hp, "2379")
		}

		d := net.Dialer{Timeout: 500 * time.Millisecond}
		if conn, err := d.Dial("tcp", hp); err == nil {
			conn.Close()
			slog.Info("etcd TCP port open; proceeding", "endpoint", hp)
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

		// You already built these earlier:
		advertiseClientURLs := scheme + "://" + net.JoinHostPort(advHost, clientPort)
		if probeOne(advertiseClientURLs) {
			slog.Info("etcd is ready")
			return nil
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

func setEtcdCmd(c *exec.Cmd) { etcdCmdMu.Lock(); etcdCmd = c; etcdCmdMu.Unlock() }

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

// --- small local helpers ---

func portInUse(hp string) bool {
	ln, err := net.Listen("tcp", hp)
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

func isClientAuthCert(c tls.Certificate) bool {
	if len(c.Certificate) == 0 {
		return false
	}
	if leaf, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
		for _, eku := range leaf.ExtKeyUsage {
			if eku == x509.ExtKeyUsageClientAuth {
				return true
			}
		}
	}
	return false
}

func healthHTTPClient(wantTLS bool, caPath, clientCrt, clientKey string) *http.Client {
	tr := &http.Transport{}
	if wantTLS {
		tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
		tlsCfg.ServerName, _ = config.GetHostname() // for SNI
		// Accept self-signed certs by default (we use our own CA)
		tlsCfg.InsecureSkipVerify = true
		// Root CAs

		// Start from system pool
		if sys, _ := x509.SystemCertPool(); sys != nil {
			tlsCfg.RootCAs = sys
		} else {
			tlsCfg.RootCAs = x509.NewCertPool()
		}
		// Add custom CA if provided
		if caPath != "" && Utility.Exists(caPath) {
			if b, err := os.ReadFile(caPath); err == nil {
				_ = tlsCfg.RootCAs.AppendCertsFromPEM(b)
			}
		}
		// Optional client cert (for mTLS)
		if Utility.Exists(clientCrt) && Utility.Exists(clientKey) {
			if crt, err := tls.LoadX509KeyPair(clientCrt, clientKey); err == nil && isClientAuthCert(crt) {
				tlsCfg.Certificates = []tls.Certificate{crt}
			}
		}
		// Do NOT force ServerName; let http set it from req URL.
		tr.TLSClientConfig = tlsCfg
	}
	return &http.Client{Transport: tr, Timeout: 3 * time.Second}
}

func alreadyHealthy(scheme, caFile, clientCertFile, clientKeyFile string) bool {
	httpClient := healthHTTPClient(scheme == "https", caFile, clientCertFile, clientKeyFile)

	for _, u := range config.GetEtcdEndpointsHostPorts() {
		resp, err := httpClient.Get(u + "/health")
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return true
		}
	}
	return false
}

//
// =====================================================================================
// Envoy
// =====================================================================================
//

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

//
// =====================================================================================
// Monitoring (Prometheus / node_exporter / alertmanager)
// =====================================================================================
//

/*
StartProcessMonitoring starts Prometheus, Alertmanager, and node_exporter (best-effort),
exposes internal metrics at /metrics, and periodically feeds per-process CPU/memory
metrics for Globular and known services.

Args:
  - protocol: "http" or "https" for Prometheus web UI
  - httpPort: Globular HTTP port to scrape internal metrics
  - exit:     signal channel to stop the feeder
*/
func StartProcessMonitoring(protocol string, httpPort int, exit chan bool) error {
	// If Prometheus is already running, do nothing.
	if ids, err := Utility.GetProcessIdsByName("prometheus"); err == nil && len(ids) > 0 {
		slog.Info("prometheus already running; skipping start")
		return nil
	}

	// Expose /metrics.
	http.Handle("/metrics", promhttp.Handler())

	domain, _ := config.GetAddress()
	domain = strings.Split(domain, ":")[0]

	// Prepare data/config files.
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

	// Register metrics.
	servicesCpuUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "globular_services_cpu_usage_counter", Help: "Monitor the CPU usage of each service."}, []string{"id", "name"})
	servicesMemoryUsage = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "globular_services_memory_usage_counter", Help: "Monitor the memory usage of each service."}, []string{"id", "name"})
	prometheus.MustRegister(servicesCpuUsage, servicesMemoryUsage)

	// Feed metrics periodically.
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

	// Best-effort Alertmanager and node_exporter.
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

//
// =====================================================================================
// Internals
// =====================================================================================
//

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

// Local event client helper (unchanged signature/behavior).
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
