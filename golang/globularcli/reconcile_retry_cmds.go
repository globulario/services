package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

const (
	annotationReconcileResume            = "globular.io/reconcile-resume"
	annotationReconcileResumeNode        = "globular.io/reconcile-node"
	annotationReconcileResumeAt          = "globular.io/reconcile-resume-at"
	annotationReconcileDependencyPresent = "globular.io/dependency-present"
)

var (
	pkgResumeNode string

	reconcileCmd                   = &cobra.Command{Use: "reconcile", Short: "Reconcile controls and unblock signals"}
	reconcileRetryCmd              = &cobra.Command{Use: "retry", Short: "Trigger retry for a blocked package release", RunE: runReconcileRetry}
	reconcileRetryPackage          string
	reconcileRetryNode             string
	reconcileRetryDependencySignal bool
)

func init() {
	pkgResumeCmd := &cobra.Command{
		Use:   "resume <package-or-release>",
		Short: "Resume a blocked package release (sets operator unblock signal)",
		Args:  cobra.ExactArgs(1),
		RunE:  runPkgResume,
	}
	pkgResumeCmd.Flags().StringVar(&pkgResumeNode, "node", "", "Optional node hint for operator resume")
	pkgCmd.AddCommand(pkgResumeCmd)

	reconcileRetryCmd.Flags().StringVar(&reconcileRetryPackage, "package", "", "Package/service name or release name (required)")
	reconcileRetryCmd.Flags().StringVar(&reconcileRetryNode, "node", "", "Optional node hint")
	reconcileRetryCmd.Flags().BoolVar(&reconcileRetryDependencySignal, "dependency-present", false, "Also assert dependency-present unblock signal")
	_ = reconcileRetryCmd.MarkFlagRequired("package")
	reconcileCmd.AddCommand(reconcileRetryCmd)
	rootCmd.AddCommand(reconcileCmd)
}

func runPkgResume(cmd *cobra.Command, args []string) error {
	return applyReleaseUnblockSignals(args[0], pkgResumeNode, false)
}

func runReconcileRetry(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(reconcileRetryPackage) == "" {
		return errors.New("--package is required")
	}
	return applyReleaseUnblockSignals(reconcileRetryPackage, reconcileRetryNode, reconcileRetryDependencySignal)
}

func applyReleaseUnblockSignals(packageOrRelease, node string, dependencyPresent bool) error {
	conn, err := controllerConnFactory()
	if err != nil {
		return err
	}
	if c, ok := conn.(*grpc.ClientConn); ok && c != nil {
		defer c.Close()
	}
	client := resourcesClientFactory(conn)
	ctx, cancel := ctxWithCLITimeout(context.Background())
	defer cancel()

	rel, err := fetchReleaseByPackageOrName(ctx, packageOrRelease, client)
	if err != nil {
		return err
	}
	if rel.Meta == nil {
		rel.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	if rel.Meta.Annotations == nil {
		rel.Meta.Annotations = make(map[string]string)
	}
	rel.Meta.Annotations[annotationReconcileResume] = "true"
	rel.Meta.Annotations[annotationReconcileResumeAt] = fmt.Sprintf("%d", time.Now().UnixMilli())
	if strings.TrimSpace(node) != "" {
		rel.Meta.Annotations[annotationReconcileResumeNode] = strings.TrimSpace(node)
	}
	if dependencyPresent {
		rel.Meta.Annotations[annotationReconcileDependencyPresent] = "true"
	}

	updated, err := applyRelease(ctx, rel, client)
	if err != nil {
		return err
	}
	fmt.Printf("resume signal applied: release=%s service=%s gen=%d\n",
		updated.Meta.Name, updated.Spec.ServiceName, updated.Meta.Generation)
	if strings.TrimSpace(node) != "" {
		fmt.Printf("node hint: %s\n", strings.TrimSpace(node))
	}
	if dependencyPresent {
		fmt.Println("dependency-present signal: true")
	}
	return nil
}

func fetchReleaseByPackageOrName(ctx context.Context, packageOrRelease string, client releaseResourcesClient) (*cluster_controllerpb.ServiceRelease, error) {
	token := strings.TrimSpace(packageOrRelease)
	if token == "" {
		return nil, errors.New("package/release is required")
	}
	if rel, err := fetchRelease(ctx, token, client); err == nil {
		return rel, nil
	}
	list, err := client.ListServiceReleases(ctx, &cluster_controllerpb.ListServiceReleasesRequest{}, jsonCallOption())
	if err != nil {
		return nil, err
	}
	canonical := canonicalServiceName(token)
	var matches []*cluster_controllerpb.ServiceRelease
	for _, rel := range list.Items {
		if rel == nil || rel.Meta == nil || rel.Spec == nil {
			continue
		}
		if rel.Meta.Name == token ||
			canonicalServiceName(rel.Spec.ServiceName) == canonical ||
			strings.HasSuffix(rel.Meta.Name, "/"+token) ||
			strings.HasSuffix(rel.Meta.Name, "/"+canonical) {
			matches = append(matches, rel)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("release/package %q not found", token)
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.Meta.Name)
		}
		return nil, fmt.Errorf("package %q is ambiguous; use release name: %s", token, strings.Join(names, ", "))
	}
	return matches[0], nil
}

