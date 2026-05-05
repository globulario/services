package rules

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/config"
)

func TestCriticalKeyRegistryPresence_MissingKeysEmitFindings(t *testing.T) {
	inv := criticalKeyRegistryPresence{}
	// All keys absent.
	snap := &collector.Snapshot{CriticalKeyPresent: map[string]bool{}}
	findings := inv.Evaluate(snap, Config{})
	expected := len(config.CriticalEtcdKeys) + len(config.CriticalEtcdPrefixes)
	if len(findings) != expected {
		t.Fatalf("expected %d findings (one per registry entry), got %d", expected, len(findings))
	}
}

func TestCriticalKeyRegistryPresence_PresentKeysNoFinding(t *testing.T) {
	inv := criticalKeyRegistryPresence{}
	present := map[string]bool{}
	for _, k := range config.CriticalEtcdKeys {
		present[k] = true
	}
	for _, p := range config.CriticalEtcdPrefixes {
		present[p] = true
	}
	snap := &collector.Snapshot{CriticalKeyPresent: present}
	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when all keys present, got %d", len(findings))
	}
}

func TestKeyToInvariantID(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"/globular/system/config", "system.config_missing"},
		{"/globular/ingress/v1/spec_backup", "ingress.spec_backup_missing"},
		{"/globular/ingress/v1/spec", "ingress.spec_missing"},
		{"/globular/resources/", "resources_missing"},
		{"/globular/nodes/", "nodes_missing"},
	}
	for _, tc := range cases {
		got := keyToInvariantID(tc.key)
		if got != tc.want {
			t.Errorf("keyToInvariantID(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
	// All generated IDs should end in "_missing"
	for _, k := range config.CriticalEtcdKeys {
		id := keyToInvariantID(k)
		if !strings.HasSuffix(id, "_missing") {
			t.Errorf("keyToInvariantID(%q) = %q, expected suffix '_missing'", k, id)
		}
	}
}

