package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/yaml"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
)

var (
	clusterCmd = &cobra.Command{
		Use:   "cluster",
		Short: "Control plane helpers",
	}

	bootstrapNodeAddr string
	bootstrapDomain   string
	bootstrapBind     string
	bootstrapProfiles []string

	joinNodeOverride       string
	joinControllerOverride string
	joinToken              string

	tokenExpires string

	reqProfiles  []string
	reqMetadata  []string
	rejectReason string

	profileSet []string

	agentNodeAddr  string
	planFile       string
	watchPlanFlag  bool
	watchAgentFlag bool
	agentOpID      string

	planNodeAddr string

	watchNodeID string
	watchOpID   string
)

func init() {
	rootCmd.AddCommand(clusterCmd)
	clusterCmd.AddCommand(
		bootstrapCmd,
		joinCmd,
		tokenCmd,
		requestsCmd,
		nodesCmd,
		agentCmd,
		planCmd,
		watchCmd,
	)

	bootstrapCmd.Flags().StringVar(&bootstrapNodeAddr, "node", "", "Node agent endpoint (required)")
	bootstrapCmd.Flags().StringVar(&bootstrapDomain, "domain", "", "Cluster domain (required)")
	bootstrapCmd.Flags().StringVar(&bootstrapBind, "bind", "0.0.0.0:10000", "Controller bind address")
	bootstrapCmd.Flags().StringSliceVar(&bootstrapProfiles, "profile", nil, "Profiles for the first node")

	joinCmd.Flags().StringVar(&joinNodeOverride, "node", "", "Node agent endpoint")
	joinCmd.Flags().StringVar(&joinControllerOverride, "controller", "", "Controller endpoint")
	joinCmd.Flags().StringVar(&joinToken, "join-token", "", "Join token")

	tokenCreateCmd.Flags().StringVar(&tokenExpires, "expires", "24h", "Token expiration (duration or RFC3339)")

	requestsApproveCmd.Flags().StringSliceVar(&reqProfiles, "profile", nil, "Profiles to assign")
	requestsApproveCmd.Flags().StringSliceVar(&reqMetadata, "meta", nil, "Metadata entries (k=v)")
	requestsRejectCmd.Flags().StringVar(&rejectReason, "reason", "", "Rejection reason")

	nodeProfilesCmd.Flags().StringSliceVar(&profileSet, "profile", nil, "Profiles to assign (required)")

	agentInventoryCmd.Flags().StringVar(&agentNodeAddr, "node", "", "Node agent endpoint")
	agentApplyCmd.Flags().StringVar(&planFile, "plan-file", "", "Path to NodePlan JSON or YAML")
	agentApplyCmd.Flags().BoolVar(&watchAgentFlag, "watch", false, "Watch operation on completion")
	agentWatchCmd.Flags().StringVar(&agentOpID, "op", "", "Operation ID to watch")

	planApplyCmd.Flags().BoolVar(&watchPlanFlag, "watch", false, "Watch operation on completion")
	planApplyCmd.Flags().StringVar(&planNodeAddr, "node", "", "Node agent endpoint (required)")

	watchCmd.Flags().StringVar(&watchNodeID, "node-id", "", "Filter by node ID")
	watchCmd.Flags().StringVar(&watchOpID, "op", "", "Filter by operation ID")
}

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap the first control-plane node",
	RunE: func(cmd *cobra.Command, args []string) error {
		if bootstrapDomain == "" {
			return errors.New("--domain is required")
		}
		nodeAddr := pick(bootstrapNodeAddr, rootCfg.nodeAddr)
		cc, err := dialGRPC(nodeAddr)
		if err != nil {
			return err
		}
		defer cc.Close()
		client := nodeagentpb.NewNodeAgentServiceClient(cc)
		req := &nodeagentpb.BootstrapFirstNodeRequest{
			ClusterDomain:  bootstrapDomain,
			ControllerBind: bootstrapBind,
			Profiles:       bootstrapProfiles,
		}
		resp, err := client.BootstrapFirstNode(ctxWithTimeout(), req)
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var joinCmd = &cobra.Command{
	Use:   "join",
	Short: "Have a node request to join an existing controller",
	RunE: func(cmd *cobra.Command, args []string) error {
		if joinToken == "" {
			return errors.New("--join-token is required")
		}
		nodeAddr := pick(joinNodeOverride, rootCfg.nodeAddr)
		controllerAddr := pick(joinControllerOverride, rootCfg.controllerAddr)
		cc, err := dialGRPC(nodeAddr)
		if err != nil {
			return err
		}
		defer cc.Close()
		client := nodeagentpb.NewNodeAgentServiceClient(cc)
		resp, err := client.JoinCluster(ctxWithTimeout(), &nodeagentpb.JoinClusterRequest{
			ControllerEndpoint: controllerAddr,
			JoinToken:          joinToken,
		})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage controller join tokens",
}

var tokenCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a join token",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		req := &clustercontrollerpb.CreateJoinTokenRequest{}
		if tokenExpires != "" {
			ts, err := parseExpiration(tokenExpires)
			if err != nil {
				return err
			}
			req.ExpiresAt = ts
		}
		resp, err := client.CreateJoinToken(ctxWithTimeout(), req)
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var requestsCmd = &cobra.Command{
	Use:   "requests",
	Short: "Manage pending join requests",
}

var requestsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending join requests",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ListJoinRequests(ctxWithTimeout(), &clustercontrollerpb.ListJoinRequestsRequest{})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var requestsApproveCmd = &cobra.Command{
	Use:   "approve <node_id>",
	Short: "Approve a pending node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ApproveJoin(ctxWithTimeout(), &clustercontrollerpb.ApproveJoinRequest{
			NodeId:   args[0],
			Profiles: reqProfiles,
			Metadata: parseMetadata(reqMetadata),
		})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var requestsRejectCmd = &cobra.Command{
	Use:   "reject <node_id>",
	Short: "Reject a pending node",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.RejectJoin(ctxWithTimeout(), &clustercontrollerpb.RejectJoinRequest{
			NodeId: args[0],
			Reason: rejectReason,
		})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var nodesCmd = &cobra.Command{
	Use:   "nodes",
	Short: "Inspect cluster nodes",
}

var nodesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all known nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ListNodes(ctxWithTimeout(), &clustercontrollerpb.ListNodesRequest{})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var nodeProfilesCmd = &cobra.Command{
	Use:   "profiles set <node_id>",
	Short: "Replace a node's profiles",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(profileSet) == 0 {
			return errors.New("--profile is required")
		}
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		_, err = client.SetNodeProfiles(ctxWithTimeout(), &clustercontrollerpb.SetNodeProfilesRequest{
			NodeId:   args[0],
			Profiles: profileSet,
		})
		if err != nil {
			return err
		}
		fmt.Println("profiles intent recorded")
		return nil
	},
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Invoke node agents directly",
}

var agentInventoryCmd = &cobra.Command{
	Use:   "inventory",
	Short: "Fetch node inventory from the agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := nodeClientWith(agentNodeAddr)
		if err != nil {
			return err
		}
		defer cc.Close()
		client := nodeagentpb.NewNodeAgentServiceClient(cc)
		resp, err := client.GetInventory(ctxWithTimeout(), &nodeagentpb.GetInventoryRequest{})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

var agentApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a plan directly to a node agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if planFile == "" {
			return errors.New("--plan-file is required")
		}
		plan, err := loadPlan(planFile)
		if err != nil {
			return err
		}
		cc, err := nodeClientWith(agentNodeAddr)
		if err != nil {
			return err
		}
		defer cc.Close()
		client := nodeagentpb.NewNodeAgentServiceClient(cc)
		resp, err := client.ApplyPlan(ctxWithTimeout(), &nodeagentpb.ApplyPlanRequest{Plan: plan})
		if err != nil {
			return err
		}
		fmt.Printf("operation_id: %s\n", resp.OperationId)
		if watchAgentFlag {
			return watchAgentOperation(resp.OperationId, agentNodeAddr)
		}
		return nil
	},
}

var agentWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch an operation on a node agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if agentOpID == "" {
			return errors.New("--op is required")
		}
		return watchAgentOperation(agentOpID, agentNodeAddr)
	},
}

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Work with node plans",
}

var planGetCmd = &cobra.Command{
	Use:   "get <node_id>",
	Short: "Get the effective node plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.GetNodePlan(ctxWithTimeout(), &clustercontrollerpb.GetNodePlanRequest{NodeId: args[0]})
		if err != nil {
			return err
		}
		printProto(resp.Plan)
		return nil
	},
}

var planApplyCmd = &cobra.Command{
	Use:   "apply <node_id>",
	Short: "Request controller apply a node plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.GetNodePlan(ctxWithTimeout(), &clustercontrollerpb.GetNodePlanRequest{NodeId: args[0]})
		if err != nil {
			return err
		}
		plan := resp.GetPlan()
		if plan == nil {
			return errors.New("controller returned empty plan")
		}
		nodeAddr := pick(planNodeAddr, rootCfg.nodeAddr)
		if nodeAddr == "" {
			return errors.New("--node is required")
		}
		nc, err := nodeClientWith(nodeAddr)
		if err != nil {
			return err
		}
		defer nc.Close()
		nodeClient := nodeagentpb.NewNodeAgentServiceClient(nc)
		applyResp, err := nodeClient.ApplyPlan(ctxWithTimeout(), &nodeagentpb.ApplyPlanRequest{Plan: plan})
		if err != nil {
			return err
		}
		fmt.Printf("operation_id: %s\n", applyResp.GetOperationId())
		if watchPlanFlag {
			return watchAgentOperation(applyResp.GetOperationId(), nodeAddr)
		}
		return nil
	},
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch controller operations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if watchNodeID == "" && watchOpID == "" {
			return errors.New("--node-id or --op is required")
		}
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		req := &clustercontrollerpb.WatchOperationsRequest{
			NodeId:      watchNodeID,
			OperationId: watchOpID,
		}
		return watchControllerStream(cc, req)
	},
}

func init() {
	tokenCmd.AddCommand(tokenCreateCmd)
	requestsCmd.AddCommand(requestsListCmd, requestsApproveCmd, requestsRejectCmd)
	nodesCmd.AddCommand(nodesListCmd, nodeProfilesCmd)
	agentCmd.AddCommand(agentInventoryCmd, agentApplyCmd, agentWatchCmd)
	planCmd.AddCommand(planGetCmd, planApplyCmd)
}

func controllerClient() (*grpc.ClientConn, error) {
	return dialGRPC(rootCfg.controllerAddr)
}

func nodeClient() (*grpc.ClientConn, error) {
	return nodeClientWith("")
}

func nodeClientWith(override string) (*grpc.ClientConn, error) {
	return dialGRPC(pick(override, rootCfg.nodeAddr))
}

func dialGRPC(addr string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithBlock()}
	if rootCfg.insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else if rootCfg.caFile != "" {
		creds, err := credentials.NewClientTLSFromFile(rootCfg.caFile, "")
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	return grpc.DialContext(ctx, addr, opts...)
}

func pick(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

func ctxWithTimeout() context.Context {
	ctx := context.Background()
	if rootCfg.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "token", rootCfg.token)
	}
	ctx, _ = context.WithTimeout(ctx, rootCfg.timeout)
	return ctx
}

func ctxWithoutTimeout() context.Context {
	ctx := context.Background()
	if rootCfg.token != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "token", rootCfg.token)
	}
	return ctx
}

func parseExpiration(value string) (*timestamppb.Timestamp, error) {
	if value == "" {
		return nil, nil
	}
	if dur, err := time.ParseDuration(value); err == nil {
		return timestamppb.New(time.Now().Add(dur)), nil
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return timestamppb.New(ts), nil
	}
	return nil, fmt.Errorf("invalid expiration: %s", value)
}

func parseMetadata(items []string) map[string]string {
	meta := map[string]string{}
	for _, entry := range items {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		meta[parts[0]] = parts[1]
	}
	return meta
}

func watchControllerStream(cc *grpc.ClientConn, req *clustercontrollerpb.WatchOperationsRequest) error {
	client := clustercontrollerpb.NewClusterControllerServiceClient(cc)
	stream, err := client.WatchOperations(ctxWithoutTimeout(), req)
	if err != nil {
		return err
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}
		printProto(event)
		if event.GetDone() {
			return nil
		}
	}
}

func watchControllerOperations(nodeID, operationID string) error {
	cc, err := controllerClient()
	if err != nil {
		return err
	}
	defer cc.Close()
	req := &clustercontrollerpb.WatchOperationsRequest{
		NodeId:      nodeID,
		OperationId: operationID,
	}
	return watchControllerStream(cc, req)
}

func watchAgentOperation(operationID, nodeOverride string) error {
	cc, err := nodeClientWith(nodeOverride)
	if err != nil {
		return err
	}
	defer cc.Close()
	client := nodeagentpb.NewNodeAgentServiceClient(cc)
	stream, err := client.WatchOperation(ctxWithoutTimeout(), &nodeagentpb.WatchOperationRequest{OperationId: operationID})
	if err != nil {
		return err
	}
	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}
		printProto(event)
		if event.GetDone() {
			return nil
		}
	}
}

func loadPlan(path string) (*clustercontrollerpb.NodePlan, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var plan clustercontrollerpb.NodePlan
	if err := protojson.Unmarshal(data, &plan); err == nil {
		return &plan, nil
	}
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, err
	}
	if err := protojson.Unmarshal(jsonData, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

func printProto(msg proto.Message) {
	if msg == nil {
		return
	}
	switch rootCfg.output {
	case "json":
		out, _ := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
		fmt.Println(string(out))
	case "yaml":
		out, _ := protojson.Marshal(msg)
		if yamlOut, err := yaml.JSONToYAML(out); err == nil {
			fmt.Println(string(yamlOut))
		} else {
			fmt.Println(string(out))
		}
	default:
		out, _ := prototext.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
		fmt.Println(string(out))
	}
}
