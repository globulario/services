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

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
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

// runIngressStatus consumes cluster_controller.GetIngressStatus. The
// prior implementation scanned /globular/ingress/v1/spec +
// /globular/ingress/v1/status/* directly from etcd — that prefix is
// owned by the cluster_controller's ingress spec guard, so a CLI
// reading raw etcd violated
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
func runIngressStatus(cmd *cobra.Command, args []string) error {
	cc, err := controllerClient()
	if err != nil {
		return fmt.Errorf("dial cluster_controller: %w", err)
	}
	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
	resp, err := client.GetIngressStatus(ctx, &cluster_controllerpb.GetIngressStatusRequest{})
	if err != nil {
		return fmt.Errorf("GetIngressStatus: %w", err)
	}

	if !resp.GetSpecPresent() {
		fmt.Println("Ingress spec: MISSING")
	} else {
		fmt.Printf("Ingress spec generation=%d mode=%s explicit_disabled=%v writer=%s updated=%d\n",
			resp.GetGeneration(),
			resp.GetMode(),
			resp.GetExplicitDisabled(),
			resp.GetWriterLeaderId(),
			resp.GetWrittenAtUnix())
	}

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NODE_ID\tPHASE\tVRRP\tHAS_VIP\tLAST_ERROR")
	for _, n := range resp.GetNodes() {
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%s\n",
			n.GetNodeId(), n.GetPhase(), n.GetVrrpState(), n.GetHasVip(), n.GetLastError())
	}
	_ = w.Flush()
	return nil
}

// runIngressRepublish consumes cluster_controller.RequestIngressRepublish
// and polls GetIngressStatus to confirm the republish landed.
// Replaces the prior direct etcd writes/reads of
// /globular/ingress/v1/{republish_request,spec}.
func runIngressRepublish(cmd *cobra.Command, args []string) error {
	cc, err := controllerClient()
	if err != nil {
		return fmt.Errorf("dial cluster_controller: %w", err)
	}
	defer cc.Close()
	client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 8*time.Second)
	reqResp, err := client.RequestIngressRepublish(reqCtx, &cluster_controllerpb.RequestIngressRepublishRequest{})
	reqCancel()
	if err != nil {
		return fmt.Errorf("RequestIngressRepublish: %w", err)
	}
	reqTS := reqResp.GetRequestUnix()

	fmt.Println("Requested ingress republish. Waiting for spec update...")
	deadline := time.Now().Add(70 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		pctx, pcancel := context.WithTimeout(context.Background(), 5*time.Second)
		status, sErr := client.GetIngressStatus(pctx, &cluster_controllerpb.GetIngressStatusRequest{})
		pcancel()
		if sErr != nil || !status.GetSpecPresent() {
			continue
		}
		if status.GetWrittenAtUnix() >= reqTS {
			fmt.Println("Ingress spec republished.")
			return nil
		}
	}
	return fmt.Errorf("timed out waiting for ingress spec republish")
}
