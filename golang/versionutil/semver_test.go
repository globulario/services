package versionutil

import "testing"

func TestPickLatestSemverBasicOrdering(t *testing.T) {
	ver, err := PickLatestSemver([]string{"1.9.0", "1.10.0", "1.8.5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.10.0" {
		t.Fatalf("expected 1.10.0, got %s", ver)
	}
}

func TestPickLatestSemverWithLeadingV(t *testing.T) {
	ver, err := PickLatestSemver([]string{"v1.2.3", "1.2.4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.2.4" {
		t.Fatalf("expected 1.2.4, got %s", ver)
	}
}

func TestPickLatestSemverReturnsCanonical(t *testing.T) {
	ver, err := PickLatestSemver([]string{"v1.3.0", "1.2.0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.3.0" {
		t.Fatalf("expected canonical 1.3.0 (no v prefix), got %s", ver)
	}
}

func TestPickLatestSemverPrefersStableOverPreRelease(t *testing.T) {
	ver, err := PickLatestSemver([]string{"1.0.0-alpha", "1.0.0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.0.0" {
		t.Fatalf("expected stable 1.0.0, got %s", ver)
	}
}

func TestPickLatestSemverAllPreRelease(t *testing.T) {
	ver, err := PickLatestSemver([]string{"1.0.0-alpha", "1.0.0-beta", "1.0.0-rc.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.0.0-rc.1" {
		t.Fatalf("expected highest pre-release rc.1, got %s", ver)
	}
}

func TestPickLatestSemverSkipsInvalid(t *testing.T) {
	ver, err := PickLatestSemver([]string{"not-a-version", "1.2.3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %s", ver)
	}
}

func TestPickLatestSemverAllInvalid(t *testing.T) {
	if _, err := PickLatestSemver([]string{"foo", "bar"}); err == nil {
		t.Fatalf("expected error when all versions invalid")
	}
}

// Example usage for docs
func ExamplePickLatestSemver() {
	ver, _ := PickLatestSemver([]string{"v1.2.0", "1.3.0-alpha", "1.2.5"})
	// ver == "1.2.5"
	_ = ver
}

func TestCanonical(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"v0.1.0", "0.1.0", false},
		{"V1.0.0-alpha.1", "1.0.0-alpha.1", false},
		{" v1.2.3 ", "1.2.3", false},
		{"", "", true},
		{"garbage", "", true},
	}
	for _, tt := range tests {
		got, err := Canonical(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Canonical(%q) expected error, got %q", tt.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Canonical(%q) unexpected error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Canonical(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeExactCanonicalizesSemver(t *testing.T) {
	got, err := NormalizeExact(" v1.2.3 ")
	if err != nil {
		t.Fatalf("NormalizeExact returned error: %v", err)
	}
	if got != "1.2.3" {
		t.Fatalf("NormalizeExact = %q, want 1.2.3", got)
	}
}

func TestNormalizeExactPreservesUpstreamTags(t *testing.T) {
	tests := []string{
		"RELEASE.2025-09-07T16-13-09Z",
		"n8.1-10-g7f5c90f77e-20260422",
		"vNotSemver",
	}
	for _, tt := range tests {
		got, err := NormalizeExact(tt)
		if err != nil {
			t.Fatalf("NormalizeExact(%q) returned error: %v", tt, err)
		}
		if got != tt {
			t.Fatalf("NormalizeExact(%q) = %q, want original", tt, got)
		}
	}
}

func TestNormalizeExactRejectsUnsafeTags(t *testing.T) {
	for _, tt := range []string{"", "../1.0.0", "one two", "pkg%1"} {
		if got, err := NormalizeExact(tt); err == nil {
			t.Fatalf("NormalizeExact(%q) expected error, got %q", tt, got)
		}
	}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"v0.1.0", "", false},
		{"1.2.3", "1.2.3", true},
		{"v0.1.0", "garbage", false},
		{"garbage", "v0.1.0", false},
		{"1.0.0-alpha", "1.0.0-alpha", true},
		{"v1.0.0", "1.0.1", false},
	}
	for _, tt := range tests {
		got := Equal(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("Equal(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b    string
		want    int
		wantErr bool
	}{
		{"1.1.0", "1.0.0", 1, false},
		{"1.0.0", "1.0.0-alpha", 1, false},
		{"0.0.1", "", 0, true},
		{"1.0.0", "1.0.0", 0, false},
		{"garbage", "1.0.0", 0, true},
		{"1.0.0", "garbage", 0, true},
	}
	for _, tt := range tests {
		got, err := Compare(tt.a, tt.b)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Compare(%q, %q) expected error", tt.a, tt.b)
			}
			continue
		}
		if err != nil {
			t.Errorf("Compare(%q, %q) unexpected error: %v", tt.a, tt.b, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMustCanonicalPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("MustCanonical(\"garbage\") should have panicked")
		}
	}()
	MustCanonical("garbage")
}

func TestMustCanonicalValid(t *testing.T) {
	got := MustCanonical("v1.2.3")
	if got != "1.2.3" {
		t.Fatalf("MustCanonical(\"v1.2.3\") = %q, want \"1.2.3\"", got)
	}
}
