package config

import (
	"fmt"
	"strings"
)

// RenderMinioEnv renders the content of /var/lib/globular/minio/minio.env
// from the authoritative ObjectStoreDesiredState published by the controller.
//
// Volume URL rules (4 cases):
//   - standalone (1 node, drivesPerNode < 2):  MINIO_VOLUMES=/base/data
//   - distributed (N nodes, drivesPerNode < 2): MINIO_VOLUMES=https://IP1:9000/base/data https://IP2:9000/base/data ...
//   - standalone multi-drive (1 node, drivesPerNode ≥ 2): MINIO_VOLUMES=/base/data1 /base/data2 ...
//   - distributed multi-drive (N nodes, drivesPerNode ≥ 2): MINIO_VOLUMES=https://IP1:9000/base/data1 https://IP1:9000/base/data2 ...
//
// MINIO_CI_CD=1 is set for distributed mode to bypass root-drive checks.
//
// This function is byte-identical to renderMinioConfig in the cluster controller.
// Both must be kept in sync — regression tests in minio_runtime_render_test.go enforce this.
func RenderMinioEnv(state *ObjectStoreDesiredState) string {
	if state == nil || len(state.Nodes) == 0 {
		return ""
	}

	poolIPs := state.Nodes
	drivesPerNode := state.DrivesPerNode

	minioBasePath := func(ip string) string {
		if state.NodePaths != nil {
			if p, ok := state.NodePaths[ip]; ok && p != "" {
				return strings.TrimRight(p, "/")
			}
		}
		return "/var/lib/globular/minio"
	}

	var sb strings.Builder

	if drivesPerNode < 2 {
		if len(poolIPs) == 1 {
			// Standalone: local path only.
			sb.WriteString(fmt.Sprintf("MINIO_VOLUMES=%s/data\n", minioBasePath(poolIPs[0])))
		} else {
			// Distributed: ordered endpoints from pool list.
			var endpoints []string
			for _, ip := range poolIPs {
				endpoints = append(endpoints, fmt.Sprintf("https://%s:9000%s/data", ip, minioBasePath(ip)))
			}
			sb.WriteString(fmt.Sprintf("MINIO_VOLUMES=%s\n", strings.Join(endpoints, " ")))
		}
	} else {
		if len(poolIPs) == 1 {
			// Single node with multiple drives — standalone erasure mode.
			base := minioBasePath(poolIPs[0])
			var drives []string
			for d := 1; d <= drivesPerNode; d++ {
				drives = append(drives, fmt.Sprintf("%s/data%d", base, d))
			}
			sb.WriteString(fmt.Sprintf("MINIO_VOLUMES=%s\n", strings.Join(drives, " ")))
		} else {
			// Distributed multi-drive.
			var endpoints []string
			for _, ip := range poolIPs {
				base := minioBasePath(ip)
				for d := 1; d <= drivesPerNode; d++ {
					endpoints = append(endpoints, fmt.Sprintf("https://%s:9000%s/data%d", ip, base, d))
				}
			}
			sb.WriteString(fmt.Sprintf("MINIO_VOLUMES=%s\n", strings.Join(endpoints, " ")))
		}
	}

	// Credentials.
	if state.AccessKey != "" {
		sb.WriteString(fmt.Sprintf("MINIO_ROOT_USER=%s\n", state.AccessKey))
		sb.WriteString(fmt.Sprintf("MINIO_ROOT_PASSWORD=%s\n", state.SecretKey))
	} else {
		sb.WriteString("MINIO_ROOT_USER=minioadmin\n")
		sb.WriteString("MINIO_ROOT_PASSWORD=minioadmin\n")
	}

	// Bypass root-drive check for distributed mode.
	if len(poolIPs) > 1 {
		sb.WriteString("MINIO_CI_CD=1\n")
	}

	return sb.String()
}

// RenderMinioSystemdOverride renders the content of
// /etc/systemd/system/globular-minio.service.d/distributed.conf
// for the given nodeIP.
//
// Returns ("", false) when no override is needed (standalone single-drive).
// Returns (content, true) for distributed mode or multi-drive standalone.
//
// The override:
//  1. Creates per-drive directories via ExecStartPre.
//  2. Replaces ExecStart to use $MINIO_VOLUMES from the env file.
//
// This function is byte-identical to renderMinioSystemdOverride in the controller.
// Regression tests in minio_runtime_render_test.go enforce this invariant.
func RenderMinioSystemdOverride(state *ObjectStoreDesiredState, nodeIP string) (string, bool) {
	if state == nil || len(state.Nodes) == 0 || nodeIP == "" {
		return "", false
	}

	poolIPs := state.Nodes
	drivesPerNode := state.DrivesPerNode

	// Only generate the override for distributed or multi-drive mode.
	if len(poolIPs) <= 1 && drivesPerNode < 2 {
		return "", false
	}

	basePath := "/var/lib/globular/minio"
	if state.NodePaths != nil {
		if p, ok := state.NodePaths[nodeIP]; ok && p != "" {
			basePath = strings.TrimRight(p, "/")
		}
	}

	var sb strings.Builder
	sb.WriteString("# Managed by Globular cluster controller — do not edit manually.\n")
	sb.WriteString("[Service]\n")

	// ExecStartPre: create and chown drive directories.
	if drivesPerNode >= 2 {
		for d := 1; d <= drivesPerNode; d++ {
			dir := fmt.Sprintf("%s/data%d", basePath, d)
			sb.WriteString(fmt.Sprintf("ExecStartPre=+/usr/bin/mkdir -p %s\n", dir))
			sb.WriteString(fmt.Sprintf("ExecStartPre=+/usr/bin/chown globular:globular %s\n", dir))
		}
	} else {
		dir := basePath + "/data"
		sb.WriteString(fmt.Sprintf("ExecStartPre=+/usr/bin/mkdir -p %s\n", dir))
		sb.WriteString(fmt.Sprintf("ExecStartPre=+/usr/bin/chown globular:globular %s\n", dir))
	}

	// Clear original ExecStart and replace with $MINIO_VOLUMES.
	sb.WriteString("ExecStart=\n")
	sb.WriteString(fmt.Sprintf("ExecStart=/usr/lib/globular/bin/minio server $MINIO_VOLUMES --address %s:9000 --console-address %s:9001\n", nodeIP, nodeIP))

	return sb.String(), true
}
