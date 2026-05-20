package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSplitEndpoints(t *testing.T) {
	cases := map[string][]string{
		"":                                       nil,
		"   ":                                    nil,
		"https://a:2379":                         {"https://a:2379"},
		"https://a:2379,https://b:2379":          {"https://a:2379", "https://b:2379"},
		"  https://a:2379  ,  https://b:2379  ":  {"https://a:2379", "https://b:2379"},
		",https://a:2379,,https://b:2379,":       {"https://a:2379", "https://b:2379"},
	}
	for in, want := range cases {
		got := splitEndpoints(in)
		if len(got) != len(want) {
			t.Errorf("splitEndpoints(%q) = %v (len %d), want %v (len %d)", in, got, len(got), want, len(want))
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("splitEndpoints(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}

func TestExtractEndpointHost(t *testing.T) {
	cases := map[string]string{
		"":                                "",
		"https://10.0.0.63:2379":          "10.0.0.63",
		"https://10.0.0.63:2379/api/v1":   "10.0.0.63",
		"http://example.com:2379":         "example.com",
		"10.0.0.63:2379":                  "10.0.0.63",
		"globule-ryzen.example.com:2379":  "globule-ryzen.example.com",
		"  https://10.0.0.63:2379  ":      "10.0.0.63",
		// IPv6 — brackets must be stripped, host returned unbracketed.
		// Selection still passes the ORIGINAL endpoint string to etcdctl
		// verbatim; the un-bracketed form is only used for local matching.
		"https://[fd00::1]:2379":          "fd00::1",
		"https://[fd00::1]:2379/api/v1":   "fd00::1",
		"[fd00::1]:2379":                  "fd00::1",
		"https://[::1]:2379":              "::1",
		"[::1]:2379":                      "::1",
		// Bare IPv6 with no port and no brackets is accepted as-is. etcd
		// endpoints normally require a port, but the local-match path
		// shouldn't crash on a malformed input — downstream connect will
		// surface the real error.
		"fd00::1":                         "fd00::1",
		"::1":                             "::1",
	}
	for in, want := range cases {
		if got := extractEndpointHost(in); got != want {
			t.Errorf("extractEndpointHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSelectEtcdSnapshotEndpoint_IPv6Roundtrip pins the behavior that the
// chosen endpoint is returned EXACTLY as configured (preserving brackets,
// scheme, port), even though host matching uses the unbracketed form. This
// matters because etcdctl expects the canonical bracketed form for IPv6.
func TestSelectEtcdSnapshotEndpoint_IPv6Roundtrip(t *testing.T) {
	restore := withProberAndLocal(t,
		stubProber(map[string]bool{
			"https://[fd00::1]:2379": true,
			"https://[fd00::2]:2379": true,
		}),
		map[string]struct{}{},
	)
	defer restore()

	srv := &server{}
	got, err := srv.selectEtcdSnapshotEndpoint(context.Background(),
		"https://[fd00::1]:2379,https://[fd00::2]:2379",
		"", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First entry is selected (no locality match, no failure on prior probe).
	if got != "https://[fd00::1]:2379" {
		t.Fatalf("got %q, want https://[fd00::1]:2379 (must round-trip the bracketed form)", got)
	}
	if strings.Contains(got, ",") {
		t.Fatalf("ipv6 input produced multi-endpoint result %q", got)
	}
}

func TestReorderLocalFirst(t *testing.T) {
	local := map[string]struct{}{"10.0.0.8": {}, "globule-nuc": {}}
	eps := []string{
		"https://10.0.0.20:2379",
		"https://10.0.0.8:2379",
		"https://10.0.0.9:2379",
		"https://globule-nuc.example.com:2379",
	}
	got := reorderLocalFirst(eps, local)
	// The two local ones (10.0.0.8 and globule-nuc.example.com) must come first
	// in their original relative order; the other two follow in original order.
	want := []string{
		"https://10.0.0.8:2379",
		"https://globule-nuc.example.com:2379",
		"https://10.0.0.20:2379",
		"https://10.0.0.9:2379",
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}

// stubProber returns a prober that pretends configured endpoints are healthy
// and everything else fails. healthy is the set of endpoint strings that
// should pass the health check.
func stubProber(healthy map[string]bool) func(context.Context, string, string, string, string) error {
	return func(_ context.Context, ep, _, _, _ string) error {
		if healthy[ep] {
			return nil
		}
		return errors.New("stub: unhealthy")
	}
}

// withProberAndLocal swaps the etcdHealthProber + locks the local matcher set
// so tests are independent of the host they run on. Returns a restore func.
func withProberAndLocal(t *testing.T, prober func(context.Context, string, string, string, string) error, local map[string]struct{}) func() {
	t.Helper()
	prevProber := etcdHealthProber
	prevLocal := localHostMatchersFn
	etcdHealthProber = prober
	localHostMatchersFn = func() map[string]struct{} { return local }
	return func() {
		etcdHealthProber = prevProber
		localHostMatchersFn = prevLocal
	}
}

func TestSelectEtcdSnapshotEndpoint_PrefersLocal(t *testing.T) {
	restore := withProberAndLocal(t,
		stubProber(map[string]bool{
			"https://10.0.0.20:2379": true,
			"https://10.0.0.63:2379": true,
			"https://10.0.0.9:2379":  true,
		}),
		map[string]struct{}{"10.0.0.63": {}},
	)
	defer restore()

	srv := &server{}
	got, err := srv.selectEtcdSnapshotEndpoint(context.Background(),
		"https://10.0.0.20:2379,https://10.0.0.63:2379,https://10.0.0.9:2379",
		"", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://10.0.0.63:2379"
	if got != want {
		t.Fatalf("got %q, want %q (local should win)", got, want)
	}
}

func TestSelectEtcdSnapshotEndpoint_FallbackWhenLocalUnhealthy(t *testing.T) {
	restore := withProberAndLocal(t,
		stubProber(map[string]bool{
			// local unhealthy
			"https://10.0.0.63:2379": false,
			// first remote unhealthy
			"https://10.0.0.20:2379": false,
			// second remote healthy
			"https://10.0.0.9:2379":  true,
		}),
		map[string]struct{}{"10.0.0.63": {}},
	)
	defer restore()

	srv := &server{}
	got, err := srv.selectEtcdSnapshotEndpoint(context.Background(),
		"https://10.0.0.20:2379,https://10.0.0.63:2379,https://10.0.0.9:2379",
		"", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://10.0.0.9:2379" {
		t.Fatalf("got %q, want https://10.0.0.9:2379", got)
	}
}

func TestSelectEtcdSnapshotEndpoint_NoneHealthy(t *testing.T) {
	restore := withProberAndLocal(t,
		stubProber(map[string]bool{}),
		map[string]struct{}{},
	)
	defer restore()

	srv := &server{}
	_, err := srv.selectEtcdSnapshotEndpoint(context.Background(),
		"https://a:2379,https://b:2379",
		"", "", "")
	if err == nil {
		t.Fatal("expected error when no endpoint is healthy")
	}
	if !errors.Is(err, errNoHealthyEtcdEndpoint) {
		t.Fatalf("expected errors.Is(err, errNoHealthyEtcdEndpoint), got %v", err)
	}
}

func TestSelectEtcdSnapshotEndpoint_SingleEndpoint(t *testing.T) {
	restore := withProberAndLocal(t,
		stubProber(map[string]bool{"https://only:2379": true}),
		map[string]struct{}{},
	)
	defer restore()

	srv := &server{}
	got, err := srv.selectEtcdSnapshotEndpoint(context.Background(),
		"https://only:2379", "", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://only:2379" {
		t.Fatalf("got %q, want https://only:2379", got)
	}
}

// TestSelectEtcdSnapshotEndpoint_ArgsNeverMulti pins the load-bearing
// invariant: the returned endpoint must never contain a comma, because
// etcdctl snapshot save rejects multi-endpoint arguments. Whatever the
// configured list looks like, the picked value is one endpoint only.
func TestSelectEtcdSnapshotEndpoint_ArgsNeverMulti(t *testing.T) {
	restore := withProberAndLocal(t,
		stubProber(map[string]bool{
			"https://a:2379": true,
			"https://b:2379": true,
			"https://c:2379": true,
		}),
		map[string]struct{}{},
	)
	defer restore()

	srv := &server{}
	for _, csv := range []string{
		"https://a:2379",
		"https://a:2379,https://b:2379",
		"https://a:2379,https://b:2379,https://c:2379",
		"  https://a:2379  ,  https://b:2379  ",
	} {
		got, err := srv.selectEtcdSnapshotEndpoint(context.Background(), csv, "", "", "")
		if err != nil {
			t.Fatalf("csv=%q unexpected error: %v", csv, err)
		}
		if strings.Contains(got, ",") {
			t.Fatalf("csv=%q produced multi-endpoint result %q — etcdctl will reject this", csv, got)
		}
	}
}

func TestSelectEtcdSnapshotEndpoint_EmptyConfig(t *testing.T) {
	srv := &server{}
	_, err := srv.selectEtcdSnapshotEndpoint(context.Background(), "", "", "", "")
	if err == nil {
		t.Fatal("expected error on empty CSV")
	}
	if !strings.Contains(err.Error(), "no endpoints configured") {
		t.Fatalf("error should mention missing endpoints, got: %v", err)
	}
}
