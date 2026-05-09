package semanticdiff

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

var (
	// Direct layer assignment: lhs = rhs where lhs and rhs are in different layers
	reAssignment = regexp.MustCompile(`(\w[\w.]*)\s*=\s*([\w.]+)`)

	// Guard patterns on removed lines
	reGenerationCompare = regexp.MustCompile(`(?i)(generation|revision)\s*!=|!=\s*(generation|revision)|checkGeneration|generationMatch|etcdRevision`)
	reHealthGate        = regexp.MustCompile(`(?i)(\.health\b|healthy\b|isHealthy|backendHealthy|workflowHealthy|circuitOpen|circuit\.Open|backendReady|\.Ready\b)`)
	reVerifyCheck       = regexp.MustCompile(`(?i)(verify|validate|checksum|digest|sha256|receipt|signatureCheck)`)
	reCASCheck          = regexp.MustCompile(`(?i)(\.cas\b|\.compare\b|compareAndSwap|txn\.If|revision\s*==|etcdRevision)`)
	reBackoff           = regexp.MustCompile(`(?i)(backoff|exponential|jitter)`)
	reTerminal          = regexp.MustCompile(`(?i)(terminal|deadletter|ErrFatal|StatusFailed\b|FAILED_TERMINAL|failedTerminal)`)
	reCircuit           = regexp.MustCompile(`(?i)(circuit|breaker|circuitOpen|isOpen\b)`)

	// Transaction atomicity
	reTxn            = regexp.MustCompile(`(?i)(\.txn\b|Txn\.|transaction\.begin|atomically|\.Then\(|etcd\.Txn|kv\.Txn)`)
	reSequentialWrite = regexp.MustCompile(`(?i)(putInstalled|setInstalled|updateDesired|writeRuntime|saveState|commitResult|deleteAction|promote\w*\()`)

	// Fallback authority
	reFallback = regexp.MustCompile(`(?i)(fallback|lastKnown|cachedState|installedFallback|heartbeatFallback|staleState)`)

	// Health gate added (positive signal)
	reHealthGateAdded = regexp.MustCompile(`(?i)if\s+.*?(healthy|health|ready|readiness|backendOK)`)
)

// ExtractSemanticAtoms extracts semantic change atoms from a parsed diff.
func ExtractSemanticAtoms(parsed *ParsedDiff) []SemanticDiffAtom {
	var atoms []SemanticDiffAtom
	for _, f := range parsed.Files {
		for _, h := range f.Hunks {
			atoms = append(atoms, extractFromHunk(f.Path, h.Symbol, h)...)
		}
	}
	return atoms
}

func extractFromHunk(filePath, symbol string, h *DiffHunk) []SemanticDiffAtom {
	var atoms []SemanticDiffAtom

	addedText := strings.Join(h.AddedLines, "\n")
	removedText := strings.Join(h.RemovedLines, "\n")

	// 1. Direct layer assignment in added lines
	for _, line := range h.AddedLines {
		if a := detectLayerAssignment(filePath, symbol, line); a != nil {
			atoms = append(atoms, *a)
		}
	}

	// 2. Fallback promoted to authority (added lines)
	for _, line := range h.AddedLines {
		if reFallback.MatchString(line) && reSequentialWrite.MatchString(line) {
			atoms = append(atoms, SemanticDiffAtom{
				ID:            "AT-" + uuid.New().String()[:8],
				FilePath:      filePath,
				Symbol:        symbol,
				AtomKind:      "fallback_promoted_to_authority",
				BeforeSummary: "State was updated from authoritative source.",
				AfterSummary:  "State is now derived from a fallback or cached value.",
				Confidence:    "high",
				Evidence:      strings.TrimSpace(line),
			})
		}
	}

	// 3. Generation/revision compare removed
	if reGenerationCompare.MatchString(removedText) && !reGenerationCompare.MatchString(addedText) {
		evidence := firstMatch(reGenerationCompare, h.RemovedLines)
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "generation_compare_removed",
			BeforeSummary: "Transition required generation/revision equality.",
			AfterSummary:  "Generation/revision check no longer required.",
			Confidence:    "high",
			Evidence:      evidence,
		})
	}

	// 4. Health gate removed
	if reHealthGate.MatchString(removedText) && !reHealthGate.MatchString(addedText) &&
		!reHealthGateAdded.MatchString(addedText) {
		evidence := firstMatch(reHealthGate, h.RemovedLines)
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "health_gate_removed",
			BeforeSummary: "Operation required healthy/ready precondition.",
			AfterSummary:  "Health gate no longer checked before proceeding.",
			Confidence:    "medium",
			Evidence:      evidence,
		})
	}

	// 5. Health gate added (positive)
	if reHealthGateAdded.MatchString(addedText) && !reHealthGate.MatchString(removedText) {
		evidence := firstMatch(reHealthGateAdded, h.AddedLines)
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "health_gate_added",
			BeforeSummary: "No health gate before operation.",
			AfterSummary:  "Operation now requires health check.",
			Confidence:    "medium",
			Evidence:      evidence,
		})
	}

	// 6. Verification weakened
	if reVerifyCheck.MatchString(removedText) && !reVerifyCheck.MatchString(addedText) {
		evidence := firstMatch(reVerifyCheck, h.RemovedLines)
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "verification_weakened",
			BeforeSummary: "Operation included verification/validation step.",
			AfterSummary:  "Verification step removed.",
			Confidence:    "medium",
			Evidence:      evidence,
		})
	}

	// 7. CAS / compare-and-swap removed
	if reCASCheck.MatchString(removedText) && !reCASCheck.MatchString(addedText) {
		evidence := firstMatch(reCASCheck, h.RemovedLines)
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "generation_compare_removed",
			BeforeSummary: "State write required compare-and-swap / revision guard.",
			AfterSummary:  "CAS / revision guard removed.",
			Confidence:    "high",
			Evidence:      evidence,
		})
	}

	// 8. Backoff weakened
	if reBackoff.MatchString(removedText) && !reBackoff.MatchString(addedText) {
		evidence := firstMatch(reBackoff, h.RemovedLines)
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "backoff_weakened",
			BeforeSummary: "Retry loop used backoff / jitter.",
			AfterSummary:  "Backoff strategy removed.",
			Confidence:    "medium",
			Evidence:      evidence,
		})
	}

	// 9. Circuit breaker removed
	if reCircuit.MatchString(removedText) && !reCircuit.MatchString(addedText) {
		evidence := firstMatch(reCircuit, h.RemovedLines)
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "health_gate_removed",
			BeforeSummary: "Operation guarded by circuit breaker.",
			AfterSummary:  "Circuit breaker guard removed.",
			Confidence:    "high",
			Evidence:      evidence,
		})
	}

	// 10. Terminal failure removed
	if reTerminal.MatchString(removedText) && !reTerminal.MatchString(addedText) {
		evidence := firstMatch(reTerminal, h.RemovedLines)
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "terminal_failure_removed",
			BeforeSummary: "Operation had a terminal failure path.",
			AfterSummary:  "Terminal failure state removed — retry may be unbounded.",
			Confidence:    "medium",
			Evidence:      evidence,
		})
	}

	// 11. Transaction atomicity removed (txn → sequential writes)
	hasTxnRemoved := reTxn.MatchString(removedText)
	hasSeqWriteAdded := reSequentialWrite.MatchString(addedText)
	hasTxnAdded := reTxn.MatchString(addedText)
	hasSeqWriteRemoved := reSequentialWrite.MatchString(removedText)

	if hasTxnRemoved && hasSeqWriteAdded && !hasTxnAdded {
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "state_transition_atomicity_removed",
			BeforeSummary: "State changes committed as one atomic transaction.",
			AfterSummary:  "State changes split across sequential writes — partial commit risk.",
			Confidence:    "high",
			Evidence:      firstMatch(reTxn, h.RemovedLines),
		})
	}

	// 12. Transaction atomicity added (sequential writes → txn)
	if hasTxnAdded && hasSeqWriteRemoved && !hasTxnRemoved {
		atoms = append(atoms, SemanticDiffAtom{
			ID:            "AT-" + uuid.New().String()[:8],
			FilePath:      filePath,
			Symbol:        symbol,
			AtomKind:      "state_transition_atomicity_added",
			BeforeSummary: "State changes were split across sequential writes.",
			AfterSummary:  "State changes now committed as one atomic transaction.",
			Confidence:    "high",
			Evidence:      firstMatch(reTxn, h.AddedLines),
		})
	}

	return atoms
}

// detectLayerAssignment detects direct layer-collapsing assignments in an added line.
// e.g. `installed.State = desired.State` → Desired→Installed (forbidden)
func detectLayerAssignment(filePath, symbol, line string) *SemanticDiffAtom {
	m := reAssignment.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	lhs, rhs := m[1], m[2]
	lhsLayer := IdentifyLayer(lhs)
	rhsLayer := IdentifyLayer(rhs)
	if lhsLayer == LayerUnknown || rhsLayer == LayerUnknown || lhsLayer == rhsLayer {
		return nil
	}
	if !ForbiddenTransition(rhsLayer, lhsLayer) {
		return nil
	}
	kind, reason := TransitionKind(rhsLayer, lhsLayer)
	return &SemanticDiffAtom{
		ID:            "AT-" + uuid.New().String()[:8],
		FilePath:      filePath,
		Symbol:        symbol,
		AtomKind:      kind,
		BeforeSummary: fmt.Sprintf("%s layer state was updated from authoritative source.", lhsLayer),
		AfterSummary:  fmt.Sprintf("%s state is derived directly from %s layer.", lhsLayer, rhsLayer),
		Confidence:    "high",
		Evidence:      fmt.Sprintf("%s (lhs=%s layer=%s, rhs=%s layer=%s): %s", strings.TrimSpace(line), lhs, lhsLayer, rhs, rhsLayer, reason),
	}
}

// firstMatch returns the first line matching re, trimmed.
func firstMatch(re *regexp.Regexp, lines []string) string {
	for _, l := range lines {
		if re.MatchString(l) {
			return strings.TrimSpace(l)
		}
	}
	return ""
}
