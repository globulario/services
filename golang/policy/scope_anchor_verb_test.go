package policy

import (
	"encoding/json"
	"testing"
)

// TestValidPermissionVerbsCoversAuthzgenVocabulary pins the validator's verb set
// to the vocabulary authzgen emits. If authzgen gains a new verb and this set is
// not updated, every service using that verb silently fails to load its policy
// (the v1.2.267 "execute" incident: node_agent's RunWorkflow → node_agent.workflow.execute
// was rejected, so the executor's policy never loaded and its workflow methods
// were denied during convergence, starving the workflow dispatch breaker). This
// list mirrors the authzgen permission-verb vocabulary; keep them in lockstep.
func TestValidPermissionVerbsCoversAuthzgenVocabulary(t *testing.T) {
	authzgenVerbs := []string{"read", "write", "delete", "admin", "execute"}
	for _, v := range authzgenVerbs {
		if !validPermissionVerbs[v] {
			t.Errorf("validPermissionVerbs is missing authzgen verb %q — services using it "+
				"will silently fail policy load (invariant rbac.enforced_service_requires_packaged_policy_vocabulary)", v)
		}
	}
}

// TestScopeAnchorResourceInheritsTopLevelVerb is the regression guard for the
// v1.2.267 incident: authzgen emits resource entries as pure scope anchors
// (field + kind, no per-resource permission verb — the verb lives at the entry's
// top level). The validator rejected these with `invalid permission verb ""`,
// so every service with resource-scoped RBAC (repository, node_agent, +~18)
// silently failed to load its policy, fell back to coarse action keys, and
// denied all role-based calls post-bootstrap. The fix: a resource inherits the
// entry's top-level verb, and validation checks the EFFECTIVE (inherited) verb.
func TestScopeAnchorResourceInheritsTopLevelVerb(t *testing.T) {
	// Mirrors repository's GetArtifactManifest entry: top-level verb "read",
	// resource is a scope anchor with no verb.
	src := `{
	  "schema_version": "2",
	  "service": "repository.PackageRepository",
	  "permissions": [
	    {
	      "method": "/repository.PackageRepository/GetArtifactManifest",
	      "action": "repository.artifact.read",
	      "permission": "read",
	      "resources": [{"field": "ref", "kind": "artifact", "scope_anchor": true}]
	    }
	  ]
	}`
	var pf PermissionsFile
	if err := json.Unmarshal([]byte(src), &pf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errs := validatePermissions(&pf); len(errs) > 0 {
		t.Fatalf("scope-anchor resource with an inheritable top-level verb must validate, got: %v", errs)
	}

	// The built resource must carry the inherited verb, never an empty one —
	// enforcement must always see a real verb.
	out := permissionsToInterface(pf.Permissions)
	entry := out[0].(map[string]interface{})
	res := entry["resources"].([]interface{})[0].(map[string]interface{})
	if got := res["permission"]; got != "read" {
		t.Fatalf("resource permission must inherit top-level verb; want \"read\", got %q", got)
	}
}

// TestVerblessEntryStillRejected ensures the loosening did not open a hole: an
// entry with NO top-level verb AND a verbless resource cannot determine a verb
// and must still be rejected.
func TestVerblessEntryStillRejected(t *testing.T) {
	src := `{
	  "schema_version": "2",
	  "service": "x.Y",
	  "permissions": [
	    {
	      "method": "/x.Y/Z",
	      "action": "x.y.z",
	      "resources": [{"field": "ref", "kind": "artifact"}]
	    }
	  ]
	}`
	var pf PermissionsFile
	if err := json.Unmarshal([]byte(src), &pf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errs := validatePermissions(&pf); len(errs) == 0 {
		t.Fatal("an entry with no top-level verb and a verbless resource must be rejected (no verb to inherit)")
	}
}
