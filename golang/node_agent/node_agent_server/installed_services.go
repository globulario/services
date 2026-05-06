package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/identity"
	"github.com/globulario/services/golang/versionutil"
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

// ObservationSource classifies how a service was discovered on this node.
// Only ManagedInstalled observations may participate in convergence checks,
// etcd installed-state records, and desired-state import/resolution.
type ObservationSource int

const (
	// ManagedInstalled — discovered via version markers written by the
	// official apply/deploy path (ApplyPackageRelease), OR via etcd
	// installed-state records, OR via Day 0 join infrastructure with a
	// real detected version. These are authoritative.
	ManagedInstalled ObservationSource = iota

	// RuntimeUnmanaged — a systemd unit exists but has no version marker
	// and no etcd installed-state record. The service was likely installed
	// out-of-band (manual copy, legacy installer) and its version is
	// unknown. Must NOT influence desired state or convergence.
	RuntimeUnmanaged

	// FallbackDiscovered — a systemd unit exists, and a version was found
	// but came from an unreliable source (e.g. stale marker, legacy
	// fallback). May be logged for diagnostics but must NOT be treated as
	// authoritative.
	FallbackDiscovered
)

func (o ObservationSource) String() string {
	switch o {
	case ManagedInstalled:
		return "managed_installed"
	case RuntimeUnmanaged:
		return "runtime_unmanaged"
	case FallbackDiscovered:
		return "fallback_discovered"
	default:
		return "unknown"
	}
}

// InstalledServiceInfo captures what the node knows about an installed service.
type InstalledServiceInfo struct {
	PublisherID  string
	ServiceName  string
	Version      string
	Config       map[string]string
	ConfigDigest string
	Source       ObservationSource
}

// IsAuthoritative returns true if this observation may participate in
// convergence checks, desired-state import, and etcd installed-state records.
func (i InstalledServiceInfo) IsAuthoritative() bool {
	return i.Source == ManagedInstalled
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
			Source:       entry.Source,
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
		entry.Source = ManagedInstalled // version markers are written by the apply path
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

// skipSystemdUnits lists packages whose kind and version come from the
// repository artifact catalog (Phase 2 of syncInstalledStateToEtcd), NOT from
// systemd unit scanning. loadSystemdUnits skips these to prevent Phase 1 from
// creating SERVICE/unknown records that mask the correct INFRASTRUCTURE/COMMAND
// records with real versions. All infrastructure packages use systemd units —
// skipping them here is about preventing version-unknown phantoms, NOT about
// them not having units.
// syncRepoArtifactsToEtcd uses this list to override kind=SERVICE to
// kind=INFRASTRUCTURE for packages wrongly classified by a stale package.json.
var skipSystemdUnits = map[string]bool{
	// Infrastructure daemons — versions come from repo artifact or binary probe
	"etcd": true, "minio": true, "envoy": true,
	"xds": true, "gateway": true, "mcp": true,
	// Monitoring / ops infrastructure
	"node-exporter": true, "prometheus": true, "alertmanager": true,
	"scylla-manager": true, "scylla-manager-agent": true,
	"scylladb": true, "keepalived": true, "sidekick": true,
	// CLI tools — not daemons (from /packages/specs/*_cmd.yaml)
	"etcdctl-cmd": true, "ffmpeg-cmd": true, "globular-cli-cmd": true,
	"mc-cmd": true, "rclone-cmd": true, "restic-cmd": true,
	"sctool-cmd": true, "sha256sum-cmd": true, "yt-dlp-cmd": true,
}

// loadSystemdUnits discovers loaded globular-*.service systemd units and adds
// them as installed services when they were not already found by markers or
// config files. This ensures services installed by the installer (which does
// not write version markers) are still reported to the controller.
func loadSystemdUnits(ctx context.Context, byService map[string]*InstalledServiceInfo, recordErr func(error)) {
	// List active globular-* units via systemctl.
	out, err := exec.CommandContext(ctx, "systemctl", "list-units",
		"--type=service", "--state=loaded", "--no-legend", "--no-pager",
		"globular-*.service").Output()
	if err != nil {
		// systemctl not available or failed — not fatal.
		return
	}

	skipSystemd := skipSystemdUnits

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
		// Skip systemd's failure bullet character (●) and any non-unit entries.
		if !strings.HasPrefix(unit, "globular-") {
			continue
		}
		svc := canonicalServiceName(unit)
		if svc == "" || skipSystemd[svc] {
			continue
		}
		// Skip if already discovered by markers or config.
		if entry := byService[svc]; entry != nil && entry.Version != "" {
			continue
		}
		// Service has a systemd unit but no version marker. Try to detect
		// the installed version from the binary via --describe. SERVICE
		// binaries follow the <name>_server naming convention.
		entry := ensureServiceEntry(byService, svc)
		entry.ServiceName = svc
		binPath := filepath.Join(globularBinDir, strings.ReplaceAll(svc, "-", "_")+"_server")
		if version := detectGlobularBinaryVersion(ctx, binPath); version != "" && version != "unknown" {
			entry.Version = version
			entry.Source = ManagedInstalled
		} else {
			// Binary probe failed — mark RuntimeUnmanaged. Phase 2.5 (peer
			// checksum lookup) may resolve it later.
			entry.Source = RuntimeUnmanaged
			if entry.Version == "" {
				entry.Version = "unknown"
			}
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
		if entry := byService["etcd"]; entry == nil || entry.Version == "" {
			version := detectEtcdVersion(ctx)
			if version == "" {
				version = "unknown"
			}
			entry := ensureServiceEntry(byService, "etcd")
			entry.ServiceName = "etcd"
			entry.Version = version
			if version != "unknown" {
				entry.Source = ManagedInstalled // real version detected from binary
			} else {
				entry.Source = RuntimeUnmanaged
			}
		}
	}

	// scylladb: OS package (apt install), not a bundled binary. Detect via
	// systemctl and resolve version from scylla --version.
	if err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "scylla-server.service").Run(); err == nil {
		if entry := byService["scylladb"]; entry == nil || entry.Version == "" {
			version := detectScyllaVersion(ctx)
			if version == "" {
				version = "unknown"
			}
			entry := ensureServiceEntry(byService, "scylladb")
			entry.ServiceName = "scylladb"
			entry.Version = version
			if version != "unknown" {
				entry.Source = ManagedInstalled // real version detected from binary
			} else {
				entry.Source = RuntimeUnmanaged
			}
		}
	}

	// Per-daemon version detectors for infrastructure packages that don't
	// support --describe. Each returns "" when the unit is absent or the
	// binary fails; on success the entry is promoted to ManagedInstalled.
	type infraDaemon struct {
		name    string
		unit    string // systemd unit name (may differ from package name)
		detect  func() string
	}
	daemons := []infraDaemon{
		{"envoy", "globular-envoy.service", func() string { return detectEnvoyVersion(ctx) }},
		{"prometheus", "globular-prometheus.service", func() string {
			return detectPrometheusLikeVersion(ctx, filepath.Join(globularBinDir, "prometheus"))
		}},
		{"alertmanager", "globular-alertmanager.service", func() string {
			return detectPrometheusLikeVersion(ctx, filepath.Join(globularBinDir, "alertmanager"))
		}},
		{"node-exporter", "globular-node-exporter.service", func() string {
			return detectPrometheusLikeVersion(ctx, filepath.Join(globularBinDir, "node_exporter"))
		}},
		{"minio", "globular-minio.service", func() string { return detectMinioVersion(ctx) }},
		{"sidekick", "globular-sidekick.service", func() string { return detectSidekickVersion(ctx) }},
	}
	for _, d := range daemons {
		if exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", d.unit).Run() != nil {
			continue
		}
		if entry := byService[d.name]; entry != nil && entry.Version != "" && entry.Version != "unknown" {
			continue
		}
		version := d.detect()
		if version == "" {
			version = "unknown"
		}
		e := ensureServiceEntry(byService, d.name)
		e.ServiceName = d.name
		e.Version = version
		// Third-party infra binaries (envoy, prometheus, etc.) are tracked via
		// the etcd installed-state registry written by the install workflow.
		// Using FallbackDiscovered keeps them visible for diagnostics but
		// excludes them from InstalledVersions so the controller does NOT
		// dispatch repeated install workflows from the heartbeat alone.
		e.Source = FallbackDiscovered
	}

	// Generic probe for remaining infrastructure daemons (Globular-built, use
	// --describe). Skips entries already handled above.
	handled := map[string]bool{
		"etcd": true, "scylladb": true,
		"envoy": true, "prometheus": true, "alertmanager": true,
		"node-exporter": true, "minio": true, "sidekick": true,
	}
	for name := range skipSystemdUnits {
		if strings.HasSuffix(name, "-cmd") || handled[name] {
			continue
		}
		unitName := "globular-" + name + ".service"
		if err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unitName).Run(); err != nil {
			continue
		}
		if entry := byService[name]; entry != nil && entry.Version != "" && entry.Version != "unknown" {
			continue
		}
		version := detectGlobularBinaryVersion(ctx, filepath.Join(globularBinDir, name))
		if version == "" {
			version = "unknown"
		}
		e := ensureServiceEntry(byService, name)
		e.ServiceName = name
		e.Version = version
		if version != "unknown" {
			e.Source = ManagedInstalled
		} else {
			e.Source = RuntimeUnmanaged
		}
	}
}

// detectGlobularBinaryVersion probes a Globular service binary with --describe
// and returns the Version field from the JSON output. Returns "" on failure.
func detectGlobularBinaryVersion(ctx context.Context, binPath string) string {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(probeCtx, binPath, "--describe").Output()
	if err != nil {
		return ""
	}
	var payload struct {
		Version string `json:"Version"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return ""
	}
	ver := strings.TrimSpace(payload.Version)
	if ver == "" {
		return ""
	}
	if cv, err := versionutil.Canonical(ver); err == nil {
		return cv
	}
	return ver
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

// detectEnvoyVersion parses `envoy --version` output.
// Output format: "envoy  version: <hash>/<semver>/Clean/RELEASE/..."
func detectEnvoyVersion(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, filepath.Join(globularBinDir, "envoy"), "--version").Output()
	if err != nil {
		return ""
	}
	// Extract the semver component: "<hash>/<semver>/..."
	line := strings.TrimSpace(string(out))
	if idx := strings.Index(line, "version:"); idx >= 0 {
		line = strings.TrimSpace(line[idx+len("version:"):])
	}
	parts := strings.Split(line, "/")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) > 0 && p[0] >= '0' && p[0] <= '9' {
			return p
		}
	}
	return ""
}

// detectPrometheusLikeVersion parses `<bin> --version` for Prometheus-family
// binaries (prometheus, alertmanager, node_exporter).
// Output format: "<name>, version <semver> ..."
func detectPrometheusLikeVersion(ctx context.Context, bin string) string {
	out, err := exec.CommandContext(ctx, bin, "--version").CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if idx := strings.Index(line, "version "); idx >= 0 {
			rest := strings.TrimSpace(line[idx+len("version "):])
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return ""
}

// detectMinioVersion parses `minio --version` output.
// Output format: "minio version RELEASE.2025-09-07T16-13-09Z ..."
func detectMinioVersion(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, filepath.Join(globularBinDir, "minio"), "--version").Output()
	if err != nil {
		return ""
	}
	for _, field := range strings.Fields(string(out)) {
		if strings.HasPrefix(field, "RELEASE.") {
			return field
		}
	}
	return ""
}

// detectSidekickVersion parses `sidekick --version` output.
// Output format: "sidekick version <semver>"
func detectSidekickVersion(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, filepath.Join(globularBinDir, "sidekick"), "--version").Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	for i, f := range fields {
		if f == "version" && i+1 < len(fields) {
			return strings.TrimSpace(fields[i+1])
		}
	}
	return ""
}

// isDay0JoinInfra returns true for infrastructure packages installed via Day 0
// installer or Day 1 join logic (not through the artifact repository).
// These must be classified as INFRASTRUCTURE in etcd installed-state records.
//
// etcd: binary installed by Day 0 installer, joined via etcd member-add state machine
// scylladb: OS package (apt install), joined via gossip/seed state machine
func isDay0JoinInfra(name string) bool {
	switch name {
	case "etcd", "scylladb", "minio", "envoy", "xds", "gateway",
		"prometheus", "alertmanager", "node-exporter", "sidekick",
		"keepalived", "scylla-manager", "scylla-manager-agent":
		return true
	}
	return false
}

// computeAppliedServicesHash returns a SHA256 (lowercase hex) over the installed service set.
//
// Canonical format per entry: "<canonical_service_name>=<version>;"
//   - Entries are sorted by canonical service name.
//   - This format matches the controller's hashDesiredServiceVersions() so that
//     the two hashes are directly comparable when the service sets agree.
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
