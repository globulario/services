package preflight

// contextnav_adapter.go — translates a populated *Report into the input
// view consumed by analysis/contextnav. The adapter lives in preflight (not
// in contextnav) so contextnav has no preflight dependency; the import
// arrow runs preflight → contextnav, never the reverse.
//
// The adapter is intentionally narrow: copy the fields contextnav already
// understands, nothing more. Later phases (3+ owner/pivot/falsifier work)
// may add fields to BuildInputs — when they do, this adapter is the one
// place that needs an update.

import (
	"context"

	"github.com/globulario/services/golang/awareness/analysis/contextnav"
	"github.com/globulario/services/golang/awareness/graph"
)

func buildContextnavInputs(ctx context.Context, r *Report, g *graph.Graph, task string, files []string) contextnav.BuildInputs {
	if r == nil {
		return contextnav.BuildInputs{}
	}
	in := contextnav.BuildInputs{
		Invariants:     append([]string(nil), r.Invariants...),
		FailureModes:   append([]string(nil), r.FailureModes...),
		ForbiddenFixes: append([]string(nil), r.ForbiddenFixes...),
		RequiredTests:  append([]string(nil), r.RequiredTests...),
		MatchedAliases: append([]string(nil), r.MatchedAliases...),
		Confidence:     contextnav.Confidence(string(r.Confidence)),
		Graph:          g,
		Ctx:            ctx,
		Task:           task,
		Files:          append([]string(nil), files...),
	}
	for _, raw := range r.RawKnowledgeMatches {
		in.RawKnowledge = append(in.RawKnowledge, contextnav.RawKnowledgeRef{
			Source:       raw.Source,
			Kind:         raw.Kind,
			ID:           raw.ID,
			MatchedTerms: append([]string(nil), raw.MatchedTerms...),
		})
	}
	if r.Runtime != nil {
		in.Runtime = contextnav.RuntimeRef{
			MatchedFailureModes: append([]string(nil), r.Runtime.MatchedFailureModes...),
			MatchedInvariants:   append([]string(nil), r.Runtime.MatchedInvariants...),
		}
	}
	if r.GraphFreshness != nil {
		in.GraphFreshnessKnown = true
		in.GraphStale = r.GraphFreshness.Stale
	}
	if r.LiveOverlay != nil {
		in.LiveOverlayStatus = r.LiveOverlay.Status
	}
	return in
}
