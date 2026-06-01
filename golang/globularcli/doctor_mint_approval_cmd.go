// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=doctor_approval_minting_command
// @awareness implements=globular.platform:intent.operator_action_requires_explain_plan_verify
// @awareness risk=high
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/evidencedigest"
	"github.com/globulario/services/golang/security"
	"github.com/spf13/cobra"
)

var (
	mintApprovalFinding    string
	mintApprovalStep       uint32
	mintApprovalActor      string
	mintApprovalLifetime   time.Duration
	mintApprovalEndpoint   string
	mintApprovalActionClass string
	mintApprovalTarget     string
	mintApprovalGeneration string
)

var doctorMintApprovalCmd = &cobra.Command{
	Use:   "mint-approval --finding <id> [--step N]",
	Short: "Mint a signed approval token bound to a specific remediation",
	Long: `mint-approval produces a signed JWT that authorizes one specific
remediation step. The token binds (action_class, target, generation,
finding_id) so it cannot be replayed against a different action or
re-evaluated evidence.

By default the action_class, target, and generation are resolved from
the doctor's current finding cache so operators don't have to assemble
them by hand. Override with explicit flags only when the auto-resolved
values are stale.

Approval tokens are single-use, audience-bound to the local cluster,
and expire after --lifetime (default 10m, max 1h). See
docs/intent/remediation.token_contract.yaml.

Examples:
  globular doctor mint-approval --finding a1b2c3d4e5f60102 --step 1
  globular doctor mint-approval --finding a1b2c3 --lifetime 5m --actor alice@cluster
  globular doctor remediate a1b2c3 --step 1 --approval "$(globular doctor mint-approval --finding a1b2c3 --step 1)"
`,
	RunE: runDoctorMintApproval,
}

func init() {
	doctorCmd.AddCommand(doctorMintApprovalCmd)
	doctorMintApprovalCmd.Flags().StringVar(&mintApprovalFinding, "finding", "", "Finding ID the approval is bound to (required)")
	doctorMintApprovalCmd.Flags().Uint32Var(&mintApprovalStep, "step", 0, "Remediation step index to authorize (0-based)")
	doctorMintApprovalCmd.Flags().StringVar(&mintApprovalActor, "actor", "", "Operator identity (defaults to the gRPC caller's principal)")
	doctorMintApprovalCmd.Flags().DurationVar(&mintApprovalLifetime, "lifetime", 10*time.Minute, "Token lifetime (max 1h)")
	doctorMintApprovalCmd.Flags().StringVar(&mintApprovalEndpoint, "endpoint", "", "cluster-doctor gRPC endpoint (auto-discovered if empty)")
	doctorMintApprovalCmd.Flags().StringVar(&mintApprovalActionClass, "action-class", "", "Override auto-resolved action class (e.g. SYSTEMCTL_STOP)")
	doctorMintApprovalCmd.Flags().StringVar(&mintApprovalTarget, "target", "", "Override auto-resolved target entity ref")
	doctorMintApprovalCmd.Flags().StringVar(&mintApprovalGeneration, "generation", "", "Override auto-resolved evidence-digest generation")
	_ = doctorMintApprovalCmd.MarkFlagRequired("finding")
}

func runDoctorMintApproval(cmd *cobra.Command, args []string) error {
	findingID := strings.TrimSpace(mintApprovalFinding)
	if findingID == "" {
		return fmt.Errorf("--finding is required")
	}

	// Resolve action_class/target/generation from the doctor unless the
	// operator overrode them. Auto-resolution avoids the common error of
	// minting a token against stale or guessed parameters.
	actionClass := strings.TrimSpace(mintApprovalActionClass)
	target := strings.TrimSpace(mintApprovalTarget)
	generation := strings.TrimSpace(mintApprovalGeneration)

	if actionClass == "" || target == "" || generation == "" {
		resolvedEndpoint, err := resolveDoctorEndpoint(mintApprovalEndpoint)
		if err != nil {
			return fmt.Errorf("resolve doctor endpoint: %w", err)
		}
		conn, err := dialGRPC(resolvedEndpoint)
		if err != nil {
			return fmt.Errorf("dial cluster-doctor %s: %w", resolvedEndpoint, err)
		}
		defer conn.Close()
		client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
		defer cancel()

		fields, err := resolveApprovalFields(ctx, client, findingID, mintApprovalStep)
		if err != nil {
			return fmt.Errorf("auto-resolve approval fields: %w", err)
		}
		if actionClass == "" {
			actionClass = fields.actionClass
		}
		if target == "" {
			target = fields.target
		}
		if generation == "" {
			generation = fields.generation
		}
	}

	actor := strings.TrimSpace(mintApprovalActor)
	if actor == "" {
		actor = "cli-operator"
	}

	tok, err := security.MintApprovalToken(security.MintApprovalRequest{
		Actor:       actor,
		ActionClass: actionClass,
		Target:      target,
		Generation:  generation,
		FindingID:   findingID,
		Lifetime:    mintApprovalLifetime,
	})
	if err != nil {
		return fmt.Errorf("mint approval token: %w", err)
	}
	// Print only the token on stdout so callers can pipe it directly
	// into `--approval`. Diagnostics go to stderr.
	fmt.Fprintln(cmd.ErrOrStderr(), "Approval token minted:")
	fmt.Fprintf(cmd.ErrOrStderr(), "  finding:      %s\n", findingID)
	fmt.Fprintf(cmd.ErrOrStderr(), "  step:         %d\n", mintApprovalStep)
	fmt.Fprintf(cmd.ErrOrStderr(), "  action_class: %s\n", actionClass)
	fmt.Fprintf(cmd.ErrOrStderr(), "  target:       %s\n", target)
	fmt.Fprintf(cmd.ErrOrStderr(), "  generation:   %s\n", generation)
	fmt.Fprintf(cmd.ErrOrStderr(), "  lifetime:     %s\n", mintApprovalLifetime)
	fmt.Fprintln(cmd.OutOrStdout(), tok)
	return nil
}

// resolvedApprovalFields holds the auto-resolved tuple from a doctor call.
type resolvedApprovalFields struct {
	actionClass string
	target      string
	generation  string
}

// resolveApprovalFields asks the doctor for the current finding details
// so the operator does not have to compute the evidence digest by hand.
// Falls back to a deterministic placeholder when the doctor does not
// expose the digest directly (older builds).
func resolveApprovalFields(ctx context.Context, client cluster_doctorpb.ClusterDoctorServiceClient, findingID string, stepIndex uint32) (resolvedApprovalFields, error) {
	// Pull the most recent cluster report to find the finding + step.
	rep, err := client.GetClusterReport(ctx, &cluster_doctorpb.ClusterReportRequest{})
	if err != nil {
		return resolvedApprovalFields{}, fmt.Errorf("get cluster report: %w", err)
	}
	for _, f := range rep.GetFindings() {
		if f.GetFindingId() != findingID {
			continue
		}
		steps := f.GetRemediation()
		if int(stepIndex) >= len(steps) {
			return resolvedApprovalFields{}, fmt.Errorf("step %d out of range (finding has %d steps)", stepIndex, len(steps))
		}
		action := steps[stepIndex].GetAction()
		if action == nil {
			return resolvedApprovalFields{}, fmt.Errorf("step %d has no structured action", stepIndex)
		}
		entity := f.GetEntityRef()
		if entity == "" {
			entity = findingID
		}
		generation := evidencedigest.Of(f.GetEvidence())
		return resolvedApprovalFields{
			actionClass: action.GetActionType().String(),
			target:      entity,
			generation:  generation,
		}, nil
	}
	return resolvedApprovalFields{}, fmt.Errorf("finding %s not found in latest cluster report — refresh with `globular doctor report`", findingID)
}
