package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

var repoExplainPackageJSON bool

var repoExplainPackageCmd = &cobra.Command{
	Use:   "explain-package <name>",
	Short: "Cross-layer version authority diagnosis for a package",
	Long: `explain-package shows the 4-layer state for a package and flags any
version authority violations — where desired state requests a version the
repository never built.

Layers reported:
  Layer 1  Repository   — which versions exist and are installable
  Layer 2  Desired      — what the controller wants installed
  Layer 3  Installed    — what each node reports as installed
  Layer 4  Runtime      — which nodes are failing (unit not active)

A CRITICAL banner is printed when the desired version is absent from the
repository — the root cause of the platform_release stamp bug that causes
install-storms (controller asks nodes to install a version that doesn't exist).

Remediation hint: never delete desired state — roll forward to a version the
repository actually has.

Examples:
  globular repository explain-package storage
  globular repository explain-package dns --json
`,
	Args: cobra.ExactArgs(1),
	RunE: runRepoExplainPackage,
}

func init() {
	repoCmd.AddCommand(repoExplainPackageCmd)
	repoExplainPackageCmd.Flags().BoolVar(&repoExplainPackageJSON, "json", false, "Output as JSON")
}

func runRepoExplainPackage(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	if name == "" {
		return fmt.Errorf("package name required")
	}

	resolveRepositoryAddr(cmd)
	conn, err := dialRepository()
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer conn.Close()

	client := repopb.NewPackageRepositoryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	rsp, err := client.DescribePackage(ctx, &repopb.DescribePackageRequest{Name: name})
	if err != nil {
		return fmt.Errorf("DescribePackage: %w", err)
	}
	info := rsp.GetInfo()
	if info == nil {
		return fmt.Errorf("no info returned for package %q", name)
	}

	if repoExplainPackageJSON {
		return explainPackageJSON(info)
	}
	explainPackagePrint(info)
	return nil
}

// versionAuthorityViolation returns the desired version when it is absent from
// the repository's known versions, or "" if no violation.
func versionAuthorityViolation(info *repopb.PackageInfo) string {
	d := info.GetDesired()
	if d == nil || !d.GetPresent() || d.GetVersion() == "" {
		return ""
	}
	desiredVer := d.GetVersion()
	for _, v := range info.GetVersions() {
		if v == desiredVer {
			return ""
		}
	}
	return desiredVer
}

func explainPackagePrint(info *repopb.PackageInfo) {
	fmt.Printf("package:  %s  (%s)\n", info.GetName(), info.GetKind().String())
	fmt.Printf("publisher: %s\n", info.GetPublisher())
	fmt.Println()

	// ── Layer 1: Repository ───────────────────────────────────────────────
	fmt.Println("LAYER 1  Repository")
	versions := info.GetVersions()
	if len(versions) == 0 {
		fmt.Println("  ⚠ no published versions found in repository")
	} else {
		fmt.Printf("  versions: %s\n", strings.Join(versions, ", "))
		if lv := info.GetLatestVersion(); lv != "" {
			fmt.Printf("  latest:   %s\n", lv)
		}
	}

	// ── Layer 2: Desired ──────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("LAYER 2  Desired")
	d := info.GetDesired()
	if d != nil && d.GetPresent() {
		fmt.Printf("  version:    %s  (generation %d)\n", d.GetVersion(), d.GetGeneration())
	} else {
		fmt.Println("  <no desired state — package is unmanaged>")
	}

	// Version authority check
	if badVer := versionAuthorityViolation(info); badVer != "" {
		fmt.Println()
		fmt.Println("  ╔══════════════════════════════════════════════════════╗")
		fmt.Println("  ║  CRITICAL: VERSION AUTHORITY VIOLATION               ║")
		fmt.Printf("  ║  Desired version %s is NOT in the repository.      \n", badVer)
		fmt.Println("  ║  This package will NEVER converge — the controller  ║")
		fmt.Println("  ║  is asking nodes to install a version that was       ║")
		fmt.Println("  ║  never built or published.                           ║")
		fmt.Println("  ║                                                      ║")
		fmt.Println("  ║  Likely cause: platform_release stamp applied to an  ║")
		fmt.Println("  ║  unchanged package (gen-version.sh / CI pipeline).   ║")
		fmt.Println("  ║                                                      ║")
		fmt.Println("  ║  Fix: roll desired state FORWARD to a repository     ║")
		fmt.Println("  ║  version (do NOT delete desired state):              ║")
		if len(versions) > 0 {
			fmt.Printf("  ║    globular services desired set %s <version>\n", info.GetName())
			fmt.Printf("  ║  Available: %s\n", strings.Join(versions, ", "))
		}
		fmt.Println("  ╚══════════════════════════════════════════════════════╝")
	}

	// ── Layer 3: Installed ────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("LAYER 3  Installed")
	installed := info.GetInstalledOn()
	if len(installed) == 0 {
		fmt.Println("  not installed on any node")
	} else {
		desiredVer := ""
		if d != nil && d.GetPresent() {
			desiredVer = d.GetVersion()
		}
		for _, n := range installed {
			drift := ""
			if desiredVer != "" && n.GetVersion() != desiredVer {
				drift = "  ← DRIFT"
			}
			fmt.Printf("  ✓ %s  %s  %s%s\n",
				shortNode(n.GetNodeId()), n.GetVersion(), n.GetStatus(), drift)
		}
	}

	// ── Layer 4: Runtime (failing nodes) ──────────────────────────────────
	fmt.Println()
	fmt.Println("LAYER 4  Runtime")
	failing := info.GetFailingOn()
	if len(failing) == 0 {
		if len(installed) > 0 {
			fmt.Printf("  ✓ all %d installed node(s) healthy\n", len(installed))
		} else {
			fmt.Println("  <no installed nodes to check>")
		}
	} else {
		for _, n := range failing {
			fmt.Printf("  ✕ %s  %s  %s\n",
				shortNode(n.GetNodeId()), n.GetVersion(), n.GetStatus())
		}
	}

	// ── Alignment summary ─────────────────────────────────────────────────
	if d != nil && d.GetPresent() && versionAuthorityViolation(info) == "" {
		fmt.Println()
		desiredVer := d.GetVersion()
		var drifting []string
		for _, n := range installed {
			if n.GetVersion() != desiredVer {
				drifting = append(drifting, shortNode(n.GetNodeId())+"@"+n.GetVersion())
			}
		}
		if len(drifting) > 0 {
			fmt.Printf("⚠ drift: %d node(s) not at desired %s: %s\n",
				len(drifting), desiredVer, strings.Join(drifting, ", "))
		} else if len(installed) > 0 {
			fmt.Printf("✓ all %d node(s) at desired %s\n", len(installed), desiredVer)
		}
	}
}

func explainPackageJSON(info *repopb.PackageInfo) error {
	d := info.GetDesired()
	desired := map[string]interface{}{"present": false}
	if d != nil && d.GetPresent() {
		desired = map[string]interface{}{
			"present":    true,
			"version":    d.GetVersion(),
			"generation": d.GetGeneration(),
			"publisher":  d.GetPublisher(),
		}
	}

	installed := make([]map[string]interface{}, 0, len(info.GetInstalledOn()))
	for _, n := range info.GetInstalledOn() {
		installed = append(installed, map[string]interface{}{
			"node_id": n.GetNodeId(),
			"version": n.GetVersion(),
			"status":  n.GetStatus(),
		})
	}
	failing := make([]map[string]interface{}, 0, len(info.GetFailingOn()))
	for _, n := range info.GetFailingOn() {
		failing = append(failing, map[string]interface{}{
			"node_id": n.GetNodeId(),
			"version": n.GetVersion(),
			"status":  n.GetStatus(),
		})
	}

	vav := versionAuthorityViolation(info)
	out := map[string]interface{}{
		"name":                       info.GetName(),
		"kind":                       info.GetKind().String(),
		"publisher":                  info.GetPublisher(),
		"repository_versions":        info.GetVersions(),
		"latest_version":             info.GetLatestVersion(),
		"desired":                    desired,
		"installed_on":               installed,
		"failing_on":                 failing,
		"version_authority_violation": vav != "",
	}
	if vav != "" {
		out["violated_desired_version"] = vav
		out["violation_hint"] = "desired version was never built — roll forward, do not delete desired state"
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
