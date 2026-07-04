// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.infrastructure_actions
// @awareness file_role=infrastructure_class_install_apply_actions_using_external_installer_package
// @awareness implements=globular.platform:intent.infrastructure.installed_artifacts.runtime_layout_and_authority_boundaries
// @awareness enforces=globular.platform:invariant.infra.desired_hash_consistency
// @awareness risk=critical
package actions

// infrastructure_actions.go — handlers for INFRASTRUCTURE-class
// packages (etcd, scylladb, minio, envoy, keepalived, xds, etc).
// These differ from SERVICE/APPLICATION installs in two ways:
//
//   1. The convergence hash schema is
//      `infra:<publisher>/<component>=<version>+b:<buildNumber>;`
//      — NOT the artifact blob sha256. The node-agent stamps
//      this hash on install; using the artifact digest produces
//      the permanent mismatch loop that caused the Envoy restart
//      storm of 2026-05-06.
//
//   2. The runtime layout and capability boundary differ per
//      infrastructure package (see the runtime_layout intent).
//      Conflating them — e.g. starting etcd via the SERVICE
//      pathway — bypasses unit-file safety checks like the
//      WorkingDirectory normalization
//      (INC-2026-0018 / c529310e).

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/globular-installer/pkg/installer"
	_ "github.com/globulario/globular-installer/pkg/platform/linux"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"google.golang.org/protobuf/types/known/structpb"
)

// ── infrastructure.install ──────────────────────────────────────────────────
//
// Installs an infrastructure component (etcd, minio, envoy, etc.) using the
// shared installer engine when a spec is present in the package, or falls back
// to legacy archive extraction for old packages without specs.
//
// Args:
//
//	name          (string, required) — component name (e.g. "etcd", "minio")
//	version       (string, required)
//	artifact_path (string, required) — path to .tar.gz archive
//	data_dirs     (string, optional) — comma-separated directories to create
type infrastructureInstallAction struct{}

func (infrastructureInstallAction) Name() string { return "infrastructure.install" }

func (infrastructureInstallAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("infrastructure.install: name is required")
	}
	if strings.TrimSpace(fields["artifact_path"].GetStringValue()) == "" {
		return fmt.Errorf("infrastructure.install: artifact_path is required")
	}
	return nil
}

func (infrastructureInstallAction) Apply(_ context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	component := strings.TrimSpace(fields["name"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	artifactPath := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	dataDirsStr := strings.TrimSpace(fields["data_dirs"].GetStringValue())

	// Stage the package: extract .tgz to a temp directory.
	stagingDir, err := installer.ExtractPackageToTemp(artifactPath)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: stage package: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	return installerEngineInstall(component, version, stagingDir, dataDirsStr)
}

// scyllaJoinScriptEnv returns the extra run_script environment for a Day-1
// ScyllaDB fresh join, or nil for anything else. It is scoped two ways:
//   - component must be scylladb (only ScyllaDB's post-install reads these vars), and
//   - joinActive must be true — the node-agent sets this only while it holds
//     Day-1 join credentials, never during steady-state re-installs.
//
// SCYLLA_INSTALL_INTENT=fresh-join switches the post-install from its fail-safe
// "preserve" default (which fails closed on any stale Raft state) to its
// fresh-join branch. The destructive wipe stays fenced
// (intent:node_recovery.fence_before_destructive_reseed): the post-install's
// CA-derived ownership fingerprint preserves data owned by THIS cluster, so an
// existing member re-installing is never wiped — only foreign stale state from a
// different cluster epoch is cleared.
func scyllaJoinScriptEnv(component string, joinActive bool) map[string]string {
	if !joinActive || !strings.EqualFold(strings.TrimSpace(component), "scylladb") {
		return nil
	}
	return map[string]string{
		"SCYLLA_INSTALL_INTENT":             "fresh-join",
		"ALLOW_STALE_SCYLLA_REINIT_ON_JOIN": "true",
	}
}

// installerEngineInstall delegates infra installation to the shared installer engine.
func installerEngineInstall(component, version, stagingDir, dataDirsStr string) (string, error) {
	log.Printf("[installer-engine] using installer engine for %s@%s", component, version)

	opts := installer.Options{
		Version:    version,
		StagingDir: stagingDir,
		Force:      true,
		Verbose:    true,
	}

	// Day-1 join: tell the ScyllaDB post-install this is a fresh join (see
	// scyllaJoinScriptEnv). Without it the post-install runs in the fail-safe
	// "preserve" default and fails closed on any stale Raft state — the common
	// re-join case.
	if env := scyllaJoinScriptEnv(component, IsJoinActive()); env != nil {
		opts.ScriptEnv = env
		log.Printf("[installer-engine] Day-1 join: ScyllaDB fresh-join intent enabled (fenced by CA-derived ownership fingerprint)")
	}

	ictx, err := installer.NewContext(opts)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: create installer context: %w", err)
	}

	report, err := installer.Install(ictx)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: installer engine failed for %s: %w", component, err)
	}
	if err := ensureScyllaManagerConfigureScript(component); err != nil {
		return "", err
	}

	// Create data directories if specified (same as legacy path).
	if dataDirsStr != "" {
		for _, dir := range strings.Split(dataDirsStr, ",") {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				continue
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("infrastructure.install: create data dir %s: %w", dir, err)
			}
		}
	}

	stepCount := len(report.Results)
	failedCount := report.ErrorCount()
	return fmt.Sprintf("infrastructure %s@%s installed via installer engine (%d steps, %d failed)", component, version, stepCount, failedCount), nil
}

func ensureScyllaManagerConfigureScript(component string) error {
	if component != "scylla-manager" {
		return nil
	}
	const scriptPath = "/usr/lib/globular/bin/scylla-manager-configure"
	if fi, err := os.Stat(scriptPath); err == nil && fi.Mode().Perm()&0o111 != 0 {
		return nil
	}
	const script = `#!/bin/sh
# Generate scylla-manager config with the correct CQL host.
CFG="/var/lib/globular/scylla-manager/scylla-manager.yaml"
if [ -f "$CFG" ]; then
  exit 0
fi
SCYLLA_CFG="/etc/scylla/scylla.yaml"
CQL_HOST=""
if [ -f "$SCYLLA_CFG" ]; then
  CQL_HOST=$(grep -E '^rpc_address:' "$SCYLLA_CFG" | awk '{print $2}' | tr -d "[:space:]'\"")
  [ -z "$CQL_HOST" ] && CQL_HOST=$(grep -E '^listen_address:' "$SCYLLA_CFG" | awk '{print $2}' | tr -d "[:space:]'\"")
fi
if [ -z "$CQL_HOST" ] || [ "$CQL_HOST" = "0.0.0.0" ]; then
  CQL_HOST=$(ss -lnt | awk '/:9042 /{split($4,a,":"); print a[1]; exit}')
fi
if [ -z "$CQL_HOST" ] || [ "$CQL_HOST" = "127.0.0.1" ]; then
  echo "scylla-manager-configure: cannot determine ScyllaDB routable IP — aborting" >&2
  exit 1
fi
cat > "$CFG" <<EOCONF
# Scylla Manager configuration (managed by Globular)
http: ${CQL_HOST}:5080

database:
  hosts:
    - ${CQL_HOST}
  port: 9042
EOCONF
chown globular:globular "$CFG"
chmod 0640 "$CFG"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		return fmt.Errorf("infrastructure.install: write %s: %w", scriptPath, err)
	}
	log.Printf("[installer-engine] repaired missing helper %s for scylla-manager", scriptPath)
	return nil
}

// ── infrastructure.uninstall ────────────────────────────────────────────────
//
// Stops and removes an infrastructure component.
//
// Args:
//
//	name (string, required) — component name
//	unit (string, optional) — systemd unit name (default: globular-{name}.service)
type infrastructureUninstallAction struct{}

func (infrastructureUninstallAction) Name() string { return "infrastructure.uninstall" }

func (infrastructureUninstallAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("infrastructure.uninstall: name is required")
	}
	return nil
}

func (infrastructureUninstallAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	component := strings.TrimSpace(fields["name"].GetStringValue())
	unit := strings.TrimSpace(fields["unit"].GetStringValue())
	if unit == "" {
		unit = "globular-" + component + ".service"
	}

	binDir, systemdDir, configDir, skipSystemd := installPaths()

	if !skipSystemd {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		// Best-effort stop+disable via the supervisor (the single allowlisted
		// systemd-control path), not raw exec (EX-2). Errors are ignored as before
		// — the unit may not exist.
		_ = supervisor.Stop(cctx, unit)
		_ = supervisor.Disable(cctx, unit)
	}

	binPath := filepath.Join(binDir, component)
	_ = os.Remove(binPath)

	if !skipSystemd {
		unitPath := filepath.Join(systemdDir, unit)
		if err := os.Remove(unitPath); err == nil {
			cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			_ = supervisor.DaemonReload(cctx)
		}
	}

	cfgDir := filepath.Join(configDir, component)
	_ = os.RemoveAll(cfgDir)

	// Remove the version marker the installed-state sync reads — same fix as
	// package.uninstall's SERVICE case. Without this the uninstall is
	// non-idempotent and syncInstalledStateToEtcd re-mints a degraded stub.
	removeSyncReadVersionMarker(component)

	return fmt.Sprintf("infrastructure %s uninstalled (unit=%s)", component, unit), nil
}

func init() {
	Register(infrastructureInstallAction{})
	Register(infrastructureUninstallAction{})
}
