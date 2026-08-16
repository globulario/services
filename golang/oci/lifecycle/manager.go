package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/oci"
)

const receiptAPIVersion = "globular.io/oci/package-observed/v1alpha1"

// Supervisor is the narrow node-local execution boundary used by the package
// lifecycle. The cluster controller never implements or calls this interface.
type Supervisor interface {
	DaemonReload(context.Context) error
	Stop(context.Context, string) error
	Disable(context.Context, string) error
}

type Layout struct {
	ConfigRoot   string
	SystemdRoot  string
	StateRoot    string
	RunnerPath   string
	PolicyPath   string
	DockerSocket string
}

func DefaultLayout() Layout {
	return Layout{
		ConfigRoot:   "/etc/globular/oci/services",
		SystemdRoot:  "/etc/systemd/system",
		StateRoot:    "/var/lib/globular/oci",
		RunnerPath:   "/usr/lib/globular/bin/globular-oci-runner",
		PolicyPath:   "/etc/globular/oci/policy.json",
		DockerSocket: "unix:///var/run/docker.sock",
	}
}

type Manager struct {
	Layout     Layout
	Supervisor Supervisor
	Now        func() time.Time
}

type Receipt struct {
	APIVersion     string    `json:"api_version"`
	ServiceName    string    `json:"service_name"`
	Instance       string    `json:"instance"`
	Version        string    `json:"version,omitempty"`
	UnitName       string    `json:"unit_name"`
	SpecPath       string    `json:"spec_path"`
	PolicyPath     string    `json:"policy_path"`
	StateRoot      string    `json:"state_root"`
	Image          string    `json:"image"`
	SpecDigest     string    `json:"spec_digest"`
	MaterializedAt time.Time `json:"materialized_at"`
}

type Verification struct {
	Receipt  Receipt           `json:"receipt"`
	Observed oci.ObservedState `json:"observed"`
}

func (m Manager) Install(ctx context.Context, inspection Inspection, policy oci.Policy, version string) (receipt Receipt, err error) {
	layout, err := normalizeLayout(m.Layout)
	if err != nil {
		return Receipt{}, err
	}
	if err := inspection.Validate(policy); err != nil {
		return Receipt{}, err
	}
	service := normalizeName(inspection.Contract.ServiceName)
	instance := normalizeInstance(inspection.Contract.Instance)
	specDigest, err := inspection.ServiceSpec.SpecDigest()
	if err != nil {
		return Receipt{}, fmt.Errorf("compute OCI service spec digest: %w", err)
	}
	paths := derivedPaths(layout, service)
	unit, err := RenderUnit(layout, inspection.ServiceSpec, service)
	if err != nil {
		return Receipt{}, err
	}
	now := time.Now().UTC()
	if m.Now != nil {
		now = m.Now().UTC()
	}
	receipt = Receipt{
		APIVersion:     receiptAPIVersion,
		ServiceName:    service,
		Instance:       instance,
		Version:        strings.TrimSpace(version),
		UnitName:       paths.unitName,
		SpecPath:       paths.specPath,
		PolicyPath:     layout.PolicyPath,
		StateRoot:      layout.StateRoot,
		Image:          inspection.ServiceSpec.CanonicalImageReference(),
		SpecDigest:     specDigest,
		MaterializedAt: now,
	}
	receiptBytes, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return Receipt{}, fmt.Errorf("marshal OCI package receipt: %w", err)
	}
	receiptBytes = append(receiptBytes, '\n')

	for _, mount := range inspection.ServiceSpec.Spec.Mounts {
		if !mount.CreateIfMissing {
			continue
		}
		if err := os.MkdirAll(filepath.Clean(mount.Source), 0o750); err != nil {
			return Receipt{}, fmt.Errorf("create OCI mount source %s: %w", mount.Source, err)
		}
	}

	policyBytes, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return Receipt{}, fmt.Errorf("marshal default OCI policy: %w", err)
	}
	policyBytes = append(policyBytes, '\n')

	writes := []plannedWrite{
		{path: paths.specPath, data: append([]byte(nil), inspection.SpecJSON...), mode: 0o640},
		{path: paths.unitPath, data: []byte(unit), mode: 0o644},
		{path: paths.receiptPath, data: receiptBytes, mode: 0o640},
	}
	policyCreated := false
	if _, statErr := os.Stat(layout.PolicyPath); os.IsNotExist(statErr) {
		writes = append([]plannedWrite{{path: layout.PolicyPath, data: policyBytes, mode: 0o640}}, writes...)
		policyCreated = true
	} else if statErr != nil {
		return Receipt{}, fmt.Errorf("inspect node OCI policy: %w", statErr)
	}

	snapshots, err := captureSnapshots(writes)
	if err != nil {
		return Receipt{}, err
	}
	committed := false
	defer func() {
		if committed || err == nil {
			return
		}
		_ = restoreSnapshots(snapshots)
		if policyCreated {
			_ = removeIfEmpty(filepath.Dir(layout.PolicyPath))
		}
		if m.Supervisor != nil {
			_ = m.Supervisor.DaemonReload(context.Background())
		}
	}()

	for _, write := range writes {
		if err = writeFileAtomic(write.path, write.data, write.mode); err != nil {
			return Receipt{}, err
		}
	}
	if m.Supervisor != nil {
		if err = m.Supervisor.DaemonReload(ctx); err != nil {
			return Receipt{}, fmt.Errorf("reload systemd after OCI unit materialization: %w", err)
		}
	}
	committed = true
	return receipt, nil
}

func (m Manager) IsManaged(service string) bool {
	layout, err := normalizeLayout(m.Layout)
	if err != nil {
		return false
	}
	_, err = os.Stat(derivedPaths(layout, normalizeName(service)).receiptPath)
	return err == nil
}

func (m Manager) ReadReceipt(service string) (Receipt, error) {
	layout, err := normalizeLayout(m.Layout)
	if err != nil {
		return Receipt{}, err
	}
	paths := derivedPaths(layout, normalizeName(service))
	b, err := os.ReadFile(paths.receiptPath)
	if err != nil {
		return Receipt{}, err
	}
	var receipt Receipt
	if err := decodeStrictJSON(b, &receipt); err != nil {
		return Receipt{}, fmt.Errorf("decode OCI package receipt: %w", err)
	}
	if err := validateReceipt(receipt, layout, paths); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func (m Manager) Verify(service string) (Verification, error) {
	layout, err := normalizeLayout(m.Layout)
	if err != nil {
		return Verification{}, err
	}
	receipt, err := m.ReadReceipt(service)
	if err != nil {
		return Verification{}, err
	}
	specBytes, err := os.ReadFile(receipt.SpecPath)
	if err != nil {
		return Verification{}, fmt.Errorf("read materialized OCI service spec: %w", err)
	}
	var spec oci.ServiceSpec
	if err := decodeStrictJSON(specBytes, &spec); err != nil {
		return Verification{}, fmt.Errorf("decode materialized OCI service spec: %w", err)
	}
	digest, err := spec.SpecDigest()
	if err != nil {
		return Verification{}, err
	}
	if digest != receipt.SpecDigest {
		return Verification{}, fmt.Errorf("OCI service spec digest drift: receipt=%s actual=%s", receipt.SpecDigest, digest)
	}
	observedPath := filepath.Join(layout.StateRoot, normalizeName(receipt.ServiceName), normalizeInstance(receipt.Instance), "observed.json")
	observedBytes, err := os.ReadFile(observedPath)
	if err != nil {
		return Verification{}, fmt.Errorf("read OCI observed state: %w", err)
	}
	var observed oci.ObservedState
	if err := decodeStrictJSON(observedBytes, &observed); err != nil {
		return Verification{}, fmt.Errorf("decode OCI observed state: %w", err)
	}
	if normalizeName(observed.ServiceName) != receipt.ServiceName || normalizeInstance(observed.Instance) != receipt.Instance {
		return Verification{}, errors.New("OCI observed state identity does not match package receipt")
	}
	if observed.SpecDigest != receipt.SpecDigest {
		return Verification{}, fmt.Errorf("OCI observed spec digest mismatch: receipt=%s observed=%s", receipt.SpecDigest, observed.SpecDigest)
	}
	if observed.Image != receipt.Image {
		return Verification{}, fmt.Errorf("OCI observed image mismatch: receipt=%s observed=%s", receipt.Image, observed.Image)
	}
	if !observed.Running || !observed.Ready || observed.Phase != oci.PhaseReady {
		return Verification{}, fmt.Errorf("OCI service is not ready: phase=%s running=%t ready=%t", observed.Phase, observed.Running, observed.Ready)
	}
	return Verification{Receipt: receipt, Observed: observed}, nil
}

func (m Manager) Uninstall(ctx context.Context, service string) (bool, error) {
	layout, err := normalizeLayout(m.Layout)
	if err != nil {
		return false, err
	}
	service = normalizeName(service)
	paths := derivedPaths(layout, service)
	receipt, err := m.ReadReceipt(service)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if m.Supervisor != nil {
		// Stop and disable are best-effort during uninstall. A unit may already be
		// stopped, failed, or absent after a partial repair; none of those states
		// may prevent removal of its admitted package files.
		_ = m.Supervisor.Stop(ctx, receipt.UnitName)
		_ = m.Supervisor.Disable(ctx, receipt.UnitName)
	}
	for _, target := range []string{paths.unitPath, paths.specPath, paths.receiptPath} {
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return true, fmt.Errorf("remove OCI package file %s: %w", target, err)
		}
	}
	_ = removeIfEmpty(filepath.Dir(paths.specPath))
	_ = removeIfEmpty(filepath.Dir(paths.receiptPath))
	if m.Supervisor != nil {
		if err := m.Supervisor.DaemonReload(ctx); err != nil {
			return true, fmt.Errorf("reload systemd after OCI uninstall: %w", err)
		}
	}
	// Deliberately retain layout.StateRoot/<service>/... and every bind mount.
	// Container/runtime evidence and persistent application data have separate
	// lifecycle authority from the package files.
	return true, nil
}

type derived struct {
	unitName    string
	unitPath    string
	specPath    string
	receiptPath string
}

func derivedPaths(layout Layout, service string) derived {
	unitName := "globular-" + normalizeName(service) + ".service"
	return derived{
		unitName:    unitName,
		unitPath:    filepath.Join(layout.SystemdRoot, unitName),
		specPath:    filepath.Join(layout.ConfigRoot, normalizeName(service), "service.json"),
		receiptPath: filepath.Join(layout.StateRoot, "package-receipts", normalizeName(service)+".json"),
	}
}

func validateReceipt(receipt Receipt, layout Layout, paths derived) error {
	if receipt.APIVersion != receiptAPIVersion {
		return fmt.Errorf("unsupported OCI package receipt api_version %q", receipt.APIVersion)
	}
	if normalizeName(receipt.ServiceName) == "" || normalizeName(receipt.ServiceName) != strings.TrimSuffix(strings.TrimPrefix(paths.unitName, "globular-"), ".service") {
		return errors.New("OCI package receipt service identity is invalid")
	}
	if receipt.UnitName != paths.unitName || filepath.Clean(receipt.SpecPath) != paths.specPath || filepath.Clean(receipt.PolicyPath) != layout.PolicyPath || filepath.Clean(receipt.StateRoot) != layout.StateRoot {
		return errors.New("OCI package receipt contains paths outside the node-owned layout")
	}
	return nil
}

func normalizeLayout(layout Layout) (Layout, error) {
	defaults := DefaultLayout()
	if strings.TrimSpace(layout.ConfigRoot) == "" {
		layout.ConfigRoot = defaults.ConfigRoot
	}
	if strings.TrimSpace(layout.SystemdRoot) == "" {
		layout.SystemdRoot = defaults.SystemdRoot
	}
	if strings.TrimSpace(layout.StateRoot) == "" {
		layout.StateRoot = defaults.StateRoot
	}
	if strings.TrimSpace(layout.RunnerPath) == "" {
		layout.RunnerPath = defaults.RunnerPath
	}
	if strings.TrimSpace(layout.PolicyPath) == "" {
		layout.PolicyPath = defaults.PolicyPath
	}
	if strings.TrimSpace(layout.DockerSocket) == "" {
		layout.DockerSocket = defaults.DockerSocket
	}
	for name, value := range map[string]string{
		"config root": layout.ConfigRoot, "systemd root": layout.SystemdRoot,
		"state root": layout.StateRoot, "runner path": layout.RunnerPath,
		"policy path": layout.PolicyPath,
	} {
		if !filepath.IsAbs(value) {
			return Layout{}, fmt.Errorf("OCI %s must be absolute", name)
		}
	}
	layout.ConfigRoot = filepath.Clean(layout.ConfigRoot)
	layout.SystemdRoot = filepath.Clean(layout.SystemdRoot)
	layout.StateRoot = filepath.Clean(layout.StateRoot)
	layout.RunnerPath = filepath.Clean(layout.RunnerPath)
	layout.PolicyPath = filepath.Clean(layout.PolicyPath)
	if !strings.HasPrefix(layout.DockerSocket, "unix:///") {
		return Layout{}, errors.New("OCI Docker socket must be an absolute unix:// endpoint")
	}
	return layout, nil
}

type plannedWrite struct {
	path string
	data []byte
	mode os.FileMode
}
type fileSnapshot struct {
	path    string
	existed bool
	data    []byte
	mode    os.FileMode
}

func captureSnapshots(writes []plannedWrite) ([]fileSnapshot, error) {
	out := make([]fileSnapshot, 0, len(writes))
	seen := map[string]struct{}{}
	for _, write := range writes {
		path := filepath.Clean(write.path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		snapshot := fileSnapshot{path: path}
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			out = append(out, snapshot)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect existing file %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("refuse to replace non-regular file %s", path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		snapshot.existed, snapshot.data, snapshot.mode = true, b, info.Mode().Perm()
		out = append(out, snapshot)
	}
	return out, nil
}

func restoreSnapshots(snapshots []fileSnapshot) error {
	var errs []string
	for i := len(snapshots) - 1; i >= 0; i-- {
		snapshot := snapshots[i]
		var err error
		if snapshot.existed {
			err = writeFileAtomic(snapshot.path, snapshot.data, snapshot.mode)
		} else {
			err = os.Remove(snapshot.path)
			if os.IsNotExist(err) {
				err = nil
			}
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	path = filepath.Clean(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".oci-package-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit %s: %w", path, err)
	}
	return syncDirectory(dir)
}

func syncDirectory(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}

func removeIfEmpty(dir string) error {
	err := os.Remove(dir)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
