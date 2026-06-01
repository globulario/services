package main

import "strings"

// lookupInstalledVersionFromMap searches the installed-versions map for a
// version matching svc. It tries exact match first, then fuzzy match by
// stripping publisher prefix and canonicalizing service names.
func lookupInstalledVersionFromMap(installed map[string]string, svc string) string {
	if len(installed) == 0 {
		return ""
	}
	// Exact match.
	if v := strings.TrimSpace(installed[svc]); v != "" {
		return v
	}
	// Fuzzy match: strip publisher prefix and canonicalize.
	canon := canonicalServiceName(svc)
	for k, v := range installed {
		parts := strings.SplitN(k, "/", 2)
		candidate := k
		if len(parts) == 2 {
			candidate = parts[1]
		}
		if canonicalServiceName(candidate) == canon {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
