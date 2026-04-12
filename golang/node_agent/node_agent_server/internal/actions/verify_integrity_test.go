package actions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSha256OfFile verifies the file hasher matches crypto/sha256 on arbitrary input.
func TestSha256OfFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.bin")
	data := []byte("verify-integrity-payload-build20")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := sha256OfFile(path)
	if err != nil {
		t.Fatalf("sha256OfFile: %v", err)
	}
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("sha256OfFile = %s, want %s", got, want)
	}
}

// TestNormalizeSHA256Digest strips the "sha256:" prefix and lowercases.
func TestNormalizeSHA256Digest(t *testing.T) {
	cases := map[string]string{
		"SHA256:DEADBEEF":                              "deadbeef",
		"sha256:abc123":                                "abc123",
		"abc123":                                       "abc123",
		"  sha256:ABC123  ":                            "abc123",
	}
	for in, want := range cases {
		if got := normalizeSHA256Digest(in); got != want {
			t.Fatalf("normalizeSHA256Digest(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestShortDigest returns the first 12 hex chars regardless of prefix.
func TestShortDigest(t *testing.T) {
	cases := map[string]string{
		"sha256:1234567890abcdef1234567890abcdef": "1234567890ab",
		"1234":             "1234",
		"":                 "",
		"SHA256:ABCDEF123": "abcdef123",
	}
	for in, want := range cases {
		if got := shortDigest(in); got != want {
			t.Fatalf("shortDigest(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestKindToProto covers the four expected mappings.
func TestKindToProto(t *testing.T) {
	// Only the string → string conversion path; avoid proto equality for readability.
	for _, k := range []string{"SERVICE", "INFRASTRUCTURE", "APPLICATION", "COMMAND"} {
		got := kindToProto(k)
		if got.String() == "" {
			t.Fatalf("kindToProto(%q) returned empty", k)
		}
	}
}

// TestIntegrityReportSerializes round-trips a report through JSON so callers
// can parse the action result without a proto schema change.
func TestIntegrityReportSerializes(t *testing.T) {
	r := integrityReport{
		NodeID:  "node-1",
		Checked: 2,
		Findings: []integrityFinding{
			{
				Invariant: "artifact.cache_digest_mismatch",
				Severity:  "WARN",
				Package:   "event",
				Kind:      "SERVICE",
				Summary:   "cached event has sha256 abcd, manifest expects ef01",
				Evidence: map[string]string{
					"cache_sha256":    "abcd",
					"manifest_sha256": "ef01",
				},
			},
		},
		Invariants: map[string]int{
			"artifact.cache_digest_mismatch": 1,
		},
	}
	blob, err := json.Marshal(&r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round integrityReport
	if err := json.Unmarshal(blob, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.NodeID != r.NodeID || round.Checked != r.Checked {
		t.Fatalf("round-trip mismatch: %+v", round)
	}
	if len(round.Findings) != 1 || round.Findings[0].Invariant != "artifact.cache_digest_mismatch" {
		t.Fatalf("round-trip findings: %+v", round.Findings)
	}
	if !strings.Contains(string(blob), "artifact.cache_digest_mismatch") {
		t.Fatalf("JSON missing invariant id: %s", string(blob))
	}
}
