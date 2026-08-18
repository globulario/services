package evolution

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

	workspace, err := filepath.Abs(opts.WorkspaceDir)
	if err != nil {
		return TestRunResult{}, fmt.Errorf("resolve workspace: %w", err)
	}
	if err := requireWorkspaceRevision(ctx, workspace, envelope.CandidateRevision); err != nil {
		return TestRunResult{}, err
	}
	workDir, err := resolveContainedWorkDir(workspace, requirement.WorkDir)
	if err != nil {
		return TestRunResult{}, err
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
			return TestRunResult{}, fmt.Errorf("run declared test %q: %w", requirement.Name, runErr)
		}
	}

	evidenceDir := filepath.Join(
		filepath.Dir(opts.EnvelopePath),
		".evolution-evidence",
		safeEvidenceName(envelope.ID),
		"tests",
	)
	if err := os.MkdirAll(evidenceDir, 0o755); err != nil {
		return TestRunResult{}, fmt.Errorf("create test evidence dir: %w", err)
	}
	evidencePath := filepath.Join(evidenceDir, safeEvidenceName(requirement.Name)+".log")
	if err := os.WriteFile(evidencePath, captured.Bytes(), 0o644); err != nil {
		return TestRunResult{}, fmt.Errorf("write test evidence: %w", err)
	}
	sum := sha256.Sum256(captured.Bytes())
	result := "FAIL"
	if exitCode == 0 {
		result = "PASS"
	}
	record := TestRecord{
		Name:                requirement.Name,
		CandidateRepository: envelope.CandidateRepository,
		CandidateRevision:   envelope.CandidateRevision,
		Command:             append([]string(nil), requirement.Command...),
		Result:              result,
		EvidenceRef:         evidencePath,
		Digest:              "sha256:" + hex.EncodeToString(sum[:]),
	}
	envelope.AddOrReplaceTest(record)
	marked := envelope.ReconcileProofStage()
	if err := SaveChangeEnvelope(opts.EnvelopePath, envelope); err != nil {
		return TestRunResult{}, err
	}
	return TestRunResult{
		ExitCode:     exitCode,
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

func requireWorkspaceRevision(ctx context.Context, workspace, expected string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("resolve workspace revision: %w", err)
	}
	actual := strings.TrimSpace(string(out))
	if actual != expected {
		return fmt.Errorf("workspace revision %q does not match candidate revision %q", actual, expected)
	}
	return nil
}

func resolveContainedWorkDir(workspace, relative string) (string, error) {
	if strings.TrimSpace(relative) == "" || relative == "." {
		return workspace, nil
	}
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("test work_dir must be relative to candidate workspace")
	}
	candidate := filepath.Join(workspace, filepath.Clean(relative))
	rel, err := filepath.Rel(workspace, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve test work_dir: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("test work_dir escapes candidate workspace")
	}
	return candidate, nil
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
