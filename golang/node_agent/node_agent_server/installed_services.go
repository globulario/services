package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/globulario/services/golang/identity"
	"github.com/globulario/services/golang/plan/versionutil"
)

// ServiceKey uniquely identifies a service by publisher and name.
// Canonical form: "<publisher_id>/<service_name>" with a sanitized service name.
type ServiceKey struct {
	PublisherID string
	ServiceName string
}

func (k ServiceKey) String() string {
	return canonicalServiceName(k.ServiceName)
}

// InstalledServiceInfo captures what the node knows about an installed service.
type InstalledServiceInfo struct {
	PublisherID  string
	ServiceName  string
	Version      string
	Config       map[string]string
	ConfigDigest string
}

// ComputeInstalledServices returns the installed services on this node and a deterministic hash.
// The hash is stable across runs and independent of map/directory iteration order.
func ComputeInstalledServices(ctx context.Context) (map[ServiceKey]InstalledServiceInfo, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	byService := map[string]*InstalledServiceInfo{}
	var firstErr error
	recordErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	loadMarkers(ctx, byService, recordErr)
	// NOTE: loadServiceConfigs intentionally removed — disk JSON files
	// (/var/lib/globular/services/*.json) are a mirror of etcd, not an
	// independent source of truth. Reading them here created a resurrection
	// vector: after package.clear_state deleted the etcd record, the disk
	// file could re-create the installed-state record on the next heartbeat.
	// Version markers + systemd units are sufficient for local discovery.
	loadSystemdUnits(ctx, byService, recordErr)
	// Detect infrastructure installed via Day 0 / join logic (e.g. etcd).
	// These are skipped by loadSystemdUnits but must be visible for
	// version resolution and reconciliation gating.
	loadDay0JoinInfra(ctx, byService, recordErr)

	inst := make(map[ServiceKey]InstalledServiceInfo, len(byService))
	for _, entry := range byService {
		if entry == nil {
			continue
		}
		if entry.ServiceName == "" || entry.Version == "" {
			continue
		}
		if entry.PublisherID == "" {
			entry.PublisherID = "unknown"
		}
		key := ServiceKey{PublisherID: entry.PublisherID, ServiceName: entry.ServiceName}
		inst[key] = InstalledServiceInfo{
			PublisherID:  entry.PublisherID,
			ServiceName:  entry.ServiceName,
			Version:      entry.Version,
			Config:       entry.Config,
			ConfigDigest: entry.ConfigDigest,
		}
	}

	hash := computeAppliedServicesHash(inst)
	return inst, hash, firstErr
}

func loadMarkers(ctx context.Context, byService map[string]*InstalledServiceInfo, recordErr func(error)) {
	markerRoot := versionutil.BaseDir()
	entries, err := os.ReadDir(markerRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			recordErr(fmt.Errorf("list version markers: %w", err))
		}
		return
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			recordErr(err)
			return
		}
		path := filepath.Join(markerRoot, name, "version")
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			recordErr(fmt.Errorf("read version marker %s: %w", path, err))
			continue
		}
		version := strings.TrimSpace(string(data))
		if version == "" {
			continue
		}
		if cv, err := versionutil.Canonical(version); err == nil {
			version = cv
		}
		svc := canonicalServiceName(name)
		entry := ensureServiceEntry(byService, svc)
		entry.ServiceName = svc
		entry.Version = version
		// Optional config digest marker.
		digestPath := filepath.Join(markerRoot, name, "config.sha256")
		if cfgData, err := os.ReadFile(digestPath); err == nil {
			digest := strings.ToLower(strings.TrimSpace(string(cfgData)))
			if digest != "" && !isHex64(digest) {
				recordErr(fmt.Errorf("invalid config digest for %s: %s", svc, digest))
				continue
			}
			entry.ConfigDigest = digest
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			recordErr(fmt.Errorf("read config digest %s: %w", digestPath, err))
		}
	}
}


// loadSystemdUnits discovers active globular-*.service systemd units and adds
// them as installed services when they were not already found by markers or
// config files. This ensures services installed by the installer (which does
// not write version markers) are still reported to the controller.
func loadSystemdUnits(ctx context.Context, byService map[string]*InstalledServiceInfo, recordErr func(error)) {
	// List active globular-* units via systemctl.
	out, err := exec.CommandContext(ctx, "systemctl", "list-units",
		"--type=service", "--state=active", "--no-legend", "--no-pager",
		"globular-*.service").Output()
	if err != nil {
		// systemctl not available or failed — not fatal.
		return
	}

	// Packages whose kind and version come from the repository artifact
	// catalog (Phase 2 of syncInstalledStateToEtcd), NOT from systemd
	// unit scanning. Without this, Phase 1 creates SERVICE/0.0.1 records
	// that mask the correct INFRASTRUCTURE/COMMAND records with real
	// versions from the repo.
	// Control-plane services (node-agent, cluster-controller, cluster-doctor)
	// ARE managed — they participate in desired state and reconciliation.
	skipSystemd := map[string]bool{
		// Core infrastructure (no desired-state model)
		"etcd": true, "minio": true, "envoy": true,
		"xds": true, "gateway": true, "mcp": true,
		// Infrastructure services (from /packages/specs/*_service.yaml)
		"node-exporter": true, "prometheus": true,
		"scylla-manager": true, "scylla-manager-agent": true,
		"scylladb": true, "keepalived": true, "sidekick": true,
		// CLI tools — not services (from /packages/specs/*_cmd.yaml)
		"etcdctl-cmd": true, "ffmpeg-cmd": true, "globular-cli-cmd": true,
		"mc-cmd": true, "rclone-cmd": true, "restic-cmd": true,
		"sctool-cmd": true, "sha256sum-cmd": true, "yt-dlp-cmd": true,
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "globular-ldap.service loaded active running ..."
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0]
		svc := canonicalServiceName(unit)
		if svc == "" || skipSystemd[svc] {
			continue
		}
		// Skip if already discovered by markers or config.
		if entry := byService[svc]; entry != nil && entry.Version != "" {
			continue
		}
		// Fallback version: "0.0.1" — the default for installer-deployed services.
		entry := ensureServiceEntry(byService, svc)
		entry.ServiceName = svc
		if entry.Version == "" {
			entry.Version = "0.0.1"
		}
	}
}

// loadDay0JoinInfra detects infrastructure packages installed via Day 0 / join
// logic (not through the artifact pipeline) and adds them to the installed map.
// These packages are skipped by loadSystemdUnits but must still be visible to
// the controller for version resolution and reconciliation gating.
func loadDay0JoinInfra(ctx context.Context, byService map[string]*InstalledServiceInfo, recordErr func(error)) {
	// etcd: installed by Day 0 installer or Day 1 etcd join. Detect via
	// systemctl and resolve version from etcdctl.
	if err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "globular-etcd.service").Run(); err == nil {
		if entry := byService["etcd"]; entry == nil || entry.Version == "" || entry.Version == "0.0.1" {
			version := detectEtcdVersion(ctx)
			if version == "" {
				version = "unknown"
			}
			entry := ensureServiceEntry(byService, "etcd")
			entry.ServiceName = "etcd"
			entry.Version = version
		}
	}

	// scylladb: OS package (apt install), not a bundled binary. Detect via
	// systemctl and resolve version from scylla --version.
	if err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "scylla-server.service").Run(); err == nil {
		if entry := byService["scylladb"]; entry == nil || entry.Version == "" || entry.Version == "0.0.1" {
			version := detectScyllaVersion(ctx)
			if version == "" {
				version = "unknown"
			}
			entry := ensureServiceEntry(byService, "scylladb")
			entry.ServiceName = "scylladb"
			entry.Version = version
		}
	}
}

// detectEtcdVersion runs "etcdctl version" and parses the etcd server version.
// Returns empty string if detection fails.
func detectEtcdVersion(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "etcdctl", "version").Output()
	if err != nil {
		return ""
	}
	// Output format: "etcdctl version: 3.5.14\nAPI version: 3.5\n"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "etcdctl version:") {
			ver := strings.TrimSpace(strings.TrimPrefix(line, "etcdctl version:"))
			if ver != "" {
				return ver
			}
		}
	}
	return ""
}

// detectScyllaVersion runs "scylla --version" and parses the version.
// Returns empty string if detection fails.
func detectScyllaVersion(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "scylla", "--version").Output()
	if err != nil {
		return ""
	}
	// Output format: "5.4.8-0.20241027..." or "2025.3.1-..."
	ver := strings.TrimSpace(string(out))
	// Strip build suffix after first hyphen-digit (e.g. "5.4.8-0.20241027" → "5.4.8")
	if idx := strings.Index(ver, "-"); idx > 0 {
		ver = ver[:idx]
	}
	return ver
}

// isDay0JoinInfra returns true for infrastructure packages installed via Day 0
// installer or Day 1 join logic (not through the artifact repository).
// These must be classified as INFRASTRUCTURE in etcd installed-state records.
//
// etcd: binary installed by Day 0 installer, joined via etcd member-add state machine
// scylladb: OS package (apt install), joined via gossip/seed state machine
func isDay0JoinInfra(name string) bool {
	switch name {
	case "etcd", "scylladb":
		return true
	}
	return false
}

// computeAppliedServicesHash returns a SHA256 (lowercase hex) over the installed service set.
//
// Canonical format per entry: "<canonical_service_name>=<version>;"
// - Entries are sorted by canonical service name.
// - This format matches the controller's hashDesiredServiceVersions() so that
//   the two hashes are directly comparable when the service sets agree.
func computeAppliedServicesHash(installed map[ServiceKey]InstalledServiceInfo) string {
	if len(installed) == 0 {
		return ""
	}
	keys := make([]ServiceKey, 0, len(installed))
	for k := range installed {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	var b strings.Builder
	for _, k := range keys {
		info := installed[k]
		b.WriteString(k.String())
		b.WriteString("=")
		b.WriteString(strings.TrimSpace(info.Version))
		b.WriteString(";")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func ensureServiceEntry(byService map[string]*InstalledServiceInfo, service string) *InstalledServiceInfo {
	if entry := byService[service]; entry != nil {
		return entry
	}
	entry := &InstalledServiceInfo{ServiceName: service}
	byService[service] = entry
	return entry
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func canonicalServiceName(name string) string {
	key, _ := identity.NormalizeServiceKey(name)
	return key
}

