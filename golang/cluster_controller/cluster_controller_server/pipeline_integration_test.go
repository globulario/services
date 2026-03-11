package main

import (
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
)

// Pipeline integration tests: verify package -> plan upgrade -> build plan -> dispatch.
// These exercise the full flow using direct function calls (no live services).

func TestPipeline_ServiceUpgrade_EndToEnd(t *testing.T) {
	// Step 1: Create node state with installed version.
	node := &nodeState{
		InstalledVersions: map[string]string{
			"gateway": "1.0.0",
		},
	}

	// Step 2: Verify installed version lookup.
	installed := lookupInstalledVersion(node, "gateway")
	if installed != "1.0.0" {
		t.Fatalf("lookupInstalledVersion: expected 1.0.0, got %q", installed)
	}

	// Step 3: Simulate upgrade available: version 2.0.0 with sha256.
	item := &cluster_controllerpb.UpgradePlanItem{
		Service:     "gateway",
		FromVersion: "1.0.0",
		ToVersion:   "2.0.0",
		Sha256:      "abc123",
	}

	// Step 4: Build the upgrade plan.
	plan := buildUpgradePlanForKind("node-1", item)
	if plan == nil {
		t.Fatal("buildUpgradePlanForKind returned nil")
	}

	// Step 5: Verify plan reason.
	if plan.GetReason() != "service_upgrade" {
		t.Fatalf("expected reason service_upgrade, got %q", plan.GetReason())
	}

	// Step 6: Verify plan steps include the expected actions.
	steps := plan.GetSpec().GetSteps()
	expectedActions := []string{
		"artifact.fetch",
		"artifact.verify",
		"service.install_payload",
		"service.write_version_marker",
		"package.report_state",
		"service.restart",
	}
	if len(steps) != len(expectedActions) {
		t.Fatalf("expected %d steps, got %d", len(expectedActions), len(steps))
	}
	for i, step := range steps {
		if step.GetAction() != expectedActions[i] {
			t.Errorf("step %d: expected %q, got %q", i, expectedActions[i], step.GetAction())
		}
	}

	// Step 7: Verify fetch step has expected_sha256 == "abc123".
	fetchStep := steps[0]
	fetchSHA := fetchStep.GetArgs().GetFields()["expected_sha256"].GetStringValue()
	if fetchSHA != "abc123" {
		t.Fatalf("fetch step expected_sha256: expected abc123, got %q", fetchSHA)
	}

	// Step 8: Verify desired state has 1 service with name "gateway" and version "2.0.0".
	desired := plan.GetSpec().GetDesired()
	if desired == nil {
		t.Fatal("desired state should not be nil")
	}
	services := desired.GetServices()
	if len(services) != 1 {
		t.Fatalf("expected 1 desired service, got %d", len(services))
	}
	if services[0].GetName() != "gateway" {
		t.Errorf("desired service name: expected gateway, got %q", services[0].GetName())
	}
	if services[0].GetVersion() != "2.0.0" {
		t.Errorf("desired service version: expected 2.0.0, got %q", services[0].GetVersion())
	}
}

func TestPipeline_InfrastructureUpgrade_EndToEnd(t *testing.T) {
	// Step 1: Create node state with installed etcd version.
	node := &nodeState{
		InstalledVersions: map[string]string{
			"etcd": "3.5.13",
		},
	}

	// Step 2: Verify installed version lookup.
	installed := lookupInstalledVersion(node, "etcd")
	if installed != "3.5.13" {
		t.Fatalf("lookupInstalledVersion: expected 3.5.13, got %q", installed)
	}

	// Step 3: Create upgrade plan item for etcd.
	item := &cluster_controllerpb.UpgradePlanItem{
		Service:     "etcd",
		FromVersion: "3.5.13",
		ToVersion:   "3.5.14",
		Sha256:      "def456",
	}

	// Step 4: Build the upgrade plan.
	plan := buildUpgradePlanForKind("node-1", item)
	if plan == nil {
		t.Fatal("buildUpgradePlanForKind returned nil")
	}

	// Step 5: Verify plan reason.
	if plan.GetReason() != "infrastructure_release" {
		t.Fatalf("expected reason infrastructure_release, got %q", plan.GetReason())
	}

	// Step 6: Verify first step is "service.stop".
	steps := plan.GetSpec().GetSteps()
	if len(steps) == 0 {
		t.Fatal("expected at least one step")
	}
	if steps[0].GetAction() != "service.stop" {
		t.Fatalf("first step: expected service.stop, got %q", steps[0].GetAction())
	}

	// Step 7: Verify steps include the expected infrastructure actions.
	infraActions := []string{
		"service.stop",
		"artifact.fetch",
		"artifact.verify",
		"infrastructure.install",
		"package.report_state",
		"service.restart",
	}
	if len(steps) != len(infraActions) {
		t.Fatalf("expected %d steps, got %d", len(infraActions), len(steps))
	}
	for i, step := range steps {
		if step.GetAction() != infraActions[i] {
			t.Errorf("step %d: expected %q, got %q", i, infraActions[i], step.GetAction())
		}
	}

	// Step 8: Verify desired state uses infra hash format.
	if plan.GetDesiredHash() == "" {
		t.Fatal("desired hash should not be empty")
	}
	// The infrastructure desired hash is computed from "infra:<publisher>/<component>=<version>;"
	// which produces a deterministic SHA256. Verify it matches the expected computation.
	expectedHash := ComputeInfrastructureDesiredHash(defaultPublisherID(), "etcd", "3.5.14")
	if plan.GetDesiredHash() != expectedHash {
		t.Errorf("desired hash mismatch: got %q, want %q", plan.GetDesiredHash(), expectedHash)
	}
}

func TestPipeline_ApplicationUpgrade_EndToEnd(t *testing.T) {
	// Step 1: Create an ApplicationRelease.
	rel := &cluster_controllerpb.ApplicationRelease{
		Spec: &cluster_controllerpb.ApplicationReleaseSpec{
			PublisherID: "core@globular.io",
			AppName:     "webadmin",
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ApplicationReleaseStatus{
			ResolvedVersion:        "2.0.0",
			ResolvedArtifactDigest: "ghi789",
		},
	}

	// Step 2: Compile the application plan.
	plan, err := CompileApplicationPlan("node-1", rel, "1.0.0", "cluster-1")
	if err != nil {
		t.Fatalf("CompileApplicationPlan failed: %v", err)
	}

	// Step 3: Verify plan steps include the expected actions.
	steps := plan.GetSpec().GetSteps()
	appActions := []string{
		"artifact.fetch",
		"artifact.verify",
		"application.install",
		"package.report_state",
	}
	if len(steps) != len(appActions) {
		t.Fatalf("expected %d steps, got %d", len(appActions), len(steps))
	}
	for i, step := range steps {
		if step.GetAction() != appActions[i] {
			t.Errorf("step %d: expected %q, got %q", i, appActions[i], step.GetAction())
		}
	}

	// Step 4: Verify the fetch step has expected_sha256 == "ghi789".
	fetchStep := steps[0]
	fetchSHA := fetchStep.GetArgs().GetFields()["expected_sha256"].GetStringValue()
	if fetchSHA != "ghi789" {
		t.Fatalf("fetch step expected_sha256: expected ghi789, got %q", fetchSHA)
	}

	// Step 5: Verify desired hash uses "app:" prefix format.
	expectedHash := ComputeApplicationDesiredHash("core@globular.io", "webadmin", "2.0.0")
	if plan.GetDesiredHash() != expectedHash {
		t.Errorf("desired hash mismatch: got %q, want %q", plan.GetDesiredHash(), expectedHash)
	}
}

func TestPipeline_MixedUpgrades_CorrectDispatch(t *testing.T) {
	cases := []struct {
		name           string
		service        string
		expectedReason string
	}{
		{"gateway is service", "gateway", "service_upgrade"},
		{"etcd is infrastructure", "etcd", "infrastructure_release"},
		{"rbac is service", "rbac", "service_upgrade"},
		{"minio is infrastructure", "minio", "infrastructure_release"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := &cluster_controllerpb.UpgradePlanItem{
				Service:     tc.service,
				FromVersion: "1.0.0",
				ToVersion:   "2.0.0",
				Sha256:      "somehash",
			}
			plan := buildUpgradePlanForKind("node-1", item)
			if plan == nil {
				t.Fatal("plan should not be nil")
			}
			if plan.GetReason() != tc.expectedReason {
				t.Errorf("service %q: expected reason %q, got %q", tc.service, tc.expectedReason, plan.GetReason())
			}
		})
	}
}

func TestPipeline_VersionLookup_AllFormats(t *testing.T) {
	// Direct name lookup.
	node := &nodeState{
		InstalledVersions: map[string]string{
			"gateway":                "1.0.0",
			"core@globular.io/rbac": "1.5.0",
		},
	}

	if v := lookupInstalledVersion(node, "gateway"); v != "1.0.0" {
		t.Errorf("direct name: expected 1.0.0, got %q", v)
	}

	// Publisher prefix lookup.
	if v := lookupInstalledVersion(node, "rbac"); v != "1.5.0" {
		t.Errorf("publisher prefix: expected 1.5.0, got %q", v)
	}

	// Nil node returns empty.
	if v := lookupInstalledVersion(nil, "gateway"); v != "" {
		t.Errorf("nil node: expected empty, got %q", v)
	}

	// Unknown service returns empty.
	if v := lookupInstalledVersion(node, "nonexistent"); v != "" {
		t.Errorf("unknown service: expected empty, got %q", v)
	}
}

func TestPipeline_UpgradeImpacts_Coverage(t *testing.T) {
	knownServices := []string{"gateway", "rbac", "resource", "authentication", "xds", "etcd", "minio", "envoy"}
	for _, svc := range knownServices {
		impacts := upgradeImpacts(svc)
		if len(impacts) == 0 {
			t.Errorf("%s: expected non-empty impacts, got nil", svc)
		}
	}

	// Unknown service returns nil.
	impacts := upgradeImpacts("unknown_service")
	if impacts != nil {
		t.Errorf("unknown_service: expected nil impacts, got %v", impacts)
	}
}

func TestPipeline_SHA256_RequiredForAllKinds(t *testing.T) {
	sha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	// Service plan: verify fetch step has expected_sha256.
	svcPlan := BuildServiceUpgradePlan("node-1", "rbac", "1.0.0", sha)
	svcFetch := svcPlan.GetSpec().GetSteps()[0]
	if svcFetch.GetAction() != "artifact.fetch" {
		t.Fatalf("service: first step should be artifact.fetch, got %q", svcFetch.GetAction())
	}
	if got := svcFetch.GetArgs().GetFields()["expected_sha256"].GetStringValue(); got != sha {
		t.Errorf("service fetch expected_sha256: got %q, want %q", got, sha)
	}

	// Infrastructure plan: verify fetch step has expected_sha256.
	infraItem := &cluster_controllerpb.UpgradePlanItem{
		Service:     "etcd",
		FromVersion: "3.5.13",
		ToVersion:   "3.5.14",
		Sha256:      sha,
	}
	infraPlan := buildUpgradePlanForKind("node-1", infraItem)
	// Infrastructure plans start with service.stop, fetch is at index 1.
	infraFetch := infraPlan.GetSpec().GetSteps()[1]
	if infraFetch.GetAction() != "artifact.fetch" {
		t.Fatalf("infra: expected artifact.fetch at step 1, got %q", infraFetch.GetAction())
	}
	if got := infraFetch.GetArgs().GetFields()["expected_sha256"].GetStringValue(); got != sha {
		t.Errorf("infra fetch expected_sha256: got %q, want %q", got, sha)
	}

	// Application plan: verify fetch step has expected_sha256.
	appRel := &cluster_controllerpb.ApplicationRelease{
		Spec: &cluster_controllerpb.ApplicationReleaseSpec{
			PublisherID: "core@globular.io",
			AppName:     "testapp",
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ApplicationReleaseStatus{
			ResolvedVersion:        "1.0.0",
			ResolvedArtifactDigest: sha,
		},
	}
	appPlan, err := CompileApplicationPlan("node-1", appRel, "", "cluster-1")
	if err != nil {
		t.Fatalf("CompileApplicationPlan: %v", err)
	}
	appFetch := appPlan.GetSpec().GetSteps()[0]
	if appFetch.GetAction() != "artifact.fetch" {
		t.Fatalf("app: first step should be artifact.fetch, got %q", appFetch.GetAction())
	}
	if got := appFetch.GetArgs().GetFields()["expected_sha256"].GetStringValue(); got != sha {
		t.Errorf("app fetch expected_sha256: got %q, want %q", got, sha)
	}
}

func TestPipeline_RollbackPolicy_AllKinds(t *testing.T) {
	// Service plan: verify FAILURE_MODE_ROLLBACK.
	svcPlan := BuildServiceUpgradePlan("node-1", "gateway", "2.0.0", "hash")
	if svcPlan.GetPolicy().GetFailureMode() != planpb.FailureMode_FAILURE_MODE_ROLLBACK {
		t.Errorf("service plan: expected FAILURE_MODE_ROLLBACK, got %v", svcPlan.GetPolicy().GetFailureMode())
	}

	// Infrastructure plan: verify FAILURE_MODE_ROLLBACK when from_version is set.
	infraItem := &cluster_controllerpb.UpgradePlanItem{
		Service:     "etcd",
		FromVersion: "3.5.13",
		ToVersion:   "3.5.14",
		Sha256:      "hash",
	}
	infraPlan := buildUpgradePlanForKind("node-1", infraItem)
	if infraPlan.GetPolicy().GetFailureMode() != planpb.FailureMode_FAILURE_MODE_ROLLBACK {
		t.Errorf("infra plan: expected FAILURE_MODE_ROLLBACK, got %v", infraPlan.GetPolicy().GetFailureMode())
	}

	// Application plan: verify FAILURE_MODE_ROLLBACK when from_version is set.
	appRel := &cluster_controllerpb.ApplicationRelease{
		Spec: &cluster_controllerpb.ApplicationReleaseSpec{
			PublisherID: "core@globular.io",
			AppName:     "webadmin",
			Platform:    "linux_amd64",
		},
		Status: &cluster_controllerpb.ApplicationReleaseStatus{
			ResolvedVersion:        "2.0.0",
			ResolvedArtifactDigest: "hash",
		},
	}
	appPlan, err := CompileApplicationPlan("node-1", appRel, "1.0.0", "cluster-1")
	if err != nil {
		t.Fatalf("CompileApplicationPlan: %v", err)
	}
	if appPlan.GetPolicy().GetFailureMode() != planpb.FailureMode_FAILURE_MODE_ROLLBACK {
		t.Errorf("app plan: expected FAILURE_MODE_ROLLBACK, got %v", appPlan.GetPolicy().GetFailureMode())
	}
}

// Compile-time check: ensure strings import is used.
var _ = strings.TrimSpace
