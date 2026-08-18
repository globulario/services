package evolution

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDeclaredTestRecordsEvidenceForExactRevision(t *testing.T) {
	workspace, revision := initTestRepo(t)
	envelopePath := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-test-run", ChangeFeature, "feature", revision, RiskLow)
	e.RequiredTests = []TestRequirement{{
		Name:       "declared-test",
		Repository: "globulario/services",
		Command:    []string{"sh", "-c", "echo trusted-proof"},
		Required:   true,
	}}
	if err := e.BindCandidate("globulario/services", revision); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}

	result, err := RunDeclaredTest(context.Background(), TestRunOptions{
		RunnerKeyPath: testRunnerKey(t),
		EnvelopePath:  envelopePath,
		WorkspaceDir:  workspace,
		TestName:      "declared-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || !result.MarkedProven || result.Record.Result != "PASS" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Record.Digest == "" || result.Record.EvidenceRef == "" {
		t.Fatalf("missing evidence identity: %+v", result.Record)
	}
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Stage != StageProven || len(stored.Tests) != 1 {
		t.Fatalf("unexpected stored envelope: %+v", stored)
	}
}

func TestRunDeclaredTestRejectsWrongCheckoutRevision(t *testing.T) {
	workspace, _ := initTestRepo(t)
	envelopePath := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-wrong-rev", ChangeFeature, "feature", "different-sha", RiskLow)
	e.RequiredTests = []TestRequirement{{
		Name:     "declared-test",
		Command:  []string{"sh", "-c", "true"},
		Required: true,
	}}
	if err := e.BindCandidate("globulario/services", "different-sha"); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}
	_, err := RunDeclaredTest(context.Background(), TestRunOptions{
		RunnerKeyPath: testRunnerKey(t),
		EnvelopePath:  envelopePath,
		WorkspaceDir:  workspace,
		TestName:      "declared-test",
	})
	if err == nil || !strings.Contains(err.Error(), "does not match candidate revision") {
		t.Fatalf("expected revision rejection, got %v", err)
	}
}

func TestFailedRerunDowngradesProvenCandidate(t *testing.T) {
	workspace, revision := initTestRepo(t)
	// The flag lives outside the candidate workspace on purpose: flipping the
	// test result must not require contaminating the checkout, which is itself
	// now a refusal.
	flag := filepath.Join(t.TempDir(), "force-test-failure")
	envelopePath := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-rerun", ChangeFeature, "feature", revision, RiskLow)
	e.RequiredTests = []TestRequirement{{
		Name:     "flippable",
		Command:  []string{"sh", "-c", "test ! -f " + flag},
		Required: true,
	}}
	if err := e.BindCandidate("globulario/services", revision); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}
	first, err := RunDeclaredTest(context.Background(), TestRunOptions{
		RunnerKeyPath: testRunnerKey(t),
		EnvelopePath:  envelopePath,
		WorkspaceDir:  workspace,
		TestName:      "flippable",
	})
	if err != nil || !first.MarkedProven {
		t.Fatalf("first run should prove candidate: result=%+v err=%v", first, err)
	}
	if err := os.WriteFile(flag, []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := RunDeclaredTest(context.Background(), TestRunOptions{
		RunnerKeyPath: testRunnerKey(t),
		EnvelopePath:  envelopePath,
		WorkspaceDir:  workspace,
		TestName:      "flippable",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ExitCode == 0 || second.MarkedProven {
		t.Fatalf("second run should fail proof: %+v", second)
	}
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Stage != StageCandidate {
		t.Fatalf("failed rerun must downgrade to CANDIDATE, got %s", stored.Stage)
	}
}

func TestResolveContainedWorkDirRejectsEscape(t *testing.T) {
	root := t.TempDir()
	if _, err := resolveContainedWorkDir(root, "../outside"); err == nil {
		t.Fatal("expected work_dir escape rejection")
	}
}

func initTestRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "evolution-test@example.invalid")
	runGit(t, dir, "config", "user.name", "Evolution Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "candidate")
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	return dir, strings.TrimSpace(string(out))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmdArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func gitOutput(dir string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", cmdArgs...).Output()
	return string(out), err
}
