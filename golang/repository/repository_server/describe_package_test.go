package main

import (
	"testing"
)

// Tests for the pure-function parts of DescribePackage: key parsing,
// name matching, desired-state JSON unmarshal, and the version sort.
// These run without etcd / MinIO / a live server, so they lock in
// the contract even when the full test suite is not exercised.

func TestExtractNodeIDFromKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/globular/nodes/eb9a2dac-05b0-52ac-9002-99d8ffd35902/packages/SERVICE/cluster-controller", "eb9a2dac-05b0-52ac-9002-99d8ffd35902"},
		{"/globular/nodes/node-1/packages/INFRASTRUCTURE/etcd", "node-1"},
		{"/globular/nodes/n/packages/COMMAND/ffmpeg", "n"},
		// Malformed / missing prefix → empty.
		{"/globular/resources/foo", ""},
		{"nodes/foo/packages/bar", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := extractNodeIDFromKey(tc.in); got != tc.want {
			t.Errorf("extractNodeIDFromKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractInfraPublisher(t *testing.T) {
	cases := []struct {
		key, name, want string
	}{
		{"/globular/resources/InfrastructureRelease/core@globular.io/etcd", "etcd", "core@globular.io"},
		{"/globular/resources/InfrastructureRelease/acme/minio", "minio", "acme"},
		// Nested publisher path (infrastructure releases shouldn't nest,
		// but the parser must still not crash).
		{"/globular/resources/InfrastructureRelease/pub/a/b/scylla", "scylla", "pub/a/b"},
		// Wrong prefix → empty.
		{"/foo/bar/etcd", "etcd", ""},
	}
	for _, tc := range cases {
		if got := extractInfraPublisher(tc.key, tc.name); got != tc.want {
			t.Errorf("extractInfraPublisher(%q, %q) = %q, want %q", tc.key, tc.name, got, tc.want)
		}
	}
}

func TestNameMatchesAny(t *testing.T) {
	cands := []string{"claude", "claude_svc"}
	cases := []struct {
		have string
		want bool
	}{
		{"claude", true},
		{"CLAUDE", true}, // case-insensitive
		{"claude_svc", true},
		{"claude-svc", false}, // not in candidates
		{"other", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := nameMatchesAny(tc.have, cands); got != tc.want {
			t.Errorf("nameMatchesAny(%q) = %v, want %v", tc.have, got, tc.want)
		}
	}
}

func TestParseDesiredServiceVersion(t *testing.T) {
	body := []byte(`{
		"meta": {"generation": 42},
		"spec": {"service_name": "cluster-controller", "version": "1.2.3"}
	}`)
	got := parseDesiredServiceVersion(body)
	if got == nil {
		t.Fatal("expected desired, got nil")
	}
	if got.GetVersion() != "1.2.3" || got.GetGeneration() != 42 || !got.GetPresent() {
		t.Errorf("parseDesiredServiceVersion: %+v", got)
	}

	// Missing version → nil (document exists but is incomplete; treat as absent).
	nv := []byte(`{"meta":{"generation":1},"spec":{"service_name":"x"}}`)
	if got := parseDesiredServiceVersion(nv); got != nil {
		t.Errorf("empty version should return nil, got %+v", got)
	}
	// Malformed JSON → nil.
	if got := parseDesiredServiceVersion([]byte("not json")); got != nil {
		t.Errorf("malformed JSON should return nil, got %+v", got)
	}
}

func TestParseDesiredInfraRelease(t *testing.T) {
	body := []byte(`{"meta":{"generation":7},"spec":{"version":"0.5.0"}}`)
	got := parseDesiredInfraRelease(body, "core@globular.io")
	if got == nil {
		t.Fatal("expected desired, got nil")
	}
	if got.GetVersion() != "0.5.0" || got.GetGeneration() != 7 {
		t.Errorf("parseDesiredInfraRelease: version/generation wrong: %+v", got)
	}
	if got.GetPublisher() != "core@globular.io" {
		t.Errorf("publisher not propagated: %q", got.GetPublisher())
	}
	if !got.GetPresent() {
		t.Errorf("present should be true when version is set")
	}
}

func TestSortSemverDesc(t *testing.T) {
	cases := []struct {
		in, want []string
	}{
		{[]string{"1.0.0", "2.0.0", "0.9.0"}, []string{"2.0.0", "1.0.0", "0.9.0"}},
		{[]string{"1.2.3", "1.2.10", "1.2.4"}, []string{"1.2.10", "1.2.4", "1.2.3"}}, // numeric, not lex
		{[]string{"0.1.0"}, []string{"0.1.0"}},
		{[]string{}, []string{}},
	}
	for _, tc := range cases {
		cp := append([]string{}, tc.in...)
		sortSemverDesc(cp)
		if !slicesEqual(cp, tc.want) {
			t.Errorf("sortSemverDesc(%v) = %v, want %v", tc.in, cp, tc.want)
		}
	}
}

func TestCompareVersionsNumeric(t *testing.T) {
	// The comparator MUST be numeric (1.2.10 > 1.2.9) not lexicographic.
	// A regression here would break "latest version" detection.
	if compareVersions("1.2.10", "1.2.9") <= 0 {
		t.Errorf("1.2.10 should be > 1.2.9 (numeric compare)")
	}
	if compareVersions("2.0.0", "1.99.99") <= 0 {
		t.Errorf("2.0.0 should be > 1.99.99")
	}
	if compareVersions("1.0", "1.0.0") != 0 {
		t.Errorf("1.0 should equal 1.0.0 (trailing zero)")
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
