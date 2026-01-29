package serviceports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/ports"
)

// EnsureServicePortConfig normalizes (or creates) the runtime config for a service,
// guaranteeing the port is inside range, not reserved by another service config,
// and not currently in use. It is safe to call before install/start/restart.
func EnsureServicePortConfig(ctx context.Context, service, binDir string) error {
	exe := executableForService(service)
	if exe == "" {
		return nil
	}
	binPath := filepath.Join(binDir, exe)

	alloc, err := ports.NewFromEnv()
	if err != nil {
		return err
	}

	stateRoot := stateRoot()
	servicesDir := filepath.Join(stateRoot, "services")

	// Seed allocator with existing configs to avoid duplicate allocations for stopped services.
	seedReservations(alloc, servicesDir)

	desc, err := runDescribe(ctx, binPath)
	if err != nil {
		return err
	}
	if desc.Id == "" {
		return fmt.Errorf("describe %s returned empty Id", binPath)
	}

	cfgPath := filepath.Join(servicesDir, desc.Id+".json")

	cfg, _ := readServiceConfig(cfgPath)
	hasFile := cfg != nil
	if cfg == nil {
		cfg = desc
	}

	currentPort := firstPort(cfg.Port, portFromAddress(cfg.Address), desc.Port)
	start, end := alloc.Range()

	// Allocate (may return same port if in-range, free, and owned by same Id).
	newPort, err := alloc.Reserve(desc.Id, currentPort)
	if err != nil {
		return err
	}

	if currentPort == newPort && currentPort >= start && currentPort <= end && hasFile {
		// Already valid and exists; nothing to rewrite.
		return nil
	}

	cfg.Port = newPort
	cfg.Address = fmt.Sprintf("localhost:%d", newPort)
	cfg.Id = desc.Id

	if err := writeServiceConfig(cfgPath, cfg); err != nil {
		return err
	}

	fmt.Printf("INFO service %s port normalized %d->%d range=%d-%d config=%s\n", service, currentPort, newPort, start, end, cfgPath)
	return nil
}

// EnsureServicePortReady is used by start/restart preflight; it delegates to EnsureServicePortConfig
// and adds a live in-use check to heal if another process is listening.
func EnsureServicePortReady(ctx context.Context, service, unit string) error {
	binDir := installBinDir()
	if err := EnsureServicePortConfig(ctx, service, binDir); err != nil {
		return err
	}

	// After config normalization, re-open to ensure port not currently in use (non-globular).
	desc, err := runDescribe(ctx, filepath.Join(binDir, executableForService(service)))
	if err != nil || desc == nil || desc.Id == "" {
		if preflightStrict() {
			if err == nil {
				return fmt.Errorf("describe missing id for service %s", service)
			}
			return err
		}
		return nil // best-effort
	}
	state := stateRoot()
	cfgPath := filepath.Join(state, "services", desc.Id+".json")
	cfg, err := readServiceConfig(cfgPath)
	if err != nil {
		if preflightStrict() {
			return err
		}
		return nil // best-effort
	}

	if portFree(cfg.Port) {
		return nil
	}

	alloc, err := ports.NewFromEnv()
	if err != nil {
		return err
	}
	servicesDir := filepath.Join(stateRoot(), "services")
	seedReservationsExcept(alloc, servicesDir, desc.Id)

	start, end := alloc.Range()
	// Prevent allocator from handing back the known-in-use port.
	oldPort := cfg.Port
	alloc.Mark("in-use", oldPort)
	newPort, err := alloc.Reserve(cfg.Id)
	if err != nil {
		return err
	}

	// We already know cfg.Port is in use; if allocator returns same port, treat as failure.
	if newPort == oldPort {
		return fmt.Errorf("unit=%s port %d is in use for %s and no alternative port could be allocated (range=%d-%d)", unit, oldPort, cfg.Id, start, end)
	}

	cfg.Port = newPort
	cfg.Address = fmt.Sprintf("localhost:%d", newPort)
	fmt.Printf("INFO unit=%s service=%s port healed %d->%d range=%d-%d config=%s\n", unit, cfg.Id, oldPort, newPort, start, end, cfgPath)
	return writeServiceConfig(cfgPath, cfg)
}

func seedReservations(alloc *ports.Allocator, servicesDir string) {
	seedReservationsExcept(alloc, servicesDir, "")
}

func seedReservationsExcept(alloc *ports.Allocator, servicesDir, skipId string) {
	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(servicesDir, e.Name())
		cfg, err := readServiceConfig(path)
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if cfg.Id != "" {
			id = cfg.Id
		}
		if skipId != "" && id == skipId {
			continue
		}
		port := firstPort(cfg.Port, portFromAddress(cfg.Address))
		alloc.Mark(id, port)
	}
}

func executableForService(svc string) string {
	switch strings.ToLower(strings.TrimSpace(svc)) {
	case "rbac":
		return "rbac_server"
	case "resource":
		return "resource_server"
	case "repository":
		return "repository_server"
	default:
		return ""
	}
}

type describePayload struct {
	Id      string `json:"Id"`
	Address string `json:"Address"`
	Port    int    `json:"Port"`
}

func runDescribe(ctx context.Context, binPath string) (*describePayload, error) {
	cmd := exec.CommandContext(ctx, binPath, "--describe")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("describe %s: %w", binPath, err)
	}
	var payload describePayload
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("parse describe: %w", err)
	}
	if payload.Port == 0 {
		payload.Port = portFromAddress(payload.Address)
	}
	return &payload, nil
}

func portFromAddress(addr string) int {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return 0
	}

	// First try strict host:port parsing
	if _, port, err := net.SplitHostPort(addr); err == nil {
		if n, err := strconv.Atoi(port); err == nil && n > 0 {
			return n
		}
	}

	// Fallback: last colon token (handles ":61001", "localhost:61001")
	if idx := strings.LastIndex(addr, ":"); idx >= 0 && idx < len(addr)-1 {
		if n, err := strconv.Atoi(addr[idx+1:]); err == nil && n > 0 {
			return n
		}
	}

	// Final fallback: entire string might be a port ("61001")
	if n, err := strconv.Atoi(addr); err == nil && n > 0 {
		return n
	}

	return 0
}

func readServiceConfig(path string) (*describePayload, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg describePayload
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func writeServiceConfig(path string, cfg *describePayload) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func firstPort(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func installBinDir() string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_INSTALL_BIN_DIR")); v != "" {
		return v
	}
	return "/usr/local/bin"
}

func stateRoot() string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_STATE_DIR")); v != "" {
		return v
	}
	return "/var/lib/globular"
}

func idFromUnit(unit, svc string) string {
	// fallback to svc; configs are named by Id, so this is best-effort only
	if svc != "" {
		return svc
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(unit, "globular-"), ".service"))
}

func portFree(port int) bool {
	addrs := []string{fmt.Sprintf("127.0.0.1:%d", port), fmt.Sprintf("[::1]:%d", port)}
	any := false
	for _, a := range addrs {
		ln, err := net.Listen("tcp", a)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "address already in use") {
				return false
			}
			continue
		}
		any = true
		ln.Close()
	}
	if any {
		return true
	}
	return false
}

func preflightStrict() bool {
	return strings.TrimSpace(os.Getenv("GLOBULAR_PORT_PREFLIGHT_STRICT")) == "1"
}
