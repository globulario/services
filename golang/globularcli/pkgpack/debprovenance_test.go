package pkgpack

// Three adversaries, three distinct threats.
//
//	provenance adversary   junk exists on the builder, but cannot be consumed
//	architecture adversary ambient multiarch cannot expand release contents
//	dependency adversary   authoritative input is itself unsatisfiable -> fail closed
//
// The provenance test is written as a literal reproduction of the 2026-08-14
// incident rather than as an abstract "junk file" case, because reproducing the
// exact causal mechanism is what proves the mechanism is dead: delete the
// tracked deb, drop an untracked one with the SAME pathname, and require that
// assembly refuses the substitution.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newDebRepo builds a throwaway repo with one committed .deb-shaped file.
// The bytes need not be a real archive: every assertion under test is about
// byte identity against the declared revision, which is format-agnostic.
func newDebRepo(t *testing.T, relPath string, content []byte) (repo, debPath string) {
	t.Helper()
	repo = t.TempDir()
	git(t, repo, "init", "-q")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "test")

	debPath = filepath.Join(repo, relPath)
	if err := os.MkdirAll(filepath.Dir(debPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(debPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "add bundled deb")
	return repo, debPath
}

// TestProvenanceAdversary_UntrackedSubstitutionAtSamePath is the 2026-08-14
// incident, reproduced exactly.
func TestProvenanceAdversary_UntrackedSubstitutionAtSamePath(t *testing.T) {
	const rel = "metadata/libnss-resolve/debs/libnss-resolve_255.4-1ubuntu8.16_amd64.deb"
	repo, tracked := newDebRepo(t, rel, []byte("committed 8.16 bytes"))

	// The remedy that caused occurrence #2: delete the tracked deb, drop a
	// newer one in its place, never commit.
	if err := os.Remove(tracked); err != nil {
		t.Fatal(err)
	}
	substituted := filepath.Join(filepath.Dir(tracked), "libnss-resolve_255.4-1ubuntu8.17_amd64.deb")
	if err := os.WriteFile(substituted, []byte("untracked 8.17 bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := VerifyDebProvenance(substituted)
	if err == nil {
		t.Fatal("untracked substitution was accepted — the 2026-08-14 vector is still open")
	}
	if !strings.Contains(err.Error(), "declared source revision") {
		t.Fatalf("refusal should name the declared source revision, got: %v", err)
	}
	_ = repo
}

// A same-pathname replacement is the nastier variant: the filename still
// matches what the revision declares, so any name-based check would pass it.
func TestProvenanceAdversary_ModifiedBytesAtTrackedPath(t *testing.T) {
	const rel = "metadata/libnss-resolve/debs/libnss-resolve_255.4-1ubuntu8.16_amd64.deb"
	_, tracked := newDebRepo(t, rel, []byte("committed 8.16 bytes"))

	if err := os.WriteFile(tracked, []byte("different bytes, identical filename"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := VerifyDebProvenance(tracked)
	if err == nil {
		t.Fatal("modified bytes at a tracked path were accepted — filename equality is not provenance")
	}
	if !strings.Contains(err.Error(), "source substitution") {
		t.Fatalf("refusal should name source substitution, got: %v", err)
	}
	// The refusal must be explainable without a debugger.
	for _, want := range []string{"declared bytes sha256", "filesystem bytes sha256"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal detail missing %q:\n%v", want, err)
		}
	}
}

// A deb outside any repository has no declared source at all. Unknown
// provenance must not improve the verdict.
func TestProvenanceAdversary_NoOwningRepositoryIsRefused(t *testing.T) {
	dir := t.TempDir()
	orphan := filepath.Join(dir, "libnss-resolve_255.4-1ubuntu8.17_amd64.deb")
	if err := os.WriteFile(orphan, []byte("ambient bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyDebProvenance(orphan); err == nil {
		t.Fatal("a deb with no owning repository was accepted as a release input")
	}
}

// TestDependencyAdversary_ExactPinAgainstBaseline is the libnss-resolve
// arithmetic: an authoritative, correctly-provenanced input that still cannot
// install on the declared target.
func TestDependencyAdversary_ExactPinAgainstBaseline(t *testing.T) {
	baseline := &PlatformBaseline{
		ID:       "ubuntu-noble-release-20260518-amd64",
		Image:    "release-20260518",
		Provides: map[string]string{"systemd-resolved": "255.4-1ubuntu8.15", "libc6": "2.39-0ubuntu8.7"},
	}
	prov := &DebProvenance{
		Package: "libnss-resolve",
		Version: "255.4-1ubuntu8.17",
		Depends: []string{"libc6 (>= 2.39)", "systemd-resolved (= 255.4-1ubuntu8.17)"},
	}

	res := CheckSatisfiable(prov, baseline, map[string]string{"libnss-resolve": "255.4-1ubuntu8.17"})
	if res.Satisfied {
		t.Fatal("exact-version pin against a mismatched baseline was reported satisfiable")
	}

	var refused string
	for _, v := range res.Verdicts {
		if !v.Satisfied {
			refused = v.Raw
		}
	}
	if !strings.Contains(refused, "systemd-resolved") {
		t.Fatalf("wrong clause refused: %q", refused)
	}
}

// The same package IS satisfiable when the baseline happens to carry the exact
// version — proving the gate discriminates rather than refusing everything. A
// rule that cannot pass is indistinguishable from a rule that cannot fire.
func TestDependencyAdversary_SatisfiableWhenBaselineMatches(t *testing.T) {
	baseline := &PlatformBaseline{
		ID:       "matching",
		Provides: map[string]string{"systemd-resolved": "255.4-1ubuntu8.17", "libc6": "2.39-0ubuntu8.7"},
	}
	prov := &DebProvenance{
		Package: "libnss-resolve",
		Depends: []string{"libc6 (>= 2.39)", "systemd-resolved (= 255.4-1ubuntu8.17)"},
	}
	res := CheckSatisfiable(prov, baseline, nil)
	if !res.Satisfied {
		t.Fatalf("matching baseline reported unsatisfiable: %+v", res.Verdicts)
	}
}

// A dependency neither bundled nor supplied by the declared baseline is
// UNKNOWN, and unknown must fail closed rather than be assumed present.
func TestDependencyAdversary_UnknownDependencyFailsClosed(t *testing.T) {
	baseline := &PlatformBaseline{ID: "b", Provides: map[string]string{"libc6": "2.39-0ubuntu8.7"}}
	prov := &DebProvenance{Package: "p", Depends: []string{"some-package-nobody-declared"}}

	res := CheckSatisfiable(prov, baseline, nil)
	if res.Satisfied {
		t.Fatal("an unprovable dependency was treated as satisfied")
	}
}

// Debs shipped together satisfy each other; that is the whole point of a bundle.
func TestSatisfiable_BundledSiblingResolves(t *testing.T) {
	baseline := &PlatformBaseline{ID: "b", Provides: map[string]string{}}
	prov := &DebProvenance{Package: "a", Depends: []string{"b (>= 1.0)"}}
	res := CheckSatisfiable(prov, baseline, map[string]string{"b": "1.2"})
	if !res.Satisfied {
		t.Fatalf("bundled sibling did not resolve: %+v", res.Verdicts)
	}
}

// TestArchitectureAdversary_AmbientMultiarchCannotExpandContents covers the
// committed i386 debs: nothing declared them, a build host merely had multiarch
// enabled. The declaration decides architecture, not the builder's apt state.
func TestArchitectureAdversary_AmbientMultiarchCannotExpandContents(t *testing.T) {
	paths := []string{
		"/tmp/libnss-resolve_255.4-1ubuntu8.17_amd64.deb",
		"/tmp/libc6_2.39-0ubuntu8.7_i386.deb",
		"/tmp/libsystemd0_255.4-1ubuntu8.15_i386.deb",
		"/tmp/gcc-14-base_14.2.0-4ubuntu2~24.04.1_i386.deb",
	}
	got := filterDebsForArch(paths, "amd64")
	for _, p := range got {
		if strings.Contains(p, "_i386.deb") {
			t.Errorf("ambient i386 deb reached release contents: %s", p)
		}
	}
	if len(got) != 1 {
		t.Fatalf("expected only the declared amd64 deb to survive, got %v", got)
	}
}

// buildRealDeb produces a genuine (if minimal) .deb so the control-reading path
// is exercised for real. The provenance tests above deliberately do not need
// this: they must refuse before any control read, and using placeholder bytes
// proves they do.
func buildRealDeb(t *testing.T, name, version, arch, depends string) []byte {
	t.Helper()
	if _, err := exec.LookPath("dpkg-deb"); err != nil {
		t.Skip("dpkg-deb unavailable — cannot build a real .deb on this host")
	}
	stage := t.TempDir()
	debianDir := filepath.Join(stage, "DEBIAN")
	if err := os.MkdirAll(debianDir, 0o755); err != nil {
		t.Fatal(err)
	}
	control := "Package: " + name + "\nVersion: " + version + "\nArchitecture: " + arch +
		"\nMaintainer: test <test@example.com>\nDescription: fixture\n"
	if depends != "" {
		control += "Depends: " + depends + "\n"
	}
	if err := os.WriteFile(filepath.Join(debianDir, "control"), []byte(control), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), name+".deb")
	cmd := exec.Command("dpkg-deb", "--build", "--nocheck", stage, out)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dpkg-deb --build: %v\n%s", err, b)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// writeSources emits a release source manifest naming repo at revision rev.
// Temp repos have no remote, so their identity is the directory basename.
func writeSources(t *testing.T, repo, rev string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package-sources.json")
	body := `{"schema_version":1,"sources":{"packages":{"repository":"` +
		filepath.Base(repo) + `","revision":"` + rev + `"}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A declared baseline is required. Assembling a bundle whose installability
// nobody checked must not be the quiet default.
func TestMissingBaselineIsRefusedNotSkipped(t *testing.T) {
	content := buildRealDeb(t, "x", "1.0", "amd64", "")
	repo, deb := newDebRepo(t, "metadata/x/debs/x_1.0_amd64.deb", content)
	sources := writeSources(t, repo, git(t, repo, "rev-parse", "HEAD"))

	_, err := verifyBundledDebs([]string{deb}, BuildOptions{PackageSourcesPath: sources})
	if err == nil {
		t.Fatal("assembly proceeded with bundled debs and no declared baseline")
	}
	// Provenance holds here, so the refusal must be about the undeclared
	// baseline rather than a provenance error.
	if !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("expected a baseline refusal, got: %v", err)
	}
}

// A declared package source is equally required: without it the authorized
// revision is unknown, and unknown must not proceed.
func TestMissingPackageSourcesIsRefusedNotSkipped(t *testing.T) {
	content := buildRealDeb(t, "x", "1.0", "amd64", "")
	_, deb := newDebRepo(t, "metadata/x/debs/x_1.0_amd64.deb", content)

	_, err := verifyBundledDebs([]string{deb}, BuildOptions{})
	if err == nil {
		t.Fatal("assembly proceeded with no declared package source")
	}
	if !strings.Contains(err.Error(), "package source manifest") {
		t.Fatalf("expected a package-source refusal, got: %v", err)
	}
}

// TestDeclaredRevisionWinsOverHEAD is the difference between "reproducible" and
// "pinned". Two revisions hold different bytes at the same path; the release
// declares the older one. Assembly must materialize the DECLARED revision's
// bytes even though HEAD is newer and perfectly tracked.
func TestDeclaredRevisionWinsOverHEAD(t *testing.T) {
	authorized := buildRealDeb(t, "x", "1.0", "amd64", "")
	repo, deb := newDebRepo(t, "metadata/x/debs/x_1.0_amd64.deb", authorized)
	declaredRev := git(t, repo, "rev-parse", "HEAD")

	// A later, fully tracked revision changes the bytes at the same path.
	newer := buildRealDeb(t, "x", "2.0", "amd64", "")
	if err := os.WriteFile(deb, newer, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "bump deb")

	sources := writeSources(t, repo, declaredRev)
	prov, materialized, err := MaterializeDebFromSource(deb, mustLoadSources(t, sources), t.TempDir())
	if err != nil {
		t.Fatalf("materialize from declared revision: %v", err)
	}
	if prov.SourceRevision != declaredRev {
		t.Errorf("provenance records %s, declared %s", prov.SourceRevision, declaredRev)
	}
	if prov.Version != "1.0" {
		t.Errorf("materialized version %q — HEAD's bytes were assembled instead of the declared revision's", prov.Version)
	}
	got, err := os.ReadFile(materialized)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == string(newer) {
		t.Error("materialized bytes are HEAD's, not the declared revision's")
	}
}

// A repository the release never declared must not supply package inputs, even
// if its contents are perfectly tracked.
func TestUndeclaredRepositoryIsRefused(t *testing.T) {
	content := buildRealDeb(t, "x", "1.0", "amd64", "")
	repo, deb := newDebRepo(t, "metadata/x/debs/x_1.0_amd64.deb", content)

	path := filepath.Join(t.TempDir(), "package-sources.json")
	body := `{"schema_version":1,"sources":{"packages":{"repository":"some-other-repo","revision":"` +
		git(t, repo, "rev-parse", "HEAD") + `"}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := MaterializeDebFromSource(deb, mustLoadSources(t, path), t.TempDir())
	if err == nil {
		t.Fatal("a deb from an undeclared repository was accepted")
	}
	if !strings.Contains(err.Error(), "not named in the release source manifest") {
		t.Fatalf("unexpected refusal reason: %v", err)
	}
}

// Materializing from the declared revision means a dirty worktree cannot
// contaminate a release — the packages checkout may legitimately carry
// unrelated in-progress work.
func TestDirtyWorktreeCannotContaminateMaterializedBytes(t *testing.T) {
	authorized := buildRealDeb(t, "x", "1.0", "amd64", "")
	repo, deb := newDebRepo(t, "metadata/x/debs/x_1.0_amd64.deb", authorized)
	rev := git(t, repo, "rev-parse", "HEAD")

	// The 2026-08-14 move: overwrite the worktree copy, never commit.
	if err := os.WriteFile(deb, buildRealDeb(t, "x", "9.9", "amd64", ""), 0o644); err != nil {
		t.Fatal(err)
	}

	prov, _, err := MaterializeDebFromSource(deb, mustLoadSources(t, writeSources(t, repo, rev)), t.TempDir())
	if err != nil {
		t.Fatalf("materialize should succeed and ignore the dirty worktree: %v", err)
	}
	if prov.Version != "1.0" {
		t.Errorf("assembled version %q — uncommitted worktree bytes reached the release", prov.Version)
	}
}

func mustLoadSources(t *testing.T, path string) *PackageSources {
	t.Helper()
	s, err := LoadPackageSources(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// End-to-end through the real choke point: a properly provenanced deb whose
// exact pin cannot be met by the declared baseline must stop assembly.
func TestVerifyBundledDebs_RefusesUnsatisfiableAtChokePoint(t *testing.T) {
	content := buildRealDeb(t, "libnss-resolve", "255.4-1ubuntu8.17", "amd64",
		"libc6 (>= 2.39), systemd-resolved (= 255.4-1ubuntu8.17)")
	repo, deb := newDebRepo(t, "metadata/libnss-resolve/debs/libnss-resolve_255.4-1ubuntu8.17_amd64.deb", content)
	sources := writeSources(t, repo, git(t, repo, "rev-parse", "HEAD"))

	baselinePath := filepath.Join(t.TempDir(), "baseline.json")
	baseline := `{"id":"ubuntu-noble-release-20260518-amd64","image":"release-20260518",
	  "provides":{"systemd-resolved":"255.4-1ubuntu8.15","libc6":"2.39-0ubuntu8.7"}}`
	if err := os.WriteFile(baselinePath, []byte(baseline), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := verifyBundledDebs([]string{deb}, BuildOptions{
		PackageSourcesPath:   sources,
		PlatformBaselinePath: baselinePath,
	})
	if err == nil {
		t.Fatal("unsatisfiable bundle passed the choke point — this is the 8.15-target failure, undetected")
	}
	if !strings.Contains(err.Error(), "satisfiability") || !strings.Contains(err.Error(), "libnss-resolve") {
		t.Fatalf("refusal should name satisfiability and the package, got: %v", err)
	}
}

// TestRegression_LibnssResolve816AgainstBaseline815 keeps the knowledge of the
// failure after the dependency itself is gone.
//
// libnss-resolve is being removed from the release package set entirely: it was
// never a Globular runtime prerequisite (systemd-resolved's stub already serves
// cluster names over standard DNS, and Go services bypass NSS via
// config.ClusterResolver). Removing a dependency, however, must not remove the
// reason it was dangerous. If anyone re-adds it — at 8.16, at 8.17, at any
// pinned version — the gate must still refuse it against the declared baseline,
// because the defect was never the version. It was pinning a base-image package
// by exact equality while that package is deliberately never bundled.
func TestRegression_LibnssResolve816AgainstBaseline815(t *testing.T) {
	baseline := &PlatformBaseline{
		ID:       "ubuntu-noble-release-20260518-amd64",
		Image:    "release-20260518",
		Provides: map[string]string{"systemd-resolved": "255.4-1ubuntu8.15", "libc6": "2.39-0ubuntu8.7", "libcap2": "1:2.66-5ubuntu2.2"},
	}
	// Exactly the Depends line of the tracked 8.16 deb.
	prov := &DebProvenance{
		Package: "libnss-resolve",
		Version: "255.4-1ubuntu8.16",
		Depends: []string{"libc6 (>= 2.39)", "libcap2 (>= 1:2.10)", "systemd-resolved (= 255.4-1ubuntu8.16)"},
	}

	res := CheckSatisfiable(prov, baseline, map[string]string{"libnss-resolve": "255.4-1ubuntu8.16"})
	if res.Satisfied {
		t.Fatal("re-added libnss-resolve 8.16 was accepted against baseline 8.15")
	}

	// Every pinned version fails the same way — the version is not the defect.
	for _, v := range []string{"255.4-1ubuntu8.16", "255.4-1ubuntu8.17", "255.4-1ubuntu8.14"} {
		p := &DebProvenance{
			Package: "libnss-resolve",
			Version: v,
			Depends: []string{"systemd-resolved (= " + v + ")"},
		}
		if CheckSatisfiable(p, baseline, nil).Satisfied {
			t.Errorf("libnss-resolve %s accepted against baseline 8.15 — exact-pin class not covered", v)
		}
	}
}
