package main

// app_cmds.go — CLI commands for application lifecycle management.
//
// Commands:
//   globular app deploy <app> --namespace <ns> --version <v>
//   globular app undeploy <app> --namespace <ns>
//   globular app status <app> --namespace <ns>
//   globular app list

import (
	"context"
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/versionutil"
	"github.com/spf13/cobra"
)

// ── Flag variables ──────────────────────────────────────────────────────────

var (
	appNamespace  string
	appVersion    string
	appRoute      string
	appIndexFile  string
	appPlatform   string
)

// ── Commands ────────────────────────────────────────────────────────────────

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Application deployment commands",
	Long: `Manage web application deployments via the declarative desired-state model.

Examples:
  globular app deploy my-app --namespace dave@globular.io --version 1.2.0
  globular app undeploy my-app --namespace dave@globular.io
  globular app status my-app --namespace dave@globular.io
  globular app list`,
}

var appDeployCmd = &cobra.Command{
	Use:   "deploy <app-name>",
	Short: "Deploy an application to the cluster",
	Long: `Create or update an ApplicationRelease in the cluster's desired state.
The controller will reconcile it: resolve the artifact, compile a plan,
dispatch to nodes, and install the application files.

The application binary must already be published in the repository.`,
	Args: cobra.ExactArgs(1),
	RunE: runAppDeploy,
}

var appUndeployCmd = &cobra.Command{
	Use:   "undeploy <app-name>",
	Short: "Remove an application from the cluster",
	Args:  cobra.ExactArgs(1),
	RunE:  runAppUndeploy,
}

var appStatusCmd = &cobra.Command{
	Use:   "status <app-name>",
	Short: "Show deployment status of an application",
	Args:  cobra.ExactArgs(1),
	RunE:  runAppStatus,
}

var appListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all deployed applications",
	RunE:    runAppList,
}

func init() {
	appDeployCmd.Flags().StringVar(&appNamespace, "namespace", "", "Publisher namespace (required)")
	appDeployCmd.Flags().StringVar(&appVersion, "version", "", "Application version (required)")
	appDeployCmd.Flags().StringVar(&appRoute, "route", "", "URL route path (e.g. /apps/myapp)")
	appDeployCmd.Flags().StringVar(&appIndexFile, "index-file", "index.html", "Entry HTML file")
	appDeployCmd.Flags().StringVar(&appPlatform, "platform", "linux_amd64", "Target platform")
	_ = appDeployCmd.MarkFlagRequired("namespace")
	_ = appDeployCmd.MarkFlagRequired("version")

	appUndeployCmd.Flags().StringVar(&appNamespace, "namespace", "", "Publisher namespace (required)")
	_ = appUndeployCmd.MarkFlagRequired("namespace")

	appStatusCmd.Flags().StringVar(&appNamespace, "namespace", "", "Publisher namespace (required)")
	_ = appStatusCmd.MarkFlagRequired("namespace")

	appCmd.AddCommand(appDeployCmd)
	appCmd.AddCommand(appUndeployCmd)
	appCmd.AddCommand(appStatusCmd)
	appCmd.AddCommand(appListCmd)
}

// ── deploy ──────────────────────────────────────────────────────────────────

func runAppDeploy(cmd *cobra.Command, args []string) error {
	appName := strings.TrimSpace(args[0])
	if appName == "" {
		return fmt.Errorf("application name is required")
	}

	version := appVersion
	if cv, err := versionutil.Canonical(version); err == nil {
		version = cv
	}

	autoDiscoverController(cmd)

	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer conn.Close()

	rc := cluster_controllerpb.NewResourcesServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	releaseName := appNamespace + "/" + appName

	release := &cluster_controllerpb.ApplicationRelease{
		Meta: &cluster_controllerpb.ObjectMeta{
			Name: releaseName,
		},
		Spec: &cluster_controllerpb.ApplicationReleaseSpec{
			PublisherID: appNamespace,
			AppName:     appName,
			Version:     version,
			Platform:    appPlatform,
			Route:       appRoute,
			IndexFile:   appIndexFile,
		},
	}

	resp, err := rc.ApplyApplicationRelease(ctx, &cluster_controllerpb.ApplyApplicationReleaseRequest{
		Object: release,
	})
	if err != nil {
		return fmt.Errorf("deploy application: %w", err)
	}

	phase := "PENDING"
	if resp.Status != nil && resp.Status.Phase != "" {
		phase = resp.Status.Phase
	}

	fmt.Printf("Application %s/%s@%s deployed.\n", appNamespace, appName, version)
	fmt.Printf("  Release: %s\n", releaseName)
	fmt.Printf("  Phase:   %s\n", phase)
	if appRoute != "" {
		fmt.Printf("  Route:   %s\n", appRoute)
	}

	return nil
}

// ── undeploy ────────────────────────────────────────────────────────────────

func runAppUndeploy(cmd *cobra.Command, args []string) error {
	appName := strings.TrimSpace(args[0])
	releaseName := appNamespace + "/" + appName

	autoDiscoverController(cmd)

	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer conn.Close()

	rc := cluster_controllerpb.NewResourcesServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	_, err = rc.DeleteApplicationRelease(ctx, &cluster_controllerpb.DeleteApplicationReleaseRequest{
		Name: releaseName,
	})
	if err != nil {
		return fmt.Errorf("undeploy application: %w", err)
	}

	fmt.Printf("Application %s undeployed.\n", releaseName)
	return nil
}

// ── status ──────────────────────────────────────────────────────────────────

func runAppStatus(cmd *cobra.Command, args []string) error {
	appName := strings.TrimSpace(args[0])
	releaseName := appNamespace + "/" + appName

	autoDiscoverController(cmd)

	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer conn.Close()

	rc := cluster_controllerpb.NewResourcesServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	rel, err := rc.GetApplicationRelease(ctx, &cluster_controllerpb.GetApplicationReleaseRequest{
		Name: releaseName,
	})
	if err != nil {
		return fmt.Errorf("get application status: %w", err)
	}

	spec := rel.Spec
	status := rel.Status

	if spec != nil {
		fmt.Printf("Application: %s/%s\n", spec.PublisherID, spec.AppName)
		fmt.Printf("  Desired Version:  %s\n", spec.Version)
		if spec.Route != "" {
			fmt.Printf("  Route:            %s\n", spec.Route)
		}
	}

	if status != nil {
		fmt.Printf("  Phase:            %s\n", status.Phase)
		if status.ResolvedVersion != "" {
			fmt.Printf("  Resolved Version: %s\n", status.ResolvedVersion)
		}
		if status.Message != "" {
			fmt.Printf("  Message:          %s\n", status.Message)
		}
		if len(status.Nodes) > 0 {
			fmt.Println("  Nodes:")
			for _, n := range status.Nodes {
				fmt.Printf("    %-20s  %s  %s\n", n.NodeID, n.Phase, n.ErrorMessage)
			}
		}
	}

	return nil
}

// ── list ────────────────────────────────────────────────────────────────────

func runAppList(cmd *cobra.Command, args []string) error {
	autoDiscoverController(cmd)

	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller %s: %w", rootCfg.controllerAddr, err)
	}
	defer conn.Close()

	rc := cluster_controllerpb.NewResourcesServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	resp, err := rc.ListApplicationReleases(ctx, &cluster_controllerpb.ListApplicationReleasesRequest{})
	if err != nil {
		return fmt.Errorf("list applications: %w", err)
	}

	releases := resp.Items
	if len(releases) == 0 {
		fmt.Println("No applications deployed.")
		return nil
	}

	fmt.Printf("%-30s  %-10s  %-12s  %-10s  %s\n", "APPLICATION", "VERSION", "RESOLVED", "PHASE", "ROUTE")
	fmt.Printf("%-30s  %-10s  %-12s  %-10s  %s\n",
		strings.Repeat("─", 30), strings.Repeat("─", 10), strings.Repeat("─", 12), strings.Repeat("─", 10), strings.Repeat("─", 15))

	for _, rel := range releases {
		spec := rel.Spec
		status := rel.Status

		name := ""
		version := ""
		route := ""
		if spec != nil {
			name = fmt.Sprintf("%s/%s", spec.PublisherID, spec.AppName)
			version = spec.Version
			route = spec.Route
		}
		resolved := "—"
		phase := "PENDING"

		if status != nil {
			if status.ResolvedVersion != "" {
				resolved = status.ResolvedVersion
			}
			if status.Phase != "" {
				phase = status.Phase
			}
		}

		fmt.Printf("%-30s  %-10s  %-12s  %-10s  %s\n", name, version, resolved, phase, route)
	}

	return nil
}
