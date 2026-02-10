package process

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
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
	"os/user"
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
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/struCoder/pidusage"
	"go.etcd.io/etcd/client/v3/concurrency"
	"gopkg.in/yaml.v3"
)

func runtimeTLSPaths() (tlsDir, fullchain, privkey, ca string) {
	return config.CanonicalTLSPaths(config.GetRuntimeConfigDir())
}

//
// =====================================================================================
// Prometheus metrics
// =====================================================================================
//

var (
	servicesCpuUsage    *prometheus.GaugeVec
	servicesMemoryUsage *prometheus.GaugeVec
)

// Ring buffer for process output
type ring struct {
	mu   sync.Mutex
	buf  []byte
	size int
}

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

// WaitForEtcdHTTPS blocks until etcd is healthy over HTTPS (or times out).
func WaitForEtcdHTTPS(timeout time.Duration) error {
	tlsDir, _, _, ca := runtimeTLSPaths()
	caFile := ca
	if alt := filepath.Join(tlsDir, "ca.crt"); Utility.Exists(alt) { // legacy name fallback
		caFile = alt
	}
	cl := filepath.Join(tlsDir, "client.crt")
	key := filepath.Join(tlsDir, "client.pem")

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if alreadyHealthy("https", caFile, cl, key) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("etcd did not become healthy over https within %s", timeout)
}

// RestartAllServicesForEtcdTLS gracefully flips every service after etcd switches to TLS.
// Order: stop proxies → stop service → start service → (re)start proxy if TLS requested.
func RestartAllServicesForEtcdTLS() error {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return err
	}
	var firstErr error
	for _, s := range services {
		// Stop proxy first to avoid flapping logs.
		_ = KillServiceProxyProcess(s)

		// Capture the desired port (don’t reassign ports on restart).
		port := Utility.ToInt(s["Port"])
		if port <= 0 {
			// fall back to configured port or skip
			continue
		}

		// If this service is exposed through grpcwebproxy with TLS, relaunch its proxy too.
		if Utility.ToBool(s["TLS"]) {
			if _, err := StartServiceProxyProcess(s, "ca.crt", "server.crt"); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
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

func newRing(n int) *ring { return &ring{size: n, buf: make([]byte, 0, n)} }
func (r *ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(p) >= r.size {
		r.buf = append([]byte{}, p[len(p)-r.size:]...)
		return len(p), nil
	}
	if len(r.buf)+len(p) > r.size {
		// drop oldest
		over := len(r.buf) + len(p) - r.size
		r.buf = r.buf[over:]
	}
	r.buf = append(r.buf, p...)
	return len(p), nil
}
func (r *ring) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte{}, r.buf...)
}
func tailLines(b []byte, n int) string {
	if n <= 0 {
		return string(b)
	}
	// find last n newlines
	cnt := 0
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] == '\n' {
			cnt++
			if cnt == n {
				return string(b[i+1:])
			}
		}
	}
	return string(b)
}

// ---- your function, improved ----
func startServiceProcessWithWritersInternal(
	s map[string]interface{},
	port int,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	seen map[string]struct{},
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

	syncFromRuntime(s)

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

	if err := KillServiceProcess(s); err != nil {
		return -1, err
	}

	// Allocate service and proxy ports, avoiding collisions with other services.
	const defaultPortStart = 10000
	const defaultPortEnd = 20000
	desiredPort := port
	if desiredPort == 0 {
		desiredPort = Utility.ToInt(s["Port"])
	}
	if desiredPort == 0 {
		desiredPort = defaultPortStart
	}
	chosenPort, err := findFreePort("0.0.0.0", defaultPortStart, defaultPortEnd, desiredPort)
	if err != nil {
		return -1, fmt.Errorf("allocate service port: %w", err)
	}
	desiredProxy := Utility.ToInt(s["Proxy"])
	if desiredProxy == 0 {
		desiredProxy = chosenPort + 1
	}
	chosenProxy, err := findFreePort("0.0.0.0", defaultPortStart, defaultPortEnd, desiredProxy)
	if err != nil {
		// Log and fall back to the desired proxy (may collide).
		slog.Warn("proxy port allocation failed; using desired value", "err", err, "desired", desiredProxy)
		chosenProxy = desiredProxy
	}
	// Avoid choosing the same port for service and proxy.
	if chosenProxy == chosenPort {
		if next, err := findFreePort("0.0.0.0", defaultPortStart, defaultPortEnd, chosenProxy+1); err == nil {
			chosenProxy = next
		}
	}
	if chosenPort != desiredPort {
		slog.Info("service port adjusted due to conflict", "id", id, "name", name, "from", desiredPort, "to", chosenPort)
	}
	if chosenProxy != desiredProxy {
		slog.Info("proxy port adjusted due to conflict", "id", id, "name", name, "from", desiredProxy, "to", chosenProxy)
	}

	s["Port"] = chosenPort
	s["Proxy"] = chosenProxy
	s["Process"] = -1
	if err := config.SaveServiceConfiguration(s); err != nil {
		slog.Warn("failed to persist desired service config before start", "id", s["Id"], "name", s["Name"], "err", err)
	}

	_ = config.PutRuntime(id, map[string]interface{}{
		"Process":   -1,
		"State":     "starting",
		"LastError": "",
	})

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

	cmd := exec.Command(path, id)
	cmd.Dir = filepath.Dir(path)
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
	}

	// Encourage rich panic info & (optional) core dumps from Go services.
	// (Service can override; these just default.)
	env := os.Environ()
	if os.Getenv("GOTRACEBACK") == "" {
		env = append(env, "GOTRACEBACK=all")
	}
	// Uncomment to force SIGABRT on panic (generates core if ulimit allows)
	// env = append(env, "GODEBUG=panicabort=1")
	cmd.Env = env

	stdoutR, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderrR, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}

	// keep last 64KB of each stream for post-mortem
	stdoutTail := newRing(64 << 10)
	stderrTail := newRing(64 << 10)

	// Prefix lines with timestamp to aid correlation
	tsWriter := func(w io.Writer) io.Writer {
		pr, pw := io.Pipe()
		go func() {
			sc := bufio.NewScanner(pr)
			// allow long lines
			buf := make([]byte, 64<<10)
			sc.Buffer(buf, 1<<20)
			for sc.Scan() {
				line := sc.Bytes()
				fmt.Fprintf(w, "%s %s\n", time.Now().Format(time.RFC3339Nano), string(line))
			}
			_ = pr.Close()
		}()
		return pw
	}

	doneStdout := make(chan struct{})
	go func() {
		defer close(doneStdout)
		dst := io.MultiWriter(tsWriter(stdoutWriter), stdoutTail)
		_, _ = io.Copy(dst, stdoutR)
	}()

	var startErrBuf bytes.Buffer // stderr collected before Start() errors
	doneStderr := make(chan struct{})
	go func() {
		defer close(doneStderr)
		tee := io.TeeReader(stderrR, &startErrBuf) // mostly useful if Start() fails
		dst := io.MultiWriter(tsWriter(stderrWriter), stderrTail)
		_, _ = io.Copy(dst, tee)
	}()

	startAt := time.Now()
	if err := cmd.Start(); err != nil {
		_ = stdoutR.Close()
		_ = stderrR.Close()
		<-doneStdout
		<-doneStderr
		slog.Error("failed to start service",
			"name", name, "id", id, "err", err,
			"stderr_head", strings.TrimSpace(startErrBuf.String()),
			"path", path, "cwd", cmd.Dir)
		_ = config.PutRuntime(id, map[string]interface{}{
			"Process":   -1,
			"State":     "failed",
			"LastError": err.Error(),
		})
		return -1, err
	}

	pid := cmd.Process.Pid
	slog.Info("service process started", "name", name, "id", id, "pid", pid, "port", port, "proxy", port+1, "cwd", cmd.Dir, "path", path)

	_ = config.PutRuntime(id, map[string]interface{}{
		"Process":   pid,
		"State":     "running",
		"LastError": "",
	})

	if _, err := config.StartLive(id, 15); err != nil {
		slog.Warn("failed to start etcd live lease", "id", id, "err", err)
	}

	go func() {
		err := cmd.Wait()
		_ = stdoutR.Close()
		_ = stderrR.Close()
		<-doneStdout
		<-doneStderr

		elapsed := time.Since(startAt)

		exitCode := 0
		var sigStr string
		if ps := cmd.ProcessState; ps != nil {
			if ws, ok := ps.Sys().(syscall.WaitStatus); ok {
				exitCode = ws.ExitStatus()
				if ws.Signaled() {
					sigStr = ws.Signal().String()
				}
			}
		}
		// Compose a richer error message
		if err != nil {
			// Tail last 200 lines to keep logs tidy
			stdoutTailStr := tailLines(stdoutTail.Bytes(), 200)
			stderrTailStr := tailLines(stderrTail.Bytes(), 200)
			slog.Error("service process exited with error",
				"name", name, "id", id, "pid", pid,
				"exit_code", exitCode, "signal", sigStr,
				"elapsed", elapsed.String(),
				"path", path, "cwd", cmd.Dir,
				"err", err,
				"stderr_tail", strings.TrimSpace(stderrTailStr),
				"stdout_tail", strings.TrimSpace(stdoutTailStr),
			)
		} else {
			slog.Info("service process stopped",
				"name", name, "id", id, "pid", pid, "exit_code", exitCode, "elapsed", elapsed.String())
		}

		_ = config.PutRuntime(id, map[string]interface{}{
			"Process": -1,
			"State": func() string {
				if err != nil {
					return "failed"
				}
				return "stopped"
			}(),
			"LastError": func() string {
				if err != nil {
					if sigStr != "" {
						return fmt.Sprintf("%v (exit=%d, signal=%s)", err, exitCode, sigStr)
					}
					return fmt.Sprintf("%v (exit=%d)", err, exitCode)
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

// add near the top of the file
// helper at top of file (or near other small helpers)
func isStoppingState(state string) bool {
	s := strings.ToLower(strings.TrimSpace(state))
	return s == "closing" || s == "closed" || s == "stopping" || s == "stopped" || s == "failed"
}

// RestartAllServiceProxiesTLS restarts grpcwebproxy for all services that declare TLS=true.
// It is best-effort and safe to call at any time; services with KeepAlive will also
// recover on their own, but this shortens the window.
func RestartAllServiceProxiesTLS() error {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return err
	}

	// Ensure creds dir exists.
	tlsDir, fullchain, _, ca := runtimeTLSPaths()
	credsDir := tlsDir
	if !Utility.Exists(credsDir) {
		return fmt.Errorf("cannot restart proxies: creds dir does not exist: %s", credsDir)
	}

	// Choose the certs used by your proxies:
	// - For the proxy's own TLS server: use the local server keypair (server.key/.crt) and trust bundle.
	// - For backend mTLS: we already pass backend CA/client in StartServiceProxyProcess.
	caBundle := ca
	if alt := filepath.Join(credsDir, "ca.crt"); Utility.Exists(alt) {
		caBundle = alt
	}
	serverCert := fullchain
	if alt := filepath.Join(credsDir, "server.crt"); Utility.Exists(alt) {
		serverCert = alt
	}

	var firstErr error
	for _, s := range services {
		// Only touch proxies for services that want TLS.
		if !Utility.ToBool(s["TLS"]) {
			continue
		}

		// Stop proxy (ignore errors), then start it again with TLS.
		_ = KillServiceProxyProcess(s)
		if _, err := StartServiceProxyProcess(s, caBundle, serverCert); err != nil {
			// Record the first error but keep going so others can restart.
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

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

	// if Envoy is running, do not start grpcwebproxy
	if _, err := Utility.GetProcessIdsByName("envoy"); err == nil {
		slog.Info("Envoy detected; skipping grpcwebproxy start", "service", s["Name"], "id", s["Id"])
		return -1, nil
	}

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

	tlsDir, fullchain, privkey, ca := runtimeTLSPaths()
	credsDir := tlsDir
	proxyPort := Utility.ToInt(s["Proxy"])
	tlsEnabled := Utility.ToBool(s["TLS"])
	caTrust := ca
	if v := strings.TrimSpace(certificateAuthorityBundle); v != "" {
		if filepath.IsAbs(v) {
			caTrust = v
		} else if candidate := filepath.Join(credsDir, v); Utility.Exists(candidate) {
			caTrust = candidate
		} else {
			caTrust = filepath.Join(credsDir, v)
		}
	}
	serverCert := fullchain
	if v := strings.TrimSpace(certificate); v != "" {
		if filepath.IsAbs(v) {
			serverCert = v
		} else if candidate := filepath.Join(credsDir, v); Utility.Exists(candidate) {
			serverCert = candidate
		} else {
			serverCert = filepath.Join(credsDir, v)
		}
	}
	serverKey := privkey
	if alt := filepath.Join(credsDir, "server.key"); Utility.Exists(alt) {
		serverKey = alt
	} else if alt := filepath.Join(credsDir, "server.pem"); Utility.Exists(alt) {
		serverKey = alt
	}

	args := []string{
		"--backend_addr=" + backend,
		"--allow_all_origins=true",
		"--use_websockets=false",
		"--server_http_max_read_timeout=48h",
		"--server_http_max_write_timeout=48h",
	}
	if tlsEnabled {
		args = append(args,
			"--backend_tls=true",
			"--backend_tls_ca_files="+caTrust,
			"--backend_client_tls_cert_file="+filepath.Join(credsDir, "client.crt"),
			"--backend_client_tls_key_file="+filepath.Join(credsDir, "client.pem"),
			"--run_http_server=false",
			"--run_tls_server=true",
			"--server_http_tls_port="+strconv.Itoa(proxyPort),
			"--server_tls_key_file="+serverKey,
			"--server_tls_client_ca_files="+caTrust,
			"--server_tls_cert_file="+serverCert,
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

	// Helper that spawns the proxy and performs the initial reachability check.
	type started struct {
		cmd     *exec.Cmd
		pid     int
		readyCh chan error
	}
	startOnce := func() (*started, error) {
		proxy := exec.Command(cmdPath, args...)
		proxy.Dir = cmdDir
		if runtime.GOOS != "windows" {
			proxy.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
		}
		proxy.Stdout = os.Stdout
		proxy.Stderr = os.Stderr

		if err := proxy.Start(); err != nil {
			return nil, err
		}
		pid := proxy.Process.Pid

		slog.Info("grpcwebproxy started (verifying reachability)", "service", name, "id", id, "backend", backend, "port", proxyPort, "tls", tlsEnabled, "pid", pid, "path", cmdPath)

		// Update runtime immediately with the PID (we’ll clear it if startup fails).
		_ = config.PutRuntime(id, map[string]interface{}{
			"ProxyProcess": pid,
			"State":        "running",
		})

		ready := make(chan error, 1)
		go func() { ready <- proxy.Wait() }()

		// Wait up to ~2s for the port to open; if the process already died, surface that error.
		checkDeadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(checkDeadline) {
			select {
			case err := <-ready:
				_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
				if err != nil {
					slog.Error("grpcwebproxy terminated during startup", "service", name, "id", id, "pid", pid, "err", err)
					return nil, fmt.Errorf("grpcwebproxy failed to start: %w", err)
				}
				slog.Info("grpcwebproxy exited immediately without error (unexpected)", "service", name, "id", id, "pid", pid)
				return nil, errors.New("grpcwebproxy exited immediately")
			default:
				hp4 := net.JoinHostPort("127.0.0.1", strconv.Itoa(proxyPort))
				hp6 := net.JoinHostPort("::1", strconv.Itoa(proxyPort))
				if waitTcpOpen(hp4, 150*time.Millisecond) || waitTcpOpen(hp6, 150*time.Millisecond) {
					return &started{cmd: proxy, pid: pid, readyCh: ready}, nil
				}
				time.Sleep(100 * time.Millisecond)
			}
		}

		// Not reachable; assume bad start.
		select {
		case err := <-ready:
			_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
			if err != nil {
				slog.Error("grpcwebproxy terminated during startup", "service", name, "id", id, "pid", pid, "err", err)
				return nil, fmt.Errorf("grpcwebproxy failed to start: %w", err)
			}
		default:
		}
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = proxy.Process.Signal(syscall.SIGTERM)
		time.Sleep(200 * time.Millisecond)
		_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
		return nil, fmt.Errorf("grpcwebproxy did not become reachable on port %d", proxyPort)
	}

	// First start
	st, err := startOnce()
	if err != nil {
		return -1, err
	}

	// -------- Supervise + auto-restart (KeepAlive + backend still running) --------
	keepAlive := Utility.ToBool(s["KeepAlive"])
	go func() {
		backoff := 500 * time.Millisecond
		const maxBackoff = 30 * time.Second

		cur := st
		for {
			err := <-cur.readyCh // wait for the current proxy to exit
			if err != nil {
				slog.Error("grpcwebproxy terminated with error", "service", name, "id", id, "pid", cur.pid, "err", err)
			} else {
				slog.Info("grpcwebproxy stopped", "service", name, "id", id, "pid", cur.pid)
			}
			_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})

			// Refresh runtime/desired for every decision
			syncFromRuntime(s)

			// honor KeepAlive flips too
			keepAlive = Utility.ToBool(s["KeepAlive"])

			// current backend state & liveness
			servicePid = Utility.ToInt(s["Process"])
			state := strings.ToLower(Utility.ToString(s["State"]))
			backendAlive, _ := Utility.PidExists(servicePid)

			// Hard stops: do not restart if KeepAlive=false OR service is stopping-ish OR backend is down
			if !keepAlive || isStoppingState(state) || servicePid <= 0 || !backendAlive {
				slog.Info("not restarting proxy: backend not healthy or service stopping",
					"service", name, "backend_pid", servicePid, "state", state)
				return
			}

			// Backend is healthy and service still running/starting → restart with backoff.
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}

			// Try to take the per-service lock again (avoid racing with external supervisors)
			if cli, err := config.GetEtcdClient(); err == nil && cli != nil {
				if sess, e := concurrency.NewSession(cli, concurrency.WithTTL(8)); e == nil {
					m := concurrency.NewMutex(sess, "/globular/locks/proxy/"+id)
					ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
					if e := m.TryLock(ctx); e == nil {
						cancel()
						ns, e2 := startOnce()
						_ = m.Unlock(context.Background())
						_ = sess.Close()
						if e2 != nil {
							slog.Warn("proxy restart failed; will retry", "service", name, "err", e2)
							continue
						}
						backoff = 500 * time.Millisecond
						cur = ns
						continue
					}
					cancel()
					_ = sess.Close()
				}
			}

			// Best-effort local start if lock not acquired
			ns, e2 := startOnce()
			if e2 != nil {
				slog.Warn("proxy restart failed (no lock); will retry", "service", name, "err", e2)
				continue
			}
			backoff = 500 * time.Millisecond
			cur = ns
		}
	}()

	return st.pid, nil
}

/*
KillServiceProxyProcess terminates grpcwebproxy for a service if present and updates etcd runtime.
*/
func KillServiceProxyProcess(s map[string]interface{}) error {
	// Reconcile from runtime to avoid stale PIDs.
	syncFromRuntime(s)

	id := Utility.ToString(s["Id"])
	pid := Utility.ToInt(s["ProxyProcess"])
	if s["State"] == nil {
		s["State"] = "unknown"
	}

	// Signal that we’re intentionally stopping the proxy to avoid auto-restart.
	curState := strings.ToLower(Utility.ToString(s["State"]))
	if curState == "" || curState == "running" || curState == "starting" {
		_ = config.PutRuntime(id, map[string]interface{}{"State": "closing"})
	}

	if pid <= 0 {
		// still ensure ProxyProcess is cleared
		_ = config.PutRuntime(id, map[string]interface{}{"ProxyProcess": -1})
		return nil
	}

	proc, _ := os.FindProcess(pid)
	if runtime.GOOS == "windows" {
		_ = proc.Kill()
	} else {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		_ = proc.Signal(syscall.SIGTERM)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := Utility.PidExists(pid); !ok {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	if ok, _ := Utility.PidExists(pid); ok {
		if runtime.GOOS == "windows" {
			_ = proc.Kill()
		} else {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			_ = proc.Kill()
		}
		esc := time.Now().Add(2 * time.Second)
		for time.Now().Before(esc) {
			if ok, _ := Utility.PidExists(pid); !ok {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Final runtime: clear proxy PID and set a non-up state
	syncFromRuntime(s) // refresh Process/State one last time
	state := strings.ToLower(Utility.ToString(s["State"]))
	if state == "" || state == "running" || state == "starting" {
		// If main service is still up but we intentionally killed the proxy,
		// keep "closing" so the restart loop won’t kick in.
		state = "closing"
	}
	if Utility.ToInt(s["Process"]) <= 0 {
		state = "closed"
	}

	return config.PutRuntime(id, map[string]interface{}{
		"ProxyProcess": -1,
		"State":        state,
	})
}

//
// -------------------------------------------------------------------------------------
// grpcwebproxy binary resolution
// -------------------------------------------------------------------------------------

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

// RestartEtcdIfCertsReady switches etcd to HTTPS if server.crt/server.pem/ca.crt exist.
// If etcd is already healthy on HTTPS, it does nothing. If it’s only on HTTP, it
// stops the current process and relaunches on TLS.
func RestartEtcdIfCertsReady() error {
	tlsDir, fullchain, privkey, ca := runtimeTLSPaths()
	certFile := fullchain
	if alt := filepath.Join(tlsDir, "server.crt"); Utility.Exists(alt) {
		certFile = alt
	}
	keyPem := filepath.Join(tlsDir, "server.pem")
	keyKey := filepath.Join(tlsDir, "server.key")
	keyFile := privkey
	if Utility.Exists(keyPem) {
		keyFile = keyPem
	} else if Utility.Exists(keyKey) {
		keyFile = keyKey
	}
	caFile := ca
	if alt := filepath.Join(tlsDir, "ca.crt"); Utility.Exists(alt) {
		caFile = alt
	}

	// Need cert + key(.pem or .key) + ca
	if !(Utility.Exists(certFile) && Utility.Exists(caFile) && Utility.Exists(keyFile)) {
		return nil
	}

	// If HTTPS is already healthy, nothing to do.
	if alreadyHealthy("https", caFile, filepath.Join(tlsDir, "client.crt"), filepath.Join(tlsDir, "client.pem")) {
		return nil
	}

	// Stop whatever is running (HTTP or stale HTTPS).
	_ = StopEtcdServer()

	// Wait for ports to be released.
	clientHP := net.JoinHostPort(config.GetLocalIP(), "2379")
	peerHP := net.JoinHostPort(config.GetLocalIP(), "2380")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !portInUse(clientHP) && !portInUse(peerHP) {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	// Start; StartEtcdServer() will prefer TLS when certs exist (see patch #3).
	return StartEtcdServer()
}

// ensureEtcdAndScyllaPerms makes etcd happy (0700 data dir) and lets scylla read TLS files.
// ensureEtcdAndScyllaPerms:
// - etcd data dir -> 0700
// - TLS dir -> 0750 (root:scylla so scylla can traverse)
// - PRIVATE keys   (*.key, client.pem, server.pem, anything with "key" in name & .pem) -> 0640 root:scylla
// - PUBLIC material (*.crt, *fullchain*.pem, *.csr, .conf, .txt, etc.) -> 0644 root:root
func ensureEtcdAndScyllaPerms(tlsDir, dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dataDir, 0o700); err != nil {
		return err
	}

	// ensure parents are traversable
	tlsRoot := filepath.Dir(tlsDir)
	_ = os.MkdirAll(tlsRoot, 0o755)
	_ = os.Chmod(tlsRoot, 0o755)

	// TLS dir: 0755 so public files are actually readable
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(tlsDir, 0o755); err != nil {
		return err
	}

	var scyllaGID = -1
	if grp, err := user.LookupGroup("scylla"); err == nil {
		if gid, e := strconv.Atoi(grp.Gid); e == nil {
			scyllaGID = gid
		}
	}
	if scyllaGID >= 0 {
		_ = os.Chown(tlsDir, 0, scyllaGID)
	}

	isPrivate := func(n string) bool {
		l := strings.ToLower(n)
		return strings.HasSuffix(l, ".key") ||
			l == "client.pem" || l == "server.pem" ||
			(strings.HasSuffix(l, ".pem") && strings.Contains(l, "key"))
	}
	isPublic := func(n string) bool {
		l := strings.ToLower(n)
		return strings.HasSuffix(l, ".crt") ||
			(strings.Contains(l, "fullchain") && strings.HasSuffix(l, ".pem")) ||
			strings.HasSuffix(l, ".csr") || strings.HasSuffix(l, ".conf") || strings.HasSuffix(l, ".txt") ||
			strings.HasSuffix(l, ".pem")
	}

	ents, err := os.ReadDir(tlsDir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if !e.Type().IsRegular() {
			continue
		}
		p := filepath.Join(tlsDir, e.Name())
		switch {
		case isPrivate(e.Name()):
			if scyllaGID >= 0 {
				_ = os.Chown(p, 0, scyllaGID)
				_ = os.Chmod(p, 0o640)
			} else {
				_ = os.Chown(p, 0, 0)
				_ = os.Chmod(p, 0o600)
			}
		case isPublic(e.Name()):
			_ = os.Chown(p, 0, 0)
			_ = os.Chmod(p, 0o644)
		default:
			_ = os.Chown(p, 0, 0)
			_ = os.Chmod(p, 0o644)
		}
	}
	return nil
}

// StartEtcdServer launches etcd using a generated config file. It:
//
//   - Builds listen/advertise URLs (DNS-first) and optional TLS sections.
//   - If Protocol=https but server certs are missing, it tries to generate
//     local CA + server/client certs using the existing `security` package.
//   - Skips start if a healthy etcd is already reachable at the advertised URL.
//   - Refuses to start if the listen ports are in use but the endpoint is unhealthy.
//   - Waits for readiness (HTTP /health or TCP).
//   - Handles SIGTERM for graceful shutdown.
//
// The generated config is written to <stateRoot>/etcd/etcd.yml and data lives in <stateRoot>/etcd.
func StartEtcdServer() error {
	const (
		readyTimeout = 60 * time.Second
		probeEvery   = 200 * time.Millisecond
		defaultDays  = 3650 // for local bootstrap certs
	)

	localConfig, err := config.GetLocalConfig(true)
	if err != nil {
		return err
	}

	protocol := strings.ToLower(Utility.ToString(localConfig["Protocol"]))
	if protocol == "" {
		protocol = "https"
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

	tlsDir, _, _, _ := runtimeTLSPaths()

	stateRoot := config.GetStateRootDir()
	dataDir := Utility.ToString(localConfig["EtcdDataDir"])
	if dataDir == "" {
		dataDir = filepath.Join(stateRoot, "etcd")
	}
	_ = Utility.CreateDirIfNotExist(dataDir)
	cfgPath := Utility.ToString(localConfig["EtcdConfigPath"])
	if cfgPath == "" {
		cfgPath = filepath.Join(dataDir, "etcd.yml")
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("create etcd config dir: %w", err)
	}

	// Set permissions for etcd data dir and TLS files (if any).
	if err := ensureEtcdAndScyllaPerms(tlsDir, dataDir); err != nil {
		return fmt.Errorf("perm bootstrap failed: %w", err)
	}

	// Seed from existing cfg if present.
	nodeCfg := make(map[string]interface{})
	if Utility.Exists(cfgPath) {
		if data, err := os.ReadFile(cfgPath); err == nil {
			_ = yaml.Unmarshal(data, &nodeCfg)
		}
	}

	// Ports / addresses
	const clientPort = "2379"
	const peerPort = "2380"
	listenAddress := config.GetLocalIP()
	clientHP := net.JoinHostPort(listenAddress, clientPort)
	peerHP := net.JoinHostPort(listenAddress, peerPort)

	// Base URLs (we’ll finalize scheme after TLS check)
	advertiseClientURLs := net.JoinHostPort(advHost, clientPort)
	initialAdvertisePeerURLs := net.JoinHostPort(advHost, peerPort)

	// --- TLS enablement (reusing existing security package) ---
	wantTLS := strings.EqualFold(protocol, "https")
	scheme := "http"
	var caFile, certFile, keyFile string
	var clientMTLS bool
	var peerMTLSDesired bool
	isSingleNode := true
	if peers, ok := localConfig["Peers"].([]interface{}); ok && len(peers) > 0 {
		isSingleNode = false
	}

	if wantTLS {
		resolveTLS := func() (string, string, string) {
			cert := filepath.Join(tlsDir, "fullchain.pem")
			if Utility.Exists(filepath.Join(tlsDir, "server.crt")) {
				cert = filepath.Join(tlsDir, "server.crt")
			}
			key := filepath.Join(tlsDir, "privkey.pem")
			if Utility.Exists(filepath.Join(tlsDir, "server.key")) {
				key = filepath.Join(tlsDir, "server.key")
			} else if Utility.Exists(filepath.Join(tlsDir, "server.pem")) {
				key = filepath.Join(tlsDir, "server.pem")
			}
			caPath := filepath.Join(tlsDir, "ca.pem")
			if Utility.Exists(filepath.Join(tlsDir, "ca.crt")) {
				caPath = filepath.Join(tlsDir, "ca.crt")
			}
			return cert, key, caPath
		}

		serverCRT, serverKEY, caCRT := resolveTLS()

		if !(Utility.Exists(serverCRT) && Utility.Exists(serverKEY) && Utility.Exists(caCRT)) {
			slog.Warn("etcd TLS requested but cert/key/CA missing; generating local certificates via security.GenerateServicesCertificates",
				"certDir", tlsDir)

			localIP := listenAddress
			var alts []interface{}
			seen := map[string]bool{}
			addAlt := func(s string) {
				s = strings.TrimSpace(s)
				if s == "" {
					return
				}
				k := strings.ToLower(s)
				if seen[k] {
					return
				}
				seen[k] = true
				alts = append(alts, s)
			}

			// concrete hostnames and IPs
			addAlt(advHost) // e.g. globule-ryzen.globular.io
			addAlt(name)    // e.g. globule-ryzen
			if domain != "" && !strings.Contains(name, ".") {
				addAlt(name + "." + domain) // e.g. globule-ryzen.globular.io (dup filtered)
			}
			addAlt("localhost")
			addAlt("127.0.0.1")
			addAlt(localIP) // e.g. 10.0.0.63 (or your current local IP)

			// merge AlternateDomains from config.json verbatim (keeps wildcards)
			if altsFromCfg, ok := localConfig["AlternateDomains"].([]interface{}); ok {
				for _, v := range altsFromCfg {
					addAlt(Utility.ToString(v)) // this will include "*.globular.io"
				}
			}

			country := Utility.ToString(localConfig["Country"])
			state := Utility.ToString(localConfig["State"])
			city := Utility.ToString(localConfig["City"])
			org := Utility.ToString(localConfig["Organization"])

			if err := security.GenerateServicesCertificates(
				"1111", defaultDays, advHost, tlsDir, country, state, city, org, alts,
			); err != nil {
				return fmt.Errorf("etcd TLS bootstrap failed (Protocol=https requires valid certificates): %w", err)
			}
			serverCRT, serverKEY, caCRT = resolveTLS()
		}

		if wantTLS && Utility.Exists(serverCRT) && Utility.Exists(serverKEY) && Utility.Exists(caCRT) {
			certFile, keyFile, caFile = serverCRT, serverKEY, caCRT
			scheme = "https"

			// client port: no mTLS during bootstrap
			clientMTLS = false

			// peer port: only require client certs if server cert has clientAuth EKU and it's multi-node
			peerHasClientAuth := func(certPath string) bool {
				b, e := os.ReadFile(certPath)
				if e != nil {
					return false
				}
				block, _ := pem.Decode(b)
				if block == nil {
					return false
				}
				c, e := x509.ParseCertificate(block.Bytes)
				if e != nil {
					return false
				}
				for _, eku := range c.ExtKeyUsage {
					if eku == x509.ExtKeyUsageClientAuth {
						return true
					}
				}
				return false
			}(serverCRT)
			peerMTLSDesired = peerHasClientAuth && !isSingleNode

			nodeCfg["client-transport-security"] = map[string]interface{}{
				"cert-file":        certFile,
				"key-file":         keyFile,
				"client-cert-auth": false, // keep simple during bootstrap
			}
			nodeCfg["peer-transport-security"] = map[string]interface{}{
				"cert-file":        certFile,
				"key-file":         keyFile,
				"client-cert-auth": peerMTLSDesired,
				"trusted-ca-file":  caFile,
			}
		} else {
			return fmt.Errorf("etcd TLS required (Protocol=https) but certificates not found: cert=%s key=%s ca=%s", serverCRT, serverKEY, caCRT)
		}
	}

	if !wantTLS {
		delete(nodeCfg, "client-transport-security")
		delete(nodeCfg, "peer-transport-security")
		scheme = "http"
	}

	// Finalize URLs with the scheme we’re actually using
	listenClientURLs := scheme + "://" + clientHP
	listenPeerURLs := scheme + "://" + peerHP
	advClientURL := scheme + "://" + advertiseClientURLs
	advPeerURL := scheme + "://" + initialAdvertisePeerURLs

	// ===================== Core node config (FIX) =====================
	nodeCfg["name"] = name
	nodeCfg["data-dir"] = dataDir

	if wantTLS {
		// Serve TLS on LAN *and* TLS on loopback for local tools
		nodeCfg["listen-client-urls"] = listenClientURLs + "," + ("https://127.0.0.1:" + clientPort)
	} else {
		// In insecure mode, keep plaintext loopback
		nodeCfg["listen-client-urls"] = "http://127.0.0.1:" + clientPort + "," + listenClientURLs
	}
	// ================================================================

	nodeCfg["listen-peer-urls"] = listenPeerURLs
	nodeCfg["advertise-client-urls"] = advClientURL
	nodeCfg["initial-advertise-peer-urls"] = advPeerURL

	// Initial cluster (use advertised DNS hosts).
	initialCluster := name + "=" + advPeerURL
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

	// Cluster state
	if Utility.Exists(filepath.Join(dataDir, "member")) {
		nodeCfg["initial-cluster-state"] = "existing"
	} else {
		nodeCfg["initial-cluster-state"] = "new"
	}

	// If etcd already healthy at advertised endpoint, do NOT start another.
	var clientCertFile, clientKeyFile string
	if wantTLS && clientMTLS {
		clientCertFile = filepath.Join(tlsDir, "client.crt")
		clientKeyFile = filepath.Join(tlsDir, "client.pem")
	}
	if alreadyHealthy(scheme, caFile, clientCertFile, clientKeyFile) {
		slog.Info("etcd already running and healthy; skipping start", "advertise-client-urls", advClientURL)
		return nil
	}

	// Refuse second server if ports busy and endpoint not healthy.
	if portInUse(peerHP) || portInUse(clientHP) {
		return fmt.Errorf("etcd ports already in use but endpoint not healthy (peer=%s, client=%s)", peerHP, clientHP)
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

	slog.Info("starting etcd", "config", cfgPath, "listen-client-urls", nodeCfg["listen-client-urls"], "advertise-client-urls", advClientURL)

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
		if !strings.Contains(u, "://") {
			if wantTLS {
				u = "https://" + u
			} else {
				u = "http://" + u
			}
		}
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

		uu, _ := url.Parse(u)
		hp := uu.Host
		if hp == "" {
			hp = raw
		}
		if !strings.Contains(hp, ":") {
			hp = net.JoinHostPort(hp, clientPort)
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
		if probeOne(advClientURL) {
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

func isTCPPortFree(host string, port int) bool {
	hp := net.JoinHostPort(host, strconv.Itoa(port))
	ln, err := net.Listen("tcp", hp)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// findFreePort returns the preferred port if available; otherwise it scans the range [start,end].
func findFreePort(host string, start, end, preferred int) (int, error) {
	if preferred >= start && preferred <= end && isTCPPortFree(host, preferred) {
		return preferred, nil
	}
	for p := start; p <= end; p++ {
		if isTCPPortFree(host, p) {
			return p, nil
		}
	}
	return 0, fmt.Errorf("no free port found in range %d-%d", start, end)
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
		// Ensure scheme
		if !strings.Contains(u, "://") {
			u = scheme + "://" + u
		}
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

	// If Envoy is already running, do nothing.
	if ids, err := Utility.GetProcessIdsByName("envoy"); err == nil && len(ids) > 0 {
		slog.Info("envoy already running; skipping start")
		return nil
	}

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
				" cert_file: " + config.GetLocalServerCertificatePath() + "\n" +
				" key_file: " + config.GetLocalServerKeyPath() + "\n"
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
