package lifecycle

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/globulario/services/golang/oci"
)

func RenderUnit(layout Layout, spec oci.ServiceSpec, service string) (string, error) {
	layout, err := normalizeLayout(layout)
	if err != nil {
		return "", err
	}
	service = normalizeName(service)
	if !packageNamePattern.MatchString(service) {
		return "", fmt.Errorf("invalid OCI service name %q", service)
	}
	paths := derivedPaths(layout, service)
	readWrite := []string{layout.StateRoot}
	for _, mount := range spec.Spec.Mounts {
		if mount.ReadOnly && !mount.CreateIfMissing {
			continue
		}
		readWrite = append(readWrite, filepath.Clean(mount.Source))
	}
	readWrite = uniqueSorted(readWrite)

	validateArgs := []string{layout.RunnerPath, "validate", "--spec", paths.specPath, "--policy", layout.PolicyPath}
	runArgs := []string{layout.RunnerPath, "run", "--spec", paths.specPath, "--policy", layout.PolicyPath, "--socket", layout.DockerSocket, "--state-root", layout.StateRoot}
	stopTimeout := spec.Spec.Lifecycle.StopTimeoutSeconds
	if stopTimeout <= 0 {
		stopTimeout = 30
	}
	if stopTimeout > 600 {
		stopTimeout = 600
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[Unit]\nDescription=Globular OCI service %s\n", service)
	b.WriteString("After=docker.service network-online.target\nRequires=docker.service\nWants=network-online.target\n\n")
	b.WriteString("[Service]\nType=notify\nNotifyAccess=main\n")
	fmt.Fprintf(&b, "ExecStartPre=%s\n", quoteCommand(validateArgs))
	fmt.Fprintf(&b, "ExecStart=%s\n", quoteCommand(runArgs))
	b.WriteString("Restart=on-failure\nRestartSec=5\n")
	fmt.Fprintf(&b, "TimeoutStartSec=%d\n", startupTimeoutSeconds(spec))
	fmt.Fprintf(&b, "TimeoutStopSec=%d\n", stopTimeout+15)
	b.WriteString("NoNewPrivileges=true\nProtectHome=true\nProtectSystem=full\nPrivateTmp=true\nUMask=0027\n")
	if len(readWrite) > 0 {
		b.WriteString("ReadWritePaths=")
		for i, p := range readWrite {
			if i > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(strconv.Quote(p))
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n[Install]\nWantedBy=multi-user.target\n")
	return b.String(), nil
}

func quoteCommand(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = strconv.Quote(arg)
	}
	return strings.Join(parts, " ")
}

func startupTimeoutSeconds(spec oci.ServiceSpec) int {
	// The runner owns probe cadence. systemd must allow that bounded probe window
	// to complete without becoming an independent, shorter failure authority.
	probe := spec.Spec.Health.Startup
	seconds := 60
	if strings.TrimSpace(probe.Type) != "" && !strings.EqualFold(probe.Type, "none") {
		failures := probe.FailureThreshold
		if failures <= 0 {
			failures = 30
		}
		interval := probe.IntervalMillis
		if interval <= 0 {
			interval = 1000
		}
		timeout := probe.TimeoutMillis
		if timeout <= 0 {
			timeout = 2000
		}
		millis := probe.InitialDelayMillis + int64(failures)*(interval+timeout) + 30_000
		seconds = int((millis + 999) / 1000)
	}
	if seconds < 60 {
		seconds = 60
	}
	if seconds > 900 {
		seconds = 900
	}
	return seconds
}

func uniqueSorted(values []string) []string {
	set := map[string]struct{}{}
	for _, v := range values {
		v = filepath.Clean(strings.TrimSpace(v))
		if v != "." && v != "" {
			set[v] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
