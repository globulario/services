package main

import (
	"testing"

	"github.com/globulario/awareness/assurance"
)

func TestParseTrustVerdictAcceptsKnownValues(t *testing.T) {
	cases := []string{"unsafe", "unknown", "stale", "limited", "usable", "trusted"}
	for _, c := range cases {
		got, err := parseTrustVerdict(c)
		if err != nil {
			t.Fatalf("parseTrustVerdict(%q) error: %v", c, err)
		}
		if got != assurance.TrustVerdict(c) {
			t.Fatalf("parseTrustVerdict(%q)=%q want %q", c, got, c)
		}
	}
}

func TestParseTrustVerdictRejectsInvalid(t *testing.T) {
	if _, err := parseTrustVerdict("best"); err == nil {
		t.Fatal("expected error for invalid min trust")
	}
}

func TestTrustRankOrdering(t *testing.T) {
	if trustRank(assurance.TrustTrusted) <= trustRank(assurance.TrustUsable) {
		t.Fatal("expected trusted > usable")
	}
	if trustRank(assurance.TrustUsable) <= trustRank(assurance.TrustLimited) {
		t.Fatal("expected usable > limited")
	}
	if trustRank(assurance.TrustLimited) <= trustRank(assurance.TrustStale) {
		t.Fatal("expected limited > stale")
	}
	if trustRank(assurance.TrustStale) <= trustRank(assurance.TrustUnknown) {
		t.Fatal("expected stale > unknown")
	}
	if trustRank(assurance.TrustUnknown) <= trustRank(assurance.TrustUnsafe) {
		t.Fatal("expected unknown > unsafe")
	}
}
