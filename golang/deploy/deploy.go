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
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// DeployOptions holds configuration for a deploy run.
type DeployOptions struct {
	ServiceName string
	Version     string
	Bump        string // "patch" | "minor" | "major" — calls AllocateUpload
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
	BuildID     string // Repository-allocated UUIDv7 (populated after publish)
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
	if opts.Version == "" && opts.Bump == "" {
		return nil, fmt.Errorf("version is required — use --version or --bump patch|minor|major")
	}
	if opts.Version != "" && opts.Bump == "" {
		fmt.Println("  ⚠ deprecated: --version without --bump — use --bump to let the repository allocate versions")
	}
	if opts.Publisher == "" {
		opts.Publisher = "core@globular.io"
	}
	if opts.Platform == "" {
		opts.Platform = "linux_amd64"
	}

	// Package name uses hyphens (ai-executor), catalog name uses underscores (ai_executor).
	pkgName := entry.PackageName()

	// ── Step 1: Build binary ────────────────────────────────────────────
	fmt.Printf("\n━━━ Deploy: %s ━━━\n\n", pkgName)
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
		cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-o", binaryPath, goPkgRel)
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
			Service:  pkgName,
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

	var nextBuild int64
	var prevChecksum string
	var reservationID string
	var allocatedBuildID string

	if opts.Bump != "" {
		// Phase A: use AllocateUpload to get version + build_number + reservation_id.
		var intent repopb.VersionIntent
		switch strings.ToLower(opts.Bump) {
		case "patch":
			intent = repopb.VersionIntent_BUMP_PATCH
		case "minor":
			intent = repopb.VersionIntent_BUMP_MINOR
		case "major":
			intent = repopb.VersionIntent_BUMP_MAJOR
		default:
			return nil, fmt.Errorf("invalid --bump value %q: use patch, minor, or major", opts.Bump)
		}

		exactVersion := opts.Version // empty if --version not set
		alloc, err := client.AllocateUpload(opts.Publisher, pkgName, opts.Platform, intent, exactVersion)
		if err != nil {
			return nil, fmt.Errorf("allocate upload: %w", err)
		}
		opts.Version = alloc.GetVersion()
		nextBuild = alloc.GetBuildNumber()
		reservationID = alloc.GetReservationId()
		allocatedBuildID = alloc.GetBuildId()
		fmt.Printf("  Allocated: version=%s build=%d build_id=%s\n",
			alloc.GetVersion(), nextBuild, allocatedBuildID[:8])

		// Query previous checksum for delta detection.
		info, _ := QueryLatestBuild(ctx, client, opts.Publisher, pkgName, opts.Version, opts.Platform)
		if info != nil {
			prevChecksum = info.Checksum
		}
	} else {
		// Legacy path: query build number directly.
		var err error
		nextBuild, prevChecksum, err = NextBuildNumber(ctx, client, opts.Publisher, pkgName, opts.Version, opts.Platform)
		if err != nil {
			return nil, fmt.Errorf("query build number: %w", err)
		}
		fmt.Printf("  Current build: %d → Next: %d\n", nextBuild-1, nextBuild)
	}

	// ── Step 3: Detect delta ────────────────────────────────────────────
	binaryChecksum := "sha256:" + newChecksum
	if prevChecksum != "" && prevChecksum == binaryChecksum && !opts.Full {
		fmt.Printf("\n  ✓ Binary unchanged (checksum match) — skipping\n")
		return &DeployResult{
			Service:     pkgName,
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
		return nil, fmt.Errorf("pkg build %s: %w", pkgName, buildResults[0].Err)
	}
	tgzPath := buildResults[0].OutputPath
	fmt.Printf("  ✓ Package built (%s)\n", filepath.Base(tgzPath))

	// Resolve CLI path for subprocess calls (publish legacy path + desired-state update).
	globularCLI, err := findGlobularCLI(paths)
	if err != nil {
		return nil, err
	}

	// Publish: upload the built package to the repository.
	if reservationID != "" {
		// Direct upload with reservation — no subprocess needed.
		tgzData, err := os.ReadFile(tgzPath)
		if err != nil {
			return nil, fmt.Errorf("read package: %w", err)
		}
		ref := &repopb.ArtifactRef{
			PublisherId: opts.Publisher,
			Name:        pkgName,
			Version:     opts.Version,
			Platform:    opts.Platform,
			Kind:        repopb.ArtifactKind_SERVICE,
		}
		if err := client.UploadWithReservation(ref, tgzData, nextBuild, reservationID); err != nil {
			return nil, fmt.Errorf("upload with reservation: %w", err)
		}
		fmt.Printf("  ✓ Published to %s (with reservation)\n", repoAddr)
	} else {
		// Legacy path: publish via CLI subprocess.
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
		)
		runPublish := func() (string, error) {
			cmd := exec.CommandContext(ctx, globularCLI, publishArgs...)
			cmd.Dir = paths.Root
			var pubOut strings.Builder
			cmd.Stdout = &pubOut
			cmd.Stderr = &pubOut
			err := cmd.Run()
			return pubOut.String(), err
		}

		var pubOut string
		var errPub error
		for attempt := 1; attempt <= 3; attempt++ {
			pubOut, errPub = runPublish()
			if errPub == nil || isTransientPublishError(errPub, pubOut) == false {
				break
			}
			fmt.Printf("  retrying publish (%d/3) after transient error: %v\n", attempt, errPub)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
		if errPub != nil {
			if !strings.Contains(pubOut, "success") && !strings.Contains(pubOut, "bundle_id") && !strings.Contains(pubOut, "verify uploaded manifest") {
				fmt.Printf("  %s\n", pubOut)
				return nil, fmt.Errorf("pkg publish: %w", errPub)
			}
			fmt.Printf("  (post-upload verify warning — bundle was uploaded)\n")
		}
		fmt.Printf("  ✓ Published to %s\n", repoAddr)
	}

	// ── Step 4: Update desired state ────────────────────────────────────
	fmt.Printf("\n→ Step 4: Updating desired state...\n")

	// Resolve controller leader address. The VIP (10.0.0.100) floats to the
	// current leader, so prefer it over auto-discovered node IPs which may
	// hit a non-leader and get "not leader" redirects.
	controllerAddr := resolveControllerAddr()

	var desiredArgs []string
	if opts.Token != "" {
		desiredArgs = append(desiredArgs, "--token", opts.Token)
	}
	if controllerAddr != "" {
		desiredArgs = append(desiredArgs, "--controller", controllerAddr)
	}
	desiredArgs = append(desiredArgs,
		"services", "desired", "set",
		pkgName, opts.Version,
		"--build-number", fmt.Sprintf("%d", nextBuild),
	)
	updateDesired := func() (string, error) {
		cmd := exec.CommandContext(ctx, globularCLI, desiredArgs...)
		cmd.Dir = paths.Root
		var desiredOut strings.Builder
		cmd.Stdout = &desiredOut
		cmd.Stderr = &desiredOut
		err := cmd.Run()
		return desiredOut.String(), err
	}
	var desiredOut string
	var desiredErr error
	for attempt := 1; attempt <= 3; attempt++ {
		desiredOut, desiredErr = updateDesired()
		if desiredErr == nil || !isTransientDesiredError(desiredErr, desiredOut) {
			break
		}
		// Parse leader address from "not leader" redirect and retry with it.
		if leaderAddr := extractLeaderAddr(desiredOut); leaderAddr != "" {
			fmt.Printf("  redirecting to leader %s\n", leaderAddr)
			// Rebuild args with the leader address.
			desiredArgs = nil
			if opts.Token != "" {
				desiredArgs = append(desiredArgs, "--token", opts.Token)
			}
			desiredArgs = append(desiredArgs, "--controller", leaderAddr)
			desiredArgs = append(desiredArgs,
				"services", "desired", "set",
				pkgName, opts.Version,
				"--build-number", fmt.Sprintf("%d", nextBuild),
			)
		} else {
			fmt.Printf("  retrying desired-state update (%d/3)\n", attempt)
		}
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	if desiredErr != nil {
		fmt.Printf("  ⚠ desired state update failed: %s\n", desiredOut)
		// Non-fatal — the artifact is published, just not auto-rolled out.
	} else {
		fmt.Printf("  ✓ Desired state: %s@%s+%d\n", pkgName, opts.Version, nextBuild)
	}

	// ── Report ──────────────────────────────────────────────────────────
	result := &DeployResult{
		Service:     pkgName,
		Version:     opts.Version,
		BuildNumber: nextBuild,
		BuildID:     allocatedBuildID,
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

func resolveControllerAddr() string {
	// Discover the controller via etcd service registration.
	// If the discovered address routes through the mesh (port 443), the subprocess
	// needs the direct port (12000) instead, since the CLI's --controller flag
	// creates a direct gRPC connection, not a mesh connection.
	addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", "")
	if addr != "" {
		// Replace mesh port 443 with direct controller port 12000.
		if strings.HasSuffix(addr, ":443") {
			addr = strings.TrimSuffix(addr, ":443") + ":12000"
		}
		return addr
	}
	return ""
}

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

// isTransientPublishError classifies publish errors that are worth retrying.
func isTransientPublishError(err error, out string) bool {
	msg := strings.ToLower(out + " " + err.Error())
	return strings.Contains(msg, "unavailable") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "rst_stream") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "try again")
}

// isTransientDesiredError classifies desired-state update errors to retry.
// extractLeaderAddr parses "leader_addr=host:port" from a "not leader" error message.
func extractLeaderAddr(out string) string {
	idx := strings.Index(out, "leader_addr=")
	if idx < 0 {
		return ""
	}
	rest := out[idx+len("leader_addr="):]
	// Find end of address (comma, paren, space, or end of string).
	end := strings.IndexAny(rest, ",) \n")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}

func isTransientDesiredError(err error, out string) bool {
	msg := strings.ToLower(out + " " + err.Error())
	// Leader redirects or transient controller outages.
	return strings.Contains(msg, "unavailable") ||
		strings.Contains(msg, "not leader") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "timeout")
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
