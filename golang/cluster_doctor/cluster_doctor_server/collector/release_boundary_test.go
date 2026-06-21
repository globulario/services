package collector

import "testing"

// The default allowlist must never include self-hosted control-plane services,
// whose install timestamps can be PID-start anchored (A4 ambiguous, Phase 1.5).
func TestDefaultReleaseBoundaryAllowlist_ExcludesSelfHosted(t *testing.T) {
	for _, svc := range DefaultReleaseBoundaryAllowlist {
		if releaseBoundarySelfHosted[svc] {
			t.Errorf("default allowlist must not contain self-hosted service %q", svc)
		}
	}
}

// Self-hosted services are filtered even if explicitly allowlisted.
func TestReleaseBoundarySelfHostedFilter(t *testing.T) {
	for _, svc := range []string{"repository", "node-agent", "cluster-controller", "cluster-doctor"} {
		if !releaseBoundarySelfHosted[svc] {
			t.Errorf("%q must be excluded from release-boundary checks", svc)
		}
	}
}

func TestBoundaryServiceShortName(t *testing.T) {
	cases := map[string]string{
		"globular/echo": "echo",
		"echo":          "echo",
		"core@x/event":  "event",
	}
	for in, want := range cases {
		if got := boundaryServiceShortName(in); got != want {
			t.Errorf("boundaryServiceShortName(%q) = %q, want %q", in, got, want)
		}
	}
}
