package deploy

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// b2 of package.release_vs_dev_channel_boundary: a DEV-channel deploy publishes a
// dev-lane artifact but must NOT write cluster desired-state; release channels do.
func TestDeployUpdatesDesiredState_DevSkips(t *testing.T) {
	for _, dev := range []string{"dev", "DEV", " dev ", "Dev"} {
		if deployUpdatesDesiredState(dev) {
			t.Fatalf("channel %q is DEV — deploy must NOT update cluster desired-state", dev)
		}
	}
}

func TestDeployUpdatesDesiredState_ReleaseChannelsWrite(t *testing.T) {
	for _, rel := range []string{"stable", "candidate", "canary", "bootstrap", "", "STABLE"} {
		if !deployUpdatesDesiredState(rel) {
			t.Fatalf("channel %q is a release tier — deploy must update cluster desired-state", rel)
		}
	}
}

func TestNormalizeDeployChannel_DefaultsToCandidate(t *testing.T) {
	if got := normalizeDeployChannel(""); got != "candidate" {
		t.Fatalf("empty channel normalized to %q, want candidate", got)
	}
	if got := normalizeDeployChannel(" CANDIDATE "); got != "candidate" {
		t.Fatalf("candidate normalization = %q, want candidate", got)
	}
}

// local_deploy.must_not_allocate_release_semver:
// the local deploy baseline is the latest strict git release tag AS-IS; it must
// never mint the next patch/minor/major version on its own.
func TestLatestLocalReleaseVersion_UsesLatestTagWithoutBumping(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Codex",
			"GIT_AUTHOR_EMAIL=codex@example.com",
			"GIT_COMMITTER_NAME=Codex",
			"GIT_COMMITTER_EMAIL=codex@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "branch", "-m", "main")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("git", "add", "README.md")
	run("git", "commit", "-m", "init")
	run("git", "tag", "v1.2.255")
	run("git", "tag", "v1.2.256")
	run("git", "tag", "not-a-release")
	run("git", "tag", "v1.2.256-rc1")

	got, err := latestLocalReleaseVersion(repo)
	if err != nil {
		t.Fatalf("latestLocalReleaseVersion returned error: %v", err)
	}
	if got != "1.2.256" {
		t.Fatalf("latestLocalReleaseVersion = %q, want 1.2.256", got)
	}
	if got == "1.2.257" {
		t.Fatal("local deploy must not allocate the next patch version from git tags")
	}
}
