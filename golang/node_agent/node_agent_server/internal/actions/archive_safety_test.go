package actions

// archive_safety_test.go — the regression envelope for root-privileged package
// extraction.
//
// Every test here drives the REAL serviceInstallPayloadAction. None of them use
// a parallel test extractor: the defect these tests pin down (an entry named
// `data/../../../x` written as root outside the state root) was invisible for
// as long as it was, precisely because nothing exercised the shipping installer
// against a hostile archive.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

type archiveEntry struct {
	name     string
	body     string
	typeflag byte
	linkname string
	mode     int64
	size     int64 // when > 0, overrides the declared size (for limit tests)
}

func regularEntry(name, body string) archiveEntry {
	return archiveEntry{name: name, body: body, typeflag: tar.TypeReg, mode: 0o644}
}

// buildArchive writes entries in the given ORDER. Order matters: the
// validate-before-mutate guarantee is only observable when a safe entry
// precedes an unsafe one.
func buildArchive(t *testing.T, path string, entries []archiveEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		flag := e.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		size := int64(len(e.body))
		if e.size > 0 {
			size = e.size
		}
		if flag != tar.TypeReg {
			size = 0
		}
		if err := tw.WriteHeader(&tar.Header{
			Name: e.name, Mode: mode, Size: size, Typeflag: flag, Linkname: e.linkname,
		}); err != nil {
			t.Fatal(err)
		}
		if flag == tar.TypeReg && e.body != "" {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// installRoots points every node-owned destination at a temp tree and returns
// the tree root, so a test can assert that nothing was written outside it.
func installRoots(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// The state root is deliberately nested so `../../..` from it still lands
	// inside the tree we can inspect — the escape must be observable.
	state := filepath.Join(root, "escape", "guard", "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}

	oldBin, oldSystemd, oldConfig, oldState, oldPolicy, oldSkip, oldStaging, oldReload :=
		ActionBinDir, ActionSystemdDir, ActionConfigDir, ActionStateDir, ActionPolicyDir,
		ActionSkipSystemd, ActionStagingRoot, ActionSkipDaemonReload
	t.Cleanup(func() {
		ActionBinDir, ActionSystemdDir, ActionConfigDir, ActionStateDir, ActionPolicyDir,
			ActionSkipSystemd, ActionStagingRoot, ActionSkipDaemonReload =
			oldBin, oldSystemd, oldConfig, oldState, oldPolicy, oldSkip, oldStaging, oldReload
	})

	ActionBinDir = filepath.Join(root, "bin")
	ActionSystemdDir = filepath.Join(root, "systemd")
	ActionConfigDir = filepath.Join(root, "etc")
	ActionStateDir = state
	ActionPolicyDir = filepath.Join(root, "policy")
	ActionStagingRoot = filepath.Join(root, "staging")
	ActionSkipSystemd = false
	ActionSkipDaemonReload = true
	for _, d := range []string{ActionBinDir, ActionSystemdDir, ActionConfigDir, ActionPolicyDir, ActionStagingRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func applyInstallPayload(t *testing.T, service, artifact string) (string, error) {
	t.Helper()
	args, err := structpb.NewStruct(map[string]interface{}{
		"service": service, "artifact_path": artifact, "version": "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	return serviceInstallPayloadAction{}.Apply(context.Background(), args)
}

// ── containment ─────────────────────────────────────────────────────────────

func TestInstallPayloadRejectsTraversalOutsideEveryDestinationRoot(t *testing.T) {
	// One case per prefix that maps an archive-relative suffix onto a host root.
	// bin/ and systemd/ use filepath.Base and were never vulnerable; they are
	// included so a future refactor that drops Base is caught here.
	for _, tc := range []struct{ name, entry, escapee string }{
		{"data", "data/../../../escaped-data.txt", "escaped-data.txt"},
		{"config", "config/../../../escaped-config.txt", "escaped-config.txt"},
		{"policy", "policy/../../../escaped-policy.txt", "escaped-policy.txt"},
		{"scripts", "scripts/../../../escaped-script.sh", "escaped-script.sh"},
		{"data-deep", "data/nested/../../../../escaped-deep.txt", "escaped-deep.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := installRoots(t)
			artifact := filepath.Join(root, "hostile.tgz")
			buildArchive(t, artifact, []archiveEntry{regularEntry(tc.entry, "PWNED")})

			_, err := applyInstallPayload(t, "demo", artifact)
			if err == nil {
				t.Fatal("expected the archive to be rejected, got success")
			}
			if !strings.Contains(err.Error(), "package archive rejected") {
				t.Fatalf("expected a rejection error, got: %v", err)
			}
			assertNothingEscaped(t, root)
		})
	}
}

// assertNothingEscaped walks the whole temp tree and fails if any file landed
// outside the node-owned destination roots. This is stronger than checking one
// expected path: it catches an escape to somewhere the test did not predict.
func assertNothingEscaped(t *testing.T, root string) {
	t.Helper()
	allowed := []string{ActionBinDir, ActionSystemdDir, ActionConfigDir, ActionStateDir, ActionPolicyDir, ActionStagingRoot}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".tgz") {
			return nil // the artifact itself
		}
		for _, root := range allowed {
			if rel, relErr := filepath.Rel(root, path); relErr == nil &&
				rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil
			}
		}
		t.Errorf("file escaped every destination root: %s", path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInstallPayloadRejectsAbsoluteAndBackslashPaths(t *testing.T) {
	for _, entry := range []string{"/etc/cron.d/pwned", `data\..\..\pwned`, "//etc/pwned"} {
		t.Run(entry, func(t *testing.T) {
			root := installRoots(t)
			artifact := filepath.Join(root, "hostile.tgz")
			buildArchive(t, artifact, []archiveEntry{regularEntry(entry, "PWNED")})
			if _, err := applyInstallPayload(t, "demo", artifact); err == nil {
				t.Fatal("expected rejection")
			}
			assertNothingEscaped(t, root)
		})
	}
}

// ── non-regular entries ─────────────────────────────────────────────────────

func TestInstallPayloadRejectsNonRegularEntries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		typeflag byte
		linkname string
	}{
		{"symlink", tar.TypeSymlink, "/etc/shadow"},
		{"hardlink", tar.TypeLink, "bin/demo"},
		{"chardev", tar.TypeChar, ""},
		{"blockdev", tar.TypeBlock, ""},
		{"fifo", tar.TypeFifo, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := installRoots(t)
			artifact := filepath.Join(root, "hostile.tgz")
			buildArchive(t, artifact, []archiveEntry{
				{name: "config/link", typeflag: tc.typeflag, linkname: tc.linkname, mode: 0o644},
			})
			_, err := applyInstallPayload(t, "demo", artifact)
			if err == nil {
				t.Fatal("expected rejection of a non-regular payload entry")
			}
			if !strings.Contains(err.Error(), ViolationNonRegular) {
				t.Fatalf("expected %s, got: %v", ViolationNonRegular, err)
			}
		})
	}
}

// ── duplicates and limits ───────────────────────────────────────────────────

func TestInstallPayloadRejectsDuplicateDestination(t *testing.T) {
	root := installRoots(t)
	artifact := filepath.Join(root, "dup.tgz")
	// Two distinct archive paths collapsing onto one host path via Base.
	buildArchive(t, artifact, []archiveEntry{
		regularEntry("bin/demo", "first"),
		regularEntry("bin/nested/demo", "second"),
	})
	_, err := applyInstallPayload(t, "demo", artifact)
	if err == nil || !strings.Contains(err.Error(), ViolationDuplicateDest) {
		t.Fatalf("expected %s, got: %v", ViolationDuplicateDest, err)
	}
}

func TestInstallPayloadRejectsEntryCountAndByteBombs(t *testing.T) {
	// The real limits are sized for multi-hundred-megabyte packages, so the
	// tests shrink them rather than materializing a bomb on disk. Restoring
	// them is mandatory: a limit left mutated would silently weaken every
	// later test in this package.
	shrinkLimits := func(t *testing.T, entries int, fileBytes, declaredBytes int64) {
		t.Helper()
		oldEntries, oldFile, oldDeclared := maxArtifactEntries, maxArtifactFileBytes, maxArtifactDeclaredBytes
		t.Cleanup(func() {
			maxArtifactEntries, maxArtifactFileBytes, maxArtifactDeclaredBytes = oldEntries, oldFile, oldDeclared
		})
		maxArtifactEntries, maxArtifactFileBytes, maxArtifactDeclaredBytes = entries, fileBytes, declaredBytes
	}

	t.Run("entry-count", func(t *testing.T) {
		shrinkLimits(t, 8, maxArtifactFileBytes, maxArtifactDeclaredBytes)
		root := installRoots(t)
		artifact := filepath.Join(root, "many.tgz")
		entries := make([]archiveEntry, 0, 10)
		for i := 0; i < 10; i++ {
			entries = append(entries, regularEntry(fmt.Sprintf("data/f%d", i), "x"))
		}
		buildArchive(t, artifact, entries)
		if _, err := applyInstallPayload(t, "demo", artifact); err == nil ||
			!strings.Contains(err.Error(), ViolationEntryCount) {
			t.Fatalf("expected %s, got: %v", ViolationEntryCount, err)
		}
	})

	t.Run("single-entry-bytes", func(t *testing.T) {
		shrinkLimits(t, maxArtifactEntries, 4, maxArtifactDeclaredBytes)
		root := installRoots(t)
		artifact := filepath.Join(root, "bomb.tgz")
		buildArchive(t, artifact, []archiveEntry{regularEntry("data/huge", "wildly oversized body")})
		if _, err := applyInstallPayload(t, "demo", artifact); err == nil ||
			!strings.Contains(err.Error(), ViolationEntryTooBig) {
			t.Fatalf("expected %s, got: %v", ViolationEntryTooBig, err)
		}
	})

	t.Run("total-declared-bytes", func(t *testing.T) {
		shrinkLimits(t, maxArtifactEntries, 1024, 16)
		root := installRoots(t)
		artifact := filepath.Join(root, "bomb.tgz")
		// Each entry is individually fine; the archive as a whole is not.
		buildArchive(t, artifact, []archiveEntry{
			regularEntry("data/a", "ten bytes."),
			regularEntry("data/b", "ten bytes."),
		})
		if _, err := applyInstallPayload(t, "demo", artifact); err == nil ||
			!strings.Contains(err.Error(), ViolationArchiveTooBig) {
			t.Fatalf("expected %s, got: %v", ViolationArchiveTooBig, err)
		}
	})
}

// ── validate before mutate ──────────────────────────────────────────────────

func TestInstallPayloadWritesNothingWhenALaterEntryIsUnsafe(t *testing.T) {
	root := installRoots(t)
	artifact := filepath.Join(root, "mixed.tgz")
	// A perfectly good binary FIRST, the traversal SECOND. A streaming
	// extractor that validated per-entry would already have written the binary
	// and restarted the service before noticing the second entry.
	buildArchive(t, artifact, []archiveEntry{
		regularEntry("bin/demo", "legit"),
		regularEntry("config/app.yaml", "legit"),
		regularEntry("data/../../../escaped.txt", "PWNED"),
	})

	if _, err := applyInstallPayload(t, "demo", artifact); err == nil {
		t.Fatal("expected rejection")
	}
	if _, err := os.Stat(filepath.Join(ActionBinDir, "demo")); !os.IsNotExist(err) {
		t.Error("binary from a rejected archive was written — validation did not precede mutation")
	}
	if _, err := os.Stat(filepath.Join(ActionConfigDir, "demo", "app.yaml")); !os.IsNotExist(err) {
		t.Error("config from a rejected archive was written — validation did not precede mutation")
	}
	assertNothingEscaped(t, root)
}

// ── the happy path must be untouched ────────────────────────────────────────

func TestInstallPayloadStillInstallsALegitimatePackage(t *testing.T) {
	root := installRoots(t)
	artifact := filepath.Join(root, "good.tgz")
	buildArchive(t, artifact, []archiveEntry{
		{name: "bin/demo", body: "#!/bin/sh\n", typeflag: tar.TypeReg, mode: 0o755},
		regularEntry("systemd/globular-demo.service", "[Unit]\nDescription=demo\n"),
		regularEntry("config/app.yaml", "seed: true\n"),
		regularEntry("data/workflows/demo.yaml", "steps: []\n"),
		regularEntry("policy/permissions.generated.json", "{}"),
		regularEntry("scripts/helper.sh", "echo hi\n"),
		regularEntry("unsupported/ignored.txt", "ignored"),
	})

	if _, err := applyInstallPayload(t, "demo", artifact); err != nil {
		t.Fatalf("a legitimate package must still install: %v", err)
	}
	for _, want := range []string{
		filepath.Join(ActionBinDir, "demo"),
		filepath.Join(ActionSystemdDir, "globular-demo.service"),
		filepath.Join(ActionConfigDir, "demo", "app.yaml"),
		filepath.Join(ActionStateDir, "workflows", "demo.yaml"),
		filepath.Join(ActionPolicyDir, "demo", "permissions.generated.json"),
	} {
		if _, err := os.Stat(want); err != nil {
			t.Errorf("expected %s to be installed: %v", want, err)
		}
	}
	// Unsupported prefixes stay ignored rather than becoming an error — the gate
	// changes what is UNSAFE, not what is UNKNOWN.
	assertNothingEscaped(t, root)
}

func TestInstallPayloadPreservesExistingConfigAfterGate(t *testing.T) {
	root := installRoots(t)
	live := filepath.Join(ActionConfigDir, "demo", "app.yaml")
	if err := os.MkdirAll(filepath.Dir(live), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live, []byte("cluster-owned"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact := filepath.Join(root, "good.tgz")
	buildArchive(t, artifact, []archiveEntry{regularEntry("config/app.yaml", "package-default")})

	if _, err := applyInstallPayload(t, "demo", artifact); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(live)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "cluster-owned" {
		t.Fatalf("seed-only config preservation regressed: %q", string(b))
	}
}

// ── planner-level unit coverage ─────────────────────────────────────────────

func TestNormalizeArchiveEntryNameDoesNotEatTraversalDots(t *testing.T) {
	// The original implementation used strings.TrimLeft(name, "./"), whose
	// cutset consumes the dots of a leading "../" and silently converts an
	// obvious traversal into an innocent relative path.
	if got := strings.TrimLeft("../../etc/passwd", "./"); got != "etc/passwd" {
		t.Fatalf("precondition changed; TrimLeft now yields %q", got)
	}
	if _, v := normalizeArchiveEntryName("../../etc/passwd"); v == nil {
		t.Fatal("traversal must be rejected, not normalized away")
	} else if v.Reason != ViolationTraversal {
		t.Fatalf("reason = %q, want %q", v.Reason, ViolationTraversal)
	}
	if name, v := normalizeArchiveEntryName("./bin/demo"); v != nil || name != "bin/demo" {
		t.Fatalf("legitimate leading ./ must normalize: name=%q v=%v", name, v)
	}
}

func TestValidateArtifactArchiveSeparatesMappedFromInertFindings(t *testing.T) {
	root := installRoots(t)
	artifact := filepath.Join(root, "mixed.tgz")
	buildArchive(t, artifact, []archiveEntry{
		regularEntry("data/../../../mapped-escape.txt", "x"),
		{name: "docs/legacy-link", typeflag: tar.TypeSymlink, linkname: "../elsewhere"},
	})
	violations, err := ValidateArtifactArchive(artifact, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 2 {
		t.Fatalf("expected 2 violations, got %d: %v", len(violations), violations)
	}
	// Dangerous findings sort first so an audit report leads with what matters.
	if !violations[0].Mapped {
		t.Errorf("expected the mapped finding first, got %v", violations[0])
	}
	if violations[1].Mapped {
		t.Errorf("a symlink under an ignored prefix is inert, not mapped: %v", violations[1])
	}
}
