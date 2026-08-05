package lifecycle

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/globulario/services/golang/oci"
)

const (
	PackageAPIVersionV1Alpha1 = "globular.io/oci/package/v1alpha1"
	KindOCIServicePackage     = "OCIServicePackage"
	PackageContractRoot       = "data/oci/packages"

	maxArchiveEntries = 4096
	maxOCIFileBytes   = 2 << 20
	maxOCIFiles       = 16
)

var packageNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9.-]{0,61}[a-z0-9])?$`)

// PackageContract is the package-owned declaration that opts an ordinary
// Globular SERVICE package into the OCI lifecycle. It intentionally contains
// no runner path, Docker socket, policy path, systemd unit, or host destination:
// those are node-owned authority selected by Layout.
type PackageContract struct {
	APIVersion  string `json:"api_version"`
	Kind        string `json:"kind"`
	ServiceName string `json:"service_name"`
	Instance    string `json:"instance,omitempty"`
	ServiceSpec string `json:"service_spec,omitempty"`
}

// Inspection is a side-effect-free, fully parsed view of an OCI package.
type Inspection struct {
	ContractPath string
	Contract     PackageContract
	ServiceSpec  oci.ServiceSpec
	ContractJSON []byte
	SpecJSON     []byte
}

// InspectArtifact returns found=false for a normal native Globular package.
// Once an OCI package contract is present, the entire archive is held to the
// stricter OCI package rules before any installation side effect occurs.
func InspectArtifact(artifactPath, expectedService string) (inspection Inspection, found bool, err error) {
	files, contractPath, hasContract, scanErr := scanOCIArchive(artifactPath)
	if !hasContract {
		return Inspection{}, false, nil
	}
	if scanErr != nil {
		return Inspection{}, true, scanErr
	}

	contractBytes := files[contractPath]
	if len(contractBytes) == 0 {
		return Inspection{}, true, errors.New("OCI package contract is empty")
	}
	var contract PackageContract
	if err := decodeStrictJSON(contractBytes, &contract); err != nil {
		return Inspection{}, true, fmt.Errorf("decode OCI package contract: %w", err)
	}
	if err := validateContract(contract, expectedService); err != nil {
		return Inspection{}, true, err
	}
	contractDir := path.Dir(contractPath)
	if strings.TrimSpace(contract.ServiceSpec) == "" {
		contract.ServiceSpec = path.Join(contractDir, "service.json")
	}
	if path.Dir(contract.ServiceSpec) != contractDir {
		return Inspection{}, true, errors.New("OCI service_spec must remain in the package contract directory")
	}
	if normalizeName(path.Base(contractDir)) != normalizeName(contract.ServiceName) {
		return Inspection{}, true, errors.New("OCI package contract directory does not match service_name")
	}

	specBytes, ok := files[contract.ServiceSpec]
	if !ok {
		return Inspection{}, true, fmt.Errorf("OCI service specification %q is missing from package", contract.ServiceSpec)
	}
	var spec oci.ServiceSpec
	if err := decodeStrictJSON(specBytes, &spec); err != nil {
		return Inspection{}, true, fmt.Errorf("decode OCI service specification: %w", err)
	}
	if normalizeName(spec.Metadata.Name) != normalizeName(contract.ServiceName) {
		return Inspection{}, true, fmt.Errorf("OCI service spec metadata.name %q does not match package service %q", spec.Metadata.Name, contract.ServiceName)
	}
	contractInstance := normalizeInstance(contract.Instance)
	specInstance := normalizeInstance(spec.Metadata.Instance)
	if contractInstance != specInstance {
		return Inspection{}, true, fmt.Errorf("OCI service spec instance %q does not match package instance %q", spec.Metadata.Instance, contract.Instance)
	}

	return Inspection{
		ContractPath: contractPath,
		Contract:     contract,
		ServiceSpec:  spec,
		ContractJSON: append([]byte(nil), contractBytes...),
		SpecJSON:     append([]byte(nil), specBytes...),
	}, true, nil
}

func (i Inspection) Validate(policy oci.Policy) error {
	if err := validateContract(i.Contract, i.Contract.ServiceName); err != nil {
		return err
	}
	expectedContractPath := ContractPath(i.Contract.ServiceName)
	if i.ContractPath != expectedContractPath {
		return fmt.Errorf("OCI package contract path %q must be %q", i.ContractPath, expectedContractPath)
	}
	expectedSpecPath := i.Contract.ServiceSpec
	if strings.TrimSpace(expectedSpecPath) == "" {
		expectedSpecPath = DefaultSpecPath(i.Contract.ServiceName)
	}
	if path.Dir(expectedSpecPath) != path.Dir(expectedContractPath) {
		return errors.New("OCI service_spec must remain beside its package contract")
	}
	if err := oci.Validate(i.ServiceSpec, policy); err != nil {
		return fmt.Errorf("validate OCI service specification: %w", err)
	}
	return nil
}

func scanOCIArchive(artifactPath string) (map[string][]byte, string, bool, error) {
	f, err := os.Open(artifactPath)
	if err != nil {
		return nil, "", false, fmt.Errorf("open package artifact: %w", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, "", false, fmt.Errorf("open package gzip stream: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	files := make(map[string][]byte)
	var archiveProblem error
	hasContract := false
	contractPath := ""
	entries := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, contractPath, hasContract, fmt.Errorf("read package tar stream: %w", err)
		}
		entries++
		if entries > maxArchiveEntries {
			archiveProblem = fmt.Errorf("package contains more than %d entries", maxArchiveEntries)
			break
		}
		name, pathErr := safeArchivePath(hdr.Name)
		if pathErr != nil {
			archiveProblem = pathErr
			continue
		}
		if isPackageContractPath(name) {
			if hasContract && contractPath != name {
				archiveProblem = errors.New("OCI package contains more than one package contract")
			}
			hasContract = true
			contractPath = name
		}
		switch {
		case strings.HasPrefix(name, "systemd/"), strings.HasPrefix(name, "units/"):
			archiveProblem = fmt.Errorf("OCI package may not ship a systemd unit; the node agent renders the governed unit")
		case strings.HasPrefix(name, "bin/"):
			archiveProblem = fmt.Errorf("OCI package may not ship host binaries; the immutable container image is its executable payload")
		case strings.HasPrefix(name, "scripts/"):
			archiveProblem = fmt.Errorf("OCI package may not ship host post-install scripts")
		case strings.HasPrefix(name, "debs/"):
			archiveProblem = fmt.Errorf("OCI package may not ship host OS packages")
		}
		if !strings.HasPrefix(name, "data/oci/") || hdr.FileInfo().IsDir() {
			continue
		}
		if hdr.Typeflag != tar.TypeReg {
			archiveProblem = fmt.Errorf("OCI package entry %q must be a regular file", name)
			continue
		}
		if hdr.Size < 0 || hdr.Size > maxOCIFileBytes {
			archiveProblem = fmt.Errorf("OCI package entry %q exceeds %d bytes", name, maxOCIFileBytes)
			continue
		}
		if len(files) >= maxOCIFiles {
			archiveProblem = fmt.Errorf("OCI package contains more than %d data/oci files", maxOCIFiles)
			continue
		}
		if _, duplicate := files[name]; duplicate {
			archiveProblem = fmt.Errorf("OCI package entry %q is duplicated", name)
			continue
		}
		b, readErr := io.ReadAll(io.LimitReader(tr, maxOCIFileBytes+1))
		if readErr != nil {
			return nil, contractPath, hasContract, fmt.Errorf("read OCI package entry %q: %w", name, readErr)
		}
		if len(b) > maxOCIFileBytes {
			archiveProblem = fmt.Errorf("OCI package entry %q exceeds %d bytes", name, maxOCIFileBytes)
			continue
		}
		files[name] = b
	}

	if !hasContract {
		return nil, "", false, nil
	}
	if archiveProblem != nil {
		return nil, contractPath, true, archiveProblem
	}
	return files, contractPath, true, nil
}

func validateContract(contract PackageContract, expectedService string) error {
	var problems []string
	if contract.APIVersion != PackageAPIVersionV1Alpha1 {
		problems = append(problems, fmt.Sprintf("api_version must be %q", PackageAPIVersionV1Alpha1))
	}
	if contract.Kind != KindOCIServicePackage {
		problems = append(problems, fmt.Sprintf("kind must be %q", KindOCIServicePackage))
	}
	service := normalizeName(contract.ServiceName)
	if !packageNamePattern.MatchString(service) {
		problems = append(problems, "service_name must be a lowercase DNS-like name")
	}
	if expected := normalizeName(expectedService); expected != "" && service != expected {
		problems = append(problems, fmt.Sprintf("service_name %q does not match requested package %q", contract.ServiceName, expectedService))
	}
	instance := normalizeInstance(contract.Instance)
	if !packageNamePattern.MatchString(instance) {
		problems = append(problems, "instance must be a lowercase DNS-like name")
	}
	specPath := strings.TrimSpace(contract.ServiceSpec)
	if specPath == "" {
		specPath = path.Join(PackageContractRoot, service, "service.json")
	}
	clean, err := safeArchivePath(specPath)
	if err != nil {
		problems = append(problems, "service_spec must be a safe package-relative path")
	} else if clean != specPath {
		problems = append(problems, "service_spec must be canonical")
	} else if !strings.HasPrefix(clean, PackageContractRoot+"/") || strings.HasSuffix(clean, "/package.json") {
		problems = append(problems, "service_spec must reside beside the service-scoped package contract")
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid OCI package contract: %s", strings.Join(problems, "; "))
	}
	return nil
}

func ContractPath(service string) string {
	return path.Join(PackageContractRoot, normalizeName(service), "package.json")
}

func DefaultSpecPath(service string) string {
	return path.Join(PackageContractRoot, normalizeName(service), "service.json")
}

func isPackageContractPath(name string) bool {
	if !strings.HasPrefix(name, PackageContractRoot+"/") || !strings.HasSuffix(name, "/package.json") {
		return false
	}
	rel := strings.TrimPrefix(name, PackageContractRoot+"/")
	parts := strings.Split(rel, "/")
	return len(parts) == 2 && parts[0] == normalizeName(parts[0]) && packageNamePattern.MatchString(parts[0]) && parts[1] == "package.json"
}

func safeArchivePath(raw string) (string, error) {
	if strings.Contains(raw, "\\") {
		return "", fmt.Errorf("package path %q contains a backslash", raw)
	}
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "./")
	for _, segment := range strings.Split(raw, "/") {
		if segment == ".." {
			return "", fmt.Errorf("package path %q contains parent traversal", raw)
		}
	}
	clean := path.Clean(raw)
	if clean == "." || clean == "" || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("package path %q escapes the archive root", raw)
	}
	return clean, nil
}

func decodeStrictJSON(b []byte, out any) error {
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

func normalizeName(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	return strings.ReplaceAll(v, "_", "-")
}

func normalizeInstance(v string) string {
	v = normalizeName(v)
	if v == "" {
		return "default"
	}
	return v
}
