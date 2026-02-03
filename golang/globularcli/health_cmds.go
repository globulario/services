package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/globulario/services/golang/config"
	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	dnspb "github.com/globulario/services/golang/dns/dnspb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	healthLocal      bool
	healthJSON       bool
	healthTimeoutSec int
)

func init() {
	healthCmd = &cobra.Command{
		Use:   "health",
		Short: "Display cluster health status",
		Long: `Display health status of the cluster or local node.

By default, queries the cluster controller for cluster-wide health.
Use --local to perform local health checks without contacting the controller.

Examples:
  globular cluster health              # Cluster-wide health from controller
  globular cluster health --local      # Local node health checks
  globular cluster health --local --json  # Local health in JSON format
`,
		RunE: runHealthCommand,
	}

	healthCmd.Flags().BoolVar(&healthLocal, "local", false, "Perform local health checks instead of cluster-wide")
	healthCmd.Flags().BoolVar(&healthJSON, "json", false, "Output health status in JSON format")
	healthCmd.Flags().IntVar(&healthTimeoutSec, "timeout", 10, "Timeout in seconds for health checks")

	// Add healthCmd to clusterCmd
	clusterCmd.AddCommand(healthCmd)
}

func runHealthCommand(cmd *cobra.Command, args []string) error {
	if healthLocal {
		return runLocalHealthChecks()
	}
	return runClusterHealthChecks()
}

// ServiceEndpoint represents a resolved service endpoint
type ServiceEndpoint struct {
	Host string
	Port int
	Proto string
}

// resolveServiceEndpoint attempts to resolve a service endpoint using config-driven discovery
// Resolution chain: 1) Runtime config 2) --describe output 3) Fallback defaults
func resolveServiceEndpoint(serviceID string) (ServiceEndpoint, error) {
	var endpoint ServiceEndpoint

	// Try to get from --describe output
	root := config.GetServicesRoot()
	if root != "" {
		binPath, err := config.FindServiceBinary(root, serviceID)
		if err == nil {
			desc, err := config.RunDescribe(binPath, 5*time.Second, nil)
			if err == nil {
				endpoint.Host = desc.Address
				if endpoint.Host == "" {
					endpoint.Host = "127.0.0.1"
				}
				endpoint.Port = desc.Port
				endpoint.Proto = desc.Proto
				if endpoint.Proto == "" {
					endpoint.Proto = desc.Protocol
				}
				if endpoint.Port > 0 {
					return endpoint, nil
				}
			}
		}
	}

	// Fallback to default ports
	switch strings.ToLower(serviceID) {
	case "etcd":
		return ServiceEndpoint{Host: "127.0.0.1", Port: 2379, Proto: "tcp"}, nil
	case "scylla", "scylladb":
		return ServiceEndpoint{Host: "127.0.0.1", Port: 9042, Proto: "tcp"}, nil
	case "minio":
		return ServiceEndpoint{Host: "127.0.0.1", Port: 9000, Proto: "http"}, nil
	case "envoy", "envoy-admin":
		return ServiceEndpoint{Host: "127.0.0.1", Port: 9901, Proto: "http"}, nil
	case "dns", "dns_server", "globular-dns":
		return ServiceEndpoint{Host: "localhost", Port: 10033, Proto: "grpc"}, nil
	default:
		return endpoint, fmt.Errorf("unknown service %q and no --describe output found", serviceID)
	}
}

// HealthCheckResult represents the result of a single health check
type HealthCheckResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Details string `json:"details"`
}

// LocalHealthStatus represents the overall local health status
type LocalHealthStatus struct {
	Healthy bool                `json:"healthy"`
	Checks  []HealthCheckResult `json:"checks"`
}

func runLocalHealthChecks() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(healthTimeoutSec)*time.Second)
	defer cancel()

	// Load network spec to determine what to check
	networkSpec, specErr := loadNetworkSpec()

	checks := []HealthCheckResult{}

	// Check etcd
	checks = append(checks, checkEtcd(ctx))

	// Check scylla
	checks = append(checks, checkScylla(ctx))

	// Check minio
	checks = append(checks, checkMinio(ctx))

	// Check envoy admin
	checks = append(checks, checkEnvoy(ctx))

	// Check gateway
	if networkSpec != nil {
		checks = append(checks, checkGateway(ctx, networkSpec))
	} else {
		checks = append(checks, HealthCheckResult{
			Name:    "gateway",
			OK:      false,
			Details: fmt.Sprintf("cannot check gateway: %v", specErr),
		})
	}

	// Check DNS
	checks = append(checks, checkDNS(ctx))

	// Check TLS (only if protocol is https)
	if networkSpec != nil && strings.EqualFold(networkSpec.GetProtocol(), "https") {
		checks = append(checks, checkTLS(ctx, networkSpec.GetClusterDomain()))
	}

	// Determine overall health
	allHealthy := true
	for _, check := range checks {
		if !check.OK {
			allHealthy = false
			break
		}
	}

	status := LocalHealthStatus{
		Healthy: allHealthy,
		Checks:  checks,
	}

	if healthJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(status)
	}

	// Human-readable output
	if allHealthy {
		fmt.Println("✅ Cluster is healthy")
	} else {
		fmt.Println("❌ Cluster is unhealthy")
	}
	fmt.Println()

	fmt.Println("Health Checks:")
	for _, check := range checks {
		icon := "✅"
		if !check.OK {
			icon = "❌"
		}
		fmt.Printf("  %s %-12s %s\n", icon, check.Name, check.Details)
	}

	if !allHealthy {
		return errors.New("cluster is unhealthy")
	}

	return nil
}

func runClusterHealthChecks() error {
	cc, err := controllerClient()
	if err != nil {
		return err
	}
	defer cc.Close()
	client := clustercontrollerpb.NewClusterControllerServiceClient(cc)

	resp, err := client.GetClusterHealth(ctxWithTimeout(), &clustercontrollerpb.GetClusterHealthRequest{})
	if err != nil {
		return err
	}

	if healthJSON {
		// Convert to JSON
		data := map[string]interface{}{
			"healthy":          strings.ToLower(resp.GetStatus()) == "healthy",
			"status":           resp.GetStatus(),
			"total_nodes":      resp.GetTotalNodes(),
			"healthy_nodes":    resp.GetHealthyNodes(),
			"unhealthy_nodes":  resp.GetUnhealthyNodes(),
			"unknown_nodes":    resp.GetUnknownNodes(),
			"node_health":      resp.GetNodeHealth(),
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	}

	// Print overall status
	fmt.Printf("Cluster Status: %s\n", strings.ToUpper(resp.GetStatus()))
	fmt.Printf("\nNode Summary:\n")
	fmt.Printf("  Total:     %d\n", resp.GetTotalNodes())
	fmt.Printf("  Healthy:   %d\n", resp.GetHealthyNodes())
	fmt.Printf("  Unhealthy: %d\n", resp.GetUnhealthyNodes())
	fmt.Printf("  Unknown:   %d\n", resp.GetUnknownNodes())

	if len(resp.GetNodeHealth()) > 0 {
		fmt.Printf("\nNode Details:\n")
		for _, node := range resp.GetNodeHealth() {
			icon := "✅"
			if node.GetStatus() != "healthy" {
				icon = "❌"
			}
			fmt.Printf("  %s %s (%s)\n", icon, node.GetNodeId(), node.GetHostname())
			if node.GetLastError() != "" {
				fmt.Printf("     Error: %s\n", node.GetLastError())
			}
		}
	}

	return nil
}

// loadNetworkSpec reads the network spec from the local file
func loadNetworkSpec() (*clustercontrollerpb.ClusterNetworkSpec, error) {
	data, err := os.ReadFile("/var/lib/globular/network.json")
	if err != nil {
		return nil, fmt.Errorf("read network spec: %w", err)
	}

	var spec clustercontrollerpb.ClusterNetworkSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal network spec: %w", err)
	}

	return &spec, nil
}

// checkEtcd checks if etcd is reachable
func checkEtcd(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{Name: "etcd"}

	endpoint, err := resolveServiceEndpoint("etcd")
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("failed to resolve endpoint: %v", err)
		return result
	}

	address := fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
	tcpResult := checkTCPPort(ctx, address)
	result.OK = tcpResult.OK
	if !result.OK {
		result.Details = fmt.Sprintf("etcd unreachable on %s", address)
	} else {
		result.Details = "etcd reachable"
	}
	return result
}

// checkScylla checks if scylla is reachable
func checkScylla(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{Name: "scylla"}

	endpoint, err := resolveServiceEndpoint("scylla")
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("failed to resolve endpoint: %v", err)
		return result
	}

	address := fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
	tcpResult := checkTCPPort(ctx, address)
	result.OK = tcpResult.OK
	if !result.OK {
		result.Details = fmt.Sprintf("scylla unreachable on %s", address)
	} else {
		result.Details = "scylla reachable"
	}
	return result
}

// checkMinio checks if minio is reachable
func checkMinio(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{Name: "minio"}

	endpoint, err := resolveServiceEndpoint("minio")
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("failed to resolve endpoint: %v", err)
		return result
	}

	address := fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)
	tcpResult := checkTCPPort(ctx, address)
	result.OK = tcpResult.OK
	if !result.OK {
		result.Details = fmt.Sprintf("minio unreachable on %s", address)
	} else {
		result.Details = "minio reachable"
	}
	return result
}

// checkEnvoy checks if envoy admin interface is healthy
func checkEnvoy(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{Name: "envoy"}

	endpoint, err := resolveServiceEndpoint("envoy-admin")
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("failed to resolve endpoint: %v", err)
		return result
	}

	url := fmt.Sprintf("http://%s:%d/ready", endpoint.Host, endpoint.Port)

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("failed to create request: %v", err)
		return result
	}

	resp, err := client.Do(req)
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("envoy admin unreachable on %s:%d", endpoint.Host, endpoint.Port)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		result.OK = true
		result.Details = "envoy ready"
	} else {
		result.OK = false
		result.Details = fmt.Sprintf("envoy not ready (status %d)", resp.StatusCode)
	}

	return result
}

// checkGateway checks if gateway is reachable on the configured port
func checkGateway(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) HealthCheckResult {
	result := HealthCheckResult{Name: "gateway"}

	port := spec.GetPortHttp()
	if strings.EqualFold(spec.GetProtocol(), "https") && spec.GetPortHttps() > 0 {
		port = spec.GetPortHttps()
	}

	if port == 0 {
		port = 80 // default
	}

	tcpResult := checkTCPPort(ctx, fmt.Sprintf("127.0.0.1:%d", port))
	result.OK = tcpResult.OK
	if result.OK {
		result.Details = fmt.Sprintf("gateway reachable on port %d", port)
	} else {
		result.Details = fmt.Sprintf("gateway unreachable on port %d", port)
	}

	return result
}

// checkDNS checks if DNS server is working
func checkDNS(ctx context.Context) HealthCheckResult {
	result := HealthCheckResult{Name: "dns"}

	endpoint, err := resolveServiceEndpoint("dns")
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("failed to resolve endpoint: %v", err)
		return result
	}

	address := fmt.Sprintf("%s:%d", endpoint.Host, endpoint.Port)

	// Try to connect to DNS service gRPC port
	cc, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.WithTimeout(3*time.Second))
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("dns service unreachable on %s", address)
		return result
	}
	defer cc.Close()

	// Try to query managed domains
	client := dnspb.NewDnsServiceClient(cc)
	dnsCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err = client.GetDomains(dnsCtx, &dnspb.GetDomainsRequest{})
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("dns service error: %v", err)
		return result
	}

	result.OK = true
	result.Details = "dns service operational"
	return result
}

// checkTLS checks if TLS certificate is present and valid for the domain
func checkTLS(ctx context.Context, domain string) HealthCheckResult {
	result := HealthCheckResult{Name: "tls"}

	certPath := "/etc/globular/tls/fullchain.pem"
	keyPath := "/etc/globular/tls/privkey.pem"

	// Check if cert exists
	if _, err := os.Stat(certPath); err != nil {
		result.OK = false
		result.Details = "certificate file missing"
		return result
	}

	// Check if key exists
	if _, err := os.Stat(keyPath); err != nil {
		result.OK = false
		result.Details = "private key file missing"
		return result
	}

	// Read and parse certificate
	data, err := os.ReadFile(certPath)
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("failed to read certificate: %v", err)
		return result
	}

	block, _ := pem.Decode(data)
	if block == nil {
		result.OK = false
		result.Details = "invalid PEM format"
		return result
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		result.OK = false
		result.Details = fmt.Sprintf("failed to parse certificate: %v", err)
		return result
	}

	// Check expiration
	now := time.Now()
	if now.Before(cert.NotBefore) {
		result.OK = false
		result.Details = "certificate not yet valid"
		return result
	}

	if now.After(cert.NotAfter) {
		result.OK = false
		result.Details = "certificate expired"
		return result
	}

	// Check domain match
	if domain != "" {
		if err := cert.VerifyHostname(domain); err != nil {
			result.OK = false
			result.Details = fmt.Sprintf("certificate not valid for domain %s", domain)
			return result
		}
	}

	// Calculate days until expiry
	daysUntilExpiry := cert.NotAfter.Sub(now).Hours() / 24

	result.OK = true
	result.Details = fmt.Sprintf("certificate valid (expires in %.0f days)", daysUntilExpiry)

	return result
}

// checkTCPPort checks if a TCP port is reachable
func checkTCPPort(ctx context.Context, address string) HealthCheckResult {
	result := HealthCheckResult{OK: false}

	dialer := &net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		result.OK = false
		result.Details = err.Error()
		return result
	}
	conn.Close()

	result.OK = true
	result.Details = "reachable"
	return result
}
