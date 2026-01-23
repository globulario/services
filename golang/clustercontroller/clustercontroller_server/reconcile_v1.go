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

func hasUnit(units []unitStatusRecord, name string) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, u := range units {
		if strings.ToLower(strings.TrimSpace(u.Name)) == target {
			return true
		}
	}
	return false
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
		planStep("network.apply_spec", map[string]interface{}{
			"spec_json": string(content),
			"mode":      "merge",
		}),
	}
	// Success probes
	var probes []*planpb.Probe
	protocol := strings.ToLower(strings.TrimSpace(spec.GetProtocol()))
	if protocol == "" {
		protocol = "http"
	}
	if protocol == "https" {
		steps = append(steps, planStep("tls.ensure", map[string]interface{}{
			"fullchain_path": "/var/lib/globular/tls/fullchain.pem",
			"privkey_path":   "/var/lib/globular/tls/privkey.pem",
		}))
	}
	steps = append(steps,
		planStep("service.restart", map[string]interface{}{
			"unit": "globular-xds.service",
		}),
		planStep("service.restart", map[string]interface{}{
			"unit": "globular-gateway.service",
		}),
	)
	if hasUnit(observed.Units, "envoy.service") {
		steps = append(steps, planStep("service.restart", map[string]interface{}{
			"unit": "envoy.service",
		}))
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
	}
	if protocol == "https" {
		probes = append(probes, &planpb.Probe{
			Type: "tls.cert_valid_for_domain",
			Args: structpbFromMap(map[string]interface{}{
				"domain":    spec.GetClusterDomain(),
				"cert_path": "/var/lib/globular/tls/fullchain.pem",
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
			Path: "/var/lib/globular/tls/fullchain.pem",
		})
	}

	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		NodeId:        nodeID,
		Reason:        "update_cluster_network",
		Locks:         []string{"network", "tls", "service:gateway", "service:xds", "service:envoy"},
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
func BuildServiceUpgradePlan(nodeID string, svcName string, desiredVersion string, desiredHash string) *planpb.NodePlan {
	if strings.TrimSpace(svcName) == "" {
		svcName = "globular"
	}
	unit := svcName
	if !strings.HasSuffix(strings.ToLower(unit), ".service") {
		unit = fmt.Sprintf("%s.service", svcName)
	}
	marker := versionutil.MarkerPath(svcName)
	return &planpb.NodePlan{
		ApiVersion:  "globular.io/plan/v1",
		Kind:        "NodePlan",
		NodeId:      nodeID,
		Reason:      "service_upgrade",
		Locks:       []string{fmt.Sprintf("service:%s", svcName)},
		DesiredHash: desiredHash,
		Policy: &planpb.PlanPolicy{
			MaxRetries:     3,
			RetryBackoffMs: 2000,
			FailureMode:    planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				planStep("artifact.fetch", map[string]interface{}{
					"service":       svcName,
					"version":       desiredVersion,
					"artifact_path": fmt.Sprintf("/var/lib/globular/staging/%s/%s.artifact", svcName, desiredVersion),
				}),
				planStep("artifact.verify", map[string]interface{}{
					"artifact_path": fmt.Sprintf("/var/lib/globular/staging/%s/%s.artifact", svcName, desiredVersion),
				}),
				planStep("service.install_payload", map[string]interface{}{
					"service":       svcName,
					"artifact_path": fmt.Sprintf("/var/lib/globular/staging/%s/%s.artifact", svcName, desiredVersion),
					"install_path":  fmt.Sprintf("/var/lib/globular/staging/%s/%s.bin", svcName, desiredVersion),
				}),
				planStep("service.write_version_marker", map[string]interface{}{
					"service": svcName,
					"version": desiredVersion,
					"path":    marker,
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
				serviceProbeForUnit(unit),
			},
		},
	}
}

// serviceProbeForUnit returns a minimal probe for a given unit.
func serviceProbeForUnit(unit string) *planpb.Probe {
	u := strings.ToLower(unit)
	switch {
	case strings.Contains(u, "gateway"):
		return &planpb.Probe{Type: "probe.tcp", Args: structpbFromMap(map[string]interface{}{"address": "127.0.0.1:80"})}
	case strings.Contains(u, "xds"):
		return &planpb.Probe{Type: "probe.tcp", Args: structpbFromMap(map[string]interface{}{"address": "127.0.0.1:7443"})}
	case strings.Contains(u, "envoy"):
		return &planpb.Probe{Type: "probe.http", Args: structpbFromMap(map[string]interface{}{"url": "http://127.0.0.1:9901/ready"})}
	default:
		return &planpb.Probe{Type: "probe.tcp", Args: structpbFromMap(map[string]interface{}{"address": "127.0.0.1:80"})}
	}
}

func structpbFromMap(fields map[string]interface{}) *structpb.Struct {
	if len(fields) == 0 {
		return nil
	}
	s, _ := structpb.NewStruct(fields)
	return s
}
