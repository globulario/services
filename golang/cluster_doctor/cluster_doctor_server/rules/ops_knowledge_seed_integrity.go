package rules

// ops_knowledge_seed_integrity verifies that the operational-knowledge
// seed packed inside the currently-active awareness bundle is intact:
//
//   1. The bundle's manifest.json declares a non-empty
//      ops_knowledge_entries list (the bundle was built with
//      ops-knowledge support — older bundles that predate
//      docs/operational-knowledge/ won't have this).
//
//   2. Every ops-knowledge YAML referenced in the manifest exists on
//      disk under <bundle>/ops-knowledge/ (no payload corruption or
//      missing file).
//
//   3. The canonical SHA256 of each entry recomputed from the on-disk
//      YAML matches the seed_sha256 the manifest declares (no silent
//      mutation between build time and run time).
//
// Why this matters: the seed is the Day-0/Day-1 baseline knowledge an
// AI agent has before runtime memory exists. If the bundle's seed has
// drifted from what it claims, every agent that consumes the bundle
// believes it has correct baseline knowledge but actually has drifted
// content — silent failure mode, exactly the class of bug invariants
// are supposed to catch.
//
// What this rule does NOT check (yet): drift between the bundle and
// what's actually loaded into ai-memory. That would require the
// collector to query ai-memory each cycle and hand the snapshot a list
// of {id, seed_sha256}. Wiring that is a separate, larger change.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/opsknowledge"
)

type opsKnowledgeSeedIntegrity struct{}

func (r opsKnowledgeSeedIntegrity) ID() string       { return "ops_knowledge.seed_integrity" }
func (r opsKnowledgeSeedIntegrity) Category() string { return "awareness" }
func (r opsKnowledgeSeedIntegrity) Scope() string    { return "cluster" }

// awarenessBundleCurrentPath is where node-agent symlinks the active
// bundle. Kept as a var so tests can swap it.
var awarenessBundleCurrentPath = "/var/lib/globular/awareness/current"

func (r opsKnowledgeSeedIntegrity) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	bundleDir := awarenessBundleCurrentPath
	if _, err := os.Stat(bundleDir); err != nil {
		// No active bundle on this node — pre-bootstrap, or the
		// awareness bundle has not been installed yet. Other rules
		// surface bundle absence; we just stay quiet.
		return nil
	}

	manifest, manifestPath, err := readBundleManifest(bundleDir)
	if err != nil {
		return []Finding{{
			FindingID:       FindingID(r.ID(), bundleDir, "manifest_read"),
			InvariantID:     r.ID(),
			Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:        r.Category(),
			EntityRef:       bundleDir,
			Summary:         fmt.Sprintf("awareness bundle manifest unreadable at %s: %v", manifestPath, err),
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
			CheckError:      err.Error(),
		}}
	}

	if len(manifest.OpsKnowledgeEntries) == 0 {
		// Bundle predates the ops-knowledge seed feature, or was built
		// without docs/operational-knowledge/ on disk. Day-0 agents
		// won't have baseline knowledge — actionable, but not fatal.
		return []Finding{{
			FindingID:   FindingID(r.ID(), bundleDir, "no_entries"),
			InvariantID: r.ID(),
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    r.Category(),
			EntityRef:   bundleDir,
			Summary: "active awareness bundle does not pack any operational-knowledge seed entries " +
				"(Day-0/Day-1 AI agents will have no baseline knowledge until the seed is loaded into ai-memory)",
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "ops_knowledge_seed_integrity", map[string]string{
					"bundle_dir":    bundleDir,
					"manifest_path": manifestPath,
					"build_id":      manifest.BuildID,
					"version":       manifest.Version,
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Rebuild the awareness bundle from a tree that contains docs/operational-knowledge/.",
					"globular awareness bundle build --version <next-version>"),
				step(2, "Publish the new bundle and let nodes pick it up via the release BOM.",
					"globular package publish --kind AWARENESS_BUNDLE --file <bundle.tar.gz>"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		}}
	}

	opsRoot := filepath.Join(bundleDir, "ops-knowledge")
	var (
		missing []string
		drifted []string
	)
	for _, declared := range manifest.OpsKnowledgeEntries {
		path := filepath.Join(opsRoot, filepath.FromSlash(declared.FilePath))
		f, err := opsknowledge.LoadFile(path)
		if err != nil {
			missing = append(missing, declared.ID)
			continue
		}
		var found bool
		for _, e := range f.Entries {
			if e.ID != declared.ID {
				continue
			}
			found = true
			actual, herr := opsknowledge.HashEntry(e)
			if herr != nil || actual != declared.SeedSHA256 {
				drifted = append(drifted, declared.ID)
			}
			break
		}
		if !found {
			missing = append(missing, declared.ID)
		}
	}

	// ai-memory drift: compare what ai-memory currently has against what
	// the bundle's manifest declares. Two distinct symptoms:
	//   - memMissing: bundle declares an entry, ai-memory does not have
	//     it (auto-seeder hasn't run yet, or someone deleted it).
	//   - memDrifted: ai-memory has the entry but with a different hash
	//     (manual mutation, or stale row from an older bundle the
	//     auto-seeder hasn't replaced yet).
	// Snapshot.OpsKnowledgeMemoryEntries is nil when the collector did
	// not query ai-memory (no client wired or call failed). Skip the
	// comparison in that case — bundle-side integrity has already been
	// surfaced above.
	var (
		memMissing []string
		memDrifted []string
	)
	if snap != nil && snap.OpsKnowledgeMemoryEntries != nil {
		for _, declared := range manifest.OpsKnowledgeEntries {
			memHash, present := snap.OpsKnowledgeMemoryEntries[declared.ID]
			if !present {
				memMissing = append(memMissing, declared.ID)
				continue
			}
			if memHash != declared.SeedSHA256 {
				memDrifted = append(memDrifted, declared.ID)
			}
		}
	}

	if len(missing) == 0 && len(drifted) == 0 && len(memMissing) == 0 && len(memDrifted) == 0 {
		return nil // healthy
	}

	var findings []Finding

	// Bundle-side integrity finding — the on-disk payload disagrees with
	// the manifest. Remediation is reinstall/rebuild the bundle.
	if len(missing) > 0 || len(drifted) > 0 {
		summary := fmt.Sprintf(
			"operational-knowledge bundle integrity violated: %d missing, %d drifted (out of %d declared)",
			len(missing), len(drifted), len(manifest.OpsKnowledgeEntries))
		kv := map[string]string{
			"bundle_dir":     bundleDir,
			"manifest_path":  manifestPath,
			"build_id":       manifest.BuildID,
			"version":        manifest.Version,
			"declared_count": fmt.Sprintf("%d", len(manifest.OpsKnowledgeEntries)),
			"missing_count":  fmt.Sprintf("%d", len(missing)),
			"drifted_count":  fmt.Sprintf("%d", len(drifted)),
		}
		if len(missing) > 0 {
			kv["missing_sample"] = sampleIDs(missing, 5)
		}
		if len(drifted) > 0 {
			kv["drifted_sample"] = sampleIDs(drifted, 5)
		}
		findings = append(findings, Finding{
			FindingID:   FindingID(r.ID(), bundleDir, fmt.Sprintf("bundle-m%d-d%d", len(missing), len(drifted))),
			InvariantID: r.ID(),
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    r.Category(),
			EntityRef:   bundleDir,
			Summary:     summary,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "ops_knowledge_seed_integrity_bundle", kv),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Inspect the active bundle to confirm what is corrupt or missing.",
					fmt.Sprintf("globular awareness bundle inspect %s", filepath.Dir(awarenessBundleCurrentPath))),
				step(2, "Reinstall the awareness bundle from the repository (clears partial / mutated payloads).",
					"globular package reinstall --kind AWARENESS_BUNDLE"),
				step(3, "If corruption persists, rebuild from source and republish.",
					"globular awareness bundle build --version <next-version>"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// ai-memory-side drift — the bundle is intact but ai-memory disagrees.
	// memMissing alone (no memDrifted) is WARN: the auto-seeder may not
	// have caught up yet (it runs hourly). memDrifted is ERROR because
	// it indicates someone bypassed the immutability layer or wrote
	// directly to scylla.
	if len(memMissing) > 0 || len(memDrifted) > 0 {
		sev := cluster_doctorpb.Severity_SEVERITY_WARN
		if len(memDrifted) > 0 {
			sev = cluster_doctorpb.Severity_SEVERITY_ERROR
		}
		summary := fmt.Sprintf(
			"ai-memory diverges from bundle ops-knowledge seed: %d missing, %d drifted (out of %d declared)",
			len(memMissing), len(memDrifted), len(manifest.OpsKnowledgeEntries))
		kv := map[string]string{
			"bundle_dir":         bundleDir,
			"build_id":           manifest.BuildID,
			"version":            manifest.Version,
			"declared_count":     fmt.Sprintf("%d", len(manifest.OpsKnowledgeEntries)),
			"mem_missing_count":  fmt.Sprintf("%d", len(memMissing)),
			"mem_drifted_count":  fmt.Sprintf("%d", len(memDrifted)),
		}
		if len(memMissing) > 0 {
			kv["mem_missing_sample"] = sampleIDs(memMissing, 5)
		}
		if len(memDrifted) > 0 {
			kv["mem_drifted_sample"] = sampleIDs(memDrifted, 5)
		}
		findings = append(findings, Finding{
			FindingID:   FindingID(r.ID(), "ai_memory", fmt.Sprintf("mem-m%d-d%d", len(memMissing), len(memDrifted))),
			InvariantID: r.ID(),
			Severity:    sev,
			Category:    r.Category(),
			EntityRef:   "ai_memory",
			Summary:     summary,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "ops_knowledge_seed_integrity_memory", kv),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Wait up to 1 hour for ai-memory's auto-seeder, or restart ai-memory to trigger an immediate pass.",
					"globular cluster service restart --service ai-memory"),
				step(2, "If drift persists, force-seed manually as the super-admin.",
					"globular auth login --user sa && globular ops-knowledge seed"),
				step(3, "If a row was overwritten by an unauthorized caller, audit ai-memory writes for the affected ids.",
					""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	return findings
}

// ─── helpers ────────────────────────────────────────────────────────────────

// bundleManifest mirrors the subset of the awareness bundle manifest we
// need to read here. Kept local (not imported from globularcli) to keep
// the doctor's dependency surface small — globularcli is a CLI binary,
// not a library.
type bundleManifest struct {
	Name                string                       `json:"name"`
	BuildID             string                       `json:"build_id"`
	Version             string                       `json:"version"`
	OpsKnowledgeEntries []bundleOpsKnowledgeManifest `json:"ops_knowledge_entries"`
}

type bundleOpsKnowledgeManifest struct {
	ID         string `json:"id"`
	FilePath   string `json:"file_path"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	SeedSHA256 string `json:"seed_sha256"`
}

func sampleIDs(ids []string, max int) string {
	if len(ids) <= max {
		return joinIDs(ids)
	}
	return joinIDs(ids[:max]) + fmt.Sprintf(" (+%d more)", len(ids)-max)
}

func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return out
}
