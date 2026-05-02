package main

// actor_service_test.go — Phase F-final tests for the WorkflowActorService
// dispatch + the input-decoding helpers used by package.rollback.

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/workflow/workflowpb"
)

func TestActor_UnknownActionRejected(t *testing.T) {
	a := &NodeAgentActorServer{srv: nil}
	resp, err := a.ExecuteAction(context.Background(), &workflowpb.ExecuteActionRequest{
		Action: "totally.fake.thing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.GetOk() {
		t.Fatal("unknown action must NOT be ok")
	}
	if !strings.Contains(resp.GetMessage(), "unknown action") {
		t.Fatalf("expected 'unknown action' in message, got %q", resp.GetMessage())
	}
}

func TestActor_EmptyActionRejected(t *testing.T) {
	a := &NodeAgentActorServer{}
	resp, _ := a.ExecuteAction(context.Background(), &workflowpb.ExecuteActionRequest{})
	if resp.GetOk() {
		t.Fatal("empty action must NOT be ok")
	}
}

func TestActor_ConfigClassifyAndApplyPolicyAreNoOpSuccesses(t *testing.T) {
	// These two actions exist in the package.rollback YAML but the work
	// they describe is folded into ApplyPackageRelease's pre-install gate
	// + post-success hook. The actor must return success so the workflow
	// can advance to the next step.
	a := &NodeAgentActorServer{}
	for _, action := range []string{"package.config.classify", "package.config.apply_policy"} {
		resp, err := a.ExecuteAction(context.Background(), &workflowpb.ExecuteActionRequest{
			Action: action,
		})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", action, err)
		}
		if !resp.GetOk() {
			t.Errorf("%s: expected ok=true, got %q", action, resp.GetMessage())
		}
	}
}

// ── Input decoding ─────────────────────────────────────────────────────────

func TestActor_DecodeWithJsonAndInputsJson(t *testing.T) {
	req := &workflowpb.ExecuteActionRequest{
		WithJson:   `{"package_name":"echo","rollback_mode":true,"build_number":42}`,
		InputsJson: `{"publisher":"core@globular.io","platform":"linux_amd64"}`,
	}
	with, inputs := decodeActionInputs(req)
	if with["package_name"] != "echo" {
		t.Errorf("with.package_name: got %v", with["package_name"])
	}
	if with["rollback_mode"] != true {
		t.Errorf("with.rollback_mode: got %v", with["rollback_mode"])
	}
	if inputs["publisher"] != "core@globular.io" {
		t.Errorf("inputs.publisher: got %v", inputs["publisher"])
	}
}

func TestActor_PickFieldPrefersWith(t *testing.T) {
	get := pickField(
		map[string]any{"package_name": "from-with"},
		map[string]any{"package_name": "from-inputs"},
	)
	if got := strField(get, "package_name"); got != "from-with" {
		t.Errorf("with should win, got %q", got)
	}
}

func TestActor_PickFieldFallsBackToInputs(t *testing.T) {
	get := pickField(
		map[string]any{},
		map[string]any{"package_name": "from-inputs"},
	)
	if got := strField(get, "package_name"); got != "from-inputs" {
		t.Errorf("inputs fallback failed, got %q", got)
	}
}

func TestActor_BoolFieldHonorsStringTrue(t *testing.T) {
	cases := map[string]bool{
		"true":  true,
		"True":  true,
		"false": false,
		"":      false,
		"yes":   false, // strconv.ParseBool only accepts true/false/1/0/T/F
	}
	for in, want := range cases {
		get := pickField(map[string]any{"x": in}, nil)
		if got := boolField(get, "x"); got != want {
			t.Errorf("boolField(%q): got %v, want %v", in, got, want)
		}
	}
}

func TestActor_Int64FieldFromFloat64(t *testing.T) {
	get := pickField(map[string]any{"build_number": float64(42)}, nil)
	if got := int64Field(get, "build_number"); got != 42 {
		t.Fatalf("int64Field from float64: got %d", got)
	}
}

func TestActor_UnitForPackageName(t *testing.T) {
	get := pickField(map[string]any{"package_name": "node_agent"}, nil)
	if got := unitFor(get); got != "globular-node-agent.service" {
		t.Errorf("unitFor: got %q", got)
	}
}

func TestActor_ServiceDrainMissingPackage(t *testing.T) {
	a := &NodeAgentActorServer{}
	resp, _ := a.ExecuteAction(context.Background(), &workflowpb.ExecuteActionRequest{
		Action:   "service.drain",
		WithJson: `{}`,
	})
	if resp.GetOk() {
		t.Fatal("drain without package must NOT be ok")
	}
}

func TestActor_ForwardRecoverWithoutPreviousFails(t *testing.T) {
	a := &NodeAgentActorServer{}
	resp, _ := a.ExecuteAction(context.Background(), &workflowpb.ExecuteActionRequest{
		Action: "package.rollback.forward_recover",
	})
	if resp.GetOk() {
		t.Fatal("forward_recover without previous identity must NOT be ok")
	}
	if !strings.Contains(resp.GetMessage(), "previous revision identity unknown") {
		t.Errorf("expected 'previous revision identity unknown' in message, got %q",
			resp.GetMessage())
	}
}
