package oci

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	APIVersionV1Alpha1 = "globular.io/oci/v1alpha1"
	KindOCIService     = "OCIService"

	NetworkHost   = "host"
	NetworkBridge = "bridge"

	PullIfNotPresent = "if-not-present"
	PullAlways       = "always"
)

// ServiceSpec is the durable, provider-neutral declaration for one persistent
// OCI-backed Globular service. Docker is the first runtime provider, but the
// contract deliberately contains no Docker CLI syntax or opaque command line.
type ServiceSpec struct {
	APIVersion string         `json:"api_version"`
	Kind       string         `json:"kind"`
	Metadata   Metadata       `json:"metadata"`
	Spec       OCIServiceSpec `json:"spec"`
}

type Metadata struct {
	Name     string            `json:"name"`
	Instance string            `json:"instance,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

type OCIServiceSpec struct {
	Image       ImageSpec     `json:"image"`
	Entrypoint  []string      `json:"entrypoint,omitempty"`
	Command     []string      `json:"command,omitempty"`
	Environment []Environment `json:"environment,omitempty"`
	Mounts      []Mount       `json:"mounts,omitempty"`
	Network     NetworkSpec   `json:"network,omitempty"`
	Resources   ResourceSpec  `json:"resources,omitempty"`
	Security    SecuritySpec  `json:"security,omitempty"`
	Health      HealthSpec    `json:"health,omitempty"`
	Lifecycle   LifecycleSpec `json:"lifecycle,omitempty"`
}

type ImageSpec struct {
	Repository   string       `json:"repository"`
	Tag          string       `json:"tag,omitempty"`
	Digest       string       `json:"digest"`
	PullPolicy   string       `json:"pull_policy,omitempty"`
	RegistryAuth RegistryAuth `json:"registry_auth,omitempty"`
}

// RegistryAuth never permits inline passwords or identity tokens. Secret bytes
// are projected from root-owned files only at pull time and never persisted in
// observed state or command arguments.
type RegistryAuth struct {
	ServerAddress     string `json:"server_address,omitempty"`
	Username          string `json:"username,omitempty"`
	PasswordFile      string `json:"password_file,omitempty"`
	IdentityTokenFile string `json:"identity_token_file,omitempty"`
}

type Environment struct {
	Name      string `json:"name"`
	Value     string `json:"value,omitempty"`
	ValueFile string `json:"value_file,omitempty"`
}

type Mount struct {
	Source          string `json:"source"`
	Target          string `json:"target"`
	ReadOnly        bool   `json:"read_only,omitempty"`
	CreateIfMissing bool   `json:"create_if_missing,omitempty"`
}

type NetworkSpec struct {
	Mode  string `json:"mode,omitempty"`
	Ports []Port `json:"ports,omitempty"`
}

type Port struct {
	ContainerPort uint16 `json:"container_port"`
	HostPort      uint16 `json:"host_port,omitempty"`
	HostIP        string `json:"host_ip,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

type ResourceSpec struct {
	MemoryBytes uint64     `json:"memory_bytes,omitempty"`
	NanoCPUs    int64      `json:"nano_cpus,omitempty"`
	ShmBytes    uint64     `json:"shm_bytes,omitempty"`
	GPU         GPURequest `json:"gpu,omitempty"`
}

type GPURequest struct {
	Count     int      `json:"count,omitempty"`
	DeviceIDs []string `json:"device_ids,omitempty"`
	Driver    string   `json:"driver,omitempty"`
}

type SecuritySpec struct {
	User                   string   `json:"user,omitempty"`
	Group                  string   `json:"group,omitempty"`
	ReadOnlyRootFilesystem bool     `json:"read_only_root_filesystem,omitempty"`
	NoNewPrivileges        bool     `json:"no_new_privileges,omitempty"`
	DropCapabilities       []string `json:"drop_capabilities,omitempty"`
	AddCapabilities        []string `json:"add_capabilities,omitempty"`
	Privileged             bool     `json:"privileged,omitempty"`
}

type HealthSpec struct {
	Startup   Probe `json:"startup,omitempty"`
	Readiness Probe `json:"readiness,omitempty"`
	Liveness  Probe `json:"liveness,omitempty"`
}

type Probe struct {
	Type               string `json:"type,omitempty"` // none | tcp | http
	Address            string `json:"address,omitempty"`
	Port               uint16 `json:"port,omitempty"`
	Scheme             string `json:"scheme,omitempty"`
	Path               string `json:"path,omitempty"`
	ExpectedStatusMin  int    `json:"expected_status_min,omitempty"`
	ExpectedStatusMax  int    `json:"expected_status_max,omitempty"`
	InitialDelayMillis int64  `json:"initial_delay_millis,omitempty"`
	IntervalMillis     int64  `json:"interval_millis,omitempty"`
	TimeoutMillis      int64  `json:"timeout_millis,omitempty"`
	FailureThreshold   int    `json:"failure_threshold,omitempty"`
}

type LifecycleSpec struct {
	StopTimeoutSeconds    int  `json:"stop_timeout_seconds,omitempty"`
	RemoveOnStop          bool `json:"remove_on_stop,omitempty"`
	RemoveReplaced        bool `json:"remove_replaced,omitempty"`
	RetainFailedContainer bool `json:"retain_failed_container,omitempty"`
}

// Policy is node-owned admission policy. A workload cannot enlarge its own
// authority by declaring additional host roots or privileged execution.
type Policy struct {
	AllowedMountRoots      []string `json:"allowed_mount_roots,omitempty"`
	AllowedWritableRoots   []string `json:"allowed_writable_roots,omitempty"`
	AllowedSecretRoots     []string `json:"allowed_secret_roots,omitempty"`
	AllowedRegistryHosts   []string `json:"allowed_registry_hosts,omitempty"`
	AllowHostNetwork       bool     `json:"allow_host_network,omitempty"`
	AllowPrivileged        bool     `json:"allow_privileged,omitempty"`
	AllowDockerSocketMount bool     `json:"allow_docker_socket_mount,omitempty"`
	AllowAddedCapabilities bool     `json:"allow_added_capabilities,omitempty"`
	MaxGPUCount            int      `json:"max_gpu_count,omitempty"`
	MaxMemoryBytes         uint64   `json:"max_memory_bytes,omitempty"`
	MaxNanoCPUs            int64    `json:"max_nano_cpus,omitempty"`
}

func DefaultPolicy() Policy {
	return Policy{
		AllowedMountRoots: []string{
			"/var/lib/globular",
			"/etc/globular",
			"/run/globular",
		},
		AllowedWritableRoots: []string{
			"/var/lib/globular/oci",
			"/var/lib/globular/data",
			"/var/lib/globular/cache",
		},
		AllowedSecretRoots: []string{
			"/etc/globular/oci/secrets",
			"/run/globular/secrets",
		},
		AllowHostNetwork:       false,
		AllowPrivileged:        false,
		AllowDockerSocketMount: false,
		AllowAddedCapabilities: false,
		MaxGPUCount:            8,
	}
}

func LoadSpec(path string) (ServiceSpec, error) {
	var spec ServiceSpec
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return spec, fmt.Errorf("read OCI service spec: %w", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&spec); err != nil {
		return spec, fmt.Errorf("decode OCI service spec: %w", err)
	}
	return spec, nil
}

func LoadPolicy(path string) (Policy, error) {
	if strings.TrimSpace(path) == "" {
		return DefaultPolicy(), nil
	}
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Policy{}, fmt.Errorf("read OCI policy: %w", err)
	}
	p := DefaultPolicy()
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Policy{}, fmt.Errorf("decode OCI policy: %w", err)
	}
	return p, nil
}

func (s ServiceSpec) CanonicalImageReference() string {
	return strings.TrimSpace(s.Spec.Image.Repository) + "@" + strings.ToLower(strings.TrimSpace(s.Spec.Image.Digest))
}

func (s ServiceSpec) ContainerName() string {
	name := normalizeName(s.Metadata.Name)
	instance := normalizeName(s.Metadata.Instance)
	if instance == "" || instance == "default" {
		return "globular-" + name
	}
	return "globular-" + name + "-" + instance
}

func (s ServiceSpec) SpecDigest() (string, error) {
	canonical := canonicalSpec(s)
	b, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal canonical OCI spec: %w", err)
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// canonicalSpec removes map iteration ambiguity before hashing. encoding/json
// already sorts string map keys, but copying maps and sorting set-like slices
// makes the intent explicit and keeps equivalent policy-free specs stable.
func canonicalSpec(s ServiceSpec) ServiceSpec {
	out := s
	out.Metadata.Labels = cloneMap(s.Metadata.Labels)
	out.Spec.Entrypoint = append([]string(nil), s.Spec.Entrypoint...)
	out.Spec.Command = append([]string(nil), s.Spec.Command...)
	out.Spec.Environment = append([]Environment(nil), s.Spec.Environment...)
	out.Spec.Mounts = append([]Mount(nil), s.Spec.Mounts...)
	out.Spec.Network.Ports = append([]Port(nil), s.Spec.Network.Ports...)
	out.Spec.Resources.GPU.DeviceIDs = append([]string(nil), s.Spec.Resources.GPU.DeviceIDs...)
	out.Spec.Security.DropCapabilities = append([]string(nil), s.Spec.Security.DropCapabilities...)
	out.Spec.Security.AddCapabilities = append([]string(nil), s.Spec.Security.AddCapabilities...)
	sort.Strings(out.Spec.Resources.GPU.DeviceIDs)
	sort.Strings(out.Spec.Security.DropCapabilities)
	sort.Strings(out.Spec.Security.AddCapabilities)
	return out
}

func cloneMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func normalizeName(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	v = strings.ReplaceAll(v, "_", "-")
	return v
}
