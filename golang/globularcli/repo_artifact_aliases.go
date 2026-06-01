package main

// repo_artifact_aliases.go — Phase CLI-E aliases.
//
// The lifecycle commands historically live under `pkg`:
//   globular pkg quarantine | revoke | yank | deprecate
//
// Some operators reach for them under `repository artifact ...` instead.
// This file adds aliases that delegate to the same RunE as the canonical
// pkg commands. The originals stay; this is purely additive.

import (
	"github.com/spf13/cobra"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

var repoArtifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Repository artifact lifecycle aliases (mirrors `pkg quarantine|revoke|yank|deprecate`)",
}

var (
	repoArtQuarantineCmd = &cobra.Command{
		Use:   "quarantine <publisher/name> <version>",
		Short: "Quarantine an artifact (alias for `pkg quarantine`)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_QUARANTINED),
	}
	repoArtUnquarantineCmd = &cobra.Command{
		Use:   "unquarantine <publisher/name> <version>",
		Short: "Lift quarantine (alias for `pkg unquarantine`)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_PUBLISHED),
	}
	repoArtRevokeCmd = &cobra.Command{
		Use:   "revoke <publisher/name> <version>",
		Short: "Revoke an artifact (alias for `pkg revoke`)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_REVOKED),
	}
	repoArtYankCmd = &cobra.Command{
		Use:   "yank <publisher/name> <version>",
		Short: "Yank an artifact (alias for `pkg yank`)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_YANKED),
	}
	repoArtDeprecateCmd = &cobra.Command{
		Use:   "deprecate <publisher/name> <version>",
		Short: "Deprecate an artifact (alias for `pkg deprecate`)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_DEPRECATED),
	}
)

func init() {
	for _, c := range []*cobra.Command{
		repoArtQuarantineCmd, repoArtUnquarantineCmd,
		repoArtRevokeCmd, repoArtYankCmd, repoArtDeprecateCmd,
	} {
		// Reuse the same flag wiring the originals use. The flag-vars
		// (stateChangeReason, stateChangePlatform, etc.) are package-globals
		// in artifact_cmds.go, so re-binding them here doesn't conflict —
		// cobra resolves whichever subcommand the operator invokes.
		c.Flags().StringVar(&stateChangeReason, "reason", "", "Reason for state change (recorded for audit)")
		c.Flags().StringVar(&stateChangePlatform, "platform", stateChangePlatform, "Target platform (goos_goarch)")
		c.Flags().Int64Var(&stateChangeBuildNumber, "build-number", 0, "Target specific build iteration (0 = all builds at this version)")
		c.Flags().StringVar(&stateChangeKind, "kind", "service", "Artifact kind: service|application|infrastructure|command")
		repoArtifactCmd.AddCommand(c)
	}
	repoCmd.AddCommand(repoArtifactCmd)
}
