package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ServiceDesc is the strongly-typed structure emitted by `<service> --describe`.
// Keep fields in sync with your services' JSON output.
type ServiceDesc struct {
	Address            string        `json:"Address"`
	AllowAllOrigins    bool          `json:"AllowAllOrigins"`
	AllowedOrigins     string        `json:"AllowedOrigins"`
	CertAuthorityTrust string        `json:"CertAuthorityTrust"`
	CertFile           string        `json:"CertFile"`
	Checksum           string        `json:"Checksum"`
	Dependencies       []string      `json:"Dependencies"`
	Description        string        `json:"Description"`
	Discoveries        []string      `json:"Discoveries"`
	Domain             string        `json:"Domain"`
	Id                 string        `json:"Id"`
	KeepAlive          bool          `json:"KeepAlive"`
	KeepUpToDate       bool          `json:"KeepUpToDate"`
	KeyFile            string        `json:"KeyFile"`
	Keywords           []string      `json:"Keywords"`
	LastError          string        `json:"LastError"`
	Mac                string        `json:"Mac"`
	ModTime            int64         `json:"ModTime"`
	Name               string        `json:"Name"`
	Path               string        `json:"Path"`
	Permissions        []interface{} `json:"Permissions"`
	Platform           string        `json:"Platform"`
	Port               int           `json:"Port"`
	Process            int           `json:"Process"`
	Proto              string        `json:"Proto"`
	Protocol           string        `json:"Protocol"`
	Proxy              int           `json:"Proxy"`
	ProxyProcess       int           `json:"ProxyProcess"`
	PublisherID        string        `json:"PublisherID"`
	Repositories       []string      `json:"Repositories"`
	State              string        `json:"State"`
	TLS                bool          `json:"TLS"`
	Version            string        `json:"Version"`
}

// DiscoverExecutables scans a root folder for service binaries named "*_server" or "*_server.exe".
func DiscoverExecutables(root string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("ServicesRoot is empty; set it in local config")
	}
	var bins []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return err
		}
		l := strings.ToLower(info.Name())
		if strings.HasSuffix(l, "_server") || strings.HasSuffix(l, "_server.exe") {
			bins = append(bins, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(bins)
	return bins, nil
}

// ResolveServiceExecutable converts a possibly-directory path into a concrete executable file.
// If `p` is a dir like ".../echo_server", it tries ".../echo_server/echo_server" then
// the first "*_server[.exe]" file inside. Ensures +x on Unix.
func ResolveServiceExecutable(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	fi, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", p, err)
	}
	if fi.Mode().IsRegular() {
		ensureExec(p, fi)
		return p, nil
	}
	if fi.IsDir() {
		base := filepath.Base(p)
		cand := filepath.Join(p, base) // same-name file inside dir
		if st, err := os.Stat(cand); err == nil && st.Mode().IsRegular() {
			ensureExec(cand, st)
			return cand, nil
		}
		// fallback: first *_server or *_server.exe in the dir
		ents, _ := os.ReadDir(p)
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			n := strings.ToLower(e.Name())
			if strings.HasSuffix(n, "_server") || strings.HasSuffix(n, "_server.exe") {
				full := filepath.Join(p, e.Name())
				if st, err := os.Stat(full); err == nil && st.Mode().IsRegular() {
					ensureExec(full, st)
					return full, nil
				}
			}
		}
		return "", fmt.Errorf("%s is a directory and no executable server was found inside", p)
	}
	return "", fmt.Errorf("unsupported file type for %s", p)
}

func ensureExec(path string, fi os.FileInfo) {
	if runtime.GOOS == "windows" {
		return // .exe doesn't need chmod
	}
	if fi.Mode()&0o111 == 0 {
		_ = os.Chmod(path, fi.Mode()|0o111)
	}
}

// FindServiceBinary walks `root` and returns a FILE whose name or parent dir contains `short`
// and ends with "*_server[.exe]". Always returns a file path (never a directory).
func FindServiceBinary(root, short string) (string, error) {
	short = strings.ToLower(strings.TrimSpace(short))
	if short == "" {
		return "", fmt.Errorf("empty short name")
	}
	var match string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || match != "" {
			return nil
		}
		name := strings.ToLower(d.Name())

		if d.IsDir() {
			if strings.HasSuffix(name, "_server") && strings.Contains(name, short) {
				// prefer same-name file inside dir
				cand := filepath.Join(p, d.Name())
				if st, err := os.Stat(cand); err == nil && st.Mode().IsRegular() {
					ensureExec(cand, st)
					match = filepath.ToSlash(cand)
					return nil
				}
				// fallback: first *_server[.exe] file in that directory
				entries, _ := os.ReadDir(p)
				for _, e := range entries {
					if e.IsDir() {
						continue
					}
					en := strings.ToLower(e.Name())
					if strings.HasSuffix(en, "_server") || strings.HasSuffix(en, "_server.exe") {
						cand = filepath.Join(p, e.Name())
						if st, err := os.Stat(cand); err == nil && st.Mode().IsRegular() {
							ensureExec(cand, st)
							match = filepath.ToSlash(cand)
							return nil
						}
					}
				}
			}
			return nil
		}

		// file case
		parent := strings.ToLower(filepath.Base(filepath.Dir(p)))
		hasSuffix := strings.HasSuffix(name, "_server") || strings.HasSuffix(name, "_server.exe")
		if hasSuffix && (strings.Contains(name, short) || strings.Contains(parent, short)) {
			if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
				ensureExec(p, st)
				match = filepath.ToSlash(p)
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if match == "" {
		return "", fmt.Errorf("no service binary found for %q under %s", short, root)
	}
	return match, nil
}

// HostOnly extracts the host from "host", "host:1234" or "[IPv6]:1234".
func HostOnly(in string) string {
	in = strings.TrimSpace(strings.Trim(in, "[]"))
	if h, _, err := splitHostPort(in); err == nil {
		return h
	}
	// best-effort strip trailing :<digits>
	if i := strings.LastIndex(in, ":"); i > 0 {
		if _, err := strconv.Atoi(in[i+1:]); err == nil {
			return in[:i]
		}
	}
	return in
}

func splitHostPort(s string) (host, port string, err error) {
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return s, "", fmt.Errorf("missing port")
	}
	return s[:i], s[i+1:], nil
}


// RunDescribe executes the specified binary with the "--describe" flag, passing the provided environment variables,
// and waits for its output up to the given timeout. It expects the command's standard output to be a JSON-encoded
// ServiceDesc object, which it unmarshals and returns. If the command fails or the output is not valid JSON,
// an error is returned containing details and any stderr output.
//
// Parameters:
//   - bin: Path to the binary to execute.
//   - timeout: Maximum duration to wait for the command to complete.
//   - env: Additional environment variables to set for the command.
//
// Returns:
//   - ServiceDesc: The unmarshaled service description from the command's output.
//   - error: An error if the command fails or the output is invalid.
func RunDescribe(bin string, timeout time.Duration, env map[string]string) (ServiceDesc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "--describe")
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		return ServiceDesc{}, fmt.Errorf("describe error: %w; stderr: %s", err, strings.TrimSpace(stderr.String()))
	}
	var d ServiceDesc
	if err := json.Unmarshal(stdout.Bytes(), &d); err != nil {
		return ServiceDesc{}, fmt.Errorf("invalid describe json from %s: %w", bin, err)
	}
	return d, nil
}
