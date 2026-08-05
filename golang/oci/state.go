package oci

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Phase string

const (
	PhaseResolvingImage Phase = "RESOLVING_IMAGE"
	PhaseCreating       Phase = "CREATING"
	PhaseStarting       Phase = "STARTING"
	PhaseAwaitingReady  Phase = "AWAITING_READINESS"
	PhaseReady          Phase = "READY"
	PhaseStopping       Phase = "STOPPING"
	PhaseStopped        Phase = "STOPPED"
	PhaseFailed         Phase = "FAILED"
)

type ObservedState struct {
	APIVersion    string       `json:"api_version"`
	ServiceName   string       `json:"service_name"`
	Instance      string       `json:"instance,omitempty"`
	ContainerName string       `json:"container_name"`
	ContainerID   string       `json:"container_id,omitempty"`
	Image         string       `json:"image"`
	ImageID       string       `json:"image_id,omitempty"`
	SpecDigest    string       `json:"spec_digest"`
	Phase         Phase        `json:"phase"`
	Running       bool         `json:"running"`
	Ready         bool         `json:"ready"`
	ExitCode      int          `json:"exit_code,omitempty"`
	FailureClass  FailureClass `json:"failure_class,omitempty"`
	Error         string       `json:"error,omitempty"`
	StartedAt     time.Time    `json:"started_at,omitempty"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type StateStore interface {
	Write(ObservedState) error
	Read(service, instance string) (ObservedState, error)
}

type FileStateStore struct {
	Root string
}

func (s FileStateStore) Write(state ObservedState) error {
	if s.Root == "" {
		s.Root = "/var/lib/globular/oci"
	}
	state.UpdatedAt = time.Now().UTC()
	dir := filepath.Join(s.Root, normalizeName(state.ServiceName), normalizeName(state.Instance))
	if state.Instance == "" {
		dir = filepath.Join(s.Root, normalizeName(state.ServiceName), "default")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create OCI state directory: %w", err)
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal OCI observed state: %w", err)
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(dir, ".observed-*.json")
	if err != nil {
		return fmt.Errorf("create OCI state temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o640); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write OCI observed state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync OCI observed state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(dir, "observed.json")); err != nil {
		return fmt.Errorf("commit OCI observed state: %w", err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open OCI state directory for sync: %w", err)
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return fmt.Errorf("sync OCI state directory: %w", err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("close OCI state directory: %w", err)
	}
	return nil
}

func (s FileStateStore) Read(service, instance string) (ObservedState, error) {
	if s.Root == "" {
		s.Root = "/var/lib/globular/oci"
	}
	if instance == "" {
		instance = "default"
	}
	var state ObservedState
	b, err := os.ReadFile(filepath.Join(s.Root, normalizeName(service), normalizeName(instance), "observed.json"))
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return state, fmt.Errorf("decode OCI observed state: %w", err)
	}
	return state, nil
}
