package main

import (
	"context"
	"fmt"
	"os"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	"github.com/spf13/cobra"
)

var (
	healEnforce bool
	healPackage string
)

var doctorHealCmd = &cobra.Command{
	Use:   "heal",
	Short: "Run the auto-heal policy against current cluster findings",
	Long: `Evaluates all doctor invariant findings against the auto-heal policy
and either reports intended actions (default dry-run) or executes safe
auto-heal actions (--enforce).

Modes:
  (default)   dry-run — classify findings, show intended actions, no mutations
  --enforce   execute auto-heal actions for HEAL_AUTO findings

Only HEAL_AUTO actions are ever executed. HEAL_PROPOSE actions are shown
as recommendations. HEAL_OBSERVE findings are reported without action.

Exit codes:
  0  all auto-heal actions succeeded (or dry-run completed)
  1  at least one auto-heal action failed
  2  RPC or connection error

Examples:
  globular doctor heal                  # dry-run
  globular doctor heal --enforce        # execute safe auto-heal
  globular doctor heal --package event  # filter to one package
`,
	RunE: runDoctorHeal,
}

func runDoctorHeal(cmd *cobra.Command, args []string) error {
	// Resolve doctor endpoint directly from etcd (no mesh rewrite).
	// Doctor is a control-plane service that should be called directly,
	// not through the Envoy mesh (which may not have a route for it).
	// Prefer the local instance; fall back to any reachable instance from etcd.
	// Port comes from etcd — never hardcoded. (CLAUDE.md rule 1 & 4)
	doctorAddr := config.ResolveLocalServiceAddr("cluster_doctor.ClusterDoctorService")
	if doctorAddr == "" {
		doctorAddr = config.ResolveServiceAddr("cluster_doctor.ClusterDoctorService", "")
	}
	if doctorAddr == "" {
		return fmt.Errorf("cluster-doctor not found in etcd service registry — is it running?")
	}

	cc, err := dialGRPC(doctorAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial doctor %s: %v\n", doctorAddr, err)
		os.Exit(2)
	}
	defer cc.Close()

	client := cluster_doctorpb.NewClusterDoctorServiceClient(cc)

	healMode := cluster_doctorpb.HealMode_HEAL_MODE_DRY_RUN
	modeLabel := "DRY-RUN"
	if healEnforce {
		healMode = cluster_doctorpb.HealMode_HEAL_MODE_ENFORCE
		modeLabel = "ENFORCE"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	resp, err := client.GetClusterReport(ctx, &cluster_doctorpb.ClusterReportRequest{
		Freshness: cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH,
		HealMode:  healMode,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetClusterReport: %v\n", err)
		os.Exit(2)
	}

	// Filter findings
	findings := resp.GetFindings()
	if healPackage != "" {
		var filtered []*cluster_doctorpb.Finding
		for _, f := range findings {
			if contains(f.GetEntityRef(), healPackage) {
				filtered = append(filtered, f)
			}
		}
		findings = filtered
	}

	// Print summary
	fmt.Printf("Doctor Heal — mode: %s\n", modeLabel)
	fmt.Printf("Findings:  %d total", len(resp.GetFindings()))
	if healPackage != "" {
		fmt.Printf(" (%d matching --package %s)", len(findings), healPackage)
	}
	fmt.Println()

	// Count by disposition
	auto, propose, observe, executed, failed := 0, 0, 0, 0, 0
	for _, f := range findings {
		hd := f.GetHealDecision()
		if hd == nil {
			continue
		}
		switch hd.GetDisposition() {
		case cluster_doctorpb.HealDisposition_HEAL_AUTO:
			auto++
			if hd.GetExecuted() {
				executed++
			}
			if hd.GetError() != "" {
				failed++
			}
		case cluster_doctorpb.HealDisposition_HEAL_PROPOSE:
			propose++
		case cluster_doctorpb.HealDisposition_HEAL_OBSERVE:
			observe++
		}
	}
	fmt.Printf("Policy:    auto=%d  propose=%d  observe=%d\n", auto, propose, observe)
	if healEnforce {
		fmt.Printf("Executed:  %d  failed=%d\n", executed, failed)
	}
	fmt.Println()

	// Print findings table
	for _, f := range findings {
		hd := f.GetHealDecision()
		if hd == nil {
			continue
		}
		disp := dispLabel(hd.GetDisposition())
		status := ""
		switch {
		case hd.GetExecuted() && hd.GetVerified():
			status = " [executed+verified]"
		case hd.GetExecuted():
			status = " [executed]"
		case healEnforce && hd.GetDisposition() == cluster_doctorpb.HealDisposition_HEAL_AUTO:
			status = " [would execute]"
		}
		errStr := ""
		if hd.GetError() != "" {
			errStr = fmt.Sprintf(" ERROR: %s", hd.GetError())
		}

		fmt.Printf("  [%s] %s %s%s%s\n", disp, f.GetInvariantId(), truncate(f.GetEntityRef(), 40), status, errStr)
		if hd.GetAction() != "" && (hd.GetDisposition() != cluster_doctorpb.HealDisposition_HEAL_OBSERVE) {
			fmt.Printf("         → %s\n", truncate(hd.GetAction(), 80))
		}
	}

	if failed > 0 {
		os.Exit(1)
	}
	return nil
}

func dispLabel(d cluster_doctorpb.HealDisposition) string {
	switch d {
	case cluster_doctorpb.HealDisposition_HEAL_AUTO:
		return "AUTO"
	case cluster_doctorpb.HealDisposition_HEAL_PROPOSE:
		return "PROPOSE"
	case cluster_doctorpb.HealDisposition_HEAL_OBSERVE:
		return "OBSERVE"
	}
	return "?"
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) > 0 && searchString(s, sub))
}

func searchString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}


func init() {
	doctorHealCmd.Flags().BoolVar(&healEnforce, "enforce", false, "Execute auto-heal actions (default: dry-run)")
	doctorHealCmd.Flags().StringVar(&healPackage, "package", "", "Filter findings to a specific package")
	doctorCmd.AddCommand(doctorHealCmd)
}
