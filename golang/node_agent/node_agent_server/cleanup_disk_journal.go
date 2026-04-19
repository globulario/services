package main

// cleanup_disk_journal.go — Journal vacuum for disk pressure recovery.
//
// Called by the cluster controller's disk invariant enforcement when a node
// is at CRITICAL disk pressure (<5% free). Uses journalctl --vacuum to reclaim
// space consumed by systemd journal logs, which are the most reliably reclaimable
// large files on a Globular node.
//
// This is best-effort: it always returns a result, even if journalctl fails
// or nothing is freed. The caller (invariantRepairDiskSpace) logs the outcome
// as a cluster event and continues — it cannot add physical disk.

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// journalVacuumFreedRe matches journalctl output like:
//
//	"Freed 1.2G, freed 512M, freed 0B"
//	"Vacuuming done, freed 2.3G of archived journals from /run/log/journal."
var journalVacuumFreedRe = regexp.MustCompile(`(?i)freed\s+([\d.]+)\s*([KMGTP]?i?B?)`)

// CleanupDiskJournal runs journalctl --vacuum on this node to reclaim disk
// space consumed by archived systemd journal logs.
//
// Defaults (when request fields are zero):
//   - max_age_days  = 7  (keep last 7 days)
//   - target_size_mb = 0 (no additional size cap beyond age limit)
//
// Idempotent: calling multiple times is safe.
func (srv *NodeAgentServer) CleanupDiskJournal(ctx context.Context, req *node_agentpb.CleanupDiskJournalRequest) (*node_agentpb.CleanupDiskJournalResponse, error) {
	maxAge := int(req.GetMaxAgeDays())
	if maxAge == 0 {
		maxAge = 7 // default: keep 7 days of logs
	}
	targetMB := req.GetTargetSizeMb()
	dryRun := req.GetDryRun()

	slog.Info("disk journal cleanup requested",
		"max_age_days", maxAge,
		"target_size_mb", targetMB,
		"dry_run", dryRun,
	)

	// Build journalctl args. We always use --vacuum-time for the age limit;
	// optionally add --vacuum-size if a target is specified.
	//
	// journalctl --vacuum-time=7d  removes archived journals older than 7 days.
	// journalctl --vacuum-size=1G  removes oldest archived journals until < 1G.
	// Both can be combined — journalctl applies the more aggressive of the two.
	//
	// NOTE: journalctl only vacuums ARCHIVED journals (rotated .journal~ files).
	// The active journal is not touched. This is intentional and safe.
	args := []string{fmt.Sprintf("--vacuum-time=%dd", maxAge)}
	if targetMB > 0 {
		args = append(args, fmt.Sprintf("--vacuum-size=%dM", targetMB))
	}

	var freed uint64
	var summary string

	if dryRun {
		// Dry run: use --disk-usage to show current usage, don't vacuum.
		usageOut, err := exec.CommandContext(ctx, "journalctl", "--disk-usage").Output()
		if err != nil {
			return &node_agentpb.CleanupDiskJournalResponse{
				Ok:      false,
				Message: "dry-run: journalctl --disk-usage failed",
				Error:   err.Error(),
			}, nil
		}
		summary = fmt.Sprintf("[dry-run] journal disk usage: %s", strings.TrimSpace(string(usageOut)))
		return &node_agentpb.CleanupDiskJournalResponse{
			Ok:         true,
			FreedBytes: 0,
			Message:    summary,
		}, nil
	}

	// Run the actual vacuum.
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()

	// Combine stdout + stderr — journalctl writes human output to stderr on
	// some distros, stdout on others.
	combined := outBuf.String() + errBuf.String()

	if err != nil {
		slog.Warn("journalctl vacuum failed", "args", args, "err", err, "output", combined)
		return &node_agentpb.CleanupDiskJournalResponse{
			Ok:      false,
			Message: "journalctl vacuum failed",
			Error:   fmt.Sprintf("%v — %s", err, strings.TrimSpace(combined)[:min(200, len(combined))]),
		}, nil
	}

	// Parse freed bytes from journalctl output.
	freed = parseJournalFreed(combined)

	if freed == 0 {
		summary = "journal vacuum complete — nothing to free or journal already within limits"
	} else {
		summary = fmt.Sprintf("journal vacuum complete — freed %s of archived logs", humanBytes(freed))
	}

	slog.Info("disk journal cleanup complete",
		"freed_bytes", freed,
		"freed_human", humanBytes(freed),
		"args", args,
	)

	return &node_agentpb.CleanupDiskJournalResponse{
		Ok:         true,
		FreedBytes: freed,
		Message:    summary,
	}, nil
}

// parseJournalFreed extracts the total freed bytes from journalctl --vacuum output.
// journalctl prints lines like:
//
//	"Vacuuming done, freed 2.3G of archived journals from /run/log/journal/abc123."
//	"Freed 512M, freed 128M."
//
// Returns 0 if no freed-bytes line is found.
func parseJournalFreed(output string) uint64 {
	var total uint64
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		matches := journalVacuumFreedRe.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) < 3 {
				continue
			}
			n, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				continue
			}
			unit := strings.ToUpper(strings.TrimSuffix(strings.TrimSuffix(m[2], "iB"), "B"))
			var multiplier float64
			switch unit {
			case "K", "KI":
				multiplier = 1024
			case "M", "MI":
				multiplier = 1024 * 1024
			case "G", "GI":
				multiplier = 1024 * 1024 * 1024
			case "T", "TI":
				multiplier = 1024 * 1024 * 1024 * 1024
			default:
				multiplier = 1
			}
			total += uint64(n * multiplier)
		}
	}
	return total
}

// humanBytes formats bytes as a human-readable string.
func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

// min returns the smaller of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
