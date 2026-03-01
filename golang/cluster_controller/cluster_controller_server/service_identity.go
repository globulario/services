package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/globulario/services/golang/identity"
)

// canonicalServiceName normalizes any service identifier to the canonical kebab-case key.
// Delegates to the shared identity registry for accurate cross-dialect mapping.
func canonicalServiceName(name string) string {
	key, _ := identity.NormalizeServiceKey(name)
	return key
}

// serviceUnitForCanonical returns the systemd unit name for a canonical service key.
func serviceUnitForCanonical(svc string) string {
	return identity.MustIdentityByKey(svc).UnitName
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
