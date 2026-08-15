package cluster_operator

import (
	"fmt"

	cok "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/opsknowledge"
	"gopkg.in/yaml.v3"
)

// RecallSeedEntries returns the flat operational-knowledge recall entries
// embedded in the binary (compiled from docs/operational-knowledge by the
// opscompile command into generated/recall.generated.yaml). ai-memory self-seeds
// these into the memories table at startup. Returns nil when the artifact is
// absent (e.g. an older build that predates the recall compile step).
func RecallSeedEntries() ([]cok.RecallEntry, error) {
	//go:uncertainty:declared-failsafe reads an embed.FS, so the only failure is "artifact not compiled into this build" — absence genuinely IS the answer, not a runtime uncertainty. Triaged 2026-08-14.
	data, err := generatedFS.ReadFile("generated/" + cok.FileRecall)
	if err != nil {
		// Absent artifact is not an error — there is simply nothing to self-seed.
		return nil, nil
	}
	var entries []cok.RecallEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", cok.FileRecall, err)
	}
	return entries, nil
}
