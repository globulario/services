package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

var (
	scyllaManagerConfigPaths = []string{
		"/var/lib/globular/scylla-manager/scylla-manager.yaml",
		"/etc/scylla-manager/scylla-manager.yaml",
	}
	scyllaAgentConfigPaths = []string{
		"/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml",
		"/etc/scylla-manager-agent/scylla-manager-agent.yaml",
	}
	execCommand            = exec.Command
	execLookPath           = exec.LookPath
	dialTimeout            = net.DialTimeout
	nativeScyllaDBDetector = detectNativeScyllaDB
)

type scyllaManagerEndpoint struct {
	Path   string
	URL    string
	Scheme string
}

type scyllaAgentConfig struct {
	Path      string
	AuthToken string
	HTTPSAddr string
	HTTPSPort string
	Owner     string
	Group     string
	Mode      string
	ReadErr   error
	StatErr   error
}

func fileOwnershipDiagnostic(path string) (owner, group, mode string, statErr error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", "", "", err
	}
	mode = fmt.Sprintf("0%o", info.Mode().Perm())
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", mode, errors.New("missing stat owner info")
	}
	owner = fmt.Sprintf("%d", st.Uid)
	group = fmt.Sprintf("%d", st.Gid)
	return owner, group, mode, nil
}

func readScyllaAgentConfig() scyllaAgentConfig {
	var lastErr error
	for _, path := range scyllaAgentConfigPaths {
		cfg := scyllaAgentConfig{Path: path}
		cfg.Owner, cfg.Group, cfg.Mode, cfg.StatErr = fileOwnershipDiagnostic(path)
		data, err := os.ReadFile(path)
		if err != nil {
			cfg.ReadErr = err
			lastErr = err
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return cfg
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.HasPrefix(line, "auth_token:") {
				cfg.AuthToken = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "auth_token:")), "\"'")
				continue
			}
			if strings.HasPrefix(line, "https:") {
				cfg.HTTPSAddr = strings.TrimSpace(strings.TrimPrefix(line, "https:"))
				if _, portStr, err := net.SplitHostPort(cfg.HTTPSAddr); err == nil {
					cfg.HTTPSPort = portStr
				}
			}
		}
		return cfg
	}
	return scyllaAgentConfig{
		Path:    scyllaAgentConfigPaths[0],
		ReadErr: lastErr,
	}
}

func readScyllaManagerEndpoint() scyllaManagerEndpoint {
	var data []byte
	var path string
	for _, p := range scyllaManagerConfigPaths {
		var err error
		data, err = os.ReadFile(p)
		if err == nil {
			path = p
			break
		}
	}
	if len(data) == 0 {
		return scyllaManagerEndpoint{}
	}
	var httpURL string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https:") {
			addr := strings.TrimSpace(strings.TrimPrefix(line, "https:"))
			if addr != "" {
				return scyllaManagerEndpoint{
					Path:   path,
					URL:    "https://" + resolveWildcardAddr(addr),
					Scheme: "https",
				}
			}
		}
		if strings.HasPrefix(line, "http:") && !strings.HasPrefix(line, "https:") {
			addr := strings.TrimSpace(strings.TrimPrefix(line, "http:"))
			if addr != "" {
				httpURL = "http://" + resolveWildcardAddr(addr)
			}
		}
	}
	if httpURL == "" {
		return scyllaManagerEndpoint{Path: path}
	}
	return scyllaManagerEndpoint{
		Path:   path,
		URL:    httpURL,
		Scheme: "http",
	}
}

func endpointReachable(rawURL string) bool {
	if strings.TrimSpace(rawURL) == "" {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return false
	}
	conn, err := dialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func addressReachable(hostport string) bool {
	if strings.TrimSpace(hostport) == "" {
		return false
	}
	conn, err := dialTimeout("tcp", hostport, 2*time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func realRegisteredClusters(clusters []string) []string {
	real := make([]string, 0, len(clusters))
	for _, c := range clusters {
		if strings.HasPrefix(c, "native:") || strings.HasPrefix(c, "scylla_host:") {
			continue
		}
		real = append(real, c)
	}
	return real
}
