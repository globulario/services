package main

import "testing"

func bom(pairs ...string) []BOMPackage {
	var out []BOMPackage
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, BOMPackage{Name: pairs[i], Version: pairs[i+1], Kind: "service"})
	}
	return out
}

func node(id string, installed map[string]string) NodeView {
	return NodeView{NodeID: id, InstalledVersions: installed}
}

// Convergence gate: empty result == cluster converged to the BOM.
func TestEvaluateActivationReadiness_AllConverged(t *testing.T) {
	b := bom("repository", "1.2.250", "node-agent", "1.2.250")
	nodes := []NodeView{
		node("n1", map[string]string{"repository": "1.2.250", "node-agent": "1.2.250"}),
	}
	if l := evaluateActivationReadiness(nodes, b); len(l) != 0 {
		t.Fatalf("want converged (0 laggards), got %v", l)
	}
}

func TestEvaluateActivationReadiness_OneLagging(t *testing.T) {
	b := bom("repository", "1.2.250", "node-agent", "1.2.250")
	nodes := []NodeView{
		node("n1", map[string]string{"repository": "1.2.250", "node-agent": "1.2.249"}), // node-agent behind
	}
	l := evaluateActivationReadiness(nodes, b)
	if len(l) != 1 || l[0].Package != "node-agent" || l[0].Installed != "1.2.249" || l[0].BOMVersion != "1.2.250" {
		t.Fatalf("want one node-agent laggard 1.2.249->1.2.250, got %v", l)
	}
}

// Not-installed packages are skipped (operator removal preserved); packages not
// in the BOM are ignored.
func TestEvaluateActivationReadiness_NotInstalledSkipped_AndExtraIgnored(t *testing.T) {
	b := bom("repository", "1.2.250", "torrent", "1.2.250")
	nodes := []NodeView{
		// torrent NOT installed (operator removed) → skip; "claude" not in BOM → ignore.
		node("n1", map[string]string{"repository": "1.2.250", "claude": "9.9.9"}),
	}
	if l := evaluateActivationReadiness(nodes, b); len(l) != 0 {
		t.Fatalf("want 0 laggards (removal skipped, extra ignored), got %v", l)
	}
}

// Native (non-SemVer) versions: equal → converged; different → laggard.
func TestEvaluateActivationReadiness_NativeVersions(t *testing.T) {
	b := bom("ffmpeg", "n8.1.2-20260627")
	conv := []NodeView{node("n1", map[string]string{"ffmpeg": "n8.1.2-20260627"})}
	if l := evaluateActivationReadiness(conv, b); len(l) != 0 {
		t.Fatalf("native equal must converge, got %v", l)
	}
	lag := []NodeView{node("n1", map[string]string{"ffmpeg": "n8.1.2-20260621"})}
	if l := evaluateActivationReadiness(lag, b); len(l) != 1 {
		t.Fatalf("native different must be a laggard, got %v", l)
	}
}

func anchor(tag, platform string) *activeReleaseAnchor {
	return &activeReleaseAnchor{ReleaseTag: tag, PlatformRelease: platform}
}

// Idempotency: re-activating the current tag is a no-op.
func TestDecideActivation_Idempotent(t *testing.T) {
	a, _ := decideActivation(anchor("v1.2.250", "1.2.250"), "v1.2.250", "1.2.250", true, false, false)
	if a != activationNoop {
		t.Fatalf("want noop for already-active tag, got %q", a)
	}
}

// No-regression: older platform_release refused unless allowRegression.
func TestDecideActivation_NoRegression(t *testing.T) {
	cur := anchor("v1.2.250", "1.2.250")
	if a, _ := decideActivation(cur, "v1.2.249", "1.2.249", true, false, false); a != activationRefuse {
		t.Fatalf("want refuse for older platform_release, got %q", a)
	}
	if a, _ := decideActivation(cur, "v1.2.249", "1.2.249", true, true, false); a != activationActivate {
		t.Fatalf("want activate with --allow-regression, got %q", a)
	}
	// forward is always allowed
	if a, _ := decideActivation(cur, "v1.2.251", "1.2.251", true, false, false); a != activationActivate {
		t.Fatalf("want activate for newer platform_release, got %q", a)
	}
}

// Convergence gate: not-converged refused unless force.
func TestDecideActivation_ConvergenceGate(t *testing.T) {
	cur := anchor("v1.2.249", "1.2.249")
	if a, _ := decideActivation(cur, "v1.2.250", "1.2.250", false, false, false); a != activationRefuse {
		t.Fatalf("want refuse when not converged, got %q", a)
	}
	if a, _ := decideActivation(cur, "v1.2.250", "1.2.250", false, false, true); a != activationActivate {
		t.Fatalf("want activate with --force despite non-convergence, got %q", a)
	}
	if a, _ := decideActivation(cur, "v1.2.250", "1.2.250", true, false, false); a != activationActivate {
		t.Fatalf("want activate when converged, got %q", a)
	}
}

// First activation (no current anchor) is allowed when converged.
func TestDecideActivation_FirstActivation(t *testing.T) {
	if a, _ := decideActivation(nil, "v1.2.250", "1.2.250", true, false, false); a != activationActivate {
		t.Fatalf("want activate for first anchor when converged, got %q", a)
	}
}

func TestPlatformReleaseFromTag(t *testing.T) {
	cases := map[string]string{"v1.2.250": "1.2.250", "1.2.250": "1.2.250", "V1.2.250": "1.2.250", " v1.2.250 ": "1.2.250"}
	for in, want := range cases {
		if got := platformReleaseFromTag(in); got != want {
			t.Errorf("platformReleaseFromTag(%q)=%q, want %q", in, got, want)
		}
	}
}
