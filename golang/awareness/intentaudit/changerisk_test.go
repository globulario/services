package intentaudit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify_RBACFile(t *testing.T) {
	dir := t.TempDir()
	writeClassifier(t, dir)
	rc, err := LoadClassifier(filepath.Join(dir, "classifier.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	result := rc.Classify("rbac/rbac_server/rbac_access.go")
	if !containsStr(result.RiskCategories, "security_sensitive") {
		t.Errorf("expected security_sensitive, got %v", result.RiskCategories)
	}
	if !containsStr(result.IntentsToAudit, "security.deny_overrides_allow") {
		t.Errorf("expected security.deny_overrides_allow in intents, got %v", result.IntentsToAudit)
	}
}

func TestClassify_ReconcilerFile(t *testing.T) {
	dir := t.TempDir()
	writeClassifier(t, dir)
	rc, err := LoadClassifier(filepath.Join(dir, "classifier.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	result := rc.Classify("cluster_controller/cluster_controller_server/reconciler.go")
	if !containsStr(result.RiskCategories, "reconciliation_sensitive") {
		t.Errorf("expected reconciliation_sensitive, got %v", result.RiskCategories)
	}
	if !containsStr(result.IntentsToAudit, "reconciliation.must_be_idempotent_and_bounded") {
		t.Errorf("expected reconciliation intent, got %v", result.IntentsToAudit)
	}
}

func TestClassify_TLSFile(t *testing.T) {
	dir := t.TempDir()
	writeClassifier(t, dir)
	rc, err := LoadClassifier(filepath.Join(dir, "classifier.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	result := rc.Classify("globular_client/clients.go")
	if !containsStr(result.RiskCategories, "tls_pki_sensitive") {
		t.Errorf("expected tls_pki_sensitive, got %v", result.RiskCategories)
	}
	if !containsStr(result.IntentsToAudit, "dns_pki.explicit_identity_over_convenient_routing") {
		t.Errorf("expected dns_pki intent, got %v", result.IntentsToAudit)
	}
}

func TestClassify_UnrelatedFile(t *testing.T) {
	dir := t.TempDir()
	writeClassifier(t, dir)
	rc, err := LoadClassifier(filepath.Join(dir, "classifier.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	result := rc.Classify("echo/echo_server/server.go")
	if len(result.RiskCategories) != 0 {
		t.Errorf("expected no risk categories for unrelated file, got %v", result.RiskCategories)
	}
}

func TestMergedPreflight(t *testing.T) {
	dir := t.TempDir()
	writeClassifier(t, dir)
	rc, err := LoadClassifier(filepath.Join(dir, "classifier.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	nodes := map[string]*Node{
		"security.deny_overrides_allow": {
			ID:            "security.deny_overrides_allow",
			RequiredTests: []string{"rbac:TestDenyOverridesSAAllow"},
		},
	}

	merged := rc.MergedPreflight([]string{
		"rbac/rbac_server/rbac_access.go",
		"security/tls.go",
	}, nodes)

	if !containsStr(merged.RiskCategories, "security_sensitive") {
		t.Errorf("missing security_sensitive in merged result")
	}
	if !containsStr(merged.RequiredTests, "TestDenyOverridesSAAllow") {
		t.Errorf("missing required test in merged result, got %v", merged.RequiredTests)
	}
}

func writeClassifier(t *testing.T, dir string) {
	t.Helper()
	content := `categories:
  security_sensitive:
    description: Security changes
    file_patterns:
      - rbac/
      - security/
    symbol_patterns: []
    intent_nodes:
      - security.deny_overrides_allow
  reconciliation_sensitive:
    description: Reconciliation changes
    file_patterns:
      - cluster_controller/cluster_controller_server/reconcil
    symbol_patterns: []
    intent_nodes:
      - reconciliation.must_be_idempotent_and_bounded
      - desired.build_id_immutable_after_resolution
  tls_pki_sensitive:
    description: TLS changes
    file_patterns:
      - globular_client/clients
      - security/tls
    symbol_patterns: []
    intent_nodes:
      - dns_pki.explicit_identity_over_convenient_routing
`
	os.WriteFile(filepath.Join(dir, "classifier.yaml"), []byte(content), 0644)
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
