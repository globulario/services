package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

// ComputeReleaseDesiredHash computes a deterministic hash for a service release.
func ComputeReleaseDesiredHash(publisherID, serviceName, resolvedVersion string, cfg map[string]string) string {
	var b strings.Builder
	b.WriteString(publisherID)
	b.WriteString("/")
	b.WriteString(serviceName)
	b.WriteString("=")
	b.WriteString(resolvedVersion)
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
func ComputeInfrastructureDesiredHash(publisherID, component, resolvedVersion string) string {
	var b strings.Builder
	b.WriteString("infra:")
	b.WriteString(publisherID)
	b.WriteString("/")
	b.WriteString(component)
	b.WriteString("=")
	b.WriteString(resolvedVersion)
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

// resolveRepositoryInfo returns the repository address.
func resolveRepositoryInfo() repositoryInfo {
	if lanAddr, _ := config.GetAddress(); lanAddr != "" {
		host := lanAddr
		if h, _, err := net.SplitHostPort(lanAddr); err == nil {
			host = h
		}
		return repositoryInfo{
			Address: net.JoinHostPort(host, "443"),
			TLS:     true,
			CAPath:  "/var/lib/globular/pki/ca.pem",
		}
	}
	cfg, err := config.GetServiceConfigurationById("repository.PackageRepository")
	if err != nil || cfg == nil {
		return repositoryInfo{Address: makeRoutable("localhost:10008")}
	}
	port := Utility.ToInt(cfg["Port"])
	host := strings.TrimSpace(Utility.ToString(cfg["Address"]))
	if host == "" {
		host = "localhost"
	}
	var addr string
	if strings.Contains(host, ":") {
		addr = host
	} else if port <= 0 {
		addr = makeRoutable("localhost:10008")
	} else {
		addr = makeRoutable(net.JoinHostPort(host, Utility.ToString(port)))
	}
	return repositoryInfo{Address: addr}
}

func makeRoutable(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return addr
}
