package oci

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"
	"time"
)

type memoryStateStore struct {
	mu    sync.Mutex
	state ObservedState
}

func (s *memoryStateStore) Write(state ObservedState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	return nil
}
func (s *memoryStateStore) Read(_, _ string) (ObservedState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, nil
}

type fakeRuntime struct {
	image      ImageState
	container  ContainerState
	created    int
	started    int
	stopped    int
	removed    int
	lastCreate ContainerCreateSpec
	inspectErr error
}

func (f *fakeRuntime) Ping(context.Context) error { return nil }
func (f *fakeRuntime) Capabilities(context.Context) (RuntimeCapabilities, error) {
	return RuntimeCapabilities{Provider: "fake"}, nil
}
func (f *fakeRuntime) PullImage(context.Context, string, RegistryCredentials, bool) (ImageState, error) {
	return f.image, nil
}
func (f *fakeRuntime) InspectImage(context.Context, string) (ImageState, error) { return f.image, nil }
func (f *fakeRuntime) InspectContainer(context.Context, string) (ContainerState, error) {
	if f.inspectErr != nil {
		return ContainerState{}, f.inspectErr
	}
	return f.container, nil
}
func (f *fakeRuntime) CreateContainer(_ context.Context, spec ContainerCreateSpec) (ContainerState, error) {
	f.created++
	f.lastCreate = spec
	f.container = ContainerState{ID: "c1", Name: spec.Name, Image: spec.Image, ImageID: f.image.ID, Labels: spec.Labels, Status: "created"}
	return f.container, nil
}
func (f *fakeRuntime) StartContainer(context.Context, string) error {
	f.started++
	f.container.Running = true
	f.container.Status = "running"
	f.container.StartedAt = time.Unix(1, 0).UTC()
	return nil
}
func (f *fakeRuntime) StopContainer(context.Context, string, time.Duration) error {
	f.stopped++
	f.container.Running = false
	f.container.Status = "exited"
	return nil
}
func (f *fakeRuntime) WaitContainer(context.Context, string) (int, error)             { return 0, nil }
func (f *fakeRuntime) StreamLogs(context.Context, string, io.Writer, io.Writer) error { return nil }
func (f *fakeRuntime) RemoveContainer(context.Context, string, bool, bool) error {
	f.removed++
	f.container = ContainerState{}
	return nil
}

func fakeImage(spec ServiceSpec) ImageState {
	return ImageState{ID: "sha256:image", Digests: []string{spec.CanonicalImageReference()}}
}

func TestReconcilerCreatesStartsAndPersistsReady(t *testing.T) {
	spec := validSpec()
	policy := DefaultPolicy()
	policy.AllowedRegistryHosts = []string{"registry.example.com"}
	fake := &fakeRuntime{image: fakeImage(spec)}
	store := &memoryStateStore{}
	r := Reconciler{Runtime: fake, Policy: policy, State: store}
	state, err := r.Apply(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	if fake.created != 1 || fake.started != 1 {
		t.Fatalf("create/start = %d/%d, want 1/1", fake.created, fake.started)
	}
	if state.Phase != PhaseReady || !state.Ready || !state.Running {
		t.Fatalf("state = %+v", state)
	}
	if fake.lastCreate.Labels[LabelManaged] != "true" || fake.lastCreate.Image != spec.CanonicalImageReference() {
		t.Fatalf("create spec does not bind managed exact image: %+v", fake.lastCreate)
	}
}

func TestReconcilerIsIdempotentForMatchingRunningContainer(t *testing.T) {
	spec := validSpec()
	policy := DefaultPolicy()
	policy.AllowedRegistryHosts = []string{"registry.example.com"}
	image := fakeImage(spec)
	digest, _ := spec.SpecDigest()
	fake := &fakeRuntime{image: image, container: ContainerState{
		ID: "existing", Name: spec.ContainerName(), ImageID: image.ID, Running: true,
		Labels: managedLabels(spec, digest),
	}}
	r := Reconciler{Runtime: fake, Policy: policy}
	if _, err := r.Apply(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if fake.created != 0 || fake.started != 0 || fake.removed != 0 {
		t.Fatalf("idempotent apply mutated runtime: create=%d start=%d remove=%d", fake.created, fake.started, fake.removed)
	}
}

func TestReconcilerReplacesManagedDrift(t *testing.T) {
	spec := validSpec()
	policy := DefaultPolicy()
	policy.AllowedRegistryHosts = []string{"registry.example.com"}
	image := fakeImage(spec)
	digest, _ := spec.SpecDigest()
	labels := managedLabels(spec, digest)
	labels[LabelSpecDigest] = "sha256:old"
	fake := &fakeRuntime{image: image, container: ContainerState{
		ID: "old", Name: spec.ContainerName(), ImageID: "old-image", Running: true, Labels: labels,
	}}
	r := Reconciler{Runtime: fake, Policy: policy}
	if _, err := r.Apply(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if fake.stopped != 1 || fake.removed != 1 || fake.created != 1 || fake.started != 1 {
		t.Fatalf("replacement counts stop=%d remove=%d create=%d start=%d", fake.stopped, fake.removed, fake.created, fake.started)
	}
}

func TestReconcilerRefusesUnmanagedNameCollision(t *testing.T) {
	spec := validSpec()
	policy := DefaultPolicy()
	policy.AllowedRegistryHosts = []string{"registry.example.com"}
	fake := &fakeRuntime{image: fakeImage(spec), container: ContainerState{ID: "foreign", Name: spec.ContainerName(), Running: true}}
	r := Reconciler{Runtime: fake, Policy: policy}
	_, err := r.Apply(context.Background(), spec)
	if FailureClassOf(err) != FailureContainerConflict {
		t.Fatalf("error = %v, class=%s", err, FailureClassOf(err))
	}
	if fake.stopped != 0 || fake.removed != 0 {
		t.Fatal("unmanaged container was mutated")
	}
}

func TestReconcilerRejectsImageDigestWithoutRuntimeAttestation(t *testing.T) {
	spec := validSpec()
	policy := DefaultPolicy()
	policy.AllowedRegistryHosts = []string{"registry.example.com"}
	fake := &fakeRuntime{image: ImageState{ID: "image", Digests: []string{"registry.example.com/team/demo@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}}
	r := Reconciler{Runtime: fake, Policy: policy}
	_, err := r.Apply(context.Background(), spec)
	if FailureClassOf(err) != FailureImageIdentity {
		t.Fatalf("error = %v, class=%s", err, FailureClassOf(err))
	}
}

func TestBuildCreateSpecLoadsEnvironmentFromOwnerOnlyFile(t *testing.T) {
	spec := validSpec()
	secret := t.TempDir() + "/secret"
	if err := os.WriteFile(secret, []byte("very-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec.Spec.Environment = []Environment{{Name: "TOKEN", ValueFile: secret}}
	created, err := buildCreateSpec(spec, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(created.Environment) != 1 || created.Environment[0] != "TOKEN=very-secret" {
		t.Fatalf("environment = %#v", created.Environment)
	}
}

func TestRunStopsContainerWhenContextCancelled(t *testing.T) {
	spec := validSpec()
	spec.Spec.Lifecycle.RemoveOnStop = true
	policy := DefaultPolicy()
	policy.AllowedRegistryHosts = []string{"registry.example.com"}
	fake := &blockingRuntime{fakeRuntime: fakeRuntime{image: fakeImage(spec)}, wait: make(chan struct{})}
	r := Reconciler{Runtime: fake, Policy: policy}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := r.Run(ctx, spec, &bytes.Buffer{}, &bytes.Buffer{}); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if fake.stopped == 0 || fake.removed == 0 {
		t.Fatalf("cancel did not stop/remove: stopped=%d removed=%d", fake.stopped, fake.removed)
	}
}

type blockingRuntime struct {
	fakeRuntime
	wait chan struct{}
}

func (f *blockingRuntime) WaitContainer(ctx context.Context, _ string) (int, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-f.wait:
		return 0, nil
	}
}
