package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// canonicalServiceName normalizes various representations (globular-*, *.service) to a canonical key.
func canonicalServiceName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.TrimPrefix(n, "globular-")
	n = strings.TrimSuffix(n, ".service")
	return n
}

// serviceUnitForCanonical returns the systemd unit name for a canonical service.
func serviceUnitForCanonical(svc string) string {
	switch svc {
	case "envoy":
		return "envoy.service"
	default:
		return fmt.Sprintf("globular-%s.service", svc)
	}
}

func hashDesiredServiceVersions(versions map[string]string) string {
	if len(versions) == 0 {
		return ""
	}
	keys := make([]string, 0, len(versions))
	for k := range versions {
		keys = append(keys, k)
	}
	// deterministic order
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(versions[k])
		b.WriteString(";")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// stableServiceDesiredHash returns a non-empty hash for the canonical service map.
func stableServiceDesiredHash(versions map[string]string) string {
	base := hashDesiredServiceVersions(versions)
	if base == "" {
		return "services:none"
	}
	return "services:" + base
}
