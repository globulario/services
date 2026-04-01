package main

import (
	"os"
	"strings"
)

// defaultPublisherID returns the publisher ID for desired-state operations.
func defaultPublisherID() string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_DEFAULT_PUBLISHER")); v != "" {
		return v
	}
	return "core@globular.io"
}
