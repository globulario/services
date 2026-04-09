package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/globulario/services/golang/deploy"
	"github.com/spf13/cobra"
)

var (
	deployComment    string
	deployVersion    string
	deployRepoAddr   string
	deployFull       bool
	deployDryRun     bool
	deployAll        bool
	deployParallel   int
)

var deployCmd = &cobra.Command{
	Use:   "deploy [service...]",
	Short: "Build, package, and publish services to the repository",
	Long: `Deploy builds Go binaries, detects what changed, and publishes to the
repository. The controller detects the new artifact and rolls it out.

Single service:
  globular deploy echo
  globular deploy dns --comment "fix trailing dot handling"

Multiple services:
  globular deploy echo dns rbac

All services:
  globular deploy --all

Binary-only deploys are automatic when only the binary changed. Use --full
to force a full package rebuild.`,
	Args: cobra.ArbitraryArgs,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringVarP(&deployComment, "comment", "c", "", "Deployment comment")
	deployCmd.Flags().StringVar(&deployVersion, "version", "0.0.2", "Package version")
	deployCmd.Flags().StringVar(&deployRepoAddr, "repository", "", "Repository gRPC endpoint (auto-discovered if empty)")
	deployCmd.Flags().BoolVar(&deployFull, "full", false, "Force full package rebuild (skip delta detection)")
	deployCmd.Flags().BoolVar(&deployDryRun, "dry-run", false, "Print actions without executing")
	deployCmd.Flags().BoolVar(&deployAll, "all", false, "Deploy all services from the catalog")
	deployCmd.Flags().IntVar(&deployParallel, "parallel", 4, "Max parallel deploys (with --all)")

	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	if !deployAll && len(args) == 0 {
		return fmt.Errorf("specify service name(s) or use --all")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	baseOpts := deploy.DeployOptions{
		Version:   deployVersion,
		Publisher: "core@globular.io",
		Platform:  "linux_amd64",
		Comment:   deployComment,
		Full:      deployFull,
		DryRun:    deployDryRun,
		RepoAddr:  deployRepoAddr,
		Token:     rootCfg.token,
	}

	if deployAll {
		results, err := deploy.DeployAll(ctx, baseOpts, deployParallel)
		printSummaryTable(results)
		if err != nil {
			return err
		}
		return nil
	}

	// Deploy specified services.
	var results []*deploy.DeployResult
	var errs []string
	for _, name := range args {
		name = strings.TrimSuffix(name, "_server")
		opts := baseOpts
		opts.ServiceName = name
		result, err := deploy.DeployService(ctx, opts)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		results = append(results, result)
	}

	if len(args) > 1 {
		printSummaryTable(results)
	}

	if len(errs) > 0 {
		return fmt.Errorf("deploy errors:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

func printSummaryTable(results []*deploy.DeployResult) {
	if len(results) == 0 {
		return
	}
	fmt.Println("\n━━━ Deploy Summary ━━━")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVICE\tVERSION\tBUILD\tACTION\tDURATION")
	for _, r := range results {
		fmt.Fprintf(w, "%s\tv%s+%d\t%d\t%s\t%s\n",
			r.Service, r.Version, r.BuildNumber, r.BuildNumber, r.Action,
			r.Duration.Round(time.Millisecond))
	}
	w.Flush()
	fmt.Println()
}
