package main

// release_promote_local_cmd_test.go — Unit tests for promote-local planner helpers.
//
//   1. suggestNextPatch increments patch correctly
//   2. suggestNextPatch handles missing parts gracefully
//   3. Official version suffix guard (reuse hasLocalVersionSuffix)

import "testing"

// ── 1. suggestNextPatch ───────────────────────────────────────────────────────

func TestSuggestNextPatch(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"1.2.43", "1.2.44"},
		{"1.2.0", "1.2.1"},
		{"2.0.9", "2.0.10"},
		{"v1.2.43", "1.2.44"}, // leading v stripped
		{"1.2.43+local.ryzen.1", "1.2.44"}, // build metadata stripped before bump
		{"1.2.43-hotfix.auth", "1.2.44"},    // prerelease stripped before bump
	}
	for _, c := range cases {
		got := suggestNextPatch(c.input)
		if got != c.want {
			t.Errorf("suggestNextPatch(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ── 2. suggestNextPatch handles bad input ────────────────────────────────────

func TestSuggestNextPatch_BadInput(t *testing.T) {
	bad := []string{"", "not-semver", "1.2", "v1"}
	for _, v := range bad {
		got := suggestNextPatch(v)
		if got != "" {
			t.Errorf("suggestNextPatch(%q) = %q, want empty string for bad input", v, got)
		}
	}
}

// ── 3. Promoted version must not carry local suffix ──────────────────────────

func TestPromoteLocal_OfficialVersionRejectsLocalSuffix(t *testing.T) {
	invalidTargets := []string{
		"1.2.44+local.ryzen.1",
		"1.2.44-hotfix.auth",
		"1.2.44-dev.fix2",
	}
	for _, v := range invalidTargets {
		if !hasLocalVersionSuffix(v) {
			t.Errorf("expected %q to be caught by hasLocalVersionSuffix — it would be accepted as a target version", v)
		}
	}

	validTargets := []string{"1.2.44", "2.0.0", "1.2.44-rc1"}
	for _, v := range validTargets {
		if hasLocalVersionSuffix(v) {
			t.Errorf("expected %q to NOT be caught by hasLocalVersionSuffix — it is a valid official target version", v)
		}
	}
}
