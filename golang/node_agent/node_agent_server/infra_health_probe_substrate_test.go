package main

import "testing"

// TestScyllaSubstrateVerified proves the Scylla probe reports Verified only on
// substrate truth (nodetool UN / REST NORMAL), never from reachability. TCP-9042
// is not even an input to the decision, so an open port alone can never verify
// (forbidden_fix:heuristic_signal_marks_substrate_verified).
func TestScyllaSubstrateVerified(t *testing.T) {
	cases := []struct {
		nodetool, restMode string
		want               bool
		note               string
	}{
		{"UN", "", true, "nodetool Up/Normal passes"},
		{"", "NORMAL", true, "REST operation mode NORMAL passes"},
		{"UN", "NORMAL", true, "both agree"},
		{"JOINING", "", false, "JOINING does not pass"},
		{"", "JOINING", false, "REST JOINING does not pass"},
		{"DN", "", false, "Down/Normal does not pass"},
		{"UJ", "", false, "Up/Joining does not pass"},
		{"", "BOOTSTRAPPING", false, "bootstrapping does not pass"},
		// The critical one: no nodetool/REST truth at all — i.e. only TCP-9042 was
		// reachable — must NOT verify. Reachability is not health.
		{"", "", false, "no substrate truth (TCP-reachable-only) does not verify"},
	}
	for _, c := range cases {
		if got := scyllaSubstrateVerified(c.nodetool, c.restMode); got != c.want {
			t.Errorf("scyllaSubstrateVerified(%q,%q)=%v want %v (%s)", c.nodetool, c.restMode, got, c.want, c.note)
		}
	}
}

// TestMinioSubstrateVerified proves the MinIO probe reports Verified only when the
// pool has BOTH write quorum and read quorum. Liveness (server answers) and
// reachability (port 9000 open) are not inputs, so neither alone can verify
// (forbidden_fix:heuristic_signal_marks_substrate_verified).
func TestMinioSubstrateVerified(t *testing.T) {
	cases := []struct {
		write, read bool
		want        bool
		note        string
	}{
		{false, false, false, "live/reachable-only (no quorum) does not verify"},
		{true, false, false, "write quorum but no read quorum does not verify"},
		{false, true, false, "read quorum but no write quorum does not verify"},
		{true, true, true, "both write and read quorum verifies"},
	}
	for _, c := range cases {
		if got := minioSubstrateVerified(c.write, c.read); got != c.want {
			t.Errorf("minioSubstrateVerified(write=%v,read=%v)=%v want %v (%s)", c.write, c.read, got, c.want, c.note)
		}
	}
}
