package main

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/config"
)

// ── buildCandidate filtering ──────────────────────────────────────────────────

func TestBuildCandidate_IgnoredFSType(t *testing.T) {
	for fs := range ignoredFSTypes {
		m := mountEntry{device: "/dev/tmpfs", mountPath: "/run/test", fsType: fs}
		dc := buildCandidate("node-1", m)
		if dc != nil {
			t.Errorf("expected nil candidate for fs type %q, got one", fs)
		}
	}
}

func TestBuildCandidate_IgnoredMountPrefix(t *testing.T) {
	for _, prefix := range ignoredMountPrefixes {
		m := mountEntry{device: "/dev/sda1", mountPath: prefix + "/sub", fsType: "ext4"}
		dc := buildCandidate("node-1", m)
		if dc != nil {
			t.Errorf("expected nil candidate for mount path under %q, got one", prefix)
		}
	}
}

func TestBuildCandidate_IgnoredMountPrefixExact(t *testing.T) {
	m := mountEntry{device: "/dev/sda1", mountPath: "/proc", fsType: "proc"}
	dc := buildCandidate("node-1", m)
	if dc != nil {
		t.Error("expected nil candidate for exact ignored mount path /proc")
	}
}

func TestBuildCandidate_LoopDevice_Ignored(t *testing.T) {
	m := mountEntry{device: "/dev/loop0", mountPath: "/snap/core", fsType: "squashfs"}
	dc := buildCandidate("node-1", m)
	if dc != nil {
		t.Error("expected nil candidate for loop device")
	}
}

func TestBuildCandidate_ValidMount_NotNil(t *testing.T) {
	m := mountEntry{device: "/dev/sdb1", mountPath: "/mnt/data", fsType: "ext4"}
	dc := buildCandidate("node-1", m)
	if dc == nil {
		t.Fatal("expected non-nil candidate for valid ext4 mount")
	}
	if dc.NodeID != "node-1" {
		t.Errorf("expected node_id=node-1, got %q", dc.NodeID)
	}
	if dc.MountPath != "/mnt/data" {
		t.Errorf("expected mount_path=/mnt/data, got %q", dc.MountPath)
	}
}

func TestBuildCandidate_RootFS_IsRootTrue(t *testing.T) {
	m := mountEntry{device: "/dev/sda1", mountPath: "/", fsType: "ext4"}
	dc := buildCandidate("node-1", m)
	if dc == nil {
		t.Fatal("root fs should produce a candidate (flagged as IsRoot)")
	}
	if !dc.IsRoot {
		t.Error("expected IsRoot=true for /")
	}
	if dc.Eligible {
		t.Error("root filesystem should be ineligible without --force-root")
	}
}

// ── computeEligibility ────────────────────────────────────────────────────────

func TestComputeEligibility_ExistingDataNoMinioSys_Ineligible(t *testing.T) {
	dc := &config.DiskCandidate{
		SizeBytes:       10 * 1024 * 1024 * 1024,
		HasExistingData: true,
		HasMinioSys:     false,
	}
	elig, reasons := computeEligibility(dc)
	if elig {
		t.Error("expected ineligible for existing non-MinIO data")
	}
	found := false
	for _, r := range reasons {
		if strings.Contains(r, "force-existing-data") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected reason mentioning --force-existing-data, got: %v", reasons)
	}
}

func TestComputeEligibility_MinioSysPresent_Eligible(t *testing.T) {
	dc := &config.DiskCandidate{
		SizeBytes:       10 * 1024 * 1024 * 1024,
		HasExistingData: true,
		HasMinioSys:     true,
	}
	elig, _ := computeEligibility(dc)
	if !elig {
		t.Error("expected eligible when .minio.sys present (prior MinIO deployment)")
	}
}

func TestComputeEligibility_TooSmall_Ineligible(t *testing.T) {
	dc := &config.DiskCandidate{SizeBytes: 512 * 1024 * 1024} // 512 MiB < 1 GiB
	elig, _ := computeEligibility(dc)
	if elig {
		t.Error("expected ineligible for disk smaller than 1 GiB")
	}
}

func TestComputeEligibility_VFat_Ineligible(t *testing.T) {
	dc := &config.DiskCandidate{
		SizeBytes: 10 * 1024 * 1024 * 1024,
		FSType:    "vfat",
	}
	elig, _ := computeEligibility(dc)
	if elig {
		t.Error("expected ineligible for vfat filesystem")
	}
}

// ── stripPartitionSuffix ──────────────────────────────────────────────────────

func TestStripPartitionSuffix(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"sda1", "sda"},
		{"sdb3", "sdb"},
		{"nvme0n1p2", "nvme0n1"},
		{"nvme1n1p3", "nvme1n1"},
		{"mmcblk0p1", "mmcblk0"},
		{"sda", "sda"},
		{"nvme0n1", "nvme0n1"},
	}
	for _, c := range cases {
		got := stripPartitionSuffix(c.in)
		if got != c.out {
			t.Errorf("stripPartitionSuffix(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}
