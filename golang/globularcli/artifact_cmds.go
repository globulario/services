package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

var (
	artifactDeprecateCmd = &cobra.Command{
		Use:   "deprecate <publisher/name> <version>",
		Short: "Mark an artifact as deprecated",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_DEPRECATED),
	}
	artifactYankCmd = &cobra.Command{
		Use:   "yank <publisher/name> <version>",
		Short: "Yank an artifact (remove from discovery, block downloads)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_YANKED),
	}
	artifactQuarantineCmd = &cobra.Command{
		Use:   "quarantine <publisher/name> <version>",
		Short: "Quarantine an artifact (admin only — security hold)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_QUARANTINED),
	}
	artifactRevokeCmd = &cobra.Command{
		Use:   "revoke <publisher/name> <version>",
		Short: "Permanently revoke an artifact (terminal, admin only)",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_REVOKED),
	}
	artifactUndeprecateCmd = &cobra.Command{
		Use:   "undeprecate <publisher/name> <version>",
		Short: "Remove deprecation from an artifact",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_PUBLISHED),
	}
	artifactUnyankCmd = &cobra.Command{
		Use:   "unyank <publisher/name> <version>",
		Short: "Restore a yanked artifact to published state",
		Args:  cobra.ExactArgs(2),
		RunE:  runArtifactStateChange(repopb.PublishState_PUBLISHED),
	}
)

var stateChangeReason string

func init() {
	for _, cmd := range []*cobra.Command{
		artifactDeprecateCmd, artifactYankCmd, artifactQuarantineCmd,
		artifactRevokeCmd, artifactUndeprecateCmd, artifactUnyankCmd,
	} {
		cmd.Flags().StringVar(&stateChangeReason, "reason", "", "Reason for state change (for audit)")
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

func runArtifactStateChange(targetState repopb.PublishState) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		publisher, name, err := parsePublisherName(args[0])
		if err != nil {
			return err
		}
		version := args[1]

		address, _ := config.GetAddress()
		if address == "" {
			address = "localhost"
		}

		client, err := repository_client.NewRepositoryService_Client(address, "repository.PackageRepository")
		if err != nil {
			return fmt.Errorf("connect to repository: %w", err)
		}
		defer client.Close()

		ref := &repopb.ArtifactRef{
			PublisherId: publisher,
			Name:        name,
			Version:     version,
			Platform:    "linux_amd64",
			Kind:        repopb.ArtifactKind_SERVICE,
		}

		resp, err := client.SetArtifactState(ref, 0, targetState, stateChangeReason)
		if err != nil {
			return fmt.Errorf("set artifact state: %w", err)
		}

		fmt.Printf("Artifact %s/%s@%s: %s → %s\n",
			publisher, name, version,
			resp.GetPreviousState(), resp.GetCurrentState())
		return nil
	}
}
