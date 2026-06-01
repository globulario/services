package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

var (
	artifactDeprecateCmd = &cobra.Command{
		Use:   "deprecate <publisher/name> <version>",
		Short: "Mark an artifact as deprecated (still installable by pin, skipped by latest resolver)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_DEPRECATED),
	}
	artifactUndeprecateCmd = &cobra.Command{
		Use:   "undeprecate <publisher/name> <version>",
		Short: "Restore a deprecated artifact to published state",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_PUBLISHED),
	}
	artifactYankCmd = &cobra.Command{
		Use:   "yank <publisher/name> <version>",
		Short: "Yank an artifact (hidden from discovery, downloads blocked)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_YANKED),
	}
	artifactUnyankCmd = &cobra.Command{
		Use:   "unyank <publisher/name> <version>",
		Short: "Restore a yanked artifact to published state",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_PUBLISHED),
	}
	artifactQuarantineCmd = &cobra.Command{
		Use:   "quarantine <publisher/name> <version>",
		Short: "Quarantine an artifact (admin only — security hold, downloads blocked)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_QUARANTINED),
	}
	artifactUnquarantineCmd = &cobra.Command{
		Use:   "unquarantine <publisher/name> <version>",
		Short: "Lift quarantine and restore artifact to published state (admin only)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_PUBLISHED),
	}
	artifactRevokeCmd = &cobra.Command{
		Use:   "revoke <publisher/name> <version>",
		Short: "Permanently revoke an artifact (terminal — no recovery, admin or owner)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_REVOKED),
	}
)

var (
	stateChangeReason      string
	stateChangePlatform    string
	stateChangeBuildNumber int64
	stateChangeKind        string
)

func init() {
	for _, cmd := range []*cobra.Command{
		artifactDeprecateCmd, artifactUndeprecateCmd,
		artifactYankCmd, artifactUnyankCmd,
		artifactQuarantineCmd, artifactUnquarantineCmd,
		artifactRevokeCmd,
	} {
		cmd.Flags().StringVar(&stateChangeReason, "reason", "", "Reason for state change (recorded for audit)")
		cmd.Flags().StringVar(&stateChangePlatform, "platform", fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH), "Target platform (goos_goarch)")
		cmd.Flags().Int64Var(&stateChangeBuildNumber, "build-number", 0, "Target specific build iteration (0 = all builds at this version)")
		cmd.Flags().StringVar(&stateChangeKind, "kind", "service", "Artifact kind: service|application|infrastructure|command")
		pkgCmd.AddCommand(cmd)
	}
}

// parsePublisherName splits "publisher/name" into (publisher, name).
func parsePublisherName(arg string) (string, string, error) {
	parts := strings.SplitN(arg, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected format publisher/name, got %q", arg)
	}
	return parts[0], parts[1], nil
}

func resolveArtifactKind(kind string) repopb.ArtifactKind {
	switch strings.ToLower(kind) {
	case "application":
		return repopb.ArtifactKind_APPLICATION
	case "infrastructure":
		return repopb.ArtifactKind_INFRASTRUCTURE
	case "command":
		return repopb.ArtifactKind_COMMAND
	default:
		return repopb.ArtifactKind_SERVICE
	}
}

func runArtifactStateChange(targetState repopb.PublishState) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		publisher, name, err := parsePublisherName(args[0])
		if err != nil {
			return err
		}
		version := args[1]

		repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
		if repoAddr == "" {
			return fmt.Errorf("cannot discover repository address")
		}

		client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
		if err != nil {
			return fmt.Errorf("connect to repository: %w", err)
		}
		defer client.Close()

		if token := rootCfg.token; token != "" {
			client.SetToken(token)
		}

		ref := &repopb.ArtifactRef{
			PublisherId: publisher,
			Name:        name,
			Version:     version,
			Platform:    stateChangePlatform,
			Kind:        resolveArtifactKind(stateChangeKind),
		}

		resp, err := client.SetArtifactState(ref, stateChangeBuildNumber, targetState, stateChangeReason)
		if err != nil {
			return fmt.Errorf("set artifact state: %w", err)
		}

		fmt.Printf("Artifact %s/%s@%s [%s]: %s → %s\n",
			publisher, name, version, stateChangePlatform,
			resp.GetPreviousState(), resp.GetCurrentState())
		return nil
	}
}
