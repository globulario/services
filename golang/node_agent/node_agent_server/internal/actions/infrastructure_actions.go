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
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/globulario/globular-installer/pkg/installer"
	_ "github.com/globulario/globular-installer/pkg/platform/linux"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	infrastructureInstallRunner = installerEngineInstall
	installerNewContext         = installer.NewContext
	installerInstall            = installer.Install
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
	transactionID := strings.TrimSpace(fields["transaction_id"].GetStringValue())
	nodeID := strings.TrimSpace(fields["node_id"].GetStringValue())
	packageID := strings.TrimSpace(fields["package_id"].GetStringValue())
	targetBuildID := strings.TrimSpace(fields["target_build_id"].GetStringValue())
	previousReceiptJSON := strings.TrimSpace(fields["previous_receipt_json"].GetStringValue())

	// Stage the package: extract .tgz to a temp directory.
	stagingDir, err := installer.ExtractPackageToTemp(artifactPath)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: stage package: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	if transactionID == "" {
		transactionID = fmt.Sprintf("%s-%d", component, time.Now().UnixNano())
	}
	if packageID == "" {
		packageID = component
	}
	prevReceipt := map[string]string(nil)
	if previousReceiptJSON != "" {
		if err := json.Unmarshal([]byte(previousReceiptJSON), &prevReceipt); err != nil {
			return "", fmt.Errorf("infrastructure.install: decode previous_receipt_json: %w", err)
		}
	}
	txRec := &InstallTransactionRecord{
		TransactionID:   transactionID,
		NodeID:          nodeID,
		PackageID:       packageID,
		TargetBuildID:   targetBuildID,
		Phase:           InstallTxnPhaseStaging,
		PreviousReceipt: prevReceipt,
		StagedPaths:     []string{stagingDir},
	}
	if err := startInstallTransaction(txRec); err != nil {
		return "", fmt.Errorf("infrastructure.install: start transaction: %w", err)
	}
	targets, err := infrastructureTransactionTargets(component, stagingDir)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: derive transaction targets: %w", err)
	}
	for _, target := range targets {
		snap, err := snapshotInstallTxnFile(transactionID, target)
		if err != nil {
			return "", fmt.Errorf("infrastructure.install: snapshot %s: %w", target, err)
		}
		txRec.PreviousFiles = append(txRec.PreviousFiles, snap)
	}
	if err := writeInstallTransaction(txRec); err != nil {
		return "", fmt.Errorf("infrastructure.install: persist transaction snapshot: %w", err)
	}
	if err := updateInstallTransactionPhase(transactionID, InstallTxnPhaseValidated, ""); err != nil {
		return "", fmt.Errorf("infrastructure.install: update transaction validated: %w", err)
	}
	msg, err := infrastructureInstallRunner(component, version, stagingDir, dataDirsStr)
	if err != nil {
		rolled, rerr := rollbackInstallTransactionByID(transactionID, err.Error())
		return "", buildInstallTransactionFailure(transactionID, err, rolled, rerr)
	}
	if err := updateInstallTransactionPhase(transactionID, InstallTxnPhasePromoted, ""); err != nil {
		return "", fmt.Errorf("infrastructure.install: update transaction promoted: %w", err)
	}
	if err := scrubInfrastructureUnitSidecars(targets); err != nil {
		rolled, rerr := rollbackInstallTransactionByID(transactionID, err.Error())
		return "", buildInstallTransactionFailure(transactionID, fmt.Errorf("infrastructure.install: scrub unit sidecars: %w", err), rolled, rerr)
	}
	if ActionInstallTxnFailPhase == "infra_after_promote" {
		rolled, rerr := rollbackInstallTransactionByID(transactionID, "injected infrastructure failure after promotion")
		return "", buildInstallTransactionFailure(transactionID, fmt.Errorf("injected infrastructure failure after promotion"), rolled, rerr)
	}
	if err := updateInstallTransactionPhase(transactionID, InstallTxnPhaseReloaded, ""); err != nil {
		return "", fmt.Errorf("infrastructure.install: update transaction reloaded: %w", err)
	}
	return fmt.Sprintf("%s transaction_id=%s", msg, transactionID), nil
}

// installerEngineInstall delegates infra installation to the shared installer engine.
func installerEngineInstall(component, version, stagingDir, dataDirsStr string) (string, error) {
	log.Printf("[installer-engine] using installer engine for %s@%s", component, version)

	opts := installer.Options{
		Prefix:     filepath.Dir(ActionBinDir),
		StateDir:   ActionStateDir,
		ConfigDir:  filepath.Join(ActionStateDir, "services"),
		Version:    version,
		StagingDir: stagingDir,
		Force:      true,
		Verbose:    true,
		// node-agent runs package apply as root and must never enter an
		// interactive installer prompt. Runtime/topology code owns any
		// operator-visible MinIO disk choice outside this apply path.
		NonInteractive: true,
		SkipStart:      true,
	}
	if component == "minio" {
		// Node-agent package apply is not an objectstore topology planner. Pin the
		// package-rendered data dir so this path never invokes installer disk
		// discovery or an operator prompt while converging desired state.
		opts.MinioDataDir = filepath.Join(ActionStateDir, "minio", "data")
	}

	ictx, err := installerNewContext(opts)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: create installer context: %w", err)
	}

	report, err := installerInstall(ictx)
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

func infrastructureTransactionTargets(component, stagingDir string) ([]string, error) {
	set := make(map[string]struct{})
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path != "" {
			set[path] = struct{}{}
		}
	}

	binDir := filepath.Join(stagingDir, "bin")
	if entries, err := os.ReadDir(binDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			add(filepath.Join(ActionBinDir, entry.Name()))
		}
	}

	configDir := filepath.Join(stagingDir, "config")
	if entries, err := os.ReadDir(configDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			root := filepath.Join(configDir, entry.Name())
			_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.IsDir() {
					return nil
				}
				rel, rerr := filepath.Rel(root, path)
				if rerr == nil {
					add(filepath.Join(ActionStateDir, entry.Name(), rel))
				}
				return nil
			})
		}
	}

	stateDir := filepath.Join(stagingDir, "state")
	_ = filepath.Walk(stateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(stateDir, path)
		if rerr == nil {
			add(filepath.Join(ActionStateDir, rel))
		}
		return nil
	})

	assetsDir := filepath.Join(stagingDir, "assets")
	assetsRoot := filepath.Join(filepath.Dir(ActionBinDir), "assets")
	_ = filepath.Walk(assetsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(assetsDir, path)
		if rerr == nil {
			add(filepath.Join(assetsRoot, rel))
		}
		return nil
	})

	systemdDir := filepath.Join(stagingDir, "systemd")
	if entries, err := os.ReadDir(systemdDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".service") {
				continue
			}
			unitPath := filepath.Join(ActionSystemdDir, entry.Name())
			add(unitPath)
			add(unitPath + ".sha256")
		}
	}

	if component == "scylla-manager" {
		add("/usr/lib/globular/bin/scylla-manager-configure")
	}

	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

func scrubInfrastructureUnitSidecars(targets []string) error {
	for _, path := range targets {
		if !strings.HasSuffix(path, ".service") {
			continue
		}
		if err := os.Remove(path + ".sha256"); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
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
