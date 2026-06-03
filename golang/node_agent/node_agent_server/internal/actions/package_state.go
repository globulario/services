// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.package_state
// @awareness file_role=canonical_installed_state_writer_for_all_package_kinds_via_etcd
// @awareness implements=globular.platform:intent.installed_state.owned_by_node_agent
// @awareness risk=critical
package actions

// package_state.go — single point of installed-state writes for
// every package kind (SERVICE / APPLICATION / INFRASTRUCTURE).
// Splitting writes across multiple files would let one path
// stamp Installed/Updated unix timestamps inconsistently with
// another and re-introduce the "wall clock vs PID start" drift
// that produced INC-2026-0016.
//
// This file MUST NOT make decisions about whether a package
// SHOULD be installed — that is the controller's job (desired
// state). It records what HAS been installed, with provenance,
// after the action handler returned success.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// reportStateInstalledBy is the install_by attribution used when the
// workflow's package.report_state action stamps a canonical receipt.
const reportStateInstalledBy = "node-agent.workflow.package_report_state"

// reportStateBinDir mirrors the main-package globularBinDir constant.
// Sub-package isolation requires us to inline the path; the convention
// is fixed and shared by every Globular install (see install_payload
// action's ActionBinDir default).
const reportStateBinDir = "/usr/lib/globular/bin"

// reportStateSystemdDir mirrors /etc/systemd/system. Inlined for the
// same reason as reportStateBinDir.
const reportStateSystemdDir = "/etc/systemd/system"

// packageReportStateAction writes an InstalledPackage record to etcd after
// successful lifecycle execution. This is the canonical installed-state writer
// for all package kinds (SERVICE, APPLICATION, INFRASTRUCTURE).
//
// Plan step args:
//
//	node_id      (string, required)
//	name         (string, required)
//	version      (string, required)
//	kind         (string, required: "SERVICE", "APPLICATION", "INFRASTRUCTURE")
//	publisher_id (string, optional)
//	platform     (string, optional)
//	checksum     (string, optional)
//	operation_id (string, optional)
//	status       (string, optional, default: "installed")
type packageReportStateAction struct{}

func (packageReportStateAction) Name() string { return "package.report_state" }

func (packageReportStateAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["node_id"].GetStringValue()) == "" {
		return fmt.Errorf("package.report_state: node_id is required")
	}
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("package.report_state: name is required")
	}
	if strings.TrimSpace(fields["version"].GetStringValue()) == "" {
		return fmt.Errorf("package.report_state: version is required")
	}
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))
	switch kind {
	case "SERVICE", "APPLICATION", "INFRASTRUCTURE":
		// valid
	default:
		return fmt.Errorf("package.report_state: kind must be SERVICE, APPLICATION, or INFRASTRUCTURE (got %q)", kind)
	}
	return nil
}

func (packageReportStateAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()

	nodeID := strings.TrimSpace(fields["node_id"].GetStringValue())
	name := strings.TrimSpace(fields["name"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))
	publisherID := strings.TrimSpace(fields["publisher_id"].GetStringValue())
	platform := strings.TrimSpace(fields["platform"].GetStringValue())
	checksum := strings.TrimSpace(fields["checksum"].GetStringValue())
	operationID := strings.TrimSpace(fields["operation_id"].GetStringValue())
	status := strings.TrimSpace(fields["status"].GetStringValue())
	if status == "" {
		status = "installed"
	}

	// Build number: read from plan args (int64 via NumberValue, 0 = legacy).
	var buildNumber int64
	if bn := fields["build_number"]; bn != nil {
		buildNumber = int64(bn.GetNumberValue())
	}

	now := time.Now().Unix()

	// Check if there's an existing record (to preserve installed_unix and
	// metadata fields like entrypoint_checksum written by other paths).
	existing, _ := installed_state.GetInstalledPackage(ctx, nodeID, kind, name)
	installedUnix := now
	if existing != nil && existing.InstalledUnix > 0 {
		installedUnix = existing.InstalledUnix
	}

	metadata := mergeReportStateMetadata(existing, fields)

	pkg := &node_agentpb.InstalledPackage{
		NodeId:        nodeID,
		Name:          name,
		Version:       version,
		PublisherId:   publisherID,
		Platform:      platform,
		Kind:          kind,
		Checksum:      checksum,
		InstalledUnix: installedUnix,
		UpdatedUnix:   now,
		Status:        status,
		OperationId:   operationID,
		BuildNumber:   buildNumber,
		Metadata:      metadata,
	}

	// Carry receipt fields from existing through the canonical chokepoint.
	// Preserve enforces NEXT-wins on conflict and suppresses
	// migration_source carry-over when pkg.Metadata signals canonical
	// install presence — see installreceipt.Preserve doc.
	installreceipt.Preserve(existing, pkg)

	// Stamp a canonical install receipt. The workflow install path
	// (package.install → service.install_payload → package.report_state)
	// has no other site that stamps installed_by and clears migration_
	// source. Without this, every workflow install leaves the package
	// looking like a legacy_sidecar migration even when the install was
	// completely fresh, because the heartbeat's checkUnitHashDrift seeds
	// migration_source the first time it sees a managed unit with no
	// receipt. See docs/architecture/retire-systemd-sidecars.md.
	//
	// Stamp deletes migration_source on success — that is the chokepoint's
	// contract for "a first-hand install observation has replaced the
	// legacy seed." Best-effort: errors from missing files are logged
	// at the caller and do not fail the report-state action. The receipt
	// is forensic, not authoritative for the action's outcome (the
	// installed_state row is the outcome).
	stampReceiptForReportState(pkg)

	if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
		return "", fmt.Errorf("package.report_state: %w", err)
	}

	return fmt.Sprintf("installed-state written: %s/%s@%s on %s", kind, name, version, nodeID), nil
}

// packageClearStateAction removes an InstalledPackage record from etcd after
// successful uninstall. This is the counterpart to packageReportStateAction.
//
// Plan step args:
//
//	node_id (string, required)
//	name    (string, required)
//	kind    (string, required: "SERVICE", "APPLICATION", "INFRASTRUCTURE")
type packageClearStateAction struct{}

func (packageClearStateAction) Name() string { return "package.clear_state" }

func (packageClearStateAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["node_id"].GetStringValue()) == "" {
		return fmt.Errorf("package.clear_state: node_id is required")
	}
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("package.clear_state: name is required")
	}
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))
	switch kind {
	case "SERVICE", "APPLICATION", "INFRASTRUCTURE":
		// valid
	default:
		return fmt.Errorf("package.clear_state: kind must be SERVICE, APPLICATION, or INFRASTRUCTURE (got %q)", kind)
	}
	return nil
}

func (packageClearStateAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	nodeID := strings.TrimSpace(fields["node_id"].GetStringValue())
	name := strings.TrimSpace(fields["name"].GetStringValue())
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))

	if err := installed_state.DeleteInstalledPackage(ctx, nodeID, kind, name); err != nil {
		return "", fmt.Errorf("package.clear_state: %w", err)
	}

	// Also clean up the service config from etcd so it no longer appears
	// in the admin catalog. Best-effort — don't fail the whole action.
	if err := config.DeleteServiceConfigurationByName(name); err != nil {
		fmt.Printf("package.clear_state: warning: failed to clean service config for %s: %v\n", name, err)
	}

	return fmt.Sprintf("installed-state cleared: %s/%s on %s", kind, name, nodeID), nil
}

// mergeReportStateMetadata is the pure metadata-merge half of
// packageReportStateAction.Apply, extracted so it is testable without
// an etcd backend.
//
// Contract:
//
//  1. Non-receipt fields from existing are copied verbatim (entrypoint_
//     checksum, proof_on_disk_sha256, anything else a sibling writer has
//     stamped that the install-receipt chokepoint does not own).
//  2. Receipt fields from existing are SKIPPED here; they are added by
//     installreceipt.Preserve at the call site so the canonical-install
//     detection rule (migration_source suppression when installed_by is
//     present on next) applies.
//  3. Workflow-arg fields are overlaid last and win over existing on the
//     same key. Fields that are part of the typed action contract
//     (node_id, name, version, kind, publisher_id, platform, checksum,
//     operation_id, status, build_number) are not carried into metadata —
//     they belong on the typed InstalledPackage fields, not the open map.
//
// Returns nil when the resulting map is empty (so the caller can leave
// pkg.Metadata nil rather than writing an empty map that downstream
// readers must special-case).
func mergeReportStateMetadata(existing *node_agentpb.InstalledPackage, fields map[string]*structpb.Value) map[string]string {
	receiptKeys := make(map[string]bool, len(installreceipt.Keys()))
	for _, k := range installreceipt.Keys() {
		receiptKeys[k] = true
	}
	metadata := make(map[string]string)
	if existing != nil {
		for k, v := range existing.GetMetadata() {
			if v == "" || receiptKeys[k] {
				continue
			}
			metadata[k] = v
		}
	}
	for k, v := range fields {
		switch k {
		case "node_id", "name", "version", "kind", "publisher_id", "platform",
			"checksum", "operation_id", "status", "build_number":
			continue
		default:
			if s := v.GetStringValue(); s != "" {
				metadata[k] = s
			}
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

// stampReceiptForReportState is the workflow-path equivalent of the
// main-package stampReceiptForInstalledPackage helper. The sub-package
// cannot import the main-package version (cyclic dep), so this helper
// applies the same conventions inline:
//
//   - unit_file_path : /etc/systemd/system/globular-<name>.service
//   - binary_path    : /usr/lib/globular/bin/<<name with - → _>>_server
//                      (SERVICE), or /usr/lib/globular/bin/<name>
//                      (INFRASTRUCTURE, fallback)
//
// Missing files at conventional paths are silently skipped — a COMMAND
// or INFRASTRUCTURE wrapper may have no systemd unit or no binary. The
// receipt is still committed with whatever evidence WAS found, plus the
// installed_by attribution.
//
// Stamp's own contract handles the migration_source clearance: when
// Stamp succeeds at all, migration_source is deleted from pkg.Metadata.
func stampReceiptForReportState(pkg *node_agentpb.InstalledPackage) {
	if pkg == nil || pkg.GetName() == "" {
		return
	}
	opts := installreceipt.ReceiptOpts{
		InstalledBy:    reportStateInstalledBy,
		PackageSha256:  pkg.GetChecksum(),
		ArtifactDigest: pkg.GetChecksum(),
	}
	unitPath := filepath.Join(reportStateSystemdDir, "globular-"+pkg.GetName()+".service")
	if fi, err := os.Stat(unitPath); err == nil && !fi.IsDir() {
		opts.UnitFilePath = unitPath
	}
	if binPath := conventionalBinaryPath(pkg.GetName(), pkg.GetKind()); binPath != "" {
		if fi, err := os.Stat(binPath); err == nil && !fi.IsDir() {
			opts.BinaryPath = binPath
		}
	}
	_ = installreceipt.Stamp(pkg, opts)
}

// conventionalBinaryPath mirrors installedBinaryPath() from the main
// package without the manifest-aware first probe. The main package
// uses versionutil.ReadEntrypoint as the authoritative source; here we
// use only the naming conventions because the action runs in a context
// where the manifest may not have been written to the conventional
// location yet. The shas the receipt records will agree with disk
// either way — the manifest path only matters when the convention
// guesses wrong (rare; e.g. scylla-manager → scylla_manager). Such
// edge cases keep their existing receipt via Preserve.
func conventionalBinaryPath(name, kind string) string {
	if name == "" {
		return ""
	}
	if strings.EqualFold(kind, "SERVICE") {
		withSuffix := filepath.Join(reportStateBinDir, strings.ReplaceAll(name, "-", "_")+"_server")
		if _, err := os.Stat(withSuffix); err == nil {
			return withSuffix
		}
		return filepath.Join(reportStateBinDir, strings.ReplaceAll(name, "-", "_"))
	}
	return filepath.Join(reportStateBinDir, name)
}

func init() {
	Register(packageReportStateAction{})
	Register(packageClearStateAction{})
}
