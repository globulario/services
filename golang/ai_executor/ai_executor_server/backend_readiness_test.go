package main

import "testing"

// TestBackendReadiness_FourStates pins the honest four-state model and the
// derived operating mode. It is the regression guard for the live-cluster bug
// where ai_available reported true purely because a claude binary existed.
func TestBackendReadiness_FourStates(t *testing.T) {
	cases := []struct {
		name string
		d    *diagnoser
		want backendReadiness
		mode string
	}{
		{
			name: "nil diagnoser",
			d:    nil,
			want: backendReadiness{},
			mode: "deterministic_fallback",
		},
		{
			name: "nothing configured",
			d:    &diagnoser{},
			want: backendReadiness{},
			mode: "deterministic_fallback",
		},
		{
			name: "binary present only, no credentials — the live-cluster lie",
			d:    &diagnoser{claude: &claudeClient{cliBinary: "/usr/local/bin/claude"}},
			want: backendReadiness{ClaudeBinaryPresent: true},
			mode: "deterministic_fallback",
		},
		{
			name: "codex binary present only, no credentials",
			d:    &diagnoser{codex: &codexClient{cliBinary: "/usr/local/bin/codex"}},
			want: backendReadiness{CodexBinaryPresent: true},
			mode: "deterministic_fallback",
		},
		{
			name: "credentials present but not usable (refresh token only, no access)",
			d:    &diagnoser{anthropic: &anthropicClient{refreshToken: "rt", expiresAt: 1}},
			want: backendReadiness{CredentialsPresent: true},
			mode: "deterministic_fallback",
		},
		{
			name: "backend ready (api key) but no analysis yet",
			d:    &diagnoser{anthropic: &anthropicClient{cfg: AnthropicConfig{APIKey: "sk-ant-x"}}},
			want: backendReadiness{CredentialsPresent: true, BackendReady: true, Backend: "anthropic"},
			mode: "ai",
		},
		{
			name: "binary + backend ready + a real analysis happened",
			d: func() *diagnoser {
				d := &diagnoser{
					claude:    &claudeClient{cliBinary: "/usr/local/bin/claude"},
					anthropic: &anthropicClient{cfg: AnthropicConfig{APIKey: "sk-ant-x"}},
				}
				d.aiAnalysesOK = 1
				return d
			}(),
			want: backendReadiness{ClaudeBinaryPresent: true, CredentialsPresent: true, BackendReady: true, AnalysisAvailable: true, Backend: "anthropic"},
			mode: "ai",
		},
		{
			name: "codex autonomous backend ready",
			d: &diagnoser{
				codex: &codexClient{cliBinary: "/usr/local/bin/codex", hasAuth: true, accessToken: "tok"},
			},
			want: backendReadiness{CodexBinaryPresent: true, CredentialsPresent: true, BackendReady: true, Backend: "codex"},
			mode: "ai",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.d.readiness()
			if got != tc.want {
				t.Errorf("readiness() = %+v, want %+v", got, tc.want)
			}
			if got.Mode() != tc.mode {
				t.Errorf("Mode() = %q, want %q", got.Mode(), tc.mode)
			}
			// aiReady (the honest ai_available) must equal BackendReady.
			if tc.d.aiReady() != tc.want.BackendReady {
				t.Errorf("aiReady() = %v, want %v", tc.d.aiReady(), tc.want.BackendReady)
			}
		})
	}
}

// TestAiReady_IgnoresClaudeBinary pins the core honesty fix: a claude CLI binary
// on disk must NOT make the autonomous backend "ready" (it is excluded from the
// autonomous diagnose path; see repeat_diagnosis_drains_personal_subscription).
func TestAiReady_IgnoresClaudeBinary(t *testing.T) {
	d := &diagnoser{claude: &claudeClient{cliBinary: "/usr/local/bin/claude"}}
	if d.aiReady() {
		t.Fatal("aiReady() must be false when only the claude CLI binary exists (no anthropic backend)")
	}
	if got := d.readiness().Mode(); got != "deterministic_fallback" {
		t.Fatalf("Mode() = %q, want deterministic_fallback when only the binary exists", got)
	}
}
