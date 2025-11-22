// globular_service/describe_health.go
package globular_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// DescribeMap returns a stable map of public, cluster-relevant fields.
// It intentionally excludes runtime-only internals like grpcServer pointers.
func DescribeMap(s Service) map[string]any {
	ensureDescribeID(s)
	return map[string]any{
		"Id":                 s.GetId(),
		"Name":               s.GetName(),
		"Description":        s.GetDescription(),
		"PublisherID":        s.GetPublisherID(),
		"Version":            s.GetVersion(),
		"Keywords":           s.GetKeywords(),
		"Repositories":       s.GetRepositories(),
		"Discoveries":        s.GetDiscoveries(),
		"Domain":             s.GetDomain(),
		"Address":            s.GetAddress(),
		"Protocol":           s.GetProtocol(),
		"Port":               s.GetPort(),
		"Proxy":              s.GetProxy(),
		"TLS":                s.GetTls(),
		"CertAuthorityTrust": s.GetCertAuthorityTrust(),
		"CertFile":           s.GetCertFile(),
		"KeyFile":            s.GetKeyFile(),
		"AllowAllOrigins":    s.GetAllowAllOrigins(),
		"AllowedOrigins":     s.GetAllowedOrigins(),
		"KeepAlive":          s.GetKeepAlive(),
		"KeepUpToDate":       s.GetKeepUpToDate(),
		"Dependencies":       s.GetDependencies(),
		"Permissions":        s.GetPermissions(),
		"Checksum":           s.GetChecksum(),
		"Platform":           s.GetPlatform(),
		"Path":               s.GetPath(),
		"Proto":              s.GetProto(),
		// Runtime snapshot (informational — helpful for the supervisor)
		"State":        s.GetState(),
		"Process":      s.GetProcess(),
		"ProxyProcess": s.GetProxyProcess(),
		"LastError":    s.GetLastError(),
		"ModTime":      s.GetModTime(),
		"Mac":          s.GetMac(),
	}
}

// DescribeJSON returns formatted JSON for --describe.
func DescribeJSON(s Service) ([]byte, error) {
	m := DescribeMap(s)
	return json.MarshalIndent(m, "", "  ")
}

// ensureDescribeID assigns a deterministic ID before emitting --describe output.
func ensureDescribeID(s Service) {
	if s.GetId() != "" {
		return
	}
	name := strings.TrimSpace(s.GetName())
	addr := strings.ToLower(strings.TrimSpace(s.GetAddress()))
	if addr == "" {
		host := strings.ToLower(strings.TrimSpace(s.GetDomain()))
		if host == "" {
			host = "localhost"
		}
		if port := s.GetPort(); port > 0 {
			addr = fmt.Sprintf("%s:%d", host, port)
		} else {
			addr = host
		}
	}
	seed := fmt.Sprintf("%s:%s", name, addr)
	s.SetId(Utility.GenerateUUID(seed))
}

/* ---------- Health probing ---------- */

type HealthStatus string

const (
	HealthUnknown    HealthStatus = "UNKNOWN"
	HealthServing    HealthStatus = "SERVING"
	HealthNotServing HealthStatus = "NOT_SERVING"
)

type HealthReport struct {
	Service   string                 `json:"service"`
	Target    string                 `json:"target"` // "host:port"
	Status    HealthStatus           `json:"status"` // SERVING / NOT_SERVING / UNKNOWN
	Checks    map[string]string      `json:"checks"` // process/tcp/grpc
	LatencyMs int64                  `json:"latency_ms"`
	When      string                 `json:"when"` // RFC3339
	Details   map[string]interface{} `json:"details,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// HealthOptions allows tuning timeouts and target.
type HealthOptions struct {
	Timeout     time.Duration
	Host        string // defaults to 127.0.0.1
	ServiceName string // gRPC health "service" name; empty = overall server
}

// HealthCheck dials the service locally and queries the gRPC health endpoint.
// It also checks TCP connectivity first. Returns a JSON-able report.
func HealthCheck(s Service, opt *HealthOptions) (*HealthReport, error) {
	host := s.GetAddress()
	if host == "" {
		host = "localhost"
	}
	
	if opt != nil && opt.Host != "" {
		host = opt.Host
	}
	timeout := 2 * time.Second
	if opt != nil && opt.Timeout > 0 {
		timeout = opt.Timeout
	}

	target := fmt.Sprintf("%s:%d", host, s.GetPort())
	start := time.Now()

	report := &HealthReport{
		Service: s.GetName(),
		Target:  target,
		Status:  HealthUnknown,
		Checks:  map[string]string{},
		When:    time.Now().UTC().Format(time.RFC3339),
		Details: map[string]any{
			"pid":   os.Getpid(),
			"state": s.GetState(),
		},
	}

	// 1) Process check (informational)
	if s.GetProcess() > 0 {
		report.Checks["process"] = "present"
	} else {
		report.Checks["process"] = "absent"
	}

	// 2) TCP check
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.Dial("tcp", target)
	if err != nil {
		report.Checks["tcp"] = "down"
		report.Status = HealthNotServing
		report.Error = err.Error()
		report.LatencyMs = time.Since(start).Milliseconds()
		return report, nil
	}
	_ = conn.Close()
	report.Checks["tcp"] = "up"

	// 3) gRPC health check
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithBlock())
	if s.GetTls() {
		// Opportunistic TLS; in many deployments the internal self-dial uses plaintext.
		creds := credentials.NewTLS(GetTLSConfig(s.GetKeyFile(), s.GetCertFile(), s.GetCertAuthorityTrust()))
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	gc, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		report.Checks["grpc"] = "down"
		report.Status = HealthNotServing
		report.Error = err.Error()
		report.LatencyMs = time.Since(start).Milliseconds()
		return report, nil
	}
	defer gc.Close()

	hc := healthpb.NewHealthClient(gc)
	serviceName := ""
	if opt != nil {
		serviceName = opt.ServiceName
	}

	resp, err := hc.Check(ctx, &healthpb.HealthCheckRequest{Service: serviceName})
	if err != nil {
		report.Checks["grpc"] = "error"
		report.Status = HealthNotServing
		report.Error = err.Error()
		report.LatencyMs = time.Since(start).Milliseconds()
		return report, nil
	}

	switch resp.Status {
	case healthpb.HealthCheckResponse_SERVING:
		report.Checks["grpc"] = "serving"
		report.Status = HealthServing
	case healthpb.HealthCheckResponse_NOT_SERVING:
		report.Checks["grpc"] = "not_serving"
		report.Status = HealthNotServing
	default:
		report.Checks["grpc"] = "unknown"
		report.Status = HealthUnknown
	}

	report.LatencyMs = time.Since(start).Milliseconds()
	return report, nil
}

// HealthJSON is a helper that returns pretty-printed health JSON.
func HealthJSON(s Service, opt *HealthOptions) ([]byte, error) {
	rep, err := HealthCheck(s, opt)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(rep, "", "  ")
}

// Small helper you can reuse when something is misconfigured.
var ErrHealthUninitialized = errors.New("--health called before ports/name are initialized")
