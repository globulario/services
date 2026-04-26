package main

// disk_inventory.go — read-only disk discovery for the node agent.
//
// The node agent scans mounted filesystems and reports facts about each mount
// to etcd under /globular/nodes/{node_id}/storage/candidates/{disk_id}.
//
// HARD INVARIANTS (enforced in code, never softened):
//   - This file NEVER writes to /globular/objectstore/config or any
//     objectstore desired-state key.
//   - This file NEVER selects, formats, partitions, or initialises a disk.
//   - This file NEVER removes .minio.sys.
//   - Eligibility is computed as a recommendation only; the operator decides.
//
// Called every syncTicker interval, just like reconcileMinioSystemdConfig.

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/globulario/services/golang/config"
)

// minDiskSizeBytes is the minimum usable size to consider a disk eligible.
const minDiskSizeBytes = 1 * 1024 * 1024 * 1024 // 1 GiB

// ignoredFSTypes are pseudo/kernel filesystems that are never MinIO targets.
var ignoredFSTypes = map[string]bool{
	"tmpfs":     true,
	"proc":      true,
	"sysfs":     true,
	"devtmpfs":  true,
	"devpts":    true,
	"cgroup":    true,
	"cgroup2":   true,
	"pstore":    true,
	"securityfs": true,
	"debugfs":   true,
	"fusectl":   true,
	"overlay":   true,
	"squashfs":  true,
	"aufs":      true,
	"fuse":      true,
	"hugetlbfs": true,
	"mqueue":    true,
	"bpf":       true,
	"tracefs":   true,
	"ramfs":     true,
	"rootfs":    true,
}

// ignoredMountPrefixes are mount paths that are never MinIO targets.
var ignoredMountPrefixes = []string{
	"/proc",
	"/sys",
	"/dev",
	"/run",
	"/boot",
	"/snap",
	"/sys/fs",
	"/var/lib/docker",
	"/var/lib/containers",
}

// reconcileDiskInventory scans local mounts and writes candidates to etcd.
// It is the ONLY place disk data is observed. No decisions are made here.
func (srv *NodeAgentServer) reconcileDiskInventory(ctx context.Context) {
	if srv.nodeID == "" {
		return
	}

	candidates, err := scanDiskCandidates(srv.nodeID)
	if err != nil {
		log.Printf("disk-inventory: scan failed: %v", err)
		return
	}

	// Write each candidate to etcd.
	activeDiskIDs := make(map[string]bool, len(candidates))
	for _, dc := range candidates {
		activeDiskIDs[dc.DiskID] = true
		if err := config.SaveDiskCandidate(ctx, dc); err != nil {
			log.Printf("disk-inventory: write %s: %v", dc.DiskID, err)
		}
	}

	// Remove stale entries for disks that are no longer mounted.
	if err := config.DeleteStaleNodeCandidates(ctx, srv.nodeID, activeDiskIDs); err != nil {
		log.Printf("disk-inventory: prune stale: %v", err)
	}
}

// scanDiskCandidates reads /proc/mounts and builds a DiskCandidate for each
// mounted filesystem. Returns observations only — never makes changes.
func scanDiskCandidates(nodeID string) ([]*config.DiskCandidate, error) {
	mounts, err := readProcMounts()
	if err != nil {
		return nil, fmt.Errorf("read /proc/mounts: %w", err)
	}

	var out []*config.DiskCandidate
	for _, m := range mounts {
		dc := buildCandidate(nodeID, m)
		if dc == nil {
			continue
		}
		out = append(out, dc)
	}
	return out, nil
}

// mountEntry is a parsed line from /proc/mounts.
type mountEntry struct {
	device    string
	mountPath string
	fsType    string
}

func readProcMounts() ([]mountEntry, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []mountEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		entries = append(entries, mountEntry{
			device:    fields[0],
			mountPath: fields[1],
			fsType:    fields[2],
		})
	}
	return entries, scanner.Err()
}

func buildCandidate(nodeID string, m mountEntry) *config.DiskCandidate {
	// Skip pseudo filesystems.
	if ignoredFSTypes[m.fsType] {
		return nil
	}
	// Skip kernel/system paths.
	for _, prefix := range ignoredMountPrefixes {
		if m.mountPath == prefix || strings.HasPrefix(m.mountPath, prefix+"/") {
			return nil
		}
	}
	// Skip loop devices (snap, docker layers, etc.)
	if strings.HasPrefix(m.device, "/dev/loop") {
		return nil
	}

	dc := &config.DiskCandidate{
		NodeID:     nodeID,
		Device:     m.device,
		MountPath:  m.mountPath,
		FSType:     m.fsType,
		ReportedAt: time.Now().UTC(),
	}

	// Stable ID: try blkid UUID via symlink, fall back to hash.
	dc.StableID = resolveStableID(m.device)
	if dc.StableID != "" {
		dc.DiskID = dc.StableID
	} else {
		dc.DiskID = config.DiskIDFromPath(m.device, m.mountPath)
	}

	// Filesystem sizes.
	var fs syscall.Statfs_t
	if err := syscall.Statfs(m.mountPath, &fs); err == nil {
		bsize := uint64(fs.Bsize)
		dc.SizeBytes = int64(fs.Blocks * bsize)
		dc.AvailableBytes = int64(fs.Bavail * bsize)
	}

	// Root filesystem flag.
	dc.IsRoot = (m.mountPath == "/")

	// Removable flag: check /sys/block/{dev}/removable.
	dc.IsRemovable = isRemovable(m.device)

	// MinIO and data presence flags.
	dc.HasMinioSys = hasMinioSys(m.mountPath)
	dc.HasExistingData = hasExistingData(m.mountPath)

	// Compute eligibility.
	dc.Eligible, dc.Reasons = computeEligibility(dc)

	return dc
}

// computeEligibility decides whether a disk is a good MinIO candidate.
// Returns (eligible, reasons). Ineligible disks can still be force-admitted.
func computeEligibility(dc *config.DiskCandidate) (bool, []string) {
	var reasons []string
	eligible := true

	if dc.SizeBytes < minDiskSizeBytes {
		reasons = append(reasons, fmt.Sprintf("too small: %d bytes (minimum 1 GiB)", dc.SizeBytes))
		eligible = false
	}
	if dc.IsRoot {
		reasons = append(reasons, "root filesystem — requires --force-root to admit")
		eligible = false
	}
	if dc.IsRemovable {
		reasons = append(reasons, "removable device — not suitable for durable storage")
		eligible = false
	}
	if dc.FSType == "vfat" || dc.FSType == "ntfs" || dc.FSType == "exfat" {
		reasons = append(reasons, "filesystem type "+dc.FSType+" is not suitable for MinIO")
		eligible = false
	}
	if dc.HasExistingData && !dc.HasMinioSys {
		reasons = append(reasons, "contains existing non-MinIO data — requires --force-existing-data to admit")
		eligible = false
	}
	if dc.HasMinioSys {
		reasons = append(reasons, "has existing .minio.sys (prior MinIO deployment — may be reused)")
	}
	if eligible && len(reasons) == 0 {
		reasons = append(reasons, "eligible for MinIO data path")
	}
	return eligible, reasons
}

// ── filesystem probes ─────────────────────────────────────────────────────────

// hasMinioSys checks for .minio.sys in the mount's data directories.
// Checks: <mount>/data/.minio.sys, <mount>/data1/.minio.sys, <mount>/.minio.sys
func hasMinioSys(mountPath string) bool {
	candidates := []string{
		filepath.Join(mountPath, ".minio.sys"),
		filepath.Join(mountPath, "data", ".minio.sys"),
		filepath.Join(mountPath, "data1", ".minio.sys"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// hasExistingData checks if the mount contains any files that aren't just
// .minio.sys (indicating prior non-MinIO use or unrelated data).
func hasExistingData(mountPath string) bool {
	entries, err := os.ReadDir(mountPath)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		// These are MinIO-only or innocuous entries.
		if name == ".minio.sys" || name == "lost+found" || name == ".Trash-0" {
			continue
		}
		// Any other entry counts as existing data.
		return true
	}
	return false
}

// isRemovable checks /sys/block/{dev}/removable for the block device.
func isRemovable(device string) bool {
	// Extract base device name from /dev/sda, /dev/nvme0n1p3 → sda, nvme0n1
	base := filepath.Base(device)
	// Strip partition suffix: sda1 → sda, nvme0n1p3 → nvme0n1
	blockDev := stripPartitionSuffix(base)
	removablePath := "/sys/block/" + blockDev + "/removable"
	data, err := os.ReadFile(removablePath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

// stripPartitionSuffix returns the block device name without partition number.
// sda3 → sda, nvme0n1p2 → nvme0n1, mmcblk0p1 → mmcblk0
func stripPartitionSuffix(dev string) string {
	// nvme and mmcblk use "p" before the partition number
	if strings.Contains(dev, "nvme") || strings.Contains(dev, "mmcblk") {
		if idx := strings.LastIndex(dev, "p"); idx > 0 {
			suffix := dev[idx+1:]
			allDigits := true
			for _, c := range suffix {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits && len(suffix) > 0 {
				return dev[:idx]
			}
		}
		return dev
	}
	// sda, sdb, etc: strip trailing digits
	i := len(dev)
	for i > 0 && dev[i-1] >= '0' && dev[i-1] <= '9' {
		i--
	}
	if i < len(dev) && i > 0 {
		return dev[:i]
	}
	return dev
}

// resolveStableID attempts to find the blkid UUID for a device by scanning
// /dev/disk/by-uuid/ symlinks. Returns "" if not found.
func resolveStableID(device string) string {
	uuidDir := "/dev/disk/by-uuid"
	entries, err := os.ReadDir(uuidDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		link := filepath.Join(uuidDir, e.Name())
		target, err := os.Readlink(link)
		if err != nil {
			continue
		}
		// Target is relative: ../../sda1 → /dev/sda1
		if !filepath.IsAbs(target) {
			target = filepath.Join(uuidDir, target)
			target = filepath.Clean(target)
		}
		if target == device {
			return e.Name()
		}
	}
	return ""
}
