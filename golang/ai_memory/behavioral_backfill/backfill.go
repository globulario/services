// Package behavioral_backfill imports selected existing ai_memory.memories into
// behavioral-memory candidate objects (signals, claims, outcomes, and — only when
// fully specified — proposed principles).
//
// It is the composition bridge between the ai-memory record store and the
// behavioral kernel. It lives OUTSIDE behavioral/ so the generic kernel never
// imports the ai_memory contract — kernel hygiene is preserved.
//
// Backfill is EVIDENCE IMPORT, not authority creation:
//   - never produces PROMOTED_PRINCIPLE / REVOKED / SUPERSEDED / NARROWED
//   - never auto-promotes; a proposed principle is created ONLY when every
//     governance field is explicitly present, otherwise a signal/claim is created
//     and the missing fields are reported
//   - is deterministic + idempotent (stable ids, get-before-write)
//   - defaults to dry-run (no writes) and is always scoped by project+domain
package behavioral_backfill

import (
	"context"
	"fmt"
	"sort"
	"strings"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// defaultConfidence is the conservative confidence stamped on imported signals
// and claims — historical memory is weaker evidence than a live runtime probe.
const defaultConfidence = 0.5

// SourcePrefix namespaces stable backfill source refs.
const SourcePrefix = "ai-memory:"

// claimKeys are the metadata keys that deterministically map to a claim. An
// ambiguous/unknown key is never guessed into a claim.
var claimKeys = []string{"root_cause", "condition", "authority", "evidence", "outcome", "related_to"}

// outcomeStatuses are the only explicit outcome values backfill will trust.
var outcomeStatuses = map[string]bool{"success": true, "failure": true, "blocked": true, "reverted": true}

// convertibleTypes are the memory types backfilled into signals. Ephemeral and
// conversational types are skipped (reported).
var convertibleTypes = map[ai_memorypb.MemoryType]bool{
	ai_memorypb.MemoryType_DEBUG:        true,
	ai_memorypb.MemoryType_DECISION:     true,
	ai_memorypb.MemoryType_ARCHITECTURE: true,
	ai_memorypb.MemoryType_FEEDBACK:     true,
	ai_memorypb.MemoryType_REFERENCE:    true,
}

// MemoryFilter scopes which memories a MemorySource returns.
type MemoryFilter struct {
	Project string
	Limit   int
}

// MemorySource reads existing ai-memory records. Implemented over the
// AiMemoryService RPC (rpc_source.go); faked in tests.
type MemorySource interface {
	Query(ctx context.Context, f MemoryFilter) ([]*ai_memorypb.Memory, error)
}

// Options controls a backfill run. DryRun defaults to true via NewOptions.
type Options struct {
	Project     string
	Domain      string
	DryRun      bool
	Limit       int
	Since       int64                  // only memories created at/after this unix time
	MemoryTypes []ai_memorypb.MemoryType // optional type allowlist (subset of convertibleTypes applies)
	Tags        []string               // optional: memory must carry ALL of these tags
	AgentID     string                 // optional: only memories from this agent
	Overwrite   bool                   // re-write existing PROPOSED rows (never promoted/revoked)
}

// Report is the dry-run / run summary.
type Report struct {
	DryRun          bool
	Scanned         int
	WouldCreate     map[string]int // kind → count (dry-run)
	Created         map[string]int // kind → count (live)
	Skipped         int
	SkipReasons     map[string]int
	Ambiguous       []string
	MissingFields   []PrincipleGap // principle candidates that lacked governance fields
}

// PrincipleGap reports a memory that could have been a principle but lacked fields.
type PrincipleGap struct {
	MemoryID string
	Missing  []string
}

func newReport(dryRun bool) *Report {
	return &Report{DryRun: dryRun, WouldCreate: map[string]int{}, Created: map[string]int{}, SkipReasons: map[string]int{}}
}

func (r *Report) record(kind string, dryRun bool) {
	if dryRun {
		r.WouldCreate[kind]++
	} else {
		r.Created[kind]++
	}
}

func (r *Report) skip(reason string) {
	r.Skipped++
	r.SkipReasons[reason]++
}

// String renders a readable dry-run/run report.
func (r *Report) String() string {
	var b strings.Builder
	mode := "LIVE"
	if r.DryRun {
		mode = "DRY-RUN (no writes)"
	}
	fmt.Fprintf(&b, "behavioral backfill [%s]\n  memories scanned: %d\n", mode, r.Scanned)
	counts := r.Created
	label := "created"
	if r.DryRun {
		counts, label = r.WouldCreate, "would create"
	}
	for _, k := range sortedKeys(counts) {
		fmt.Fprintf(&b, "  %s %s: %d\n", label, k, counts[k])
	}
	fmt.Fprintf(&b, "  skipped: %d\n", r.Skipped)
	for _, k := range sortedKeys(r.SkipReasons) {
		fmt.Fprintf(&b, "    - %s: %d\n", k, r.SkipReasons[k])
	}
	if len(r.MissingFields) > 0 {
		fmt.Fprintf(&b, "  principle candidates missing governance fields: %d\n", len(r.MissingFields))
		for _, g := range r.MissingFields {
			fmt.Fprintf(&b, "    - %s: missing %s\n", g.MemoryID, strings.Join(g.Missing, ", "))
		}
	}
	if len(r.Ambiguous) > 0 {
		fmt.Fprintf(&b, "  ambiguous (skipped): %d\n", len(r.Ambiguous))
	}
	return b.String()
}

// Run executes (or dry-runs) the backfill. It writes via the store port so it
// can be exercised with an in-memory store in tests.
func Run(ctx context.Context, src MemorySource, st store.Store, opts Options) (*Report, error) {
	if opts.Project == "" || opts.Domain == "" {
		return nil, fmt.Errorf("backfill: project and domain are required (never scan unscoped)")
	}
	rep := newReport(opts.DryRun)
	mems, err := src.Query(ctx, MemoryFilter{Project: opts.Project, Limit: opts.Limit})
	if err != nil {
		return nil, fmt.Errorf("backfill: query memories: %w", err)
	}
	for _, m := range mems {
		if !passesFilters(m, opts) {
			continue // pre-scan exclusion, not a per-memory skip
		}
		rep.Scanned++
		bf := &memoryBackfill{opts: opts, st: st, rep: rep}
		if err := bf.process(ctx, m); err != nil {
			return nil, err
		}
	}
	return rep, nil
}

func passesFilters(m *ai_memorypb.Memory, o Options) bool {
	if o.Since > 0 && m.GetCreatedAt() < o.Since {
		return false
	}
	if o.AgentID != "" && m.GetAgentId() != o.AgentID {
		return false
	}
	if len(o.MemoryTypes) > 0 {
		ok := false
		for _, t := range o.MemoryTypes {
			if m.GetType() == t {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(o.Tags) > 0 {
		have := map[string]bool{}
		for _, t := range m.GetTags() {
			have[t] = true
		}
		for _, want := range o.Tags {
			if !have[want] {
				return false
			}
		}
	}
	return true
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
