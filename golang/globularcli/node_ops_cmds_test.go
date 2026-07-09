package main

import "testing"

// TestBuildUninstallPackageWorkflow_NodeScoped locks SCAR-5
// (placement.orphan_removal_needs_a_lawful_node_scoped_path): the node-scoped
// operator uninstall entry must invoke the node-agent's existing "uninstall-package"
// workflow with only the package identity — never a cluster-desired mutation.
func TestBuildUninstallPackageWorkflow_NodeScoped(t *testing.T) {
	req := buildUninstallPackageWorkflow("torrent", "")
	if req.GetWorkflowName() != "uninstall-package" {
		t.Fatalf("workflow_name = %q, want %q", req.GetWorkflowName(), "uninstall-package")
	}
	if got := req.GetInputs()["package_name"]; got != "torrent" {
		t.Fatalf("package_name input = %q, want torrent", got)
	}
	if got := req.GetInputs()["kind"]; got != "SERVICE" {
		t.Fatalf("kind default = %q, want SERVICE", got)
	}

	// kind is upper-cased.
	req2 := buildUninstallPackageWorkflow("yt-dlp", "command")
	if got := req2.GetInputs()["kind"]; got != "COMMAND" {
		t.Fatalf("kind = %q, want COMMAND (upper-cased)", got)
	}

	// Node-scoped guard: the request carries EXACTLY {package_name, kind} — nothing
	// that could reach or mutate cluster desired-state (no service_id, no desired_*).
	if n := len(req2.GetInputs()); n != 2 {
		t.Fatalf("inputs must be exactly {package_name, kind}, got %d: %v", n, req2.GetInputs())
	}
}
