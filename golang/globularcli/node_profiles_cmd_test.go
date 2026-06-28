package main

import (
	"strings"
	"testing"
)

// TestNodeProfilesCommandTree guards against the cobra "two command names
// wearing one trench coat" regression: previously both the set and preview
// commands used multi-word Use strings ("profiles set <node_id>" / "profiles
// preview <node_id>"), so cobra registered BOTH under the single command name
// "profiles". That made `profiles preview` unreachable (it routed to set) and
// made `profiles set <id>` parse the literal "set" as a positional arg.
//
// The fix introduces a real command tree:
//
//	nodes
//	  profiles            (parent, no RunE)
//	    set <node_id>
//	    preview <node_id>
//
// These assertions prove `... profiles set` and `... profiles preview` resolve
// to DIFFERENT handlers, with the node_id left as the single remaining arg.
func TestNodeProfilesCommandTree(t *testing.T) {
	// Parent must be the bare "profiles" — no embedded subcommand word, or the
	// collision returns.
	if got := nodeProfilesCmd.Name(); got != "profiles" {
		t.Fatalf("profiles parent command name = %q, want %q", got, "profiles")
	}
	if nodeProfilesCmd.RunE != nil || nodeProfilesCmd.Run != nil {
		t.Fatalf("profiles parent must not have a Run/RunE of its own")
	}

	setCmd, setArgs, err := nodeProfilesCmd.Find([]string{"set", "node-1"})
	if err != nil {
		t.Fatalf("Find(set node-1): %v", err)
	}
	previewCmd, previewArgs, err := nodeProfilesCmd.Find([]string{"preview", "node-1"})
	if err != nil {
		t.Fatalf("Find(preview node-1): %v", err)
	}

	// Distinct handlers — the core of the regression.
	if setCmd == previewCmd {
		t.Fatalf("set and preview resolved to the same command %q", setCmd.Name())
	}
	if setCmd != nodeProfilesSetCmd {
		t.Fatalf("`profiles set` routed to %q, want the set handler", setCmd.Name())
	}
	if previewCmd != nodeProfilesPreviewCmd {
		t.Fatalf("`profiles preview` routed to %q, want the preview handler", previewCmd.Name())
	}

	// The subcommand word is consumed; node_id is the lone positional arg.
	if len(setArgs) != 1 || setArgs[0] != "node-1" {
		t.Fatalf("set residual args = %v, want [node-1]", setArgs)
	}
	if len(previewArgs) != 1 || previewArgs[0] != "node-1" {
		t.Fatalf("preview residual args = %v, want [node-1]", previewArgs)
	}

	// Each leaf accepts exactly one positional (node_id) and validates it.
	if err := setCmd.Args(setCmd, setArgs); err != nil {
		t.Fatalf("set rejects its node_id arg: %v", err)
	}
	if err := setCmd.Args(setCmd, []string{"set", "node-1"}); err == nil {
		t.Fatalf("set should reject 2 positional args (the old bug let this through)")
	}

	// Both leaves expose the --profile flag.
	if nodeProfilesSetCmd.Flags().Lookup("profile") == nil {
		t.Fatalf("set is missing the --profile flag")
	}
	if nodeProfilesPreviewCmd.Flags().Lookup("profile") == nil {
		t.Fatalf("preview is missing the --profile flag")
	}

	// No leaf Use string may carry an embedded subcommand word (the root cause).
	for _, c := range []*struct {
		name string
		use  string
	}{
		{"set", nodeProfilesSetCmd.Use},
		{"preview", nodeProfilesPreviewCmd.Use},
	} {
		if first := strings.Fields(c.use)[0]; first != c.name {
			t.Fatalf("%s leaf Use=%q starts with %q, want %q", c.name, c.use, first, c.name)
		}
	}

	// Parent is reachable from the nodes command and carries exactly the two leaves.
	pc, _, err := nodesCmd.Find([]string{"profiles", "preview", "node-1"})
	if err != nil {
		t.Fatalf("nodes Find(profiles preview): %v", err)
	}
	if pc != nodeProfilesPreviewCmd {
		t.Fatalf("nodes->profiles->preview routed to %q", pc.Name())
	}
}
