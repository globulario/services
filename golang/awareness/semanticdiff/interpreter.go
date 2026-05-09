package semanticdiff

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// InterpretSemanticDiff runs the full semantic diff pipeline.
func InterpretSemanticDiff(ctx context.Context, req SemanticDiffRequest) (*SemanticDiffReport, error) {
	diffText := req.DiffText
	if diffText == "" && req.GitBase != "" {
		// Caller should have populated DiffText from git diff. We don't exec git here.
		return nil, fmt.Errorf("semanticdiff: diff_text is empty — provide diff text or call from_git which populates it")
	}

	parsed, err := ParseDiff(diffText)
	if err != nil {
		return nil, fmt.Errorf("semanticdiff: parse diff: %w", err)
	}

	atoms := ExtractSemanticAtoms(parsed)
	transitions := InferLayerTransitions(atoms)
	findings := AtomsToFindings(atoms, transitions)

	report := &SemanticDiffReport{
		ID:          "SDIFF-" + uuid.New().String()[:8],
		SessionID:   req.SessionID,
		Task:        req.Task,
		DiffSource:  req.DiffSource,
		GitBase:     req.GitBase,
		GitHead:     req.GitHead,
		Fingerprint: parsed.Fingerprint,
		Atoms:       atoms,
		Transitions: transitions,
		Findings:    findings,
		CreatedAt:   time.Now().Unix(),
	}

	report.Verdict, report.Severity, report.Summary = EvaluateSemanticVerdict(report)
	return report, nil
}

// InferLayerTransitions derives explicit layer transition records from atoms.
func InferLayerTransitions(atoms []SemanticDiffAtom) []LayerTransition {
	var transitions []LayerTransition
	for _, a := range atoms {
		lt := atomToLayerTransition(a)
		if lt != nil {
			transitions = append(transitions, *lt)
		}
	}
	return transitions
}

func atomToLayerTransition(a SemanticDiffAtom) *LayerTransition {
	// Map atom kind to layer transition.
	switch a.AtomKind {
	case "desired_state_promoted_to_installed_without_proof":
		return &LayerTransition{
			FilePath:       a.FilePath,
			Symbol:         a.Symbol,
			LayerFrom:      LayerDesired,
			LayerTo:        LayerInstalled,
			TransitionKind: a.AtomKind,
			Allowed:        false,
			Reason:         "Desired may not write Installed without apply proof.",
		}
	case "runtime_state_promoted_to_desired":
		return &LayerTransition{
			FilePath:       a.FilePath,
			Symbol:         a.Symbol,
			LayerFrom:      LayerRuntime,
			LayerTo:        LayerDesired,
			TransitionKind: a.AtomKind,
			Allowed:        false,
			Reason:         "Runtime may not directly rewrite Desired.",
		}
	case "artifact_metadata_treated_as_installed":
		return &LayerTransition{
			FilePath:       a.FilePath,
			Symbol:         a.Symbol,
			LayerFrom:      LayerArtifact,
			LayerTo:        LayerInstalled,
			TransitionKind: a.AtomKind,
			Allowed:        false,
			Reason:         "Artifact metadata may not produce Installed state without resolved build_id and install action.",
		}
	case "state_transition_atomicity_added":
		return &LayerTransition{
			FilePath:       a.FilePath,
			Symbol:         a.Symbol,
			TransitionKind: a.AtomKind,
			Allowed:        true,
			Reason:         "Atomicity strengthened — partial commit risk reduced.",
		}
	case "state_transition_atomicity_removed":
		return &LayerTransition{
			FilePath:       a.FilePath,
			Symbol:         a.Symbol,
			TransitionKind: a.AtomKind,
			Allowed:        false,
			Reason:         "Atomicity removed — partial commit risk introduced.",
		}
	}
	return nil
}

// AtomsToFindings converts semantic atoms into structured findings.
func AtomsToFindings(atoms []SemanticDiffAtom, _ []LayerTransition) []SemanticDiffFinding {
	var findings []SemanticDiffFinding
	seen := map[string]bool{} // deduplicate by kind+file

	for _, a := range atoms {
		dedupeKey := a.AtomKind + "|" + a.FilePath
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true

		f := atomToFinding(a)
		findings = append(findings, f)
	}
	return findings
}

func atomToFinding(a SemanticDiffAtom) SemanticDiffFinding {
	f := SemanticDiffFinding{
		ID:       "FND-" + uuid.New().String()[:8],
		Kind:     a.AtomKind,
		FilePath: a.FilePath,
		Symbol:   a.Symbol,
		Evidence: a.Evidence,
	}
	switch a.AtomKind {
	case "desired_state_promoted_to_installed_without_proof":
		f.Severity = SeverityForbidden
		f.LayerFrom = LayerDesired
		f.LayerTo = LayerInstalled
		f.Message = "Forbidden state layer collapse: Desired state directly writes Installed state without apply proof."
		f.Recommendation = "Keep Desired as intent. Promote Installed only through controller-owned apply result commit with generation match."
	case "runtime_state_promoted_to_desired":
		f.Severity = SeverityForbidden
		f.LayerFrom = LayerRuntime
		f.LayerTo = LayerDesired
		f.Message = "Forbidden: Runtime observation directly rewrites Desired state."
		f.Recommendation = "Runtime may produce drift findings only. Desired must be written through an explicit controller decision."
	case "artifact_metadata_treated_as_installed":
		f.Severity = SeverityForbidden
		f.LayerFrom = LayerArtifact
		f.LayerTo = LayerInstalled
		f.Message = "Forbidden: Artifact metadata used to produce Installed state without install action."
		f.Recommendation = "Installed state requires resolved build_id, install action completion, and authoritative commit."
	case "installed_state_treated_as_desired":
		f.Severity = SeverityCritical
		f.LayerFrom = LayerInstalled
		f.LayerTo = LayerDesired
		f.Message = "Installed state directly modifies Desired without controller authority."
		f.Recommendation = "Desired state is owned by the controller. Installed observations may inform drift detection only."
	case "generation_compare_removed":
		f.Severity = SeverityCritical
		f.Message = "Verification weakened: generation/revision compare removed before state transition."
		f.Recommendation = "Restore generation equality check to prevent stale desired generation from producing installed state."
	case "health_gate_removed":
		f.Severity = SeverityCritical
		f.Message = "Runtime safety weakened: health/ready gate removed before dispatch or state write."
		f.Recommendation = "Restore health precondition to prevent operations during known-unhealthy backend state."
	case "health_gate_added":
		f.Severity = SeverityInfo
		f.Message = "Runtime safety strengthened: health gate added before operation."
	case "verification_weakened":
		f.Severity = SeverityWarning
		f.Message = "Verification weakened: validation, checksum, or receipt check removed."
		f.Recommendation = "Restore verification step to maintain proof of correctness."
	case "checksum_validation_removed":
		f.Severity = SeverityCritical
		f.Message = "Checksum validation removed — artifact integrity no longer verified."
		f.Recommendation = "Restore checksum/digest validation before using artifact."
	case "receipt_bypassed":
		f.Severity = SeverityCritical
		f.Message = "Receipt requirement bypassed — operation can proceed without proof of completion."
		f.Recommendation = "Restore receipt check as proof that prior step completed successfully."
	case "state_transition_atomicity_removed":
		f.Severity = SeverityCritical
		f.Message = "State transition atomicity removed — partial commit risk introduced."
		f.Recommendation = "Restore atomic transaction to prevent partial state writes."
	case "state_transition_atomicity_added":
		f.Severity = SeverityInfo
		f.Message = "State transition atomicity strengthened — partial commit risk reduced."
	case "fallback_promoted_to_authority":
		f.Severity = SeverityForbidden
		f.Message = "Forbidden: Fallback/cached state promoted to authoritative write."
		f.Recommendation = "Fallback reads may inform recovery but must not become authoritative state. Write only through owning controller path."
	case "backoff_weakened":
		f.Severity = SeverityWarning
		f.Message = "Retry backoff strategy removed or weakened."
		f.Recommendation = "Restore backoff to prevent tight retry loops on transient failures."
	case "terminal_failure_removed":
		f.Severity = SeverityWarning
		f.Message = "Terminal failure state removed — retry may become unbounded."
		f.Recommendation = "Restore terminal failure path to prevent infinite retry on non-recoverable errors."
	default:
		f.Severity = SeverityWarning
		f.Message = fmt.Sprintf("Semantic change detected: %s", strings.ReplaceAll(a.AtomKind, "_", " "))
	}
	return f
}
