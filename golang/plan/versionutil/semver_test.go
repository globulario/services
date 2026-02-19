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
