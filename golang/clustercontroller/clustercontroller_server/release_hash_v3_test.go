package main

import "testing"

func TestComputeReleaseDesiredHashV3Deterministic(t *testing.T) {
	h1 := ComputeReleaseDesiredHashV3("pub", "svc", "1.0.0", "cfg123")
	h2 := ComputeReleaseDesiredHashV3("pub", "svc", "1.0.0", "cfg123")
	if h1 != h2 {
		t.Fatalf("expected hashes to match, got %s vs %s", h1, h2)
	}
}

func TestComputeReleaseDesiredHashV3VersionChangesHash(t *testing.T) {
	h1 := ComputeReleaseDesiredHashV3("pub", "svc", "1.0.0", "cfg123")
	h2 := ComputeReleaseDesiredHashV3("pub", "svc", "1.1.0", "cfg123")
	if h1 == h2 {
		t.Fatalf("expected hash to change when version changes")
	}
}

func TestComputeReleaseDesiredHashV3ConfigChangesHash(t *testing.T) {
	h1 := ComputeReleaseDesiredHashV3("pub", "svc", "1.0.0", "cfg123")
	h2 := ComputeReleaseDesiredHashV3("pub", "svc", "1.0.0", "cfg999")
	if h1 == h2 {
		t.Fatalf("expected hash to change when config digest changes")
	}
}

func TestComputeReleaseDesiredHashV3PublisherChangesHash(t *testing.T) {
	h1 := ComputeReleaseDesiredHashV3("pub1", "svc", "1.0.0", "cfg123")
	h2 := ComputeReleaseDesiredHashV3("pub2", "svc", "1.0.0", "cfg123")
	if h1 == h2 {
		t.Fatalf("expected hash to change when publisher changes")
	}
}
