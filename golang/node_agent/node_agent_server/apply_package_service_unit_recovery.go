// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.install
// @awareness file_role=service_unit_recovery_for_missing_systemd_unit
// @awareness implements=globular.platform:invariant.install.join_path_must_complete_install_contract
// @awareness implements=globular.platform:invariant.package.install.service_unit_missing_must_not_be_binary_only
// @awareness risk=critical
// @awareness failure_mode=observability.service_unit_missing_classified_binary_only
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	installer "github.com/globulario/globular-installer/pkg/installer"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// recreateServiceUnitFromSpec rebuilds a SERVICE package's missing systemd unit
// by running the package's OWN bundled spec through the shared installer engine.
//
// Why this exists: the SERVICE install path (installPayload) only writes systemd
// units that a package bundles as files under systemd/ in its .tgz. Packages
// built by service-dist instead carry the unit *inline* in the spec's
// install_services step (the .tgz ships specs/<svc>.yaml, not systemd/<unit>),
// so installPayload writes the binary but never the unit. A SERVICE with no unit
// is not "binary-only" — it is a daemon that cannot run. The installer engine
// (the same one INFRASTRUCTURE packages use) executes the full spec, including
// install_services/enable_services/start_services, which materialises the unit.
//
// The engine touches the filesystem + systemd only; it does NOT write the etcd
// installed-state record. The caller (ApplyPackageRelease) remains the single
// installed-state writer (four_layer.layer_has_single_writing_actor), and only
// declares success after its own enable/restart/verify + hash-proof gate.
// isServiceKind reports whether a package kind denotes a long-running daemon
// that MUST have a systemd unit. Only SERVICE qualifies: COMMAND packages are
// CLI binaries and INFRASTRUCTURE packages whose spec sets install_systemd=false
// (etcdctl, mc, rclone, ...) are legitimately unit-less. The missing-unit
// recovery path applies to SERVICE only; everything else stays binary-only.
func isServiceKind(kind string) bool {
	return strings.EqualFold(strings.TrimSpace(kind), "SERVICE")
}

func (srv *NodeAgentServer) recreateServiceUnitFromSpec(ctx context.Context, name, version string) error {
	platform := runtime.GOOS + "_" + runtime.GOARCH
	pkgPath := srv.findLocalPackage(name, version, platform)
	if pkgPath == "" {
		return fmt.Errorf("package archive for %s@%s not found in local package dirs — cannot recover unit", name, version)
	}

	stagingDir, err := installer.ExtractPackageToTemp(pkgPath)
	if err != nil {
		return fmt.Errorf("extract %s for unit recovery: %w", pkgPath, err)
	}
	defer os.RemoveAll(stagingDir)

	ictx, err := installer.NewContext(installer.Options{
		Prefix:         filepath.Dir(globularBinDir),
		StateDir:       "/var/lib/globular",
		ConfigDir:      "/var/lib/globular/services",
		Version:        version,
		StagingDir:     stagingDir,
		Force:          true,
		Verbose:        true,
		NonInteractive: true,
		SkipStart:      true,
		MinioDataDir:   "/var/lib/globular/minio/data",
	})
	if err != nil {
		return fmt.Errorf("installer context for %s unit recovery: %w", name, err)
	}

	log.Printf("apply-package: recovering missing unit for SERVICE %s@%s via bundled spec (installer engine)", name, version)
	if _, err := installer.Install(ictx); err != nil {
		return fmt.Errorf("installer engine failed to apply %s spec: %w", name, err)
	}
	return nil
}

// writeServiceUnitUnrecoverableInstalledState records — and reports — a SERVICE
// apply that left no runnable systemd unit. This is the fail-loud counterpart to
// the binary-only success path: a SERVICE with no unit must NEVER be reported as
// "installed" (meta.half_done_must_not_look_done,
// install.join_path_must_complete_install_contract). Returning a hard failure
// also contracts the failure instead of amplifying it: the workflow surfaces a
// real FAILED (which defers and ultimately abandons, bounded) rather than a
// false "installed" that the reconciler re-attempts forever via the
// installSkipDeniedUnitGone reinstall loop
// (meta.failure_response_must_contract_not_amplify).
func (srv *NodeAgentServer) writeServiceUnitUnrecoverableInstalledState(
	ctx context.Context,
	req *node_agentpb.ApplyPackageReleaseRequest,
	name, kind, version, unitPath string,
	cause error,
) *node_agentpb.ApplyPackageReleaseResponse {
	errMsg := fmt.Sprintf(
		"SERVICE %s installed but has no systemd unit at %s and one could not be created from its package spec: %v",
		name, unitPath, cause)
	log.Printf("apply-package: REJECTED %s/%s@%s — %s", kind, name, version, errMsg)

	_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
		NodeId:      srv.nodeID,
		Name:        name,
		Version:     version,
		Kind:        kind,
		Status:      "failed",
		UpdatedUnix: time.Now().Unix(),
		OperationId: req.GetOperationId(),
		BuildNumber: req.GetBuildNumber(),
		BuildId:     strings.TrimSpace(req.GetBuildId()),
		Metadata:    map[string]string{"error": errMsg, "reason": "service_unit_missing"},
	})

	return serviceUnitUnrecoverableResponse(name, version, errMsg, req.GetOperationId())
}

// serviceUnitUnrecoverableResponse builds the hard-failure reply for a SERVICE
// left without a runnable unit. Pure (no I/O) so the fail-loud contract — never
// Ok/Status="installed" — is unit-testable without etcd.
func serviceUnitUnrecoverableResponse(name, version, errMsg, operationID string) *node_agentpb.ApplyPackageReleaseResponse {
	return &node_agentpb.ApplyPackageReleaseResponse{
		Ok:          false,
		Message:     errMsg,
		PackageName: name,
		Version:     version,
		Status:      "failed",
		ErrorDetail: errMsg,
		OperationId: operationID,
	}
}
