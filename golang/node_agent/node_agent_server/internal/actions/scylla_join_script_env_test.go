package actions

import "testing"

// TestScyllaJoinScriptEnv verifies the Day-1 ScyllaDB fresh-join script env is
// produced only for scylladb during an active join, and nil otherwise — so the
// destructive fresh-join branch can never be activated during steady-state
// re-installs or for other infrastructure packages.
func TestScyllaJoinScriptEnv(t *testing.T) {
	tests := []struct {
		name      string
		component string
		joinActive bool
		wantFresh bool
	}{
		{"scylladb joining", "scylladb", true, true},
		{"scylladb case-insensitive", "ScyllaDB", true, true},
		{"scylladb trimmed", "  scylladb  ", true, true},
		{"scylladb not joining", "scylladb", false, false},
		{"etcd joining", "etcd", true, false},
		{"minio joining", "minio", true, false},
		{"empty component", "", true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := scyllaJoinScriptEnv(tc.component, tc.joinActive)
			if !tc.wantFresh {
				if env != nil {
					t.Fatalf("expected nil env, got %v", env)
				}
				return
			}
			if env["SCYLLA_INSTALL_INTENT"] != "fresh-join" {
				t.Errorf("SCYLLA_INSTALL_INTENT = %q, want fresh-join", env["SCYLLA_INSTALL_INTENT"])
			}
			if env["ALLOW_STALE_SCYLLA_REINIT_ON_JOIN"] != "true" {
				t.Errorf("ALLOW_STALE_SCYLLA_REINIT_ON_JOIN = %q, want true", env["ALLOW_STALE_SCYLLA_REINIT_ON_JOIN"])
			}
		})
	}
}
