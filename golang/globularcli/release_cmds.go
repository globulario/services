package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

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

	releaseFile string
	releaseDry  bool
)

// seam for tests
var resourcesClientFactory = func(conn *grpc.ClientConn) clustercontrollerpb.ResourcesServiceClient {
	return clustercontrollerpb.NewResourcesServiceClient(conn)
}

func init() {
	releaseApplyCmd.Flags().StringVarP(&releaseFile, "file", "f", "", "Path to ServiceRelease YAML/JSON (required)")
	releaseApplyCmd.Flags().BoolVar(&releaseDry, "dry-run", false, "Validate only; do not send to controller")
	releaseCmd.AddCommand(releaseApplyCmd)
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

// canonicalServiceName mirrors controller normalization (lowercase, drop prefixes/suffixes).
func canonicalServiceName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimPrefix(n, "globular-")
	n = strings.TrimSuffix(n, ".service")
	return n
}
