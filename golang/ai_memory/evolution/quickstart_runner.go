package evolution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ProofInvocation is the identity of exactly one scenario execution. The runner
// mints it, hands it to the harness, and refuses any artifact that does not
// carry it back. It is what makes "this proof came from this run" checkable
// rather than assumed.
type ProofInvocation struct {
	ID     string `json:"id"`
	RunDir string `json:"run_dir,omitempty"`
}

type QuickstartProofArtifact struct {
	Scenario       string          `json:"scenario"`
	Suite          string          `json:"suite"`
	SourceRevision string          `json:"source_revision"`
	Status         string          `json:"status"`
	Change         ChangeBinding   `json:"change"`
	Invocation     ProofInvocation `json:"invocation"`
	Execution      struct {
		Result        string `json:"result"`
		ProofEligible bool   `json:"proof_eligible"`
	} `json:"execution"`
}

type QuickstartRunOptions struct {
	// RunnerKeyPath is the trusted runner's signing key; see TestRunOptions.
	RunnerKeyPath string
	QuickstartDir string
	Scenario      string
	// ScenarioName is the required-scenario name this run answers for. It is
	// what a stale proof would otherwise be credited against, so the runner
	// needs it to invalidate that claim when this invocation proves nothing.
	ScenarioName  string
	EnvelopePath  string
	KeepArtifacts bool
	Verbose       bool
}

type QuickstartRunResult struct {
	ExitCode int
	// Requirement is the frozen obligation this run answered. The CLI needs it
	// to ask certification's question rather than its own — comparing the proof
	// against itself always agrees.
	Requirement ScenarioRequirement
	// ChangeID is carried so a caller ingesting the learning artifact can bind
	// it to this exact proof occurrence without reopening the envelope.
	ChangeID     string
	InvocationID string
	ProofPath    string
	LearningPath string
	Proof        ProofRecord
	MarkedProven bool
}

// RunQuickstartScenario executes one proof-boundary scenario against an exact
// candidate revision and frozen proof plan, then persists the resulting proof
// into its ChangeEnvelope. Required local/static tests must already be green.
//
// The proof it consumes is the one this invocation produced, in a directory this
// invocation created, stamped with an identity this invocation minted. It never
// reads the shared `tests/reports/latest` pointer: that pointer names whichever
// run rotated it last, so a failing rerun that exits before rotating it, or a
// concurrent run for the same change, could otherwise hand back an earlier PASS.
func RunQuickstartScenario(ctx context.Context, opts QuickstartRunOptions) (QuickstartRunResult, error) {
	if strings.TrimSpace(opts.QuickstartDir) == "" ||
		strings.TrimSpace(opts.Scenario) == "" ||
		strings.TrimSpace(opts.EnvelopePath) == "" {
		return QuickstartRunResult{}, fmt.Errorf("quickstart-dir, scenario, and envelope path are required")
	}
	envelope, err := LoadChangeEnvelope(opts.EnvelopePath)
	if err != nil {
		return QuickstartRunResult{}, err
	}
	if envelope.Stage != StageCandidate && envelope.Stage != StageProven {
		return QuickstartRunResult{}, fmt.Errorf(
			"quickstart proof requires envelope stage CANDIDATE or PROVEN, got %s",
			envelope.Stage,
		)
	}
	if envelope.CandidateRepository == "" || envelope.CandidateRevision == "" || envelope.PlanDigest == "" {
		return QuickstartRunResult{}, fmt.Errorf("candidate repository/revision/plan digest are required before simulation")
	}
	if err := envelope.ValidateRequiredTestClosure(); err != nil {
		return QuickstartRunResult{}, fmt.Errorf("local proof gate: %w", err)
	}

	scenarioName := strings.TrimSpace(opts.ScenarioName)
	if scenarioName == "" {
		scenarioName = strings.TrimSuffix(filepath.Base(opts.Scenario), filepath.Ext(opts.Scenario))
	}

	requirement, err := requiredScenarioByName(envelope.RequiredScenarios, scenarioName)
	if err != nil {
		return QuickstartRunResult{}, err
	}
	// The plan digest covers each obligation's repository and path, so the file
	// about to run must be the file the plan froze. Another file declaring the
	// same scenario name — what a moved or copied scenario leaves behind — would
	// otherwise discharge the obligation without executing the plan-digested
	// path. Checked before launching, not after the artifact comes back.
	if err := requireFrozenScenarioPath(opts.QuickstartDir, requirement, opts.Scenario); err != nil {
		return QuickstartRunResult{}, err
	}

	identity := envelope.Identity()

	// From here on the invocation owns a proof slot. Any path that fails to fill
	// it with a valid current proof must drop whatever older claim stood for this
	// scenario, or a failed rerun would leave the durable envelope at PROVEN.
	fail := func(runResult QuickstartRunResult, cause error) (QuickstartRunResult, error) {
		if invalidateErr := invalidateScenarioProof(opts.EnvelopePath, identity, scenarioName); invalidateErr != nil {
			return runResult, fmt.Errorf("%w (and could not withdraw the prior proof claim: %v)", cause, invalidateErr)
		}
		return runResult, cause
	}

	// Closure above is metadata only. Re-derive the artifacts behind it before
	// anything destructive starts, or the gate passes on paper while the evidence
	// it names no longer exists. Refusing to launch is not enough on its own: an
	// envelope already at PROVEN would keep reporting PROVEN from metadata while
	// the evidence under it is gone, so the claim is withdrawn through the locked
	// path before the gate error is returned.
	if gateErr := envelope.VerifyRequiredTestEvidence(); gateErr != nil {
		if err := withdrawUnreproducibleProof(opts.EnvelopePath, identity); err != nil {
			return QuickstartRunResult{ChangeID: envelope.ID}, fmt.Errorf(
				"local proof gate: %w (and could not withdraw the PROVEN claim: %v)", gateErr, err,
			)
		}
		return QuickstartRunResult{ChangeID: envelope.ID}, fmt.Errorf("local proof gate: %w", gateErr)
	}

	// The simulator is the other half of the experiment, so it needs the same
	// exactness as the candidate. A quickstart checkout can carry tracked,
	// staged, or untracked edits while still reporting a clean HEAD, which would
	// let modified simulator code earn proof attributed to the committed
	// revision. Executing from a throwaway detached worktree makes the simulator
	// contents exactly that revision by construction:
	//
	//     exact candidate revision × exact simulator revision = proof occurrence
	simulationRevision, err := resolveHeadRevision(ctx, opts.QuickstartDir)
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, err)
	}
	// Observed, never asserted. The runner used to stamp the canonical simulator
	// name onto every record regardless of what it actually executed, which made
	// the later "does the proof repository match the frozen requirement" check
	// vacuous — a fabricated constant compared against a constant. What is
	// recorded now is what this checkout says it is, and empty when that cannot
	// be established, so admission can see an unproven simulator identity rather
	// than inheriting a claim nobody checked.
	simulationRepository := observeRepositoryIdentity(ctx, opts.QuickstartDir)
	simTree, releaseSimTree, err := materializeCandidateTree(ctx, opts.QuickstartDir, simulationRevision)
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, fmt.Errorf("materialize simulator revision: %w", err))
	}
	defer releaseSimTree()

	testBin := filepath.Join(simTree, "tests", "harness", "bin", "globular-test")
	if _, err := os.Stat(testBin); err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, fmt.Errorf("quickstart proof runner unavailable: %w", err))
	}
	scenarioPath, err := rebaseScenarioPath(opts.QuickstartDir, simTree, opts.Scenario)
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, err)
	}

	invocationID, err := newInvocationID()
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, err)
	}
	runDir, err := createInvocationRunDir(opts.EnvelopePath, envelope.ID, invocationID)
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, err)
	}

	args := []string{"scenario", scenarioPath}
	if opts.KeepArtifacts {
		args = append(args, "--keep-artifacts")
	}
	if opts.Verbose {
		args = append(args, "--verbose")
	}
	cmd := exec.CommandContext(ctx, testBin, args...)
	cmd.Dir = simTree
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(
		os.Environ(),
		"GLOBULAR_CHANGE_ID="+envelope.ID,
		"GLOBULAR_CHANGE_ENVELOPE_REF="+opts.EnvelopePath,
		"GLOBULAR_CANDIDATE_REPOSITORY="+envelope.CandidateRepository,
		"GLOBULAR_CANDIDATE_REVISION="+envelope.CandidateRevision,
		"GLOBULAR_CHANGE_PLAN_DIGEST="+envelope.PlanDigest,
		"GLOBULAR_REQUIRE_CHANGE_BINDING=1",
		"GLOBULAR_PROOF_RUN_DIR="+runDir,
		"GLOBULAR_PROOF_INVOCATION_ID="+invocationID,
	)

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return fail(
				QuickstartRunResult{ChangeID: envelope.ID, InvocationID: invocationID},
				fmt.Errorf("run quickstart scenario: %w", runErr),
			)
		}
	}

	proofPath := filepath.Join(runDir, "scenario-proof.json")
	learningPath := filepath.Join(runDir, "learning.json")
	partial := QuickstartRunResult{
		ExitCode:     exitCode,
		Requirement:  requirement,
		ChangeID:     envelope.ID,
		InvocationID: invocationID,
		ProofPath:    proofPath,
		LearningPath: learningPath,
	}

	// A run that did not succeed cannot certify, whatever it wrote. A harness can
	// emit a valid PASS artifact and then exit nonzero — teardown or artifact
	// cleanup failing after the scenario itself passed — and recording that would
	// leave the durable envelope PROVEN off a run the operator was told failed.
	if exitCode != 0 {
		// The run does not certify, but it is still an occurrence worth learning
		// from. Carry its identity and verdict when the artifact is present and
		// consistent, so the failure that stops the plan can be ingested — marked
		// explicitly ineligible, and never recorded into the envelope.
		observed, obsErr := loadQuickstartProof(proofPath)
		if obsErr == nil {
			obsErr = observed.requireDescribesOccurrence(envelope, invocationID, scenarioName, simulationRevision)
		}
		if obsErr == nil {
			partial.Proof = ProofRecord{
				Scenario:            observed.Scenario,
				Repository:          simulationRepository,
				SimulationRevision:  observed.SourceRevision,
				CandidateRepository: observed.Change.CandidateRepository,
				CandidateRevision:   observed.Change.CandidateRevision,
				PlanDigest:          observed.Change.PlanDigest,
				InvocationID:        invocationID,
				Result:              observed.Execution.Result,
				ProofEligible:       false,
			}
		}
		return fail(partial, fmt.Errorf(
			"quickstart scenario exited %d; a run that did not succeed cannot certify a candidate",
			exitCode,
		))
	}

	artifact, err := loadQuickstartProof(proofPath)
	if err != nil {
		return fail(partial, err)
	}
	if err := artifact.requireDescribesOccurrence(envelope, invocationID, scenarioName, simulationRevision); err != nil {
		return fail(partial, err)
	}
	proof := ProofRecord{
		Scenario:            artifact.Scenario,
		Repository:          simulationRepository,
		SimulationRevision:  artifact.SourceRevision,
		CandidateRepository: artifact.Change.CandidateRepository,
		CandidateRevision:   artifact.Change.CandidateRevision,
		PlanDigest:          artifact.Change.PlanDigest,
		InvocationID:        invocationID,
		Result:              artifact.Execution.Result,
		ProofEligible:       artifact.Execution.ProofEligible && artifact.Status == "SUPPORTED",
		ProofRef:            proofPath,
	}
	evidencePath := filepath.Join(runDir, "evidence.json")
	if _, statErr := os.Stat(evidencePath); statErr == nil {
		proof.EvidenceRef = evidencePath
	}
	// The digest is taken over exactly the artifacts the record names, in the
	// order VerifyEvidenceArtifacts recomputes them.
	digest, err := DigestFiles(proof.evidenceArtifacts()...)
	if err != nil {
		return fail(partial, fmt.Errorf("digest scenario proof artifacts: %w", err))
	}
	proof.Digest = digest

	if opts.RunnerKeyPath != "" {
		key, keyErr := LoadRunnerKey(opts.RunnerKeyPath)
		if keyErr != nil {
			return fail(partial, fmt.Errorf("load runner key: %w", keyErr))
		}
		receipt, signErr := ProofOccurrenceReceipt{
			ChangeID:            envelope.ID,
			CandidateRepository: envelope.CandidateRepository,
			CandidateRevision:   envelope.CandidateRevision,
			PlanDigest:          envelope.PlanDigest,
			ObligationKind:      "scenario",
			ObligationName:      scenarioName,
			ObligationRef:       requirement.Path,
			InvocationID:        invocationID,
			SimulationRevision:  simulationRevision,
			Result:              proof.Result,
			ProofEligible:       proof.ProofEligible,
			EvidenceDigest:      proof.Digest,
			ObservedAt:          time.Now().UTC().Format(time.RFC3339Nano),
		}.Sign(key)
		if signErr != nil {
			return fail(partial, fmt.Errorf("attest scenario occurrence: %w", signErr))
		}
		proof.Receipt = &receipt
	}

	marked, err := MutateEnvelope(opts.EnvelopePath, identity, func(e *ChangeEnvelope) {
		e.AddOrReplaceProof(proof)
	})
	if err != nil {
		return partial, err
	}

	return QuickstartRunResult{
		ExitCode:     exitCode,
		Requirement:  requirement,
		ChangeID:     envelope.ID,
		InvocationID: invocationID,
		ProofPath:    proofPath,
		LearningPath: learningPath,
		Proof:        proof,
		MarkedProven: marked,
	}, nil
}

func newInvocationID() (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("generate proof invocation id: %w", err)
	}
	return fmt.Sprintf(
		"inv-%s-%s",
		time.Now().UTC().Format("20060102T150405Z"),
		hex.EncodeToString(entropy[:]),
	), nil
}

// createInvocationRunDir creates the directory this invocation owns.
//
// It sits beside the envelope's other evidence, deliberately outside both the
// candidate and the simulator checkouts. The simulator now executes from a
// throwaway worktree that is discarded afterwards, so anything written inside it
// would not survive to be verified; and the real quickstart checkout is not a
// scratch space for proof output either.
//
// It must not already exist: an existing directory could hold a sibling run's
// artifacts, and consuming those is the ambiguity this mechanism removes.
func createInvocationRunDir(envelopePath, changeID, invocationID string) (string, error) {
	base := filepath.Join(
		filepath.Dir(envelopePath),
		".evolution-evidence",
		safeEvidenceName(changeID),
		"scenarios",
	)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("create proof evidence dir: %w", err)
	}
	runDir := filepath.Join(base, safeEvidenceName(invocationID))
	if err := os.Mkdir(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create proof invocation run dir: %w", err)
	}
	abs, err := filepath.Abs(runDir)
	if err != nil {
		return "", fmt.Errorf("resolve proof invocation run dir: %w", err)
	}
	return abs, nil
}

// invalidateScenarioProof withdraws any standing proof claim for one required
// scenario at the current candidate revision and reconciles the stage. It only
// ever removes a claim; it never asserts a result this invocation cannot show.
func invalidateScenarioProof(envelopePath string, identity EnvelopeIdentity, scenario string) error {
	_, err := MutateEnvelope(envelopePath, identity, func(e *ChangeEnvelope) {
		kept := e.Proofs[:0]
		for _, proof := range e.Proofs {
			if proof.Scenario == scenario && proof.CandidateRevision == e.CandidateRevision {
				continue
			}
			kept = append(kept, proof)
		}
		e.Proofs = kept
	})
	return err
}

func loadQuickstartProof(path string) (QuickstartProofArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return QuickstartProofArtifact{}, fmt.Errorf("read quickstart proof: %w", err)
	}
	var proof QuickstartProofArtifact
	if err := json.Unmarshal(data, &proof); err != nil {
		return QuickstartProofArtifact{}, fmt.Errorf("decode quickstart proof: %w", err)
	}
	if proof.Scenario == "" || proof.SourceRevision == "" || proof.Execution.Result == "" {
		return QuickstartProofArtifact{}, fmt.Errorf(
			"quickstart proof missing scenario/source_revision/execution.result",
		)
	}
	if strings.TrimSpace(proof.Invocation.ID) == "" {
		return QuickstartProofArtifact{}, fmt.Errorf(
			"quickstart proof carries no invocation identity; the harness must stamp the invocation that produced it",
		)
	}
	return proof, nil
}

// CertifiesRequirement reports whether this run produced a record that could
// stand as evidence for its obligation, using the same predicate validation
// uses. Callers deciding an exit status must not re-derive that answer.
//
// It compares against the *frozen* requirement. Building the expectation out of
// the record being judged — Repository taken from the proof itself — makes the
// comparison vacuous: a foreign checkout's claim agrees with itself, so the
// command exited zero for a run whose contradiction with the plan had already
// stopped it certifying and left the envelope at CANDIDATE.
func (r QuickstartRunResult) CertifiesRequirement(name string) error {
	requirement := r.Requirement
	if strings.TrimSpace(requirement.Name) == "" {
		requirement.Name = name
	}
	if strings.TrimSpace(requirement.Name) == "" {
		requirement.Name = r.Proof.Scenario
	}
	return r.Proof.certifies(requirement)
}

func resolveHeadRevision(ctx context.Context, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("resolve simulator revision in %q: %w", dir, err)
	}
	revision := strings.TrimSpace(string(out))
	if revision == "" {
		return "", fmt.Errorf("simulator checkout %q has no resolvable revision", dir)
	}
	return revision, nil
}

// rebaseScenarioPath maps a scenario named against the operator's quickstart
// checkout onto the detached worktree that is actually executing. Running the
// committed harness against an uncommitted scenario file would reintroduce the
// contamination the worktree exists to prevent.
func rebaseScenarioPath(quickstartDir, simTree, scenario string) (string, error) {
	root, err := filepath.Abs(quickstartDir)
	if err != nil {
		return "", fmt.Errorf("resolve quickstart dir: %w", err)
	}
	target := scenario
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("resolve scenario path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("scenario %q is outside the quickstart checkout", scenario)
	}
	rebased := filepath.Join(simTree, rel)
	if _, err := os.Lstat(rebased); err != nil {
		return "", fmt.Errorf(
			"scenario %q is not present at simulator revision: %w; commit it before proving against it",
			rel, err,
		)
	}
	// A committed scenario path may itself be a symlink. Following it out of the
	// detached tree would execute mutable external contents while stamping the
	// frozen commit as the simulation revision — the same escape already closed
	// for a declared work_dir, which this path never inherited.
	treeRoot, err := filepath.EvalSymlinks(simTree)
	if err != nil {
		return "", fmt.Errorf("resolve simulator tree: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(rebased)
	if err != nil {
		return "", fmt.Errorf("resolve scenario path at simulator revision: %w", err)
	}
	if relResolved, err := filepath.Rel(treeRoot, resolved); err != nil {
		return "", fmt.Errorf("resolve scenario path: %w", err)
	} else if relResolved == ".." || strings.HasPrefix(relResolved, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf(
			"scenario %q resolves outside the simulator revision; proof would describe content absent from that commit",
			rel,
		)
	}
	return resolved, nil
}

func requiredScenarioByName(requirements []ScenarioRequirement, name string) (ScenarioRequirement, error) {
	for _, requirement := range requirements {
		if requirement.Name == name {
			return requirement, nil
		}
	}
	return ScenarioRequirement{}, fmt.Errorf(
		"scenario %q is not declared in the change envelope; there is no obligation for it to discharge",
		name,
	)
}

// requireFrozenScenarioPath compares the requested file against the one the plan
// digest covers. Both sides are resolved relative to the quickstart checkout so
// an absolute and a relative spelling of the same file agree.
func requireFrozenScenarioPath(quickstartDir string, requirement ScenarioRequirement, requested string) error {
	if requirement.Repository != "" && requirement.Repository != "globulario/globular-quickstart" {
		return fmt.Errorf(
			"scenario %q is declared against repository %q, which this runner does not execute",
			requirement.Name,
			requirement.Repository,
		)
	}
	if strings.TrimSpace(requirement.Path) == "" {
		return fmt.Errorf(
			"required scenario %q declares no path, so no file can be shown to be the frozen one",
			requirement.Name,
		)
	}
	root, err := filepath.Abs(quickstartDir)
	if err != nil {
		return fmt.Errorf("resolve quickstart dir: %w", err)
	}
	resolve := func(p string) string {
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		return filepath.Clean(p)
	}
	frozen := resolve(requirement.Path)
	if resolve(requested) != frozen {
		return fmt.Errorf(
			"scenario %q is frozen in the plan as %q, but %q was requested; "+
				"proving a different file would not discharge the declared obligation",
			requirement.Name,
			requirement.Path,
			requested,
		)
	}
	return nil
}

// requireDescribesOccurrence is the single acceptance predicate for a proof
// artifact. Both the certifying path and the failed-occurrence path ask it, so
// there is one definition of "this artifact describes the run we just executed".
//
// Two predicates drift: the failed-occurrence path was written separately and
// omitted the candidate repository and the executed simulator revision, which
// would have let a learning artifact be ingested under a repository or simulator
// revision that never ran.
func (a QuickstartProofArtifact) requireDescribesOccurrence(
	e ChangeEnvelope, invocationID, scenarioName, simulationRevision string,
) error {
	if a.Invocation.ID != invocationID {
		return fmt.Errorf(
			"proof invocation %q does not belong to this run %q; refusing a proof this invocation did not produce",
			a.Invocation.ID, invocationID,
		)
	}
	// A stamped, otherwise valid artifact that answers a different obligation
	// would land under the other scenario's name and leave this scenario's
	// earlier record standing, so closure would stay green on evidence this
	// invocation never produced.
	if a.Scenario != scenarioName {
		return fmt.Errorf(
			"proof answers scenario %q, not the requested %q; a proof for another obligation cannot satisfy this one",
			a.Scenario, scenarioName,
		)
	}
	if a.Change.ID != e.ID {
		return fmt.Errorf("proof change id %q does not match envelope %q", a.Change.ID, e.ID)
	}
	if a.Change.CandidateRepository != e.CandidateRepository ||
		a.Change.CandidateRevision != e.CandidateRevision {
		return fmt.Errorf(
			"proof candidate %s@%s does not match envelope %s@%s",
			a.Change.CandidateRepository, a.Change.CandidateRevision,
			e.CandidateRepository, e.CandidateRevision,
		)
	}
	if a.Change.PlanDigest != e.PlanDigest {
		return fmt.Errorf("proof plan digest %q does not match envelope %q", a.Change.PlanDigest, e.PlanDigest)
	}
	if a.SourceRevision != simulationRevision {
		return fmt.Errorf(
			"proof simulation revision %q is not the simulator revision this run executed %q",
			a.SourceRevision, simulationRevision,
		)
	}
	if a.Change.SimulationRevision != a.SourceRevision {
		return fmt.Errorf(
			"proof simulation revision %q does not match proof source revision %q",
			a.Change.SimulationRevision, a.SourceRevision,
		)
	}
	return nil
}

// observeRepositoryIdentity reports the repository a checkout says it is, as
// "owner/name", or empty when that cannot be established.
//
// It never guesses. A mirror, an airgapped clone, or a bare test fixture has no
// remote to speak of, and inventing a name for those would recreate exactly the
// fabrication this replaced. Empty is a truthful answer that admission can weigh;
// a confident wrong answer is not.
func observeRepositoryIdentity(ctx context.Context, dir string) string {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return normalizeRepositoryURL(strings.TrimSpace(string(out)))
}

// normalizeRepositoryURL reduces the shapes git accepts for one repository —
// ssh, https, with or without .git — to the single "owner/name" form records are
// compared on. Two spellings of one repository must not read as two identities.
func normalizeRepositoryURL(raw string) string {
	if raw == "" {
		return ""
	}
	value := strings.TrimSuffix(raw, ".git")
	if idx := strings.Index(value, "://"); idx >= 0 {
		value = value[idx+3:]
		if at := strings.Index(value, "@"); at >= 0 {
			value = value[at+1:]
		}
	} else if at := strings.Index(value, "@"); at >= 0 {
		value = value[at+1:]
	}
	value = strings.ReplaceAll(value, ":", "/")
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}
