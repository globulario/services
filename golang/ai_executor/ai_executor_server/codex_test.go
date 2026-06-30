package main

import "testing"

func TestCodexEffectiveModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		c    *codexClient
		want string
	}{
		{
			name: "nil client",
			c:    nil,
			want: "",
		},
		{
			name: "explicit config wins",
			c: &codexClient{
				cfg: CodexConfig{Model: "custom-model"},
			},
			want: "custom-model",
		},
		{
			name: "api key auth gets codex default",
			c: &codexClient{
				apiKey: "sk-test",
			},
			want: "gpt-5-codex",
		},
		{
			name: "chatgpt token auth leaves model unset",
			c: &codexClient{
				accessToken: "tok-test",
			},
			want: "",
		},
		{
			name: "mixed auth still avoids forced model",
			c: &codexClient{
				apiKey:      "sk-test",
				accessToken: "tok-test",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.c.effectiveModel(); got != tt.want {
				t.Fatalf("effectiveModel() = %q, want %q", got, tt.want)
			}
		})
	}
}
