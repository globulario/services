package packagekind

import "testing"

func TestKindOf(t *testing.T) {
	cases := map[string]string{
		"xds":            KindInfrastructure,
		"scylladb":       KindInfrastructure,
		"gateway":        KindInfrastructure,
		"mc":             KindCommand,
		"yt-dlp":         KindCommand,
		"globular-cli":   KindCommand,
		"node-agent":     KindService,
		"dns":            KindService,
		"cluster-doctor": KindService,
	}
	for name, want := range cases {
		got, ok := KindOf(name)
		if !ok || got != want {
			t.Errorf("KindOf(%q) = (%q, %v); want (%q, true)", name, got, ok, want)
		}
	}
}

func TestKindOf_CaseInsensitiveAndTrimmed(t *testing.T) {
	if k, ok := KindOf("  XDS "); !ok || k != KindInfrastructure {
		t.Errorf("KindOf(\"  XDS \") = (%q, %v); want (%q, true)", k, ok, KindInfrastructure)
	}
}

func TestUnknownFailsOpenToService(t *testing.T) {
	const unknown = "acme-thirdparty-service"
	if _, ok := KindOf(unknown); ok {
		t.Errorf("%q must be unknown to the registry projection", unknown)
	}
	if IsInfrastructure(unknown) {
		t.Errorf("unknown %q must not be classified infrastructure (fail-open to service)", unknown)
	}
	if IsCommand(unknown) {
		t.Errorf("unknown %q must not be classified command (fail-open to service)", unknown)
	}
}

func TestProjectionNonEmpty(t *testing.T) {
	if n := len(Names()); n < 40 {
		t.Errorf("registry projection has only %d packages — generator likely parsed the wrong file", n)
	}
}
