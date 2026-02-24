package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Config holds thresholds and flags passed to invariant Evaluate calls.
type Config struct {
	HeartbeatStale time.Duration
	EmitAuditEvents bool
}

// Finding is the internal (pre-proto) representation of a single invariant result.
type Finding struct {
	FindingID       string
	InvariantID     string
	Severity        clusterdoctorpb.Severity
	Category        string
	EntityRef       string
	Summary         string
	Evidence        []*clusterdoctorpb.Evidence
	Remediation     []*clusterdoctorpb.RemediationStep
	InvariantStatus clusterdoctorpb.InvariantStatus
}

// ToProto converts a Finding to its protobuf representation.
func (f Finding) ToProto() *clusterdoctorpb.Finding {
	return &clusterdoctorpb.Finding{
		FindingId:       f.FindingID,
		InvariantId:     f.InvariantID,
		Severity:        f.Severity,
		Category:        f.Category,
		EntityRef:       f.EntityRef,
		Summary:         f.Summary,
		Evidence:        f.Evidence,
		Remediation:     f.Remediation,
		InvariantStatus: f.InvariantStatus,
	}
}

// Invariant is the interface every rule must implement.
type Invariant interface {
	ID() string       // stable identifier, e.g. "node.reachable"
	Category() string // e.g. "availability", "drift", "systemd", "plan"
	Scope() string    // "cluster" | "node" | "service"
	Evaluate(snap *collector.Snapshot, cfg Config) []Finding
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// FindingID computes a deterministic 16-char hex ID from the invariant id,
// entity reference, and primary evidence key. Stable across snapshots for the
// same condition, so ExplainFinding works reliably.
func FindingID(invariantID, entityRef, primaryKey string) string {
	raw := fmt.Sprintf("%s:%s:%s", invariantID, entityRef, primaryKey)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:8])
}

// kvEvidence builds a single Evidence message from a service/rpc name + key-value pairs.
func kvEvidence(service, rpc string, kv map[string]string) *clusterdoctorpb.Evidence {
	return &clusterdoctorpb.Evidence{
		SourceService: service,
		SourceRpc:     rpc,
		KeyValues:     kv,
		Timestamp:     timestamppb.Now(),
	}
}

// step builds a RemediationStep.
func step(order uint32, desc, cli string) *clusterdoctorpb.RemediationStep {
	return &clusterdoctorpb.RemediationStep{
		Order:       order,
		Description: desc,
		CliCommand:  cli,
	}
}
