// mount_detect.go — detect whether a path is on an external mount (different device than /).
package main

import (
	"syscall"
)

// isExternalMount returns true when path resides on a different block device
// than the root filesystem.  This is used by AddPublicDir to auto-detect
// NTFS/Samba/NFS mounts so they can be classified as PUBLIC_DIR_EXTERNAL.
func isExternalMount(path string) bool {
	var rootStat, pathStat syscall.Stat_t
	if err := syscall.Stat("/", &rootStat); err != nil {
		return false
	}
	if err := syscall.Stat(path, &pathStat); err != nil {
		return false
	}
	return rootStat.Dev != pathStat.Dev
}
