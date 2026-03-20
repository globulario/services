package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/versionutil"
	Utility "github.com/globulario/utility"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

// ClusterDesiredState represents the reconciler inputs for generating plans.
type ClusterDesiredState struct {
	Network         *cluster_controllerpb.ClusterNetworkSpec
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
		// If ACME is enabled, run certificate issuance/renewal before validation
		if spec.GetAcmeEnabled() {
			steps = append(steps, planStep("tls.acme.ensure", map[string]interface{}{
				"domain":         spec.GetClusterDomain(),
				"admin_email":    spec.GetAdminEmail(),
				"acme_enabled":   spec.GetAcmeEnabled(),
				"dns_addr":       "", // Empty: node agent will discover DNS endpoint
				"fullchain_path": "/etc/globular/tls/fullchain.pem",
				"privkey_path":   "/etc/globular/tls/privkey.pem",
			}))
		}
		// Always run tls.ensure to validate (never issues certs)
		steps = append(steps, planStep("tls.ensure", map[string]interface{}{
			"fullchain_path": "/etc/globular/tls/fullchain.pem",
			"privkey_path":   "/etc/globular/tls/privkey.pem",
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
	if hasUnit(observed.Units, "globular-envoy.service") {
		steps = append(steps, planStep("service.restart", map[string]interface{}{
			"unit": "globular-envoy.service",
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
				"cert_path": "/etc/globular/tls/fullchain.pem",
			}),
		})
	}

	desiredState := &planpb.DesiredState{
		Services: []*planpb.DesiredService{},
		Files: []*planpb.DesiredFile{
			{Path: "/var/lib/globular/network.json"},
		},
	}
	for _, svc := range []struct {
		name string
		unit string
	}{
		{name: "globular-gateway", unit: "globular-gateway.service"},
		{name: "globular-xds", unit: "globular-xds.service"},
	} {
		if ver, ok := desired.ServiceVersions[svc.name]; ok {
			desiredState.Services = append(desiredState.Services, &planpb.DesiredService{
				Name:    svc.name,
				Unit:    svc.unit,
				Version: ver,
			})
		}
	}
	if protocol == "https" {
		desiredState.Files = append(desiredState.Files, &planpb.DesiredFile{
			Path: "/etc/globular/tls/fullchain.pem",
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
func BuildServiceUpgradePlan(nodeID string, svcName string, desiredVersion string, desiredHash string, buildNumber int64) *planpb.NodePlan {
	if strings.TrimSpace(svcName) == "" {
		svcName = "globular"
	}
	platform := "linux_amd64"
	svcCanonical := canonicalServiceName(svcName)
	unit := serviceUnitForCanonical(svcCanonical)
	marker := versionutil.MarkerPath(svcName)

	repo := resolveRepositoryInfo()
	repoAddr := strings.TrimSpace(os.Getenv(repositoryAddressEnv))
	if repoAddr == "" {
		repoAddr = repo.Address
	}

	publisherID := strings.TrimSpace(os.Getenv("GLOBULAR_DEFAULT_PUBLISHER"))
	if publisherID == "" {
		publisherID = "core@globular.io"
	}

	artifactPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s.artifact", svcName, desiredVersion)

	fetchArgs := map[string]interface{}{
		"service":         svcName,
		"version":         desiredVersion,
		"platform":        platform,
		"artifact_path":   artifactPath,
		"repository_addr": repoAddr,
		"publisher_id":    publisherID,
		"build_number":    buildNumber,
	}
	if desiredHash != "" {
		fetchArgs["expected_sha256"] = desiredHash
	}
	if !repo.TLS {
		fetchArgs["repository_insecure"] = true
	}
	if repo.CAPath != "" {
		fetchArgs["repository_ca_path"] = repo.CAPath
	}

	verifyArgs := map[string]interface{}{
		"artifact_path": artifactPath,
	}
	if desiredHash != "" {
		verifyArgs["expected_sha256"] = desiredHash
	}

	return &planpb.NodePlan{
		ApiVersion:  "globular.io/plan/v1",
		Kind:        "NodePlan",
		NodeId:      nodeID,
		Reason:      "service_upgrade",
		Locks:       []string{fmt.Sprintf("service:%s", svcCanonical)},
		DesiredHash: desiredHash,
		Policy: &planpb.PlanPolicy{
			MaxRetries:     3,
			RetryBackoffMs: 2000,
			FailureMode:    planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				planStep("artifact.fetch", fetchArgs),
				planStep("artifact.verify", verifyArgs),
				planStep("service.install_payload", map[string]interface{}{
					"service":       svcName,
					"version":       desiredVersion,
					"artifact_path": fmt.Sprintf("/var/lib/globular/staging/%s/%s.artifact", svcName, desiredVersion),
				}),
				planStep("service.write_version_marker", map[string]interface{}{
					"service": svcName,
					"version": desiredVersion,
					"path":    marker,
				}),
				planStep("package.report_state", map[string]interface{}{
					"node_id":      nodeID,
					"name":         svcCanonical,
					"version":      desiredVersion,
					"kind":         "SERVICE",
					"publisher_id": publisherID,
					"platform":     platform,
					"checksum":     desiredHash,
					"build_number": buildNumber,
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
	case strings.Contains(u, "rbac"):
		return &planpb.Probe{Type: "probe.service_config_tcp", Args: structpbFromMap(map[string]interface{}{"service": "rbac", "timeout_ms": 1500})}
	case strings.Contains(u, "resource"):
		return &planpb.Probe{Type: "probe.service_config_tcp", Args: structpbFromMap(map[string]interface{}{"service": "resource", "timeout_ms": 1500})}
	case strings.Contains(u, "repository"):
		return &planpb.Probe{Type: "probe.service_config_tcp", Args: structpbFromMap(map[string]interface{}{"service": "repository", "timeout_ms": 1500})}
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

// BuildServiceRemovePlan creates a removal plan that stops, disables, removes
// the binary and unit file, clears config, version marker, and installed-state
// registry entry for the service.
func BuildServiceRemovePlan(nodeID string, svcCanonical string, desiredHash string) *planpb.NodePlan {
	unit := serviceUnitForCanonical(svcCanonical)

	return &planpb.NodePlan{
		ApiVersion:  "globular.io/plan/v1",
		Kind:        "NodePlan",
		NodeId:      nodeID,
		Reason:      "service_remove",
		Locks:       []string{fmt.Sprintf("service:%s", svcCanonical)},
		DesiredHash: desiredHash,
		Policy: &planpb.PlanPolicy{
			MaxRetries:     3,
			RetryBackoffMs: 2000,
			FailureMode:    planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				planStep("service.stop", map[string]interface{}{"unit": unit}),
				planStep("service.disable", map[string]interface{}{"unit": unit}),
				planStep("package.uninstall", map[string]interface{}{
					"kind":         "SERVICE",
					"name":         svcCanonical,
					"unit":         unit,
				}),
				planStep("package.clear_state", map[string]interface{}{
					"node_id": nodeID,
					"name":    svcCanonical,
					"kind":    "SERVICE",
				}),
			},
			Desired: &planpb.DesiredState{Services: []*planpb.DesiredService{}},
			SuccessProbes: []*planpb.Probe{
				{Type: "probe.exec", Args: structpbFromMap(map[string]interface{}{"cmd": fmt.Sprintf("! systemctl is-active --quiet %s", unit)})},
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

// repositoryInfo holds resolved repository connection details.
type repositoryInfo struct {
	Address string
	TLS     bool
	CAPath  string
}

// resolveRepositoryInfo looks up the repository service in the Globular
// service registry and returns connection details. Falls back to "localhost:10101".
func resolveRepositoryInfo() repositoryInfo {
	const fallback = "localhost:10101"
	cfg, err := config.GetServiceConfigurationById("repository.PackageRepository")
	if err != nil || cfg == nil {
		return repositoryInfo{Address: fallback}
	}
	port := Utility.ToInt(cfg["Port"])
	host := strings.TrimSpace(Utility.ToString(cfg["Address"]))
	if host == "" {
		host = "localhost"
	}
	// If the Address already contains a colon (host:port), use it as-is.
	var addr string
	if strings.Contains(host, ":") {
		addr = host
	} else if port <= 0 {
		addr = fallback
	} else {
		addr = fmt.Sprintf("%s:%d", host, port)
	}

	tlsEnabled := false
	if v, ok := cfg["TLS"]; ok {
		switch t := v.(type) {
		case bool:
			tlsEnabled = t
		case string:
			tlsEnabled = strings.EqualFold(t, "true")
		}
	}
	caPath := ""
	if s, ok := cfg["CertAuthorityTrust"]; ok {
		caPath = strings.TrimSpace(Utility.ToString(s))
	}
	return repositoryInfo{Address: addr, TLS: tlsEnabled, CAPath: caPath}
}

// resolveRepositoryAddress returns "host:port" for backward compat.
func resolveRepositoryAddress() string {
	return resolveRepositoryInfo().Address
}
