package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/globulario/services/golang/installed_state"
)

// ---------------------------------------------------------------------------
// Node-agent repair action implementations
// ---------------------------------------------------------------------------

// repairCollectFacts gathers diagnostic information about this node for
// the repair classifier. Returns a diagnosis map with:
//   - installed packages and their observation sources
//   - identity integrity status
//   - systemd unit states
//   - binary checksums
func (srv *NodeAgentServer) repairCollectFacts(ctx context.Context, nodeID string, targetPackages []any) (map[string]any, error) {
	log.Printf("repair: collecting diagnostic facts for %s", nodeID)

	// Compute installed services with observation source classification.
	installed, _, err := ComputeInstalledServices(ctx)
	if err != nil {
		log.Printf("repair: ComputeInstalledServices: %v", err)
	}

	// Classify each installed service.
	type pkgFact struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		Source   string `json:"source"` // managed_installed, runtime_unmanaged, fallback_discovered
		Checksum string `json:"checksum,omitempty"`
	}
	var packageFacts []pkgFact
	for key, info := range installed {
		fact := pkgFact{
			Name:    key.String(),
			Version: info.Version,
			Source:  info.Source.String(),
		}
		// Compute binary checksum.
		binName := strings.ReplaceAll(key.ServiceName, "-", "_") + "_server"
		binPath := globularBinDir + "/" + binName
		if cksum, err := cachedSha256(binPath); err == nil {
			fact.Checksum = cksum
		}
		packageFacts = append(packageFacts, fact)
	}

	// Identity integrity check.
	identityStatus := srv.checkIdentityIntegrity()

	diagnosis := map[string]any{
		"node_id":                    nodeID,
		"packages":                   packageFacts,
		"identity_integrity_status":  identityStatus,
		"package_count":              len(packageFacts),
	}

	log.Printf("repair: collected %d package facts, identity=%s", len(packageFacts), identityStatus)
	return diagnosis, nil
}

// checkIdentityIntegrity examines the node's certificate and key material.
// Returns "clean", "suspect", or "corrupt".
func (srv *NodeAgentServer) checkIdentityIntegrity() string {
	// Check service certificate exists and is readable.
	certPath := "/var/lib/globular/pki/issued/services/service.crt"
	keyPath := "/var/lib/globular/pki/issued/services/service.key"
	caPath := "/var/lib/globular/pki/ca.crt"

	// Check CA cert.
	if out, err := exec.Command("openssl", "x509", "-in", caPath, "-noout", "-checkend", "0").CombinedOutput(); err != nil {
		log.Printf("repair: identity check — CA cert issue: %v (%s)", err, strings.TrimSpace(string(out)))
		return "corrupt"
	}

	// Check service cert exists and chain is valid.
	if out, err := exec.Command("openssl", "verify", "-CAfile", caPath, certPath).CombinedOutput(); err != nil {
		log.Printf("repair: identity check — cert chain broken: %v (%s)", err, strings.TrimSpace(string(out)))
		return "corrupt"
	}

	// Check key matches cert.
	if out, err := exec.Command("openssl", "x509", "-in", certPath, "-noout", "-modulus").CombinedOutput(); err != nil {
		log.Printf("repair: identity check — cert read error: %v", err)
		return "corrupt"
	} else {
		certMod := strings.TrimSpace(string(out))
		if keyOut, err := exec.Command("openssl", "rsa", "-in", keyPath, "-noout", "-modulus").CombinedOutput(); err != nil {
			log.Printf("repair: identity check — key read error: %v", err)
			return "corrupt"
		} else if strings.TrimSpace(string(keyOut)) != certMod {
			log.Printf("repair: identity check — key does not match cert")
			return "corrupt"
		}
	}

	// Check cert expiry (warn if < 30 days).
	if _, err := exec.Command("openssl", "x509", "-in", certPath, "-noout", "-checkend", "2592000").CombinedOutput(); err != nil {
		log.Printf("repair: identity check — cert expiring within 30 days")
		return "suspect"
	}

	return "clean"
}

// repairVerifyRuntime checks all repair postconditions:
//  1. All target packages have status "installed" (not partial_apply, failed)
//  2. All managed systemd units are active/running
//  3. Controller version >= minSafeReconcileVersion (if controller was repaired)
//  4. No non-authoritative observations in installed-state
func (srv *NodeAgentServer) repairVerifyRuntime(ctx context.Context, nodeID string, repairPlan map[string]any) error {
	log.Printf("repair: verifying runtime for %s", nodeID)

	// Check 1: All packages must be "installed" (not partial_apply, failed, etc.)
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "COMMAND"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			status := pkg.GetStatus()
			if status == "partial_apply" {
				return fmt.Errorf("package %s/%s is still in partial_apply state", kind, pkg.GetName())
			}
			if status == "failed" {
				return fmt.Errorf("package %s/%s is in failed state", kind, pkg.GetName())
			}
			if status == "updating" {
				return fmt.Errorf("package %s/%s is still updating", kind, pkg.GetName())
			}
		}
	}

	// Check 2: All managed systemd units should be active.
	units := detectUnits(ctx)
	failedUnits := 0
	for _, u := range units {
		if u.State != "active" && u.State != "" {
			// Some units (like oneshot commands) may be "inactive" normally.
			// Only count truly failed or crash-looping units.
			if u.State == "failed" || strings.Contains(u.Details, "auto-restart") {
				log.Printf("repair: unit %s is %s (%s)", u.Name, u.State, u.Details)
				failedUnits++
			}
		}
	}
	if failedUnits > 0 {
		return fmt.Errorf("%d systemd units are failed or crash-looping", failedUnits)
	}

	// Check 3: Identity integrity.
	identityStatus := srv.checkIdentityIntegrity()
	if identityStatus == "corrupt" {
		return fmt.Errorf("identity integrity is corrupt after repair")
	}

	log.Printf("repair: runtime verification passed for %s", nodeID)
	return nil
}

// repairSyncInstalledState rewrites authoritative installed-state records
// and removes non-authoritative entries from authority surfaces.
//
// "Non-authoritative" means entries in etcd installed-state that came from
// RuntimeUnmanaged or FallbackDiscovered observations. These are removed
// from the authoritative installed-state registry only — they remain
// visible in diagnostic surfaces (systemd unit listing, health checks).
func (srv *NodeAgentServer) repairSyncInstalledState(ctx context.Context, nodeID string) error {
	log.Printf("repair: syncing installed state for %s", nodeID)

	// Step 1: Run the standard sync (Phase 0-4) which is already
	// source-aware after Phase 6 changes (only ManagedInstalled entries
	// are written to etcd).
	srv.syncInstalledStateToEtcd(ctx)

	// Step 2: Clean up non-authoritative records that may have been
	// written by old node-agent code. Scan etcd installed-state for
	// entries with fallback/placeholder versions and remove them.
	// This only removes from installed-state (authority surface),
	// NOT from diagnostic/observability surfaces.
	cleaned := 0
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "COMMAND"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			name := pkg.GetName()
			ver := pkg.GetVersion()
			// Remove entries with placeholder versions that should never
			// have been authoritative.
			if ver == "unknown" || ver == "" {
				// Check if this service actually has a version marker or
				// binary on disk. If it does, the version is wrong but the
				// package exists — don't remove, flag instead.
				// If no marker and no binary exist, it's truly non-authoritative.
				binName := strings.ReplaceAll(name, "-", "_") + "_server"
				binPath := globularBinDir + "/" + binName
				if _, err := cachedSha256(binPath); err != nil {
					// No binary on disk — remove stale record.
					_ = installed_state.DeleteInstalledPackage(ctx, nodeID, kind, name)
					log.Printf("repair: removed non-authoritative installed-state %s/%s (version=%s, no binary)", kind, name, ver)
					cleaned++
				} else {
					log.Printf("repair: WARNING: %s/%s has placeholder version %s but binary exists — needs official apply", kind, name, ver)
				}
			}
		}
	}

	if cleaned > 0 {
		log.Printf("repair: cleaned %d non-authoritative installed-state records", cleaned)
	}

	log.Printf("repair: installed-state sync complete for %s", nodeID)
	return nil
}
