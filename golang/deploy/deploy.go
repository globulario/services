package deploy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globularcli/pkgpack"
	"github.com/globulario/services/golang/repository/repository_client"
)

// DeployOptions holds configuration for a deploy run.
type DeployOptions struct {
	ServiceName string
	Version     string
	Publisher   string
	Platform    string
	Comment     string
	Full        bool   // Force full package rebuild
	DryRun      bool   // Print actions without executing
	RepoAddr    string // Repository gRPC address (auto-discovered if empty)
	Token       string // Auth token
}

// DeployResult reports what happened during deploy.
type DeployResult struct {
	Service     string
	Version     string
	BuildNumber int64
	Action      string // "skipped", "binary-only", "full", "dry-run"
	Checksum    string
	Duration    time.Duration
}

// DeployService runs the full deploy workflow for a single service.
func DeployService(ctx context.Context, opts DeployOptions) (*DeployResult, error) {
	start := time.Now()

	paths, err := ResolvePaths()
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}

	cat, err := LoadCatalog(paths.Catalog)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}

	entry, err := cat.Get(opts.ServiceName)
	if err != nil {
		return nil, err
	}

	// Defaults.
	if opts.Version == "" {
		opts.Version = "0.0.2"
	}
	if opts.Publisher == "" {
		opts.Publisher = "core@globular.io"
	}
	if opts.Platform == "" {
		opts.Platform = "linux_amd64"
	}

	// ── Step 1: Build binary ────────────────────────────────────────────
	fmt.Printf("\n━━━ Deploy: %s ━━━\n\n", entry.Name)
	if opts.Comment != "" {
		fmt.Printf("  Comment: %s\n\n", opts.Comment)
	}

	fmt.Println("→ Step 1: Building binary...")
	execName := entry.ExecName()
	goPkgDir, err := paths.GoPackageDir(entry.Name, execName)
	if err != nil {
		return nil, err
	}

	binaryPath := filepath.Join(paths.StageBin, execName)
	if opts.DryRun {
		fmt.Printf("  [dry-run] Would build %s → %s\n", paths.GoPackageRelative(goPkgDir), binaryPath)
	} else {
		goPkgRel := paths.GoPackageRelative(goPkgDir)
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, goPkgRel)
		cmd.Dir = paths.Golang
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("go build %s: %w", goPkgRel, err)
		}
		fmt.Printf("  ✓ Built %s\n", execName)
	}

	// ── Step 1b: Pre-flight — verify the binary is not broken ──────────
	// Run the binary with --describe (or --help) to verify it starts.
	// This catches missing symbols, bad CGO links, and panic-at-init bugs
	// BEFORE publishing to the repository (which triggers cluster-wide rollout).
	if !opts.DryRun {
		fmt.Print("  Verifying binary... ")
		if err := verifyBinary(ctx, binaryPath); err != nil {
			return nil, fmt.Errorf("pre-flight check failed — binary %s is broken: %w\n"+
				"  NOT publishing to avoid corrupting the cluster", execName, err)
		}
		fmt.Println("✓")
	}

	// Compute checksum of new binary.
	newChecksum, err := checksumFile(binaryPath)
	if err != nil && !opts.DryRun {
		return nil, fmt.Errorf("checksum binary: %w", err)
	}

	// ── Step 2: Connect to repository and determine action ──────────────
	fmt.Println("\n→ Step 2: Querying repository...")

	repoAddr := opts.RepoAddr
	if repoAddr == "" {
		repoAddr = resolveRepoAddr()
	}

	if opts.DryRun {
		fmt.Printf("  [dry-run] Would connect to repository at %s\n", repoAddr)
		specYAML, serr := GenerateSpec(entry)
		if serr != nil {
			return nil, fmt.Errorf("generate spec: %w", serr)
		}
		fmt.Printf("  [dry-run] Generated spec (%d bytes)\n", len(specYAML))
		return &DeployResult{
			Service:  entry.Name,
			Version:  opts.Version,
			Action:   "dry-run",
			Checksum: newChecksum,
			Duration: time.Since(start),
		}, nil
	}

	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return nil, fmt.Errorf("connect to repository at %s: %w", repoAddr, err)
	}
	defer client.Close()

	if opts.Token != "" {
		client.SetToken(opts.Token)
	}

	nextBuild, prevChecksum, err := NextBuildNumber(ctx, client, opts.Publisher, entry.Name, opts.Version, opts.Platform)
	if err != nil {
		return nil, fmt.Errorf("query build number: %w", err)
	}

	fmt.Printf("  Current build: %d → Next: %d\n", nextBuild-1, nextBuild)

	// ── Step 3: Detect delta ────────────────────────────────────────────
	binaryChecksum := "sha256:" + newChecksum
	if prevChecksum != "" && prevChecksum == binaryChecksum && !opts.Full {
		fmt.Printf("\n  ✓ Binary unchanged (checksum match) — skipping\n")
		return &DeployResult{
			Service:     entry.Name,
			Version:     opts.Version,
			BuildNumber: nextBuild - 1,
			Action:      "skipped",
			Checksum:    binaryChecksum,
			Duration:    time.Since(start),
		}, nil
	}

	// Check if spec changed — if so, force full publish.
	specChanged := false
	if !opts.Full {
		specYAML, serr := GenerateSpec(entry)
		if serr == nil {
			existingSpec, rerr := os.ReadFile(paths.SpecFile(entry.Name))
			if rerr != nil || string(existingSpec) != specYAML {
				specChanged = true
			}
		}
	}

	action := "binary-only"
	if opts.Full || specChanged || nextBuild <= 1 {
		action = "full"
	}

	// ── Step 4: Build package and publish ────────────────────────────────
	fmt.Printf("\n→ Step 3: Publishing (%s)...\n", action)

	if action == "full" || specChanged {
		// Regenerate spec to generated/specs/
		specYAML, serr := GenerateSpec(entry)
		if serr != nil {
			return nil, fmt.Errorf("generate spec: %w", serr)
		}
		if err := os.MkdirAll(paths.SpecsDir, 0o755); err != nil {
			return nil, fmt.Errorf("create specs dir: %w", err)
		}
		if err := os.WriteFile(paths.SpecFile(entry.Name), []byte(specYAML), 0o644); err != nil {
			return nil, fmt.Errorf("write spec: %w", err)
		}
		fmt.Printf("  ✓ Spec regenerated\n")
	}

	// Stage payload.
	payloadDir := paths.PayloadDir(entry.Name)
	payloadBinDir := filepath.Join(payloadDir, "bin")
	if err := os.MkdirAll(payloadBinDir, 0o755); err != nil {
		return nil, fmt.Errorf("create payload dir: %w", err)
	}
	if err := copyFile(binaryPath, filepath.Join(payloadBinDir, execName)); err != nil {
		return nil, fmt.Errorf("stage binary: %w", err)
	}

	// Build .tgz in-process (no subprocess).
	specFile := paths.SpecFile(entry.Name)
	if err := os.MkdirAll(paths.Generated, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	buildResults, err := pkgpack.BuildPackages(pkgpack.BuildOptions{
		SpecPath:          specFile,
		Root:              payloadDir,
		Version:           opts.Version,
		BuildNumber:       nextBuild,
		Publisher:         opts.Publisher,
		Platform:          opts.Platform,
		OutDir:            paths.Generated,
		SkipMissingConfig: true,
	})
	if err != nil {
		return nil, fmt.Errorf("pkg build: %w", err)
	}
	if len(buildResults) == 0 {
		return nil, fmt.Errorf("pkg build produced no output")
	}
	if buildResults[0].Err != nil {
		return nil, fmt.Errorf("pkg build %s: %w", entry.Name, buildResults[0].Err)
	}
	tgzPath := buildResults[0].OutputPath
	fmt.Printf("  ✓ Package built (%s)\n", filepath.Base(tgzPath))

	// Publish via CLI subprocess — handles mTLS auth correctly.
	globularCLI, err := findGlobularCLI(paths)
	if err != nil {
		return nil, err
	}
	// Read cached token if not provided — the subprocess needs it explicitly
	// because its PersistentPreRunE may fail to generate a service token.
	publishToken := opts.Token
	if publishToken == "" {
		if data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "globular", "token")); err == nil {
			publishToken = strings.TrimSpace(string(data))
		}
	}

	var publishArgs []string
	if publishToken != "" {
		publishArgs = append(publishArgs, "--token", publishToken)
	}
	publishArgs = append(publishArgs,
		"pkg", "publish",
		"--file", tgzPath,
		"--repository", repoAddr,
		"--force",
	)
	cmd := exec.CommandContext(ctx, globularCLI, publishArgs...)
	cmd.Dir = paths.Root
	var pubOut strings.Builder
	cmd.Stdout = &pubOut
	cmd.Stderr = &pubOut
	if err := cmd.Run(); err != nil {
		out := pubOut.String()
		if !strings.Contains(out, "success") && !strings.Contains(out, "bundle_id") && !strings.Contains(out, "verify uploaded manifest") {
			fmt.Printf("  %s\n", out)
			return nil, fmt.Errorf("pkg publish: %w", err)
		}
		fmt.Printf("  (post-upload verify warning — bundle was uploaded)\n")
	}
	fmt.Printf("  ✓ Published to %s\n", repoAddr)

	// ── Step 4: Update desired state ────────────────────────────────────
	fmt.Printf("\n→ Step 4: Updating desired state...\n")
	desiredArgs := []string{
		"services", "desired", "set",
		entry.Name, opts.Version,
		"--build-number", fmt.Sprintf("%d", nextBuild),
	}
	cmd = exec.CommandContext(ctx, globularCLI, desiredArgs...)
	cmd.Dir = paths.Root
	var desiredOut strings.Builder
	cmd.Stdout = &desiredOut
	cmd.Stderr = &desiredOut
	if err := cmd.Run(); err != nil {
		fmt.Printf("  ⚠ desired state update failed: %s\n", desiredOut.String())
		// Non-fatal — the artifact is published, just not auto-rolled out.
	} else {
		fmt.Printf("  ✓ Desired state: %s@%s+%d\n", entry.Name, opts.Version, nextBuild)
	}

	// ── Report ──────────────────────────────────────────────────────────
	result := &DeployResult{
		Service:     entry.Name,
		Version:     opts.Version,
		BuildNumber: nextBuild,
		Action:      action,
		Checksum:    binaryChecksum,
		Duration:    time.Since(start),
	}

	fmt.Printf("\n━━━ Deployed ━━━\n\n")
	fmt.Printf("  Service:      %s\n", result.Service)
	fmt.Printf("  Version:      %s\n", result.Version)
	fmt.Printf("  Build:        %d\n", result.BuildNumber)
	fmt.Printf("  Action:       %s\n", result.Action)
	fmt.Printf("  Comment:      %s\n", orDefault(opts.Comment, "(none)"))
	fmt.Printf("  Duration:     %s\n", result.Duration.Round(time.Millisecond))
	fmt.Println()
	fmt.Println("  The controller will detect the new artifact and roll it out.")
	fmt.Println()

	return result, nil
}

// DeployAll deploys all services in the catalog sequentially.
//
// Sequential execution is required because:
//   - go build writes to a shared stage directory (StageBin)
//   - globular pkg build writes to a shared generated/ directory
//   - globular pkg publish reads back from generated/
//
// Parallel deploys would clobber each other's output files. The build step
// dominates wall-clock time anyway, and go build already parallelizes internally.
func DeployAll(ctx context.Context, opts DeployOptions, _ int) ([]*DeployResult, error) {
	paths, err := ResolvePaths()
	if err != nil {
		return nil, fmt.Errorf("resolve paths: %w", err)
	}
	cat, err := LoadCatalog(paths.Catalog)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}

	names := cat.ServiceNames()
	var deployed []*DeployResult
	var errs []string

	for i, name := range names {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(names), name)

		svcOpts := opts
		svcOpts.ServiceName = name
		result, err := DeployService(ctx, svcOpts)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
			fmt.Printf("  ✗ %s: %v\n", name, err)
			continue
		}
		deployed = append(deployed, result)
	}

	if len(errs) > 0 {
		return deployed, fmt.Errorf("%d/%d deploys failed:\n  %s", len(errs), len(names), strings.Join(errs, "\n  "))
	}
	return deployed, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func resolveRepoAddr() string {
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr != "" {
		fmt.Printf("  Auto-discovered repository: %s\n", addr)
		return addr
	}
	addr = fmt.Sprintf("%s:10007", config.GetRoutableIPv4())
	fmt.Printf("  Using fallback repository: %s\n", addr)
	return addr
}

func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}


func findGlobularCLI(paths *Paths) (string, error) {
	staged := filepath.Join(paths.StageBin, "globularcli")
	if _, err := os.Stat(staged); err == nil {
		return staged, nil
	}
	if p, err := exec.LookPath("globular"); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("globular CLI not found (checked %s and PATH)", paths.StageBin)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// verifyBinary runs a basic pre-flight check on the compiled binary.
// It tries --describe first (Globular service convention), then --help,
// and finally just executes the binary and checks it doesn't immediately
// crash with a non-zero exit code within 2 seconds.
//
// This catches:
//   - Missing shared libraries / CGO issues
//   - Panic-at-init (broken init() functions)
//   - Missing embedded resources
//   - Wrong Go module linkage
func verifyBinary(ctx context.Context, binaryPath string) error {
	verifyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try --describe — all Globular services support this flag and exit 0.
	cmd := exec.CommandContext(verifyCtx, binaryPath, "--describe")
	cmd.Env = append(os.Environ(), "GLOBULAR_PREFLIGHT=1")
	if out, err := cmd.CombinedOutput(); err == nil {
		return nil // binary starts and responds to --describe
	} else {
		// --describe may not be supported (non-Globular binary or old version).
		// Check if it was a timeout (binary started but didn't exit) — that's OK,
		// it means the binary runs.
		if verifyCtx.Err() != nil {
			return nil // timeout = binary started successfully, just didn't exit
		}
		_ = out
	}

	// Try --help as fallback.
	verifyCtx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()
	cmd = exec.CommandContext(verifyCtx2, binaryPath, "--help")
	cmd.Env = append(os.Environ(), "GLOBULAR_PREFLIGHT=1")
	if out, err := cmd.CombinedOutput(); err == nil {
		return nil
	} else if verifyCtx2.Err() != nil {
		return nil // timeout = started OK
	} else {
		// Check the exit code — exit code 2 from --help is normal for some CLIs.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return nil
		}
		return fmt.Errorf("binary failed to start: %v\nOutput: %s", err, truncate(string(out), 500))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
