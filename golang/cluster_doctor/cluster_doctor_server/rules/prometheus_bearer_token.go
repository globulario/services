package rules

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

const defaultPrometheusConfig = "/var/lib/globular/prometheus/prometheus.yml"

type prometheusBearerTokenFile struct{}

func (prometheusBearerTokenFile) ID() string       { return "prometheus.bearer_token_file" }
func (prometheusBearerTokenFile) Category() string { return "observability" }
func (prometheusBearerTokenFile) Scope() string    { return "cluster" }

func (prometheusBearerTokenFile) Evaluate(_ *collector.Snapshot, _ Config) []Finding {
	tokenFiles := extractBearerTokenFiles(defaultPrometheusConfig)
	if len(tokenFiles) == 0 {
		return nil
	}

	var findings []Finding
	for _, path := range tokenFiles {
		if _, err := os.Stat(path); err == nil {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("prometheus.bearer_token_file", path, path),
			InvariantID: "prometheus.bearer_token_file",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "observability",
			EntityRef:   path,
			Summary:     fmt.Sprintf("Prometheus config references bearer_token_file %q but the file does not exist", path),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("prometheus", "config_parse", map[string]string{
					"config_file":       defaultPrometheusConfig,
					"bearer_token_file": path,
					"status":            "not_found",
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, fmt.Sprintf("Generate the token file: provision-minio-token.sh"), "sudo provision-minio-token.sh"),
				step(2, "Verify file permissions (must be readable by the prometheus user)", fmt.Sprintf("ls -la %s", path)),
				step(3, "Reload Prometheus after creating the file", "curl -sS -X POST http://127.0.0.1:9090/-/reload"),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// extractBearerTokenFiles parses a Prometheus YAML config and extracts
// all bearer_token_file values. Uses simple line scanning to avoid a
// full YAML dependency.
func extractBearerTokenFiles(configPath string) []string {
	f, err := os.Open(configPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	var paths []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "bearer_token_file:") {
			val := strings.TrimPrefix(line, "bearer_token_file:")
			val = strings.TrimSpace(val)
			val = strings.Trim(val, `"'`)
			if val != "" {
				paths = append(paths, val)
			}
		}
	}
	return paths
}
