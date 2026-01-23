package main

import (
	"fmt"
	"strings"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/versionutil"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

// ClusterDesiredState represents the reconciler inputs for generating plans.
type ClusterDesiredState struct {
	Network         *clustercontrollerpb.ClusterNetworkSpec
	ServiceVersions map[string]string
}

// NodeObservedState captures lightweight observed data used for drift detection.
type NodeObservedState struct {
	Units []unitStatusRecord
}

// BuildNetworkTransitionPlan constructs a reconciliation plan for network/protocol changes.
func BuildNetworkTransitionPlan(nodeID string, desired ClusterDesiredState, observed NodeObservedState) (*planpb.NodePlan, error) {
	if desired.Network == nil {
		return nil, fmt.Errorf("desired network is required")
	}
	spec := desired.Network
	content, err := protojson.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal network spec: %w", err)
	}
	steps := []*planpb.PlanStep{
		planStep("file.write_atomic", map[string]interface{}{
			"path":    "/var/lib/globular/network.json",
			"content": string(content),
		}),
		planStep("service.restart", map[string]interface{}{
			"unit": "globular-xds.service",
		}),
		planStep("service.restart", map[string]interface{}{
			"unit": "globular-gateway.service",
		}),
	}
	// Success probes
	var probes []*planpb.Probe
	protocol := strings.ToLower(strings.TrimSpace(spec.GetProtocol()))
	if protocol == "" {
		protocol = "http"
	}
	port := spec.GetPortHttp()
	if protocol == "https" && spec.GetPortHttps() > 0 {
		port = spec.GetPortHttps()
	}
	if port > 0 {
		probes = append(probes, &planpb.Probe{
			Type: "probe.tcp",
			Args: structpbFromMap(map[string]interface{}{
				"address": fmt.Sprintf("127.0.0.1:%d", port),
			}),
		})
		probes = append(probes, &planpb.Probe{
			Type: "probe.http",
			Args: structpbFromMap(map[string]interface{}{
				"url": fmt.Sprintf("%s://127.0.0.1:%d/health", protocol, port),
			}),
		})
	}

	desiredState := &planpb.DesiredState{
		Services: []*planpb.DesiredService{
			{Name: "globular-gateway", Unit: "globular-gateway.service", Version: desired.ServiceVersions["globular-gateway"]},
			{Name: "globular-xds", Unit: "globular-xds.service", Version: desired.ServiceVersions["globular-xds"]},
		},
		Files: []*planpb.DesiredFile{
			{Path: "/var/lib/globular/network.json"},
		},
	}
	if protocol == "https" {
		desiredState.Files = append(desiredState.Files, &planpb.DesiredFile{
			Path: fmt.Sprintf("/etc/globular/tls/%s/fullchain.pem", spec.GetClusterDomain()),
		})
	}

	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		NodeId:        nodeID,
		Reason:        "update_cluster_network",
		Locks:         []string{"network", "tls", "service:gateway", "service:xds"},
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		Policy: &planpb.PlanPolicy{
			MaxRetries:     3,
			RetryBackoffMs: 2000,
			FailureMode:    planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps:         steps,
			SuccessProbes: probes,
			Desired:       desiredState,
		},
	}, nil
}

// BuildServiceUpgradePlan scaffolds a service upgrade plan with version invariants.
func BuildServiceUpgradePlan(nodeID string, svcName string, desiredVersion string) *planpb.NodePlan {
	if strings.TrimSpace(svcName) == "" {
		svcName = "globular"
	}
	unit := fmt.Sprintf("%s.service", svcName)
	marker := versionutil.MarkerPath(svcName)
	return &planpb.NodePlan{
		ApiVersion: "globular.io/plan/v1",
		Kind:       "NodePlan",
		NodeId:     nodeID,
		Reason:     "service_upgrade",
		Locks:      []string{fmt.Sprintf("service:%s", svcName)},
		Policy: &planpb.PlanPolicy{
			MaxRetries:     3,
			RetryBackoffMs: 2000,
			FailureMode:    planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				planStep("file.write_atomic", map[string]interface{}{
					"path":    marker,
					"content": desiredVersion,
				}),
				planStep("service.restart", map[string]interface{}{
					"unit": unit,
				}),
			},
			Desired: &planpb.DesiredState{
				Services: []*planpb.DesiredService{
					{Name: svcName, Version: desiredVersion, Unit: unit},
				},
				Files: []*planpb.DesiredFile{
					{Path: marker},
				},
			},
			SuccessProbes: []*planpb.Probe{
				{
					Type: "probe.http",
					Args: structpbFromMap(map[string]interface{}{
						"url": "http://127.0.0.1/health",
					}),
				},
			},
		},
	}
}

func structpbFromMap(fields map[string]interface{}) *structpb.Struct {
	if len(fields) == 0 {
		return nil
	}
	s, _ := structpb.NewStruct(fields)
	return s
}
