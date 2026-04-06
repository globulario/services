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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcInsecure "google.golang.org/grpc/credentials/insecure"
)

// serviceState tracks the last known state of a systemd service.
type serviceState struct {
	ActiveState    string // "active", "inactive", "failed", etc.
	SubState       string // "running", "dead", "exited", etc.
	MainPID        string // main process ID — changes on restart
	Result         string // "success", "exit-code", "signal", etc.
	ExecMainStatus string // exit code (e.g., "203" = binary not found)
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
	rawAddr := discoverServiceAddr(10010) // event.EventService default port
	dt := config.ResolveDialTarget(rawAddr)

	// Try TLS first (production), fall back to insecure (development).
	var creds grpc.DialOption
	caPath := config.GetCACertificatePath()
	if caData, err := os.ReadFile(caPath); err == nil {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caData)
		creds = grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			ServerName: dt.ServerName,
			RootCAs:    pool,
			MinVersion: tls.VersionTLS12,
		}))
	} else {
		creds = grpc.WithTransportCredentials(grpcInsecure.NewCredentials())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, dt.Address, creds)
	if err != nil {
		return err
	}
	ep.conn = conn
	ep.client = eventpb.NewEventServiceClient(conn)
	return nil
}

// run polls systemd unit states every 10 seconds and publishes events on changes.
func (ep *eventPublisher) run(ctx context.Context) {
	// Wait for services to stabilize before starting.
	time.Sleep(30 * time.Second)

	ticker := time.NewTicker(10 * time.Second)
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
			Result: u.Result, ExecMainStatus: u.ExecMainStatus,
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

		// Detect crash-loop: PID changed while state stayed active/running.
		pidChanged := prev.MainPID != "" && u.MainPID != "" && prev.MainPID != u.MainPID
		crashRestart := pidChanged && u.ActiveState == "active" && prev.ActiveState == "active"

		var eventName string
		switch {
		case crashRestart:
			eventName = "service.exited"
		case u.ActiveState == "failed":
			eventName = "service.exited"
		case u.ActiveState == "activating" && u.SubState == "auto-restart":
			// Crash-loop: systemd is auto-restarting after failure.
			// The unit flips so fast between failed→activating that polls
			// may never see "failed" — treat auto-restart as a crash signal.
			eventName = "service.exited"
		case u.ActiveState == "inactive" && prev.ActiveState == "active":
			eventName = "service.stopped"
		case u.ActiveState == "active" && u.SubState == "running" && prev.ActiveState != "active":
			eventName = "service.started"
		case u.ActiveState == "activating":
			continue // Transient state (starting up normally), skip.
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
			log.Printf("event-publisher: publish %s failed: %v", eventName, err)
			ep.conn.Close()
			ep.client = nil // Reconnect on next cycle.
			ep.conn = nil
			return
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
}

// listGlobularUnits queries systemd for all globular-* service units.
func listGlobularUnits(ctx context.Context) []unitInfo {
	out, err := exec.CommandContext(ctx, "systemctl", "show",
		"--property=Id,ActiveState,SubState,MainPID,Result,ExecMainStatus",
		"--no-pager", "globular-*.service").Output()
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
