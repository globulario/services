package rules

import (
	"os"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// defaultOpsKnowledgeDir is the installed path for ops-knowledge YAML files.
// The day-0 installer extracts the awareness bundle to this location so the
// doctor can seed ai-memory at day-1 without a separate operator step.
const defaultOpsKnowledgeDir = "/var/lib/globular/awareness/current/ops-knowledge"

// opsKnowledgeSeedDeferred fires when ai-memory is reachable (the collector
// has a client and returned a non-nil OpsKnowledgeMemoryEntries map) but has
// zero seed entries. This is the "day-0 deferred" state: the installer ran
// before ai-memory was available and left a note to seed later.
//
// The healer auto-action (seed_ops_knowledge) loads the ops-knowledge YAML
// files from defaultOpsKnowledgeDir and upserts every entry into ai-memory.
// Idempotent — same id + same sha256 = no-op.
type opsKnowledgeSeedDeferred struct{}

func (opsKnowledgeSeedDeferred) ID() string       { return "ops_knowledge.seed_deferred" }
func (opsKnowledgeSeedDeferred) Category() string { return "ai" }
func (opsKnowledgeSeedDeferred) Scope() string    { return "cluster" }

func (r opsKnowledgeSeedDeferred) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// nil = ai-memory client was not available at collection time (service not
	// registered in etcd or dial failed). Do not fire — we can't distinguish
	// "service running but empty" from "service not running".
	if snap.OpsKnowledgeMemoryEntries == nil {
		return nil
	}
	// Non-empty map = already seeded. Nothing to do.
	if len(snap.OpsKnowledgeMemoryEntries) > 0 {
		return nil
	}
	// Verify the bundle dir exists so the auto-heal action won't fail
	// immediately on a first attempt.
	if _, err := os.Stat(defaultOpsKnowledgeDir); err != nil {
		return []Finding{newOpsKnowledgeDirMissingFinding()}
	}
	return []Finding{newOpsKnowledgeSeedDeferredFinding()}
}

func newOpsKnowledgeSeedDeferredFinding() Finding {
	const id = "ops_knowledge.seed_deferred"
	return Finding{
		FindingID:       FindingID(id, "ai-memory", "seed_entries_zero"),
		InvariantID:     id,
		Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:        "ai",
		EntityRef:       "ai-memory",
		Summary:         "ai-memory is running but operational-knowledge seed entries are missing (day-0 deferred seed not yet applied)",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("ai_memory", "Query(seed)", map[string]string{
				"seed_entry_count": "0",
				"ops_knowledge_dir": defaultOpsKnowledgeDir,
				"reason":           "seed was deferred at day-0 because ai-memory was not yet running",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"The doctor will auto-seed on the next healer cycle (seed_ops_knowledge action). "+
					"To seed manually: globular ops-knowledge seed --dir "+defaultOpsKnowledgeDir,
				"globular ops-knowledge seed --dir "+defaultOpsKnowledgeDir),
		},
	}
}

func newOpsKnowledgeDirMissingFinding() Finding {
	const id = "ops_knowledge.seed_deferred"
	return Finding{
		FindingID:       FindingID(id, "ai-memory", "bundle_dir_missing"),
		InvariantID:     id,
		Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:        "ai",
		EntityRef:       "ai-memory",
		Summary:         "ai-memory has no seed entries and the ops-knowledge bundle directory is missing — manual seed required",
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("ai_memory", "Query(seed)", map[string]string{
				"seed_entry_count":  "0",
				"ops_knowledge_dir": defaultOpsKnowledgeDir,
				"dir_present":       "false",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1,
				"Install the awareness bundle first: globular awareness install <bundle.tar.gz>. "+
					"Then seed: globular ops-knowledge seed --dir "+defaultOpsKnowledgeDir,
				""),
		},
	}
}
