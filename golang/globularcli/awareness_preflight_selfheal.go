package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/globulario/services/golang/awareness/assurance"
	"github.com/globulario/services/golang/awareness/preflight"
)

func preflightNeedsSelfHeal(r *preflight.Report) bool {
	if r == nil {
		return false
	}
	if r.SafetyStatus == preflight.SafetyStatusUnknownNotSafe {
		return true
	}
	if r.GraphFreshness != nil && r.GraphFreshness.Stale {
		return true
	}
	if r.Trust != nil {
		if r.Trust.Verdict == assurance.TrustStale {
			return true
		}
		if strings.HasPrefix(string(r.Trust.Freshness), "stale_") {
			return true
		}
	}
	return false
}

func runAwarenessSelfHealBuild(ctx context.Context, repoRoot, dbPath string) error {
	if repoRoot == "" {
		return fmt.Errorf("repo root is required")
	}
	args := []string{"awareness", "build", "--clean", "--repo", repoRoot}
	if strings.TrimSpace(dbPath) != "" {
		args = append(args, "--db", dbPath)
	}
	cmd := exec.CommandContext(ctx, os.Args[0], args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

