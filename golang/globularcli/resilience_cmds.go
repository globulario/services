// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=resilience_testing_commands
// @awareness risk=medium
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	scyllaCmd = &cobra.Command{
		Use:   "scylla",
		Short: "ScyllaDB safety operations",
	}
	scyllaSchemaCmd = &cobra.Command{
		Use:   "schema",
		Short: "Scylla schema guard operations",
	}
	scyllaSchemaStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show schema guard status for critical keyspaces",
		RunE:  runScyllaSchemaStatus,
	}
	scyllaSchemaEnforceCmd = &cobra.Command{
		Use:   "enforce",
		Short: "Request immediate schema guard enforcement",
		RunE:  runScyllaSchemaEnforce,
	}

	ingressCmd = &cobra.Command{
		Use:   "ingress",
		Short: "Ingress/VIP operations",
	}
	ingressStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show ingress desired state and node statuses",
		RunE:  runIngressStatus,
	}
	ingressRepublishCmd = &cobra.Command{
		Use:   "republish",
		Short: "Request cluster-controller to republish canonical ingress spec",
		RunE:  runIngressRepublish,
	}
)

const (
	scyllaSchemaGuardPrefix       = "/globular/scylla/schema_guard/"
	scyllaSchemaGuardEnforceKey   = "/globular/scylla/schema_guard/enforce_request"
	ingressSpecKeyCLI             = "/globular/ingress/v1/spec"
	ingressStatusPrefixCLI        = "/globular/ingress/v1/status/"
	ingressRepublishRequestKeyCLI = "/globular/ingress/v1/republish_request"
)

func init() {
	scyllaSchemaCmd.AddCommand(scyllaSchemaStatusCmd, scyllaSchemaEnforceCmd)
	scyllaCmd.AddCommand(scyllaSchemaCmd)

	ingressCmd.AddCommand(ingressStatusCmd, ingressRepublishCmd)
}

func runScyllaSchemaStatus(cmd *cobra.Command, args []string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx, scyllaSchemaGuardPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("read schema guard status: %w", err)
	}
	type row struct {
		keyspace string
		current  any
		required any
		phase    string
		lastErr  string
	}
	rows := make([]row, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		keyspace := strings.TrimPrefix(string(kv.Key), scyllaSchemaGuardPrefix)
		if keyspace == "" || strings.Contains(keyspace, "/") {
			continue
		}
		var st map[string]any
		if err := json.Unmarshal(kv.Value, &st); err != nil {
			continue
		}
		phase := "ok"
		if v, _ := st["violation"].(bool); v {
			phase = "violation"
		}
		rows = append(rows, row{
			keyspace: keyspace,
			current:  st["current_rf"],
			required: st["required_rf"],
			phase:    phase,
			lastErr:  fmt.Sprint(st["last_error"]),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].keyspace < rows[j].keyspace })
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "KEYSPACE\tCURRENT_RF\tREQUIRED_RF\tSTATUS\tLAST_ERROR")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%v\t%v\t%s\t%s\n", r.keyspace, r.current, r.required, r.phase, r.lastErr)
	}
	_ = w.Flush()
	return nil
}

func runScyllaSchemaEnforce(cmd *cobra.Command, args []string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}
	reqTS := time.Now().Unix()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if _, err := cli.Put(ctx, scyllaSchemaGuardEnforceKey, fmt.Sprintf("%d", reqTS)); err != nil {
		return fmt.Errorf("write enforce request: %w", err)
	}
	fmt.Println("Requested schema guard enforcement. Waiting for status update...")
	deadline := time.Now().Add(70 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := cli.Get(pctx, scyllaSchemaGuardPrefix, clientv3.WithPrefix())
		pcancel()
		if err != nil {
			continue
		}
		for _, kv := range resp.Kvs {
			var st map[string]any
			if json.Unmarshal(kv.Value, &st) != nil {
				continue
			}
			if updated, ok := st["updated_at_unix"].(float64); ok && int64(updated) >= reqTS {
				fmt.Println("Schema guard run observed.")
				return nil
			}
		}
	}
	return fmt.Errorf("timed out waiting for schema guard status update")
}

func runIngressStatus(cmd *cobra.Command, args []string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	specResp, err := cli.Get(ctx, ingressSpecKeyCLI)
	if err != nil {
		return fmt.Errorf("read ingress spec: %w", err)
	}
	if len(specResp.Kvs) == 0 {
		fmt.Println("Ingress spec: MISSING")
	} else {
		var spec map[string]any
		_ = json.Unmarshal(specResp.Kvs[0].Value, &spec)
		fmt.Printf("Ingress spec generation=%v mode=%v explicit_disabled=%v writer=%v updated=%v\n",
			spec["generation"], spec["mode"], spec["explicit_disabled"], spec["writer_leader_id"], spec["written_at_unix"])
	}

	statusResp, err := cli.Get(ctx, ingressStatusPrefixCLI, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("read ingress node statuses: %w", err)
	}
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NODE_ID\tPHASE\tVRRP\tHAS_VIP\tLAST_ERROR")
	for _, kv := range statusResp.Kvs {
		nodeID := strings.TrimPrefix(string(kv.Key), ingressStatusPrefixCLI)
		if nodeID == "" {
			continue
		}
		var st map[string]any
		if json.Unmarshal(kv.Value, &st) != nil {
			continue
		}
		fmt.Fprintf(w, "%s\t%v\t%v\t%v\t%v\n", nodeID, st["phase"], st["vrrp_state"], st["has_vip"], st["last_error"])
	}
	_ = w.Flush()
	return nil
}

func runIngressRepublish(cmd *cobra.Command, args []string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}
	reqTS := time.Now().Unix()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if _, err := cli.Put(ctx, ingressRepublishRequestKeyCLI, fmt.Sprintf("%d", reqTS)); err != nil {
		return fmt.Errorf("write republish request: %w", err)
	}
	fmt.Println("Requested ingress republish. Waiting for spec update...")
	deadline := time.Now().Add(70 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := cli.Get(pctx, ingressSpecKeyCLI)
		pcancel()
		if err != nil || len(resp.Kvs) == 0 {
			continue
		}
		var spec map[string]any
		if json.Unmarshal(resp.Kvs[0].Value, &spec) != nil {
			continue
		}
		if updated, ok := spec["written_at_unix"].(float64); ok && int64(updated) >= reqTS {
			fmt.Println("Ingress spec republished.")
			return nil
		}
	}
	return fmt.Errorf("timed out waiting for ingress spec republish")
}
