package oci

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"
)

const (
	LabelManaged     = "io.globular.managed"
	LabelService     = "io.globular.service"
	LabelInstance    = "io.globular.instance"
	LabelSpecDigest  = "io.globular.spec-digest"
	LabelImageDigest = "io.globular.image-digest"
	LabelImageRef    = "io.globular.image-reference"
	LabelRuntime     = "io.globular.runtime"
)

// Reconciler is the node-local authority for one OCI-backed service instance.
// The caller owns retry cadence and process supervision. Reconciler owns only
// the deterministic transition from an admitted ServiceSpec to Docker state.
type Reconciler struct {
	Runtime Runtime
	Policy  Policy
	State   StateStore
	Now     func() time.Time
}

func (r *Reconciler) Apply(ctx context.Context, spec ServiceSpec) (ObservedState, error) {
	if r.Runtime == nil {
		return ObservedState{}, Wrap(FailureRuntimeUnavailable, "apply", errors.New("OCI runtime is nil"))
	}
	if err := Validate(spec, r.Policy); err != nil {
		return r.fail(spec, ObservedState{}, FailureOf(err), err)
	}

	digest, err := spec.SpecDigest()
	if err != nil {
		return r.fail(spec, ObservedState{}, FailureInvalidSpec, err)
	}
	state := r.baseState(spec, digest)
	state.Phase = PhaseResolvingImage
	_ = r.writeState(state)

	if err := r.Runtime.Ping(ctx); err != nil {
		return r.fail(spec, state, FailureRuntimeUnavailable, Wrap(FailureRuntimeUnavailable, "ping runtime", err))
	}
	creds, err := loadRegistryCredentials(spec.Spec.Image.RegistryAuth)
	if err != nil {
		return r.fail(spec, state, FailureRegistryAuth, err)
	}
	forcePull := defaultString(spec.Spec.Image.PullPolicy, PullIfNotPresent) == PullAlways
	image, err := r.Runtime.PullImage(ctx, spec.CanonicalImageReference(), creds, forcePull)
	if err != nil {
		return r.fail(spec, state, FailureImagePull, Wrap(FailureImagePull, "pull image", err))
	}
	if err := verifyImageIdentity(image, spec.CanonicalImageReference(), spec.Spec.Image.Digest); err != nil {
		return r.fail(spec, state, FailureImageIdentity, err)
	}
	state.ImageID = image.ID

	current, err := r.Runtime.InspectContainer(ctx, spec.ContainerName())
	if err != nil {
		return r.fail(spec, state, FailureStateInspection, Wrap(FailureStateInspection, "inspect container", err))
	}
	labels := managedLabels(spec, digest)
	if current.Exists() {
		if err := verifyManagedOwnership(current, spec); err != nil {
			return r.fail(spec, state, FailureContainerConflict, err)
		}
		if !containerMatches(current, labels, image.ID) {
			if err := r.replace(ctx, spec, current); err != nil {
				return r.fail(spec, state, FailureOf(err), err)
			}
			current = ContainerState{}
		}
	}

	if !current.Exists() {
		if err := ensureMountSources(spec.Spec.Mounts); err != nil {
			return r.fail(spec, state, FailureInvalidSpec, err)
		}
		state.Phase = PhaseCreating
		_ = r.writeState(state)
		createSpec, err := buildCreateSpec(spec, labels)
		if err != nil {
			return r.fail(spec, state, FailureInvalidSpec, err)
		}
		current, err = r.Runtime.CreateContainer(ctx, createSpec)
		if err != nil {
			return r.fail(spec, state, FailureContainerCreate, Wrap(FailureContainerCreate, "create container", err))
		}
	}
	state.ContainerID = current.ID

	if !current.Running {
		state.Phase = PhaseStarting
		_ = r.writeState(state)
		if err := r.Runtime.StartContainer(ctx, current.ID); err != nil {
			if !spec.Spec.Lifecycle.RetainFailedContainer {
				_ = r.Runtime.RemoveContainer(context.WithoutCancel(ctx), current.ID, true, false)
			}
			return r.fail(spec, state, FailureContainerStart, Wrap(FailureContainerStart, "start container", err))
		}
	}

	if err := WaitProbe(ctx, spec.Spec.Health.Startup); err != nil {
		return r.fail(spec, state, FailureReadiness, Wrap(FailureReadiness, "startup probe", err))
	}
	state.Phase = PhaseAwaitingReady
	state.Running = true
	_ = r.writeState(state)
	if err := WaitProbe(ctx, spec.Spec.Health.Readiness); err != nil {
		return r.fail(spec, state, FailureReadiness, Wrap(FailureReadiness, "readiness probe", err))
	}

	latest, inspectErr := r.Runtime.InspectContainer(ctx, spec.ContainerName())
	if inspectErr != nil {
		return r.fail(spec, state, FailureStateInspection, Wrap(FailureStateInspection, "inspect ready container", inspectErr))
	}
	if !latest.Exists() || !latest.Running {
		return r.fail(spec, state, FailureContainerStart, Wrap(FailureContainerStart, "verify running container",
			fmt.Errorf("container exited before readiness (status=%q exit=%d error=%q)", latest.Status, latest.ExitCode, latest.Error)))
	}
	current = latest
	state.ContainerID = current.ID
	state.Running = true
	state.Ready = true
	state.Phase = PhaseReady
	state.FailureClass = ""
	state.Error = ""
	state.StartedAt = current.StartedAt
	if state.StartedAt.IsZero() {
		state.StartedAt = r.now()
	}
	if err := r.writeState(state); err != nil {
		return state, Wrap(FailureStatePersistence, "persist ready state", err)
	}
	return state, nil
}

func (r *Reconciler) Remove(ctx context.Context, spec ServiceSpec) (ObservedState, error) {
	if r.Runtime == nil {
		return ObservedState{}, Wrap(FailureRuntimeUnavailable, "remove", errors.New("OCI runtime is nil"))
	}
	if err := Validate(spec, r.Policy); err != nil {
		return ObservedState{}, err
	}
	digest, _ := spec.SpecDigest()
	state := r.baseState(spec, digest)
	current, err := r.Runtime.InspectContainer(ctx, spec.ContainerName())
	if err != nil {
		return r.fail(spec, state, FailureStateInspection, Wrap(FailureStateInspection, "inspect container", err))
	}
	if !current.Exists() {
		state.Phase = PhaseStopped
		state.Ready = false
		state.Running = false
		_ = r.writeState(state)
		return state, nil
	}
	if err := verifyManagedOwnership(current, spec); err != nil {
		return r.fail(spec, state, FailureContainerConflict, err)
	}
	state.ContainerID = current.ID
	state.Phase = PhaseStopping
	state.Running = current.Running
	_ = r.writeState(state)
	if current.Running {
		if err := r.Runtime.StopContainer(ctx, current.ID, stopTimeout(spec)); err != nil {
			return r.fail(spec, state, FailureContainerStop, Wrap(FailureContainerStop, "stop container", err))
		}
	}
	if spec.Spec.Lifecycle.RemoveOnStop {
		if err := r.Runtime.RemoveContainer(ctx, current.ID, false, false); err != nil {
			return r.fail(spec, state, FailureContainerRemove, Wrap(FailureContainerRemove, "remove container", err))
		}
		state.ContainerID = ""
	}
	state.Phase = PhaseStopped
	state.Running = false
	state.Ready = false
	state.FailureClass = ""
	state.Error = ""
	if err := r.writeState(state); err != nil {
		return state, Wrap(FailureStatePersistence, "persist stopped state", err)
	}
	return state, nil
}

func (r *Reconciler) replace(ctx context.Context, spec ServiceSpec, current ContainerState) error {
	if current.Running {
		if err := r.Runtime.StopContainer(ctx, current.ID, stopTimeout(spec)); err != nil {
			return Wrap(FailureContainerStop, "stop replaced container", err)
		}
	}
	// Same deterministic name cannot be created until the old managed instance
	// is removed. Volumes are deliberately retained; bind-mounted data is never
	// deleted by this operation.
	if err := r.Runtime.RemoveContainer(ctx, current.ID, false, false); err != nil {
		return Wrap(FailureContainerRemove, "remove replaced container", err)
	}
	return nil
}

func ensureMountSources(mounts []Mount) error {
	for _, mount := range mounts {
		if !mount.CreateIfMissing {
			continue
		}
		if err := os.MkdirAll(mount.Source, 0o750); err != nil {
			return Wrap(FailureInvalidSpec, "create mount source", fmt.Errorf("%s: %w", mount.Source, err))
		}
	}
	return nil
}

func buildCreateSpec(spec ServiceSpec, labels map[string]string) (ContainerCreateSpec, error) {
	env := make([]string, 0, len(spec.Spec.Environment))
	for _, item := range spec.Spec.Environment {
		value := item.Value
		if item.ValueFile != "" {
			secret, err := readSecretFile(item.ValueFile)
			if err != nil {
				return ContainerCreateSpec{}, Wrap(FailureInvalidSpec, "load environment value file", err)
			}
			value = secret
		}
		env = append(env, item.Name+"="+value)
	}
	sort.Strings(env)
	return ContainerCreateSpec{
		Name:               spec.ContainerName(),
		Image:              spec.CanonicalImageReference(),
		Hostname:           normalizeName(spec.Metadata.Name),
		Entrypoint:         append([]string(nil), spec.Spec.Entrypoint...),
		Command:            append([]string(nil), spec.Spec.Command...),
		Environment:        env,
		Labels:             labels,
		Mounts:             append([]Mount(nil), spec.Spec.Mounts...),
		Network:            spec.Spec.Network,
		Resources:          spec.Spec.Resources,
		Security:           spec.Spec.Security,
		StopTimeoutSeconds: int(stopTimeout(spec).Seconds()),
	}, nil
}

func managedLabels(spec ServiceSpec, specDigest string) map[string]string {
	labels := make(map[string]string, len(spec.Metadata.Labels)+7)
	for k, v := range spec.Metadata.Labels {
		labels[k] = v
	}
	labels[LabelManaged] = "true"
	labels[LabelService] = normalizeName(spec.Metadata.Name)
	labels[LabelInstance] = normalizedInstance(spec.Metadata.Instance)
	labels[LabelSpecDigest] = specDigest
	labels[LabelImageDigest] = strings.ToLower(strings.TrimSpace(spec.Spec.Image.Digest))
	labels[LabelImageRef] = spec.CanonicalImageReference()
	labels[LabelRuntime] = "oci"
	return labels
}

func verifyManagedOwnership(current ContainerState, spec ServiceSpec) error {
	if !strings.EqualFold(current.Labels[LabelManaged], "true") ||
		current.Labels[LabelService] != normalizeName(spec.Metadata.Name) ||
		current.Labels[LabelInstance] != normalizedInstance(spec.Metadata.Instance) {
		return Wrap(FailureContainerConflict, "verify container ownership",
			fmt.Errorf("container name %q is already owned outside this Globular OCI service instance", spec.ContainerName()))
	}
	return nil
}

func containerMatches(current ContainerState, labels map[string]string, imageID string) bool {
	for _, key := range []string{LabelManaged, LabelService, LabelInstance, LabelSpecDigest, LabelImageDigest, LabelImageRef, LabelRuntime} {
		if current.Labels[key] != labels[key] {
			return false
		}
	}
	return imageID == "" || current.ImageID == "" || current.ImageID == imageID
}

func verifyImageIdentity(image ImageState, canonicalRef, expectedDigest string) error {
	expectedDigest = strings.ToLower(strings.TrimSpace(expectedDigest))
	for _, d := range image.Digests {
		ld := strings.ToLower(strings.TrimSpace(d))
		if ld == strings.ToLower(canonicalRef) || strings.HasSuffix(ld, "@"+expectedDigest) {
			return nil
		}
	}
	return Wrap(FailureImageIdentity, "verify image digest",
		fmt.Errorf("runtime image %q does not attest requested digest %s (reported digests: %v)", canonicalRef, expectedDigest, image.Digests))
}

func loadRegistryCredentials(auth RegistryAuth) (RegistryCredentials, error) {
	creds := RegistryCredentials{
		ServerAddress: strings.TrimSpace(auth.ServerAddress),
		Username:      strings.TrimSpace(auth.Username),
	}
	var err error
	if auth.PasswordFile != "" {
		creds.Password, err = readSecretFile(auth.PasswordFile)
		if err != nil {
			return RegistryCredentials{}, Wrap(FailureRegistryAuth, "read registry password", err)
		}
	}
	if auth.IdentityTokenFile != "" {
		creds.IdentityToken, err = readSecretFile(auth.IdentityTokenFile)
		if err != nil {
			return RegistryCredentials{}, Wrap(FailureRegistryAuth, "read registry identity token", err)
		}
	}
	return creds, nil
}

func readSecretFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("secret path %q must be a regular non-symlink file", path)
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok && int(stat.Uid) != os.Geteuid() {
		return "", fmt.Errorf("secret path %q must be owned by runner uid %d", path, os.Geteuid())
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("secret path %q permissions %04o are too broad; require owner-only access", path, info.Mode().Perm())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\r\n"), nil
}

func (r *Reconciler) baseState(spec ServiceSpec, digest string) ObservedState {
	return ObservedState{
		APIVersion:    APIVersionV1Alpha1,
		ServiceName:   normalizeName(spec.Metadata.Name),
		Instance:      normalizedInstance(spec.Metadata.Instance),
		ContainerName: spec.ContainerName(),
		Image:         spec.CanonicalImageReference(),
		SpecDigest:    digest,
		UpdatedAt:     r.now(),
	}
}

func (r *Reconciler) fail(spec ServiceSpec, state ObservedState, class FailureClass, err error) (ObservedState, error) {
	if state.ServiceName == "" {
		digest, _ := spec.SpecDigest()
		state = r.baseState(spec, digest)
	}
	state.Phase = PhaseFailed
	state.Ready = false
	state.FailureClass = class
	state.Error = err.Error()
	_ = r.writeState(state)
	return state, err
}

func (r *Reconciler) writeState(state ObservedState) error {
	if r.State == nil {
		return nil
	}
	return r.State.Write(state)
}

func (r *Reconciler) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func stopTimeout(spec ServiceSpec) time.Duration {
	seconds := spec.Spec.Lifecycle.StopTimeoutSeconds
	if seconds <= 0 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func normalizedInstance(instance string) string {
	instance = normalizeName(instance)
	if instance == "" {
		return "default"
	}
	return instance
}
