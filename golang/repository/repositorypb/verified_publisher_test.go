package repositorypb

import "testing"

func TestIsVerifiedPublisher_OfficialNamespace(t *testing.T) {
	// Official namespaces are always verified, regardless of owner.
	officials := []string{"globular", "system", "core", "globular.io", "system-infra", "core.utils", "core@globular.io"}
	for _, ns := range officials {
		if !IsVerifiedPublisher(ns, false) {
			t.Errorf("IsVerifiedPublisher(%q, false) should be true (official namespace)", ns)
		}
	}
}

func TestIsVerifiedPublisher_RealOwner(t *testing.T) {
	if !IsVerifiedPublisher("acme", true) {
		t.Error("namespace with real owner should be verified")
	}
}

func TestIsVerifiedPublisher_NoOwner(t *testing.T) {
	if IsVerifiedPublisher("acme", false) {
		t.Error("namespace without owner should not be verified")
	}
}

func TestIsOfficialNamespace_Prefixes(t *testing.T) {
	tests := []struct {
		ns   string
		want bool
	}{
		{"globular", true},
		{"system", true},
		{"core", true},
		{"globular.io", true},
		{"globular-services", true},
		{"system-infra", true},
		{"system.auth", true},
		{"core.utils", true},
		{"core-networking", true},
		{"core@globular.io", true},
		{"globular@io", true},
		{"system@infra", true},
	}
	for _, tt := range tests {
		got := IsOfficialNamespace(tt.ns)
		if got != tt.want {
			t.Errorf("IsOfficialNamespace(%q) = %v, want %v", tt.ns, got, tt.want)
		}
	}
}

func TestIsOfficialNamespace_NonOfficial(t *testing.T) {
	tests := []string{
		"acme",
		"mycompany",
		"dave",
		"unofficial-core",  // "core" is not a prefix here
		"xglobular",
		"mysystem",
	}
	for _, ns := range tests {
		if IsOfficialNamespace(ns) {
			t.Errorf("IsOfficialNamespace(%q) should be false", ns)
		}
	}
}
