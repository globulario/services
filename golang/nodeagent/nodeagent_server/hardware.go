package main

import (
	"runtime"
	"syscall"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

// buildNodeCapabilities collects hardware stats from the local machine.
// CPU count uses runtime.NumCPU (logical cores).
// RAM and disk use syscall to avoid external dependencies.
func buildNodeCapabilities() *clustercontrollerpb.NodeCapabilities {
	caps := &clustercontrollerpb.NodeCapabilities{
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

	return caps
}
