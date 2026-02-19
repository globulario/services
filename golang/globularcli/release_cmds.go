package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"sigs.k8s.io/yaml"
)

var (
	releaseCmd = &cobra.Command{
		Use:   "release",
		Short: "Manage service releases",
	}

	releaseApplyCmd = &cobra.Command{
		Use:   "apply",
		Short: "Apply a ServiceRelease from file",
		RunE:  runReleaseApply,
	}

	releaseListCmd = &cobra.Command{
		Use:   "list",
		Short: "List service releases",
		RunE:  runReleaseList,
	}

	releaseShowCmd = &cobra.Command{
		Use:   "show <name>",
		Short: "Show a service release",
		Args:  cobra.ExactArgs(1),
		RunE:  runReleaseShow,
	}

	releaseStatusCmd = &cobra.Command{
		Use:   "status <name>",
		Short: "Show service release status",
		Args:  cobra.ExactArgs(1),
		RunE:  runReleaseStatus,
	}

	releaseFile   string
	releaseDry    bool
	releaseOutput string
)

// seam for tests
type releaseResourcesClient interface {
	ApplyServiceRelease(ctx context.Context, req *clustercontrollerpb.ApplyServiceReleaseRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error)
	GetServiceRelease(ctx context.Context, req *clustercontrollerpb.GetServiceReleaseRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ServiceRelease, error)
	ListServiceReleases(ctx context.Context, req *clustercontrollerpb.ListServiceReleasesRequest, opts ...grpc.CallOption) (*clustercontrollerpb.ListServiceReleasesResponse, error)
}

var resourcesClientFactory = func(conn *grpc.ClientConn) releaseResourcesClient {
	return clustercontrollerpb.NewResourcesServiceClient(conn)
}

func init() {
	releaseApplyCmd.Flags().StringVarP(&releaseFile, "file", "f", "", "Path to ServiceRelease YAML/JSON (required)")
	releaseApplyCmd.Flags().BoolVar(&releaseDry, "dry-run", false, "Validate only; do not send to controller")
	releaseShowCmd.Flags().StringVarP(&releaseOutput, "output", "o", "json", "Output format (json|yaml)")
	releaseCmd.AddCommand(releaseApplyCmd, releaseListCmd, releaseShowCmd, releaseStatusCmd)
	rootCmd.AddCommand(releaseCmd)
}

func runReleaseApply(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(releaseFile) == "" {
		return errors.New("--file is required")
	}
	data, err := os.ReadFile(releaseFile)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	rel, err := parseServiceRelease(data)
	if err != nil {
		return err
	}
	if releaseDry {
		fmt.Printf("validated ServiceRelease %s/%s (service=%s) [dry-run]\n",
			rel.Meta.Name, rel.Spec.PublisherID, rel.Spec.ServiceName)
		return nil
	}

	conn, err := controllerClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	client := resourcesClientFactory(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	resp, err := client.ApplyServiceRelease(ctx, &clustercontrollerpb.ApplyServiceReleaseRequest{Object: rel})
	if err != nil {
		return err
	}
	fmt.Printf("applied release name=%s publisher=%s service=%s gen=%d phase=%s\n",
		resp.Meta.Name,
		resp.Spec.PublisherID,
		resp.Spec.ServiceName,
		resp.Meta.Generation,
		resp.Status.Phase)
	return nil
}

func runReleaseList(cmd *cobra.Command, args []string) error {
	conn, err := controllerClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	client := resourcesClientFactory(conn)
	ctx, cancel := ctxWithCLITimeout(context.Background())
	defer cancel()
	resp, err := client.ListServiceReleases(ctx, &clustercontrollerpb.ListServiceReleasesRequest{})
	if err != nil {
		return err
	}
	rows := [][]string{{"NAME", "SERVICE", "PHASE", "RESOLVED_VERSION", "AGE"}}
	for _, rel := range resp.Items {
		if rel == nil || rel.Spec == nil || rel.Meta == nil || rel.Status == nil {
			continue
		}
		rows = append(rows, []string{
			rel.Meta.Name,
			fmt.Sprintf("%s/%s", rel.Spec.PublisherID, rel.Spec.ServiceName),
			rel.Status.Phase,
			rel.Status.ResolvedVersion,
			"-",
		})
	}
	printTable(rows)
	return nil
}

func runReleaseShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	conn, err := controllerClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	client := resourcesClientFactory(conn)
	ctx, cancel := ctxWithCLITimeout(context.Background())
	defer cancel()
	rel, err := client.GetServiceRelease(ctx, &clustercontrollerpb.GetServiceReleaseRequest{Name: name})
	if err != nil {
		return err
	}
	switch strings.ToLower(releaseOutput) {
	case "json":
		return printJSON(rel)
	case "yaml":
		return printYAML(rel)
	default:
		return fmt.Errorf("unsupported output: %s", releaseOutput)
	}
}

func runReleaseStatus(cmd *cobra.Command, args []string) error {
	name := args[0]
	conn, err := controllerClient()
	if err != nil {
		return err
	}
	defer conn.Close()
	client := resourcesClientFactory(conn)
	ctx, cancel := ctxWithCLITimeout(context.Background())
	defer cancel()
	rel, err := client.GetServiceRelease(ctx, &clustercontrollerpb.GetServiceReleaseRequest{Name: name})
	if err != nil {
		return err
	}
	if rel.Status == nil {
		fmt.Printf("release %s has no status\n", name)
		return nil
	}
	st := rel.Status
	fmt.Printf("Phase:            %s\n", st.Phase)
	fmt.Printf("Resolved Version: %s\n", st.ResolvedVersion)
	if st.DesiredHash != "" {
		fmt.Printf("Desired Hash:     %s\n", st.DesiredHash)
	}
	if len(st.Nodes) > 0 {
		healthy := 0
		for _, n := range st.Nodes {
			if n.Phase == clustercontrollerpb.ReleasePhaseAvailable {
				healthy++
			}
		}
		fmt.Printf("Nodes:            %d total, %d healthy\n", len(st.Nodes), healthy)
		fmt.Printf("\n  %-12s %-12s %-12s %s\n", "NODE", "VERSION", "PHASE", "ERROR")
		for _, n := range st.Nodes {
			fmt.Printf("  %-12s %-12s %-12s %s\n", n.NodeID, n.InstalledVersion, n.Phase, n.ErrorMessage)
		}
	}
	return nil
}

func parseServiceRelease(raw []byte) (*clustercontrollerpb.ServiceRelease, error) {
	if len(raw) == 0 {
		return nil, errors.New("empty file")
	}
	jsonBytes, err := yaml.YAMLToJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	rel := &clustercontrollerpb.ServiceRelease{}
	if err := json.Unmarshal(jsonBytes, rel); err != nil {
		return nil, fmt.Errorf("decode ServiceRelease: %w", err)
	}
	if rel.Spec == nil {
		return nil, errors.New("spec is required")
	}
	if strings.TrimSpace(rel.Spec.PublisherID) == "" {
		return nil, errors.New("spec.publisher_id is required")
	}
	if strings.TrimSpace(rel.Spec.ServiceName) == "" {
		return nil, errors.New("spec.service_name is required")
	}
	// Minimal version policy validation: require either version or channel.
	if strings.TrimSpace(rel.Spec.Version) == "" && strings.TrimSpace(rel.Spec.Channel) == "" {
		return nil, errors.New("spec.version or spec.channel is required")
	}
	// Default name to service name if not provided.
	if rel.Meta == nil {
		rel.Meta = &clustercontrollerpb.ObjectMeta{}
	}
	if strings.TrimSpace(rel.Meta.Name) == "" {
		rel.Meta.Name = canonicalServiceName(rel.Spec.ServiceName)
	}
	// Strip status on apply.
	rel.Status = nil
	return rel, nil
}

func printTable(rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r[0], r[1], r[2], r[3], r[4])
	}
	w.Flush()
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func printYAML(v interface{}) error {
	j, err := json.Marshal(v)
	if err != nil {
		return err
	}
	y, err := yaml.JSONToYAML(j)
	if err != nil {
		return err
	}
	fmt.Println(string(y))
	return nil
}

// canonicalServiceName mirrors controller normalization (lowercase, drop prefixes/suffixes).
func canonicalServiceName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimPrefix(n, "globular-")
	n = strings.TrimSuffix(n, ".service")
	return n
}
