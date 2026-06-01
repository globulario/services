// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=package_rollback_commands
// @awareness implements=globular.platform:intent.rollback.must_be_operator_chosen
// @awareness risk=high
package main

// pkg_rollback_cmds.go — Phase CLI-C rollback commands.
//
//   globular pkg rollback <name> --to-version <version> [--previous]
//   globular pkg rollback-candidates <name>
//
// These are operator frontends. The repository RPC ListRollbackCandidates
// supplies the candidate set + per-candidate eligibility; the actual
// rollback execution runs through the package.rollback workflow (the
// node-agent applies the target package, the controller orchestrates
// drain / install / health-probe).

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

var (
	rollbackPublisher string
	rollbackKind      string
	rollbackPlatform  string
	rollbackToVersion string
	rollbackPrevious  bool
	rollbackNodes     []string
	rollbackAllNodes  bool
	rollbackPreserve  bool
	rollbackRestoreSnap bool
	rollbackAllowDown bool
	rollbackDryRun    bool
	rollbackYes       bool
	rollbackJSON      bool
)

var pkgRollbackCmd = &cobra.Command{
	Use:   "rollback <name>",
	Short: "Roll back a package to a previous verified revision (Phase CLI-C)",
	Long: `Roll a package back to a previously installed revision.

Resolves the target via ListRollbackCandidates, verifies blob/signature/policy
gates pass, and triggers the package.rollback workflow. Refuses REVOKED targets
unconditionally; refuses QUARANTINED targets without explicit override.

NOTE: this Phase CLI-C iteration ships the candidate-listing surface and the
workflow definition. The node-agent rollback execution is wired in a follow-up
pass — running this command will currently print the resolution result and
exit without mutating cluster state.

Examples:
  globular pkg rollback echo --previous
  globular pkg rollback echo --to-version 1.0.82
  globular pkg rollback echo --previous --json --dry-run`,
	Args: cobra.ExactArgs(1),
	RunE: runPkgRollback,
}

var pkgRollbackCandidatesCmd = &cobra.Command{
	Use:   "rollback-candidates <name>",
	Short: "List previous installable revisions for a package",
	Args:  cobra.ExactArgs(1),
	RunE:  runPkgRollbackCandidates,
}

func init() {
	defaultPlatform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)

	for _, c := range []*cobra.Command{pkgRollbackCmd, pkgRollbackCandidatesCmd} {
		c.Flags().StringVar(&rollbackPublisher, "publisher", "core@globular.io", "Publisher namespace")
		c.Flags().StringVar(&rollbackKind, "kind", "service", "Artifact kind")
		c.Flags().StringVar(&rollbackPlatform, "platform", defaultPlatform, "Target platform")
		c.Flags().BoolVar(&rollbackJSON, "json", false, "Emit JSON output")
	}
	pkgRollbackCmd.Flags().StringVar(&rollbackToVersion, "to-version", "", "Target version (alternative to --previous)")
	pkgRollbackCmd.Flags().BoolVar(&rollbackPrevious, "previous", false, "Rollback to the immediately previous installed revision")
	pkgRollbackCmd.Flags().StringSliceVar(&rollbackNodes, "nodes", nil, "Limit rollback to these nodes")
	pkgRollbackCmd.Flags().BoolVar(&rollbackAllNodes, "all-nodes", false, "Rollback every node currently running this package")
	pkgRollbackCmd.Flags().BoolVar(&rollbackPreserve, "preserve-configs", true, "Preserve operator configs (default true)")
	pkgRollbackCmd.Flags().BoolVar(&rollbackRestoreSnap, "restore-config-snapshot", false, "Restore the configs snapshot from install time")
	pkgRollbackCmd.Flags().BoolVar(&rollbackAllowDown, "allow-downgrade", false, "Required when target version < current version")
	pkgRollbackCmd.Flags().BoolVar(&rollbackDryRun, "dry-run", false, "Preview only — no workflow run created")
	pkgRollbackCmd.Flags().BoolVar(&rollbackYes, "yes", false, "Skip confirmation prompt")

	pkgCmd.AddCommand(pkgRollbackCmd, pkgRollbackCandidatesCmd)
}

// ── Helpers ────────────────────────────────────────────────────────────────

func runPkgRollbackCandidates(cmd *cobra.Command, args []string) error {
	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()
	resp, err := client.ListRollbackCandidates(&repopb.ListRollbackCandidatesRequest{
		PublisherId: rollbackPublisher,
		Name:        args[0],
		Kind:        resolveArtifactKind(rollbackKind),
		Platform:    rollbackPlatform,
	})
	if err != nil {
		return err
	}
	if rollbackJSON {
		emitJSON(resp)
		return nil
	}
	if cur := resp.GetCurrentRef(); cur != nil {
		fmt.Printf("currently installed: %s/%s @ %s [%s]\n",
			cur.GetPublisherId(), cur.GetName(), cur.GetVersion(), cur.GetPlatform())
	}
	fmt.Printf("\n%-12s %-10s %-12s %-12s %s\n",
		"VERSION", "BUILD", "ELIGIBLE", "VERIFY", "REASON")
	for _, c := range resp.GetCandidates() {
		rev := c.GetRevision()
		eli := c.GetEligibility()
		fmt.Printf("%-12s %-10d %-12s %-12s %s\n",
			rev.GetVersion(), rev.GetBuildNumber(),
			yesNo(eli.GetEligible()),
			strings.TrimPrefix(eli.GetVerifyStatus().String(), "ARTIFACT_VERIFY_"),
			eli.GetReason(),
		)
	}
	return nil
}

func runPkgRollback(cmd *cobra.Command, args []string) error {
	if rollbackToVersion == "" && !rollbackPrevious {
		return fmt.Errorf("either --to-version <ver> or --previous is required")
	}
	if rollbackToVersion != "" && rollbackPrevious {
		return fmt.Errorf("cannot use both --to-version and --previous")
	}
	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()
	resp, err := client.ListRollbackCandidates(&repopb.ListRollbackCandidatesRequest{
		PublisherId: rollbackPublisher, Name: args[0],
		Kind: resolveArtifactKind(rollbackKind), Platform: rollbackPlatform,
	})
	if err != nil {
		return err
	}
	cands := resp.GetCandidates()
	if len(cands) == 0 {
		return fmt.Errorf("no rollback candidates found for %s/%s [%s]",
			rollbackPublisher, args[0], rollbackPlatform)
	}

	var picked *repopb.RollbackCandidate
	switch {
	case rollbackPrevious:
		picked = cands[0]
	case rollbackToVersion != "":
		for _, c := range cands {
			if c.GetRevision().GetVersion() == rollbackToVersion {
				picked = c
				break
			}
		}
	}
	if picked == nil {
		return fmt.Errorf("could not resolve target rollback revision (version=%q)", rollbackToVersion)
	}
	if !picked.GetEligibility().GetEligible() {
		return fmt.Errorf("target is not eligible for rollback: %s",
			picked.GetEligibility().GetReason())
	}

	if rollbackJSON {
		emitJSON(picked)
		fmt.Fprintln(cmd.OutOrStderr(), "// NOTE: --dry-run mode; node-agent rollback execution is Phase CLI-C-next")
		return nil
	}

	current := resp.GetCurrentRef()
	target := picked.GetTargetRef()
	fmt.Printf("rollback plan:\n")
	if current != nil {
		fmt.Printf("  current: %s/%s @ %s [%s]\n",
			current.GetPublisherId(), current.GetName(), current.GetVersion(), current.GetPlatform())
	}
	fmt.Printf("  target:  %s/%s @ %s build=%d [%s]\n",
		target.GetPublisherId(), target.GetName(), target.GetVersion(),
		picked.GetRevision().GetBuildNumber(), target.GetPlatform())
	fmt.Printf("  verify:  %s\n", strings.TrimPrefix(picked.GetEligibility().GetVerifyStatus().String(), "ARTIFACT_VERIFY_"))
	fmt.Printf("  signature: %s\n", strings.TrimPrefix(picked.GetEligibility().GetSignatureStatus().String(), "SIGNATURE_"))
	if rollbackDryRun {
		fmt.Printf("  --dry-run: no workflow run created\n")
		return nil
	}

	// Phase F: actually start the package.rollback workflow run.
	// The workflow definition orchestrates resolve_current → resolve_target →
	// verify_target_artifact → verify_target_signature → snapshot →
	// install_target_package → verify_runtime_health → record_installed_revision.
	// Node-agent step execution is wired in the next session — until then the
	// workflow run will block at the install_target_package step waiting on
	// the node-agent actor. The CLI returns the run id so operators can
	// inspect progress with `globular workflow get <run-id>`.
	addr := workflowEndpoint()
	cc, err := dialGRPC(addr)
	if err != nil {
		return fmt.Errorf("connect to workflow service: %w", err)
	}
	defer cc.Close()
	wfClient := workflowpb.NewWorkflowServiceClient(cc)

	correlation := fmt.Sprintf("Rollback/%s/%s/%s",
		rollbackPublisher, args[0], target.GetVersion())
	run := &workflowpb.WorkflowRun{
		CorrelationId: correlation,
		WorkflowName:  "package.rollback",
		TriggerReason: workflowpb.TriggerReason_TRIGGER_REASON_MANUAL,
		Context: &workflowpb.WorkflowContext{
			ComponentName:    args[0],
			ComponentVersion: target.GetVersion(),
			ReleaseKind:      "PackageRollback",
			ReleaseObjectId:  fmt.Sprintf("%s/%s/%s", rollbackPublisher, args[0], target.GetVersion()),
		},
	}
	startedRun, startErr := wfClient.StartRun(ctxWithTimeout(),
		&workflowpb.StartRunRequest{Run: run})
	if startErr != nil {
		return fmt.Errorf("start package.rollback run: %w", startErr)
	}
	fmt.Printf("  workflow_run_id: %s\n", startedRun.GetId())
	fmt.Printf("  revision_id:     %s\n", picked.GetRevision().GetRevisionId())
	fmt.Printf("\nFollow progress: globular workflow get %s\n", startedRun.GetId())
	return nil
}
