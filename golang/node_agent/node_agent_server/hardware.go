package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// buildNodeCapabilities collects hardware stats from the local machine.
// CPU count uses runtime.NumCPU (logical cores).
// RAM and disk use syscall to avoid external dependencies.
func buildNodeCapabilities() *cluster_controllerpb.NodeCapabilities {
	caps := &cluster_controllerpb.NodeCapabilities{
		CpuCount: uint32(runtime.NumCPU()),
	}

	// Total RAM via Sysinfo (Linux)
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err == nil {
		unit := uint64(si.Unit)
		if unit == 0 {
			unit = 1
		}
		caps.RamBytes = si.Totalram * unit
	}

	// Disk capacity and free space on root volume
	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err == nil {
		bsize := uint64(fs.Bsize)
		caps.DiskBytes = fs.Blocks * bsize
		caps.DiskFreeBytes = fs.Bfree * bsize
	}

	can, reason := canApplyPrivileged()
	caps.CanApplyPrivileged = can
	caps.PrivilegeReason = reason

	return caps
}

// canApplyPrivileged returns true when the node-agent process can write
// systemd unit files and manage services. This is the case when:
//   - running as root, or
//   - the user has write access to the systemd directory, or
//   - the user has sudo access to systemctl (sudoers rules for globular user)
func canApplyPrivileged() (bool, string) {
	if os.Geteuid() == 0 {
		return true, "running as root"
	}
	// Direct write access to systemd directory.
	testPath := filepath.Join("/etc/systemd/system", ".globular-probe")
	f, err := os.Create(testPath)
	if err == nil {
		f.Close()
		os.Remove(testPath)
		return true, "direct systemd write access"
	}
	// sudo access to systemctl (the globular user has sudoers rules).
	if err := exec.Command("sudo", "-n", "systemctl", "is-system-running").Run(); err == nil {
		return true, "sudo access"
	}
	return false, fmt.Sprintf("euid=%d, systemd write denied, sudo -n failed", os.Geteuid())
}
