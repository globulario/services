package intentaudit

import (
	"context"
	"testing"
)

// mockProvider is a simple RuntimeEvidenceProvider backed by an in-memory map.
type mockProvider struct {
	data        map[string][]byte
	listKeysErr error
}

func (m *mockProvider) GetJSON(_ context.Context, key string) ([]byte, error) {
	v, ok := m.data[key]
	if !ok {
		return nil, context.DeadlineExceeded // simulate missing key
	}
	return v, nil
}

func (m *mockProvider) ListKeys(_ context.Context, prefix string) ([]string, error) {
	if m.listKeysErr != nil {
		return nil, m.listKeysErr
	}
	var keys []string
	for k := range m.data {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func TestRuntimeCheck_MockProvider_Pass(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/": []byte(`{"build_id":"abc123","version":"1.2.3"}`),
		},
	}

	check := DesiredBuildImmutabilityCheck()
	results := EvaluateRuntimeChecks(context.Background(), provider, []RuntimeCheck{check})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "pass" {
		t.Errorf("expected pass, got %s: %s", results[0].Status, results[0].Detail)
	}
}

func TestRuntimeCheck_MockProvider_Unknown(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{},
	}

	check := DesiredBuildImmutabilityCheck()
	results := EvaluateRuntimeChecks(context.Background(), provider, []RuntimeCheck{check})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "unknown" {
		t.Errorf("expected unknown, got %s: %s", results[0].Status, results[0].Detail)
	}
}

func TestRuntimeCheck_MockProvider_MultipleChecks(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/": []byte(`{"build_id":"abc"}`),
		},
	}

	failCheck := RuntimeCheck{
		IntentID:    "test.fail_check",
		Description: "always fails",
		Keys:        []string{"/nonexistent/key"},
		Evaluate: func(evidence map[string][]byte) RuntimeResult {
			if len(evidence) == 0 {
				return RuntimeResult{Status: "fail", Detail: "no evidence"}
			}
			return RuntimeResult{Status: "pass", Detail: "ok"}
		},
	}

	checks := []RuntimeCheck{DesiredBuildImmutabilityCheck(), failCheck}
	results := EvaluateRuntimeChecks(context.Background(), provider, checks)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != "pass" {
		t.Errorf("check 0: expected pass, got %s", results[0].Status)
	}
	if results[1].Status != "fail" {
		t.Errorf("check 1: expected fail, got %s", results[1].Status)
	}
}

func TestMockProvider_ListKeys(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/svc1": []byte(`{}`),
			"/globular/resources/ServiceDesiredVersion/svc2": []byte(`{}`),
			"/globular/nodes/node1/status":                   []byte(`{}`),
		},
	}

	keys, err := provider.ListKeys(context.Background(), "/globular/resources/ServiceDesiredVersion")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// --- EvaluateDesiredBuildImmutability tests ---

func TestDesiredBuildImmutability_AllPinned(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/svc1": []byte(`{
				"spec": {"version": "1.2.3", "build_number": 42, "build_id": "abc-123"}
			}`),
			"/globular/resources/ServiceDesiredVersion/svc2": []byte(`{
				"spec": {"version": "2.0.0", "build_number": 7, "build_id": "def-456"}
			}`),
		},
	}

	result := EvaluateDesiredBuildImmutability(context.Background(), provider)
	if result.Status != "pass" {
		t.Errorf("expected pass, got %s: %s", result.Status, result.Detail)
	}
}

func TestDesiredBuildImmutability_NotPinnedYet(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/svc1": []byte(`{
				"spec": {"version": "1.0.0", "build_number": 0, "build_id": ""}
			}`),
			"/globular/resources/ServiceDesiredVersion/svc2": []byte(`{
				"spec": {"version": "2.0.0", "build_number": 0, "build_id": ""}
			}`),
		},
	}

	result := EvaluateDesiredBuildImmutability(context.Background(), provider)
	if result.Status != "not_applicable" {
		t.Errorf("expected not_applicable, got %s: %s", result.Status, result.Detail)
	}
}

func TestDesiredBuildImmutability_PartiallyPinned(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/svc1": []byte(`{
				"spec": {"version": "1.2.3", "build_number": 5, "build_id": ""}
			}`),
		},
	}

	result := EvaluateDesiredBuildImmutability(context.Background(), provider)
	if result.Status != "fail" {
		t.Errorf("expected fail, got %s: %s", result.Status, result.Detail)
	}
}

func TestDesiredBuildImmutability_NoData(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{},
	}

	result := EvaluateDesiredBuildImmutability(context.Background(), provider)
	if result.Status != "unknown" {
		t.Errorf("expected unknown, got %s: %s", result.Status, result.Detail)
	}
}

func TestDesiredBuildImmutability_MixedState(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			// Fully pinned — would be PASS on its own.
			"/globular/resources/ServiceDesiredVersion/svc1": []byte(`{
				"spec": {"version": "1.2.3", "build_number": 10, "build_id": "aaa-111"}
			}`),
			// Not yet pinned — NOT_APPLICABLE on its own.
			"/globular/resources/ServiceDesiredVersion/svc2": []byte(`{
				"spec": {"version": "2.0.0", "build_number": 0, "build_id": ""}
			}`),
			// Partially pinned — FAIL on its own (worst wins).
			"/globular/resources/ServiceDesiredVersion/svc3": []byte(`{
				"spec": {"version": "3.0.0", "build_number": 5, "build_id": ""}
			}`),
		},
	}

	result := EvaluateDesiredBuildImmutability(context.Background(), provider)
	if result.Status != "fail" {
		t.Errorf("expected fail (worst wins), got %s: %s", result.Status, result.Detail)
	}
}

func TestInstalledStateOwnership_Pass(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/nodes/node-1/packages/SERVICE/gateway": []byte(`{
				"nodeId":"node-1",
				"kind":"SERVICE",
				"name":"gateway",
				"version":"1.2.3",
				"buildNumber":"1"
			}`),
		},
	}

	result := EvaluateInstalledStateOwnership(context.Background(), provider)
	if result.Status != "pass" {
		t.Fatalf("expected pass, got %s: %s", result.Status, result.Detail)
	}
}

func TestInstalledStateOwnership_Fail(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/nodes/node-1/packages/SERVICE/gateway": []byte(`{
				"nodeId":"",
				"kind":"SERVICE",
				"name":"gateway",
				"version":"1.2.3"
			}`),
		},
	}

	result := EvaluateInstalledStateOwnership(context.Background(), provider)
	if result.Status != "fail" {
		t.Fatalf("expected fail, got %s: %s", result.Status, result.Detail)
	}
}

func TestInstalledStateOwnership_Unknown(t *testing.T) {
	provider := &mockProvider{
		data:        map[string][]byte{},
		listKeysErr: context.DeadlineExceeded,
	}
	result := EvaluateInstalledStateOwnership(context.Background(), provider)
	if result.Status != "unknown" {
		t.Fatalf("expected unknown, got %s: %s", result.Status, result.Detail)
	}
}

func TestInstalledStateOwnership_NotApplicable(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/gateway": []byte(`{"spec":{"version":"1.0.0"}}`),
		},
	}
	result := EvaluateInstalledStateOwnership(context.Background(), provider)
	if result.Status != "not_applicable" {
		t.Fatalf("expected not_applicable, got %s: %s", result.Status, result.Detail)
	}
}

func TestRuntimeObservationDoesNotMutateDesired_Pass(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/gateway": []byte(`{"spec":{"service_name":"gateway","version":"1.0.0","build_number":1,"build_id":"b1"}}`),
			"/globular/resources/ServiceDesiredVersion/dns":     []byte(`{"spec":{"service_name":"dns","version":"1.0.0","build_number":1,"build_id":"b2"}}`),
			"/globular/audit/desired_writes/1":                  []byte(`{"service":"gateway","actor":"cluster-controller","source":"reconciler","action":"upsert_desired"}`),
			"/globular/audit/desired_writes/2":                  []byte(`{"service":"dns","actor":"canonicalize-tool","source":"fix-safe-A2","action":"repair_desired_build_id"}`),
		},
	}
	result := EvaluateRuntimeObservationDoesNotMutateDesired(context.Background(), provider)
	if result.Status != "pass" {
		t.Fatalf("expected pass, got %s: %s", result.Status, result.Detail)
	}
}

func TestRuntimeObservationDoesNotMutateDesired_Fail(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/gateway": []byte(`{"spec":{"service_name":"gateway","version":"1.0.0","build_number":1,"build_id":"b1"}}`),
			"/globular/audit/desired_writes/1":                  []byte(`{"service":"gateway","actor":"node-agent-heartbeat","source":"heartbeat","action":"upsert_desired"}`),
		},
	}
	result := EvaluateRuntimeObservationDoesNotMutateDesired(context.Background(), provider)
	if result.Status != "fail" {
		t.Fatalf("expected fail, got %s: %s", result.Status, result.Detail)
	}
}

func TestRuntimeObservationDoesNotMutateDesired_FailOnForbiddenSource(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/gateway": []byte(`{"spec":{"service_name":"gateway","version":"1.0.0","build_number":1,"build_id":"b1"}}`),
			"/globular/audit/desired_writes/1":                  []byte(`{"service":"gateway","actor":"cluster-controller","source":"heartbeat","action":"upsert_desired"}`),
		},
	}
	result := EvaluateRuntimeObservationDoesNotMutateDesired(context.Background(), provider)
	if result.Status != "fail" {
		t.Fatalf("expected fail, got %s: %s", result.Status, result.Detail)
	}
}

func TestRuntimeObservationDoesNotMutateDesired_FailOnVerifierActor(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/gateway": []byte(`{"spec":{"service_name":"gateway","version":"1.0.0","build_number":1,"build_id":"b1"}}`),
			"/globular/audit/desired_writes/1":                  []byte(`{"service":"gateway","actor":"verifier","source":"verification-loop","action":"upsert_desired"}`),
		},
	}
	result := EvaluateRuntimeObservationDoesNotMutateDesired(context.Background(), provider)
	if result.Status != "fail" {
		t.Fatalf("expected fail, got %s: %s", result.Status, result.Detail)
	}
}

func TestRuntimeObservationDoesNotMutateDesired_Unknown(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/resources/ServiceDesiredVersion/gateway": []byte(`{"spec":{"service_name":"gateway","version":"1.0.0","build_number":1,"build_id":"b1"}}`),
		},
	}
	result := EvaluateRuntimeObservationDoesNotMutateDesired(context.Background(), provider)
	if result.Status != "unknown" {
		t.Fatalf("expected unknown, got %s: %s", result.Status, result.Detail)
	}
}

func TestRuntimeObservationDoesNotMutateDesired_NotApplicable(t *testing.T) {
	provider := &mockProvider{
		data: map[string][]byte{
			"/globular/nodes/node-1/packages/SERVICE/gateway": []byte(`{"nodeId":"node-1","name":"gateway","kind":"SERVICE","version":"1.2.3"}`),
		},
	}
	result := EvaluateRuntimeObservationDoesNotMutateDesired(context.Background(), provider)
	if result.Status != "not_applicable" {
		t.Fatalf("expected not_applicable, got %s: %s", result.Status, result.Detail)
	}
}
