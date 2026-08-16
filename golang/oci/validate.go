package oci

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	namePattern   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,61}[a-z0-9])?$`)
	envPattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	digestPattern = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)
)

func Validate(spec ServiceSpec, policy Policy) error {
	var problems []string
	if spec.APIVersion != APIVersionV1Alpha1 {
		problems = append(problems, fmt.Sprintf("api_version must be %q", APIVersionV1Alpha1))
	}
	if spec.Kind != KindOCIService {
		problems = append(problems, fmt.Sprintf("kind must be %q", KindOCIService))
	}
	name := normalizeName(spec.Metadata.Name)
	if !namePattern.MatchString(name) {
		problems = append(problems, "metadata.name must be a lowercase DNS-like name")
	}
	if instance := normalizeName(spec.Metadata.Instance); instance != "" && !namePattern.MatchString(instance) {
		problems = append(problems, "metadata.instance must be a lowercase DNS-like name")
	}
	for key := range spec.Metadata.Labels {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(key)), "io.globular.") {
			problems = append(problems, fmt.Sprintf("metadata label %q uses reserved io.globular.* authority namespace", key))
		}
	}
	if strings.TrimSpace(spec.Spec.Image.Repository) == "" {
		problems = append(problems, "spec.image.repository is required")
	}
	if !digestPattern.MatchString(strings.TrimSpace(spec.Spec.Image.Digest)) {
		problems = append(problems, "spec.image.digest must be an immutable sha256 digest")
	}
	pull := defaultString(spec.Spec.Image.PullPolicy, PullIfNotPresent)
	if pull != PullIfNotPresent && pull != PullAlways {
		problems = append(problems, "spec.image.pull_policy must be if-not-present or always")
	}
	if tag := strings.TrimSpace(spec.Spec.Image.Tag); strings.EqualFold(tag, "latest") && spec.Spec.Image.Digest == "" {
		problems = append(problems, "mutable latest tag cannot be execution authority")
	}
	if host := registryHost(spec.Spec.Image.Repository); len(policy.AllowedRegistryHosts) > 0 && !containsFold(policy.AllowedRegistryHosts, host) {
		problems = append(problems, fmt.Sprintf("registry host %q is not admitted by node policy", host))
	}
	if a := spec.Spec.Image.RegistryAuth; a.PasswordFile != "" && a.IdentityTokenFile != "" {
		problems = append(problems, "registry_auth password_file and identity_token_file are mutually exclusive")
	}
	for _, p := range []string{spec.Spec.Image.RegistryAuth.PasswordFile, spec.Spec.Image.RegistryAuth.IdentityTokenFile} {
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			problems = append(problems, "registry secret file paths must be absolute")
			continue
		}
		if !pathWithinAny(p, policy.AllowedSecretRoots) {
			problems = append(problems, fmt.Sprintf("registry secret path %q is outside allowed secret roots", p))
		}
	}

	seenEnv := map[string]struct{}{}
	for i, env := range spec.Spec.Environment {
		if !envPattern.MatchString(env.Name) {
			problems = append(problems, fmt.Sprintf("environment[%d].name is invalid", i))
		}
		if _, ok := seenEnv[env.Name]; ok {
			problems = append(problems, fmt.Sprintf("environment variable %q is declared more than once", env.Name))
		}
		seenEnv[env.Name] = struct{}{}
		if env.Value != "" && env.ValueFile != "" {
			problems = append(problems, fmt.Sprintf("environment[%d] value and value_file are mutually exclusive", i))
		}
		if env.ValueFile != "" {
			if !filepath.IsAbs(env.ValueFile) {
				problems = append(problems, fmt.Sprintf("environment[%d].value_file must be absolute", i))
			} else if !pathWithinAny(env.ValueFile, policy.AllowedSecretRoots) {
				problems = append(problems, fmt.Sprintf("environment[%d].value_file %q is outside allowed secret roots", i, env.ValueFile))
			}
		}
	}

	seenTargets := map[string]struct{}{}
	for i, m := range spec.Spec.Mounts {
		source := filepath.Clean(m.Source)
		target := filepath.Clean(m.Target)
		if !filepath.IsAbs(source) || !filepath.IsAbs(target) {
			problems = append(problems, fmt.Sprintf("mounts[%d] source and target must be absolute", i))
			continue
		}
		if source == "/" || target == "/" {
			problems = append(problems, fmt.Sprintf("mounts[%d] may not mount a host or container root", i))
		}
		if isDockerSocket(source) || isDockerSocket(target) {
			if !policy.AllowDockerSocketMount {
				problems = append(problems, fmt.Sprintf("mounts[%d] Docker socket access is forbidden", i))
			}
		}
		if !pathWithinAny(source, policy.AllowedMountRoots) {
			problems = append(problems, fmt.Sprintf("mounts[%d] source %q is outside allowed host roots", i, source))
		}
		if !m.ReadOnly && !pathWithinAny(source, policy.AllowedWritableRoots) {
			problems = append(problems, fmt.Sprintf("mounts[%d] writable source %q is outside allowed writable roots", i, source))
		}
		if m.CreateIfMissing && m.ReadOnly {
			problems = append(problems, fmt.Sprintf("mounts[%d] create_if_missing cannot be used with read_only", i))
		}
		if _, ok := seenTargets[target]; ok {
			problems = append(problems, fmt.Sprintf("mount target %q is declared more than once", target))
		}
		seenTargets[target] = struct{}{}
	}

	mode := defaultString(spec.Spec.Network.Mode, NetworkBridge)
	if mode != NetworkBridge && mode != NetworkHost {
		problems = append(problems, "network.mode must be bridge or host")
	}
	if mode == NetworkHost && !policy.AllowHostNetwork {
		problems = append(problems, "host network is not admitted by node policy")
	}
	seenHostPorts := map[string]struct{}{}
	for i, p := range spec.Spec.Network.Ports {
		proto := strings.ToLower(defaultString(p.Protocol, "tcp"))
		if proto != "tcp" && proto != "udp" {
			problems = append(problems, fmt.Sprintf("ports[%d].protocol must be tcp or udp", i))
		}
		if p.ContainerPort == 0 {
			problems = append(problems, fmt.Sprintf("ports[%d].container_port is required", i))
		}
		if mode == NetworkHost && p.HostPort != 0 && p.HostPort != p.ContainerPort {
			problems = append(problems, fmt.Sprintf("ports[%d] host networking cannot remap ports", i))
		}
		if p.HostIP != "" && net.ParseIP(p.HostIP) == nil {
			problems = append(problems, fmt.Sprintf("ports[%d].host_ip must be an IP address", i))
		}
		if p.HostPort != 0 {
			key := fmt.Sprintf("%s/%d/%s", p.HostIP, p.HostPort, proto)
			if _, ok := seenHostPorts[key]; ok {
				problems = append(problems, fmt.Sprintf("host port reservation %q is duplicated", key))
			}
			seenHostPorts[key] = struct{}{}
		}
	}

	gpu := spec.Spec.Resources.GPU
	if gpu.Count < 0 {
		problems = append(problems, "resources.gpu.count cannot be negative")
	}
	if gpu.Count > 0 && len(gpu.DeviceIDs) > 0 {
		problems = append(problems, "resources.gpu.count and device_ids are mutually exclusive")
	}
	requestedGPU := gpu.Count
	if len(gpu.DeviceIDs) > requestedGPU {
		requestedGPU = len(gpu.DeviceIDs)
	}
	if policy.MaxGPUCount > 0 && requestedGPU > policy.MaxGPUCount {
		problems = append(problems, fmt.Sprintf("requested GPU count %d exceeds node policy maximum %d", requestedGPU, policy.MaxGPUCount))
	}
	if policy.MaxMemoryBytes > 0 && spec.Spec.Resources.MemoryBytes > policy.MaxMemoryBytes {
		problems = append(problems, "requested memory exceeds node policy maximum")
	}
	if policy.MaxNanoCPUs > 0 && spec.Spec.Resources.NanoCPUs > policy.MaxNanoCPUs {
		problems = append(problems, "requested CPU quota exceeds node policy maximum")
	}

	if spec.Spec.Security.Privileged && !policy.AllowPrivileged {
		problems = append(problems, "privileged containers are forbidden by node policy")
	}
	if len(spec.Spec.Security.AddCapabilities) > 0 && !policy.AllowAddedCapabilities {
		problems = append(problems, "added Linux capabilities are forbidden by node policy")
	}
	for _, probe := range []struct {
		name string
		p    Probe
	}{{"startup", spec.Spec.Health.Startup}, {"readiness", spec.Spec.Health.Readiness}, {"liveness", spec.Spec.Health.Liveness}} {
		if err := validateProbe(probe.p); err != nil {
			problems = append(problems, probe.name+" probe: "+err.Error())
		}
	}
	if spec.Spec.Lifecycle.StopTimeoutSeconds < 0 || spec.Spec.Lifecycle.StopTimeoutSeconds > 600 {
		problems = append(problems, "lifecycle.stop_timeout_seconds must be between 0 and 600")
	}

	if len(problems) > 0 {
		sort.Strings(problems)
		return Wrap(FailureInvalidSpec, "validate", fmt.Errorf("%s", strings.Join(problems, "; ")))
	}
	return nil
}

func validateProbe(p Probe) error {
	t := strings.ToLower(strings.TrimSpace(p.Type))
	if t == "" || t == "none" {
		return nil
	}
	if t != "tcp" && t != "http" {
		return fmt.Errorf("type must be none, tcp, or http")
	}
	if strings.TrimSpace(p.Address) == "" {
		return fmt.Errorf("address is required for an enabled probe")
	}
	if p.Port == 0 {
		return fmt.Errorf("port is required")
	}
	if t == "http" {
		scheme := strings.ToLower(defaultString(p.Scheme, "http"))
		if scheme != "http" && scheme != "https" {
			return fmt.Errorf("scheme must be http or https")
		}
		if p.Path != "" && !strings.HasPrefix(p.Path, "/") {
			return fmt.Errorf("path must begin with /")
		}
	}
	for name, v := range map[string]int64{
		"initial_delay_millis": p.InitialDelayMillis,
		"interval_millis":      p.IntervalMillis,
		"timeout_millis":       p.TimeoutMillis,
	} {
		if v < 0 {
			return fmt.Errorf("%s cannot be negative", name)
		}
	}
	if p.FailureThreshold < 0 {
		return fmt.Errorf("failure_threshold cannot be negative")
	}
	return nil
}

func registryHost(repository string) string {
	repository = strings.TrimSpace(repository)
	if strings.Contains(repository, "://") {
		if u, err := url.Parse(repository); err == nil {
			return strings.ToLower(u.Hostname())
		}
	}
	first := strings.Split(repository, "/")[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return strings.ToLower(strings.Split(first, ":")[0])
	}
	return "docker.io"
}

func pathWithinAny(path string, roots []string) bool {
	resolvedPath, err := resolvePolicyPath(path)
	if err != nil {
		return false
	}
	for _, root := range roots {
		resolvedRoot, err := resolvePolicyPath(root)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(resolvedRoot, resolvedPath)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// resolvePolicyPath resolves every existing symlink-bearing ancestor and then
// appends the still-missing suffix. This prevents a lexically admitted path
// under /var/lib/globular from escaping through a symlinked parent.
func resolvePolicyPath(path string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	current := abs
	var suffix []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return abs, nil
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func isDockerSocket(path string) bool {
	p := filepath.Clean(path)
	return p == "/var/run/docker.sock" || p == "/run/docker.sock"
}

func containsFold(values []string, target string) bool {
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func defaultString(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return strings.TrimSpace(v)
}
