package planexec

import "testing"

func TestRolloutPhaseForAction(t *testing.T) {
	tests := []struct {
		action string
		phase  string
	}{
		{"artifact.fetch", "DOWNLOADING"},
		{"artifact.verify", "VERIFYING"},
		{"service.install_payload", "STAGING"},
		{"service.restart", "RESTARTING"},
		{"service.write_version_marker", "COMMITTING"},
		{"package.report_state", "COMMITTING"},
		{"unknown_action", ""},
	}

	for _, tt := range tests {
		got := rolloutPhaseForAction(tt.action)
		if got != tt.phase {
			t.Errorf("rolloutPhaseForAction(%q) = %q, want %q", tt.action, got, tt.phase)
		}
	}
}
