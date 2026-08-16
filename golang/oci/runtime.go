package oci

import (
	"context"
	"io"
	"time"
)

type RegistryCredentials struct {
	ServerAddress string
	Username      string
	Password      string
	IdentityToken string
}

type Runtime interface {
	Ping(context.Context) error
	Capabilities(context.Context) (RuntimeCapabilities, error)
	PullImage(context.Context, string, RegistryCredentials, bool) (ImageState, error)
	InspectImage(context.Context, string) (ImageState, error)
	InspectContainer(context.Context, string) (ContainerState, error)
	CreateContainer(context.Context, ContainerCreateSpec) (ContainerState, error)
	StartContainer(context.Context, string) error
	StopContainer(context.Context, string, time.Duration) error
	WaitContainer(context.Context, string) (int, error)
	StreamLogs(context.Context, string, io.Writer, io.Writer) error
	RemoveContainer(context.Context, string, bool, bool) error
}

type RuntimeCapabilities struct {
	Provider               string            `json:"provider"`
	ServerVersion          string            `json:"server_version"`
	APIVersion             string            `json:"api_version"`
	MinAPIVersion          string            `json:"min_api_version,omitempty"`
	OperatingSystem        string            `json:"operating_system,omitempty"`
	Architecture           string            `json:"architecture,omitempty"`
	StorageDriver          string            `json:"storage_driver,omitempty"`
	DockerRootDir          string            `json:"docker_root_dir,omitempty"`
	NodeName               string            `json:"node_name,omitempty"`
	CPUs                   int               `json:"cpus,omitempty"`
	MemoryBytes            int64             `json:"memory_bytes,omitempty"`
	Runtimes               []string          `json:"runtimes,omitempty"`
	NVIDIARuntimeAvailable bool              `json:"nvidia_runtime_available"`
	SecurityOptions        []string          `json:"security_options,omitempty"`
	Metadata               map[string]string `json:"metadata,omitempty"`
}

type ImageState struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags,omitempty"`
	Digests  []string `json:"repo_digests,omitempty"`
}

type ContainerCreateSpec struct {
	Name               string
	Image              string
	Hostname           string
	Entrypoint         []string
	Command            []string
	Environment        []string
	Labels             map[string]string
	Mounts             []Mount
	Network            NetworkSpec
	Resources          ResourceSpec
	Security           SecuritySpec
	StopTimeoutSeconds int
}

type ContainerState struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	ImageID    string            `json:"image_id"`
	Labels     map[string]string `json:"labels,omitempty"`
	Status     string            `json:"status"`
	Running    bool              `json:"running"`
	ExitCode   int               `json:"exit_code"`
	Error      string            `json:"error,omitempty"`
	StartedAt  time.Time         `json:"started_at,omitempty"`
	FinishedAt time.Time         `json:"finished_at,omitempty"`
}

func (c ContainerState) Exists() bool { return c.ID != "" }
