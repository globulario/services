// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions
// @awareness file_role=native_dependency_install_actions
// @awareness risk=medium
package actions

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// ParseLDDOutput parses the text output of `ldd <binary>` and returns the
// names of shared libraries reported as "not found". Exported for testing.
//
// ldd output format (per library line):
//
//	libodbc.so.2 => not found
//	libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f...)
//	linux-vdso.so.1 (0x00007f...)
func ParseLDDOutput(output string) []string {
	var missing []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "=> not found") {
			continue
		}
		// Extract library name: "libodbc.so.2 => not found" → "libodbc.so.2"
		parts := strings.SplitN(trimmed, "=>", 2)
		if len(parts) != 2 {
			continue
		}
		lib := strings.TrimSpace(parts[0])
		if lib != "" {
			missing = append(missing, lib)
		}
	}
	return missing
}

// MissingNativeLibs runs ldd on the given ELF binary and returns the names
// of shared libraries not resolvable on the current system. Returns nil (not
// an error) when ldd is not available — the check is best-effort.
func MissingNativeLibs(ctx context.Context, binaryPath string) ([]string, error) {
	lddPath, err := exec.LookPath("ldd")
	if err != nil {
		// ldd not available (e.g. musl-libc system, stripped environment).
		// Skip the check rather than blocking installation.
		return nil, nil
	}
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, lddPath, binaryPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	// ldd exits non-zero when the binary is not a valid ELF or has missing libs.
	// We ignore the exit code and parse output instead.
	_ = cmd.Run()
	return ParseLDDOutput(out.String()), nil
}

// ── package.check_native_deps action ─────────────────────────────────────────
//
// Verifies that all shared libraries required by a service binary are present
// on the current system. Fails with a clear error listing missing libraries
// so that install steps fail immediately rather than leading to crash-loops.
//
// Args:
//
//	binary_path (string, required) — absolute path to the ELF binary to check
//	name        (string, optional) — package name for logging
type nativeDepsCheckAction struct{}

func (nativeDepsCheckAction) Name() string { return "package.check_native_deps" }

func (nativeDepsCheckAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return fmt.Errorf("package.check_native_deps: args required")
	}
	if strings.TrimSpace(args.GetFields()["binary_path"].GetStringValue()) == "" {
		return fmt.Errorf("package.check_native_deps: binary_path is required")
	}
	return nil
}

func (nativeDepsCheckAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	binaryPath := strings.TrimSpace(fields["binary_path"].GetStringValue())
	name := strings.TrimSpace(fields["name"].GetStringValue())
	if name == "" {
		name = binaryPath
	}

	missing, err := MissingNativeLibs(ctx, binaryPath)
	if err != nil {
		// ldd invocation failed — log and skip, don't block installation
		log.Printf("check_native_deps: ldd check for %s failed: %v (skipping)", name, err)
		return "ldd check skipped (error)", nil
	}
	if len(missing) == 0 {
		return fmt.Sprintf("all native dependencies satisfied for %s", name), nil
	}
	return "", fmt.Errorf(
		"NATIVE_LIBRARY_DEPENDENCY_MISSING: %s requires native libraries not installed on this node: %v — install the OS packages providing them (e.g. for libodbc.so.2: apt install unixodbc)",
		name, missing,
	)
}

func init() {
	Register(nativeDepsCheckAction{})
}
