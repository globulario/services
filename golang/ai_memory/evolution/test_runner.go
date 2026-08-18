package evolution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type TestRunOptions struct {
	EnvelopePath string
	WorkspaceDir string
	TestName     string
}

type TestRunResult struct {
	ExitCode     int
	InvocationID string
	EvidencePath string
	Record       TestRecord
	MarkedProven bool
}

// RunDeclaredTest executes exactly the command frozen in the ChangeEnvelope.
// It first verifies that WorkspaceDir is checked out at the candidate revision,
// captures stdout/stderr as evidence, hashes that evidence, records the result,
// and reconciles CANDIDATE/PROVEN. It has no admission or release authority.
func RunDeclaredTest(ctx context.Context, opts TestRunOptions) (TestRunResult, error) {
	if strings.TrimSpace(opts.EnvelopePath) == "" ||
		strings.TrimSpace(opts.WorkspaceDir) == "" ||
		strings.TrimSpace(opts.TestName) == "" {
		return TestRunResult{}, fmt.Errorf("envelope, workspace-dir, and test-name are required")
	}
	envelope, err := LoadChangeEnvelope(opts.EnvelopePath)
	if err != nil {
		return TestRunResult{}, err
	}
	if envelope.Stage != StageCandidate && envelope.Stage != StageProven {
		return TestRunResult{}, fmt.Errorf(
			"test execution requires envelope stage CANDIDATE or PROVEN, got %s",
			envelope.Stage,
		)
	}

	requirement, err := requiredTestByName(envelope.RequiredTests, opts.TestName)
	if err != nil {
		return TestRunResult{}, err
	}
	if requirement.Repository != "" && requirement.Repository != envelope.CandidateRepository {
		return TestRunResult{}, fmt.Errorf(
			"test %q repository %q does not match candidate repository %q",
			requirement.Name,
			requirement.Repository,
			envelope.CandidateRepository,
		)
	}
	if len(requirement.Command) == 0 {
		return TestRunResult{}, fmt.Errorf("test %q has no declared command", requirement.Name)
	}

	invocationID, err := newInvocationID()
	if err != nil {
		return TestRunResult{}, err
	}
	identity := envelope.Identity()
	// A rerun that cannot execute must not leave the previous PASS standing. A
	// missing executable, a cancelled context, an unusable workspace — none of
	// them re-prove anything, so the standing claim for this test is withdrawn
	// and the stage reconciled, exactly as an unsuccessful scenario attempt is.
	fail := func(cause error) (TestRunResult, error) {
		if _, invalidateErr := MutateEnvelope(opts.EnvelopePath, identity, func(e *ChangeEnvelope) {
			kept := e.Tests[:0]
			for _, record := range e.Tests {
				if record.Name == requirement.Name && record.CandidateRevision == e.CandidateRevision {
					continue
				}
				kept = append(kept, record)
			}
			e.Tests = kept
		}); invalidateErr != nil {
			return TestRunResult{}, fmt.Errorf("%w (and could not withdraw the prior proof claim: %v)", cause, invalidateErr)
		}
		return TestRunResult{}, cause
	}

	workspace, err := filepath.Abs(opts.WorkspaceDir)
	if err != nil {
		return fail(fmt.Errorf("resolve workspace: %w", err))
	}
	// Two independent guards, because they answer different questions.
	// The first refuses to proceed while the operator's checkout disagrees with
	// the revision being certified, so contaminated work is reported rather than
	// silently ignored. The second makes the executed contents exactly the
	// candidate commit by construction, so nothing outside that commit — ignored
	// build output, a stale generated file, a mid-run edit — can reach the test.
	if err := requirePristineCandidateWorkspace(ctx, workspace, envelope.CandidateRevision); err != nil {
		return fail(err)
	}
	candidateTree, releaseTree, err := materializeCandidateTree(ctx, workspace, envelope.CandidateRevision)
	if err != nil {
		return fail(err)
	}
	defer releaseTree()
	workDir, err := resolveContainedWorkDir(candidateTree, requirement.WorkDir)
	if err != nil {
		return fail(err)
	}

	cmd := exec.CommandContext(ctx, requirement.Command[0], requirement.Command[1:]...)
	cmd.Dir = workDir
	var captured bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &captured)
	cmd.Stderr = io.MultiWriter(os.Stderr, &captured)
	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			// The command never produced a verdict, so nothing was proven.
			return fail(fmt.Errorf("run declared test %q: %w", requirement.Name, runErr))
		}
	}

	// One invocation, one artifact. A shared <test-name>.log lets two concurrent
	// runs of the same required test write the same file, so one can overwrite it
	// between the other's write and its digest — and a passing run could then
	// record PASS over a failing sibling's output and land as the last mutation,
	// leaving the candidate PROVEN on evidence from an execution that is not the
	// one it names. The scenario runner already owns its output this way; the
	// test leg never inherited it.
	evidenceDir := filepath.Join(
		filepath.Dir(opts.EnvelopePath),
		".evolution-evidence",
		safeEvidenceName(envelope.ID),
		"tests",
		safeEvidenceName(invocationID),
	)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return fail(fmt.Errorf("create test evidence dir: %w", err))
	}
	evidencePath := filepath.Join(evidenceDir, safeEvidenceName(requirement.Name)+".log")
	if err := os.WriteFile(evidencePath, captured.Bytes(), 0o644); err != nil {
		return fail(fmt.Errorf("write test evidence: %w", err))
	}
	// Digest through the same helper the verifier uses. Two different hash
	// framings over the same bytes are two different identities, and evidence
	// that cannot be re-derived exactly as recorded is not evidence.
	digest, err := DigestFiles(evidencePath)
	if err != nil {
		return fail(err)
	}
	result := "FAIL"
	if exitCode == 0 {
		result = "PASS"
	}
	record := TestRecord{
		Name:                requirement.Name,
		CandidateRepository: envelope.CandidateRepository,
		CandidateRevision:   envelope.CandidateRevision,
		PlanDigest:          envelope.PlanDigest,
		InvocationID:        invocationID,
		Command:             append([]string(nil), requirement.Command...),
		Result:              result,
		EvidenceRef:         evidencePath,
		Digest:              digest,
	}
	marked, err := MutateEnvelope(opts.EnvelopePath, identity, func(e *ChangeEnvelope) {
		e.AddOrReplaceTest(record)
	})
	if err != nil {
		return TestRunResult{}, err
	}
	return TestRunResult{
		ExitCode:     exitCode,
		InvocationID: invocationID,
		EvidencePath: evidencePath,
		Record:       record,
		MarkedProven: marked,
	}, nil
}

func requiredTestByName(requirements []TestRequirement, name string) (TestRequirement, error) {
	for _, requirement := range requirements {
		if requirement.Name == name {
			return requirement, nil
		}
	}
	return TestRequirement{}, fmt.Errorf("test %q is not declared in the change envelope", name)
}

// requirePristineCandidateWorkspace refuses a workspace whose contents are not
// exactly the candidate commit. `git rev-parse HEAD` alone is not sufficient:
// HEAD can equal the candidate while tracked edits, staged edits, or untracked
// sources change what a build or test would actually see, which would stamp a
// PASS with a revision that never produced it.
func requirePristineCandidateWorkspace(ctx context.Context, workspace, expected string) error {
	out, err := exec.CommandContext(ctx, "git", "-C", workspace, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("resolve workspace revision: %w", err)
	}
	actual := strings.TrimSpace(string(out))
	if actual != expected {
		return fmt.Errorf("workspace revision %q does not match candidate revision %q", actual, expected)
	}

	status, err := exec.CommandContext(
		ctx, "git", "-C", workspace, "status", "--porcelain=v1", "--untracked-files=all",
	).Output()
	if err != nil {
		return fmt.Errorf("inspect candidate workspace contents: %w", err)
	}
	var staged, tracked, untracked []string
	for _, line := range strings.Split(strings.TrimRight(string(status), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		switch {
		case strings.HasPrefix(line, "??"):
			untracked = append(untracked, path)
		default:
			if line[0] != ' ' && line[0] != '?' {
				staged = append(staged, path)
			}
			if line[1] != ' ' && line[1] != '?' {
				tracked = append(tracked, path)
			}
		}
	}
	if len(staged) == 0 && len(tracked) == 0 && len(untracked) == 0 {
		return nil
	}
	var detail []string
	if len(staged) > 0 {
		detail = append(detail, fmt.Sprintf("staged: %s", strings.Join(limitPaths(staged), ", ")))
	}
	if len(tracked) > 0 {
		detail = append(detail, fmt.Sprintf("modified: %s", strings.Join(limitPaths(tracked), ", ")))
	}
	if len(untracked) > 0 {
		detail = append(detail, fmt.Sprintf("untracked: %s", strings.Join(limitPaths(untracked), ", ")))
	}
	return fmt.Errorf(
		"candidate workspace is not exactly revision %s (%s); commit the work into a candidate revision before proving it",
		expected,
		strings.Join(detail, "; "),
	)
}

func limitPaths(paths []string) []string {
	const max = 5
	if len(paths) <= max {
		return paths
	}
	return append(append([]string(nil), paths[:max]...), fmt.Sprintf("and %d more", len(paths)-max))
}

// materializeCandidateTree checks the candidate revision out into a throwaway
// detached worktree and returns it with its release function. The operator's
// checkout is only read from; nothing in it is modified, cleaned, or deleted.
func materializeCandidateTree(ctx context.Context, workspace, revision string) (string, func(), error) {
	parent, err := os.MkdirTemp("", "evolution-candidate-")
	if err != nil {
		return "", nil, fmt.Errorf("create candidate worktree parent: %w", err)
	}
	tree := filepath.Join(parent, "candidate")
	add := exec.CommandContext(
		ctx, "git", "-C", workspace, "worktree", "add", "--detach", tree, revision,
	)
	if out, err := add.CombinedOutput(); err != nil {
		_ = os.RemoveAll(parent)
		return "", nil, fmt.Errorf(
			"check candidate revision %s out into a clean worktree: %w\n%s",
			revision,
			err,
			strings.TrimSpace(string(out)),
		)
	}
	release := func() {
		remove := exec.Command("git", "-C", workspace, "worktree", "remove", "--force", tree)
		if out, err := remove.CombinedOutput(); err != nil {
			fmt.Fprintf(
				os.Stderr,
				"evolution: release candidate worktree %s: %v\n%s\n",
				tree, err, strings.TrimSpace(string(out)),
			)
		}
		_ = os.RemoveAll(parent)
		_ = exec.Command("git", "-C", workspace, "worktree", "prune").Run()
	}
	return tree, release, nil
}

// resolveContainedWorkDir locates the declared work_dir inside the candidate
// tree and refuses anything that leaves it.
//
// A lexical check alone is not enough. A commit may legitimately contain
// symlinks, so the candidate under test can ship a link at its own declared
// work_dir pointing anywhere on the host. filepath.Join and filepath.Rel never
// touch the filesystem, so such a path looks contained while cmd.Dir follows it
// out — the command would then run against arbitrary external contents and
// still earn PASS evidence stamped with the candidate revision, which is the
// isolation guarantee the detached worktree exists to provide.
//
// So containment is decided on fully resolved paths, on both sides.
func resolveContainedWorkDir(workspace, relative string) (string, error) {
	root, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return "", fmt.Errorf("resolve candidate workspace: %w", err)
	}
	if strings.TrimSpace(relative) == "" || relative == "." {
		return root, nil
	}
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("test work_dir must be relative to candidate workspace")
	}
	candidate := filepath.Join(root, filepath.Clean(relative))
	// Refuse an obviously escaping path first, so a work_dir that does not
	// exist still gets the accurate reason rather than a resolution error.
	if err := requireBeneath(root, candidate); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve test work_dir: %w", err)
	}
	if err := requireBeneath(root, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func requireBeneath(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve test work_dir: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("test work_dir escapes candidate workspace")
	}
	return nil
}

var unsafeEvidenceName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeEvidenceName(value string) string {
	value = unsafeEvidenceName.ReplaceAllString(value, "_")
	value = strings.Trim(value, "._-")
	if value == "" {
		return "unnamed"
	}
	return value
}
