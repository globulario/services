package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/security"
)

func ctxWithSubject(subject string) context.Context {
	return (&security.AuthContext{Subject: subject}).ToContext(context.Background())
}

func TestIsProtectedSeed(t *testing.T) {
	tests := []struct {
		name string
		mem  *ai_memorypb.Memory
		want bool
	}{
		{"nil", nil, false},
		{"no metadata", &ai_memorypb.Memory{Id: "x"}, false},
		{
			"source=seed but immutable missing",
			&ai_memorypb.Memory{Metadata: map[string]string{"source": "seed"}},
			false,
		},
		{
			"immutable=true but source missing",
			&ai_memorypb.Memory{Metadata: map[string]string{"immutable": "true"}},
			false,
		},
		{
			"protected seed",
			&ai_memorypb.Memory{Metadata: map[string]string{"source": "seed", "immutable": "true"}},
			true,
		},
		{
			"source=human, immutable=true is NOT a seed",
			&ai_memorypb.Memory{Metadata: map[string]string{"source": "human", "immutable": "true"}},
			false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isProtectedSeed(tc.mem); got != tc.want {
				t.Fatalf("isProtectedSeed = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAuthorizedSeedMutator(t *testing.T) {
	if authorizedSeedMutator(context.Background()) {
		t.Fatal("anonymous context must NOT be authorized")
	}
	if !authorizedSeedMutator(ctxWithSubject("sa")) {
		t.Fatal("subject sa MUST be authorized")
	}
	if authorizedSeedMutator(ctxWithSubject("dave")) {
		t.Fatal("non-sa subject must NOT be authorized")
	}
	if authorizedSeedMutator(ctxWithSubject("ops-knowledge-seeder")) {
		t.Fatal("agent_id is not the same as subject — must NOT be authorized")
	}
}

func TestGuardSeedMutation_AllowsNonSeed(t *testing.T) {
	mem := &ai_memorypb.Memory{Id: "x", Metadata: map[string]string{"source": "agent"}}
	if err := guardSeedMutation(context.Background(), mem, "update"); err != nil {
		t.Fatalf("non-seed memory must not be guarded: %v", err)
	}
}

func TestGuardSeedMutation_BlocksAnonymous(t *testing.T) {
	mem := &ai_memorypb.Memory{
		Id:       "ops.test.entry",
		Metadata: map[string]string{"source": "seed", "immutable": "true"},
	}
	err := guardSeedMutation(context.Background(), mem, "update")
	if err == nil {
		t.Fatal("anonymous mutation must be blocked")
	}
	if !strings.Contains(err.Error(), "ops.test.entry") {
		t.Errorf("error must reference id: %v", err)
	}
	if !strings.Contains(err.Error(), "<anonymous>") {
		t.Errorf("error must label caller as anonymous: %v", err)
	}
}

func TestGuardSeedMutation_BlocksRandomSubject(t *testing.T) {
	mem := &ai_memorypb.Memory{
		Id:       "ops.test.entry",
		Metadata: map[string]string{"source": "seed", "immutable": "true"},
	}
	err := guardSeedMutation(ctxWithSubject("dave"), mem, "delete")
	if err == nil {
		t.Fatal("non-sa subject must be blocked")
	}
	if !strings.Contains(err.Error(), `"dave"`) {
		t.Errorf("error must name the caller: %v", err)
	}
	if !strings.Contains(err.Error(), "delete denied") {
		t.Errorf("error must name the operation: %v", err)
	}
}

func TestGuardSeedMutation_AllowsSA(t *testing.T) {
	mem := &ai_memorypb.Memory{
		Id:       "ops.test.entry",
		Metadata: map[string]string{"source": "seed", "immutable": "true"},
	}
	if err := guardSeedMutation(ctxWithSubject("sa"), mem, "update"); err != nil {
		t.Fatalf("sa must be allowed to mutate seed: %v", err)
	}
}
