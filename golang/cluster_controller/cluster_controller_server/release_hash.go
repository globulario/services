package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

// ComputeReleaseDesiredHash computes a deterministic hash for a service release.
// Includes build number so that publishing a new build of the same version
// triggers a rollout (e.g., hotfix rebuild without version bump).
func ComputeReleaseDesiredHash(publisherID, serviceName, resolvedVersion string, buildNumber int64, cfg map[string]string) string {
	var b strings.Builder
	b.WriteString(publisherID)
	b.WriteString("/")
	b.WriteString(serviceName)
	b.WriteString("=")
	b.WriteString(resolvedVersion)
	if buildNumber > 0 {
		b.WriteString("+b:")
		b.WriteString(strings.TrimSpace(fmt.Sprintf("%d", buildNumber)))
	}
	b.WriteString(";")
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// ComputeReleaseDesiredHashV3 computes a v3 hash that includes config content.
func ComputeReleaseDesiredHashV3(publisherID, serviceName, resolvedVersion, configHash string) string {
	var b strings.Builder
	b.WriteString("v3:")
	b.WriteString(publisherID)
	b.WriteString("/")
	b.WriteString(serviceName)
	b.WriteString("=")
	b.WriteString(resolvedVersion)
	if configHash != "" {
		b.WriteString("+cfg:")
		b.WriteString(configHash)
	}
	b.WriteString(";")
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// ComputeApplicationDesiredHash computes a deterministic hash for an application release.
func ComputeApplicationDesiredHash(publisherID, appName, resolvedVersion string) string {
	var b strings.Builder
	b.WriteString("app:")
	b.WriteString(publisherID)
	b.WriteString("/")
	b.WriteString(appName)
	b.WriteString("=")
	b.WriteString(resolvedVersion)
	b.WriteString(";")
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// ComputeInfrastructureDesiredHash computes a deterministic hash for an infrastructure release.
func ComputeInfrastructureDesiredHash(publisherID, component, resolvedVersion string, buildNumber int64) string {
	var b strings.Builder
	b.WriteString("infra:")
	b.WriteString(publisherID)
	b.WriteString("/")
	b.WriteString(component)
	b.WriteString("=")
	b.WriteString(resolvedVersion)
	if buildNumber > 0 {
		b.WriteString("+b:")
		b.WriteString(strings.TrimSpace(fmt.Sprintf("%d", buildNumber)))
	}
	b.WriteString(";")
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// repositoryInfo holds resolved repository connection details.
type repositoryInfo struct {
	Address string
	TLS     bool
	CAPath  string
}

// resolveRepositoryInfo returns the repository address from etcd (source of truth).
func resolveRepositoryInfo() repositoryInfo {
	// Direct gRPC connection to the repository, bypassing Envoy.
	// Envoy strips custom gRPC metadata (token, cluster_id) which breaks auth.
	// Query etcd for ALL repository instances, pick the first healthy one.
	cfgs, err := config.GetServicesConfigurationsByName("repository.PackageRepository")
	if err == nil && len(cfgs) > 0 {
		for _, cfg := range cfgs {
			port := Utility.ToInt(cfg["Port"])
			host := strings.TrimSpace(Utility.ToString(cfg["Address"]))
			// Strip port if already embedded in the address field.
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			if host == "" || host == "localhost" || host == "127.0.0.1" {
				host = config.GetRoutableIPv4()
			}
			if host != "" && port > 0 {
				addr := net.JoinHostPort(host, Utility.ToString(port))
				slog.Info("resolveRepositoryInfo: resolved", "addr", addr, "host", host, "port", port)
				return repositoryInfo{
					Address: addr,
					TLS:     true,
					CAPath:  "/var/lib/globular/pki/ca.pem",
				}
			}
		}
	}
	return repositoryInfo{} // caller must handle empty Address
}
