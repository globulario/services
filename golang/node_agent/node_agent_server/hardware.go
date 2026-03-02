package main

import (
	"os"
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

	caps.CanApplyPrivileged = canApplyPrivileged()

	return caps
}

// canApplyPrivileged returns true when the node-agent process can write
// systemd unit files and manage services. This is the case when running
// as root or when the user has write access to the systemd directory.
func canApplyPrivileged() bool {
	if os.Geteuid() == 0 {
		return true
	}
	testPath := filepath.Join("/etc/systemd/system", ".globular-probe")
	f, err := os.Create(testPath)
	if err == nil {
		f.Close()
		os.Remove(testPath)
		return true
	}
	return false
}
