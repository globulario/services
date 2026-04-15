package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"crypto/tls"
	"crypto/x509"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcInsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// serviceState tracks the last known state of a systemd service.
type serviceState struct {
	ActiveState    string // "active", "inactive", "failed", etc.
	SubState       string // "running", "dead", "exited", etc.
	MainPID        string // main process ID — changes on restart
	Result         string // "success", "exit-code", "signal", etc.
	ExecMainStatus string // exit code (e.g., "203" = binary not found)
	NRestarts      string // systemd restart counter — increments on auto-restart after failure, NOT on manual restart
}

// eventPublisher monitors systemd unit states and publishes changes
// to the event service so the ai_watcher can react.
type eventPublisher struct {
	mu             sync.Mutex
	lastStates     map[string]serviceState
	badStatePublished map[string]time.Time // cooldown for crash-loop/failed re-publishing
	conn           *grpc.ClientConn
	client         eventpb.EventServiceClient
	nodeID         string
}

const badStateCooldown = 60 * time.Second // re-publish crash-loop/failed at most once per minute

func newEventPublisher(nodeID string) *eventPublisher {
	return &eventPublisher{
		lastStates:     make(map[string]serviceState),
		badStatePublished: make(map[string]time.Time),
		nodeID:         nodeID,
	}
}

// connect establishes a gRPC connection to the event service.
// On Day 1 nodes the event service is on the control-plane node, so we
// route through the gateway (same host as controller, port 443).
func (ep *eventPublisher) connect() error {
	// Resolve event service from etcd — source of truth for address and port.
	rawAddr := config.ResolveLocalServiceAddr("event.EventService")
	dt := config.ResolveDialTarget(rawAddr)

	// mTLS: client cert for authentication + CA for server verification.
	var creds grpc.DialOption
	certFile := config.GetLocalServerCertificatePath()
	keyFile := config.GetLocalServerKeyPath()
	caPath := config.GetCACertificatePath()
	if certFile != "" && keyFile != "" && caPath != "" {
		cert, certErr := tls.LoadX509KeyPair(certFile, keyFile)
		caData, caErr := os.ReadFile(caPath)
		if certErr == nil && caErr == nil {
			pool := x509.NewCertPool()
			pool.AppendCertsFromPEM(caData)
			creds = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{cert},
				ServerName:   dt.ServerName,
				RootCAs:      pool,
				MinVersion:   tls.VersionTLS12,
			}))
		} else {
			creds = grpc.WithTransportCredentials(grpcInsecure.NewCredentials())
		}
	} else {
		creds = grpc.WithTransportCredentials(grpcInsecure.NewCredentials())
	}

	// Load node token for authentication metadata.
	mac, _ := config.GetMacAddress()
	token, _ := security.GetLocalToken(mac)

	dialOpts := []grpc.DialOption{creds}
	if token != "" {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(
			func(ctx context.Context, method string, req, reply interface{},
				cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				md, _ := metadata.FromOutgoingContext(ctx)
				if md == nil {
					md = metadata.New(nil)
				} else {
					md = md.Copy()
				}
				md.Set("token", token)
				return invoker(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, opts...)
			}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, dt.Address, dialOpts...)
	if err != nil {
		return err
	}
	ep.conn = conn
	ep.client = eventpb.NewEventServiceClient(conn)
	return nil
}

// run polls systemd unit states every 5 seconds and publishes events on changes.
func (ep *eventPublisher) run(ctx context.Context) {
	// Brief stabilization delay — long enough for systemd to start units,
	// short enough to catch early failures.
	time.Sleep(10 * time.Second)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ep.checkAndPublish(ctx)
		}
	}
}

// checkAndPublish checks all globular systemd units and publishes events for state changes.
func (ep *eventPublisher) checkAndPublish(ctx context.Context) {
	if ep.client == nil {
		if err := ep.connect(); err != nil {
			return // Event service not available yet.
		}
		log.Printf("event-publisher: connected to event service")
	}

	units := listGlobularUnits(ctx)
	if len(units) == 0 {
		return
	}

	ep.mu.Lock()
	defer ep.mu.Unlock()

	// Clean up units that no longer exist (uninstalled).
	activeUnits := make(map[string]bool, len(units))
	for _, u := range units {
		activeUnits[u.Name] = true
	}
	for name := range ep.lastStates {
		if !activeUnits[name] {
			delete(ep.lastStates, name)
			delete(ep.badStatePublished, name)
		}
	}

	for _, u := range units {
		// Skip units whose file was removed (systemd ghost after reset-failed).
		if u.ActiveState == "inactive" && u.SubState == "dead" && u.Result == "" {
			continue
		}

		prev, existed := ep.lastStates[u.Name]
		current := serviceState{
			ActiveState: u.ActiveState, SubState: u.SubState, MainPID: u.MainPID,
			Result: u.Result, ExecMainStatus: u.ExecMainStatus, NRestarts: u.NRestarts,
		}

		// Crash-loops sit in activating/auto-restart permanently — publish
		// on first sight and re-publish after a cooldown.
		isCrashLoop := u.ActiveState == "activating" && u.SubState == "auto-restart"
		isFailed := u.ActiveState == "failed"
		isBadState := isCrashLoop || isFailed

		if existed && prev == current {
			if !isBadState {
				continue // No change and healthy — skip.
			}
			// Bad state unchanged — only re-publish after cooldown.
			if last, ok := ep.badStatePublished[u.Name]; ok && time.Since(last) < badStateCooldown {
				continue
			}
		}

		ep.lastStates[u.Name] = current

		if !existed && !isBadState {
			continue // First observation of a healthy state — don't publish, just record.
		}

		svcName := strings.TrimPrefix(u.Name, "globular-")
		svcName = strings.TrimSuffix(svcName, ".service")

		// ── Service State Machine ─────────────────────────────────────
		// Event semantics (each is a FACT, not a guess):
		//
		//   service.exited   — unit crashed or failed (non-zero exit, signal, crash-loop)
		//                      Triggers awareness loop: watcher → executor → remediation.
		//   service.failed   — unit is in systemd "failed" state (terminal failure).
		//   service.stopped  — unit cleanly stopped (active → inactive, result=success).
		//                      Informational — NOT a crash, should NOT trigger remediation.
		//   service.started  — unit became active from a non-active state.
		//   service.state_changed — any other state transition (informational).
		//
		// The controller ALSO detects active→non-active via 30s heartbeat and
		// emits service.exited. The two paths are complementary:
		// - Node agent publisher: fast (5s poll), local detection
		// - Controller heartbeat: authoritative, cross-node visibility

		pidChanged := prev.MainPID != "" && u.MainPID != "" && prev.MainPID != u.MainPID
		crashRestart := pidChanged && u.ActiveState == "active" && prev.ActiveState == "active"

		var eventName string
		switch {
		case crashRestart:
			// PID changed while unit stayed active — systemd restarted it.
			// Use NRestarts to distinguish crashes from clean restarts:
			//   NRestarts > prev.NRestarts → systemd auto-restarted after failure (crash)
			//   NRestarts unchanged or 0  → manual restart (systemctl restart, stop+start)
			// NRestarts only increments on Restart=on-failure/always auto-recovery,
			// NOT on manual systemctl restart.
			if u.NRestarts != prev.NRestarts && u.NRestarts != "0" {
				eventName = "service.exited" // Crash recovery.
			} else {
				eventName = "service.started" // Clean restart — not a crash.
			}

		case u.ActiveState == "failed":
			// Terminal failure state. Distinct from exited because the unit
			// won't recover without manual intervention or Restart=on-failure.
			eventName = "service.failed"

		case u.ActiveState == "activating" && u.SubState == "auto-restart":
			// Crash-loop: systemd is auto-restarting after failure. Polls may
			// never catch the brief "failed" state — treat auto-restart as crash.
			eventName = "service.exited"

		case u.ActiveState == "inactive" && prev.ActiveState == "active":
			// Transition from running to stopped. Check the Result field to
			// distinguish clean stops from unexpected exits.
			if u.Result == "success" || u.Result == "" {
				eventName = "service.stopped" // Clean stop — no remediation needed.
			} else {
				eventName = "service.exited" // Non-zero exit — treat as crash.
			}

		case u.ActiveState == "active" && u.SubState == "running" && prev.ActiveState != "active":
			eventName = "service.started"

		case u.ActiveState == "activating":
			continue // Transient startup state, skip.

		default:
			eventName = "service.state_changed"
		}

		payload := map[string]string{
			"unit":          u.Name,
			"service":       svcName,
			"node_id":       ep.nodeID,
			"active_state":  u.ActiveState,
			"sub_state":     u.SubState,
			"prev_active":   prev.ActiveState,
			"prev_sub":      prev.SubState,
			"main_pid":      u.MainPID,
			"prev_pid":      prev.MainPID,
			"result":        u.Result,
			"exit_status":   u.ExecMainStatus,
			"exit_reason":   exitReason(u.ExecMainStatus),
		}
		data, _ := json.Marshal(payload)

		_, err := ep.client.Publish(ctx, &eventpb.PublishRequest{
			Evt: &eventpb.Event{
				Name: eventName,
				Data: data,
			},
		})
		if err != nil {
			log.Printf("event-publisher: publish %s failed: %v (will reconnect)", eventName, err)
			ep.conn.Close()
			ep.client = nil // Reconnect on next tick.
			ep.conn = nil
			break // Exit unit loop, not run(). Next tick will reconnect.
		}

		if isBadState {
			ep.badStatePublished[u.Name] = time.Now()
		} else {
			delete(ep.badStatePublished, u.Name) // Recovered — clear cooldown.
		}

		log.Printf("event-publisher: %s unit=%s (%s/%s → %s/%s)",
			eventName, u.Name, prev.ActiveState, prev.SubState, u.ActiveState, u.SubState)
	}
}

// unitInfo holds parsed systemd unit state.
type unitInfo struct {
	Name           string
	ActiveState    string
	SubState       string
	MainPID        string
	Result         string // "success", "exit-code", "signal", etc.
	ExecMainStatus string // numeric exit code: "0", "203", etc.
	NRestarts      string // systemd auto-restart counter
}

// listGlobularUnits queries systemd for all globular-* service units.
func listGlobularUnits(ctx context.Context) []unitInfo {
	// Step 1: Discover all globular-* units including stopped/inactive ones.
	// "systemctl show globular-*.service" only returns active units because
	// the glob is expanded by systemd against running units. We need --all
	// to also see inactive/dead units (e.g., after a clean stop).
	listOut, err := exec.CommandContext(ctx, "systemctl", "list-units",
		"--all", "--no-legend", "--no-pager", "--plain",
		"--type=service", "globular-*").Output()
	if err != nil {
		return nil
	}

	// Parse unit names from list-units output (first column).
	var unitNames []string
	for _, line := range strings.Split(string(listOut), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasPrefix(fields[0], "globular-") {
			unitNames = append(unitNames, fields[0])
		}
	}
	if len(unitNames) == 0 {
		return nil
	}

	// Step 2: Query properties for all discovered units by name (not glob).
	args := append([]string{"show",
		"--property=Id,ActiveState,SubState,MainPID,Result,ExecMainStatus,NRestarts",
		"--no-pager"}, unitNames...)
	out, err := exec.CommandContext(ctx, "systemctl", args...).Output()
	if err != nil {
		return nil
	}

	var units []unitInfo
	var current unitInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			// Empty line separates units.
			if current.Name != "" {
				units = append(units, current)
			}
			current = unitInfo{}
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k {
		case "Id":
			if strings.HasPrefix(v, "globular-") {
				current.Name = v
			}
		case "ActiveState":
			current.ActiveState = v
		case "SubState":
			current.SubState = v
		case "MainPID":
			current.MainPID = v
		case "Result":
			current.Result = v
		case "ExecMainStatus":
			current.ExecMainStatus = v
		case "NRestarts":
			current.NRestarts = v
		}
	}
	// Don't forget the last unit.
	if current.Name != "" {
		units = append(units, current)
	}
	return units
}

// exitReason maps systemd ExecMainStatus codes to human-readable reasons.
func exitReason(code string) string {
	switch code {
	case "0":
		return "clean exit"
	case "1":
		return "general error"
	case "2":
		return "misuse of shell command"
	case "126":
		return "command not executable (permission denied)"
	case "127":
		return "command not found"
	case "200":
		return "working directory missing (CHDIR)"
	case "203":
		return "binary not found (EXEC) — check if the executable is installed"
	case "217":
		return "user not found — check User= in systemd unit"
	case "226":
		return "namespace setup failed"
	default:
		if code != "" {
			return "exit code " + code
		}
		return ""
	}
}
