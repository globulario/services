package v1alpha1

import (
	"fmt"
	"log"

	"github.com/globulario/services/golang/config"
)

// EnableMinIOFetcher configures the package-level MinIOFetcher to read workflow
// definitions from globular-config/workflows/ in MinIO. Services should call
// this at startup to make MinIO the authoritative source for workflow YAMLs,
// with local disk as a fallback.
func EnableMinIOFetcher() {
	MinIOFetcher = func(name string) ([]byte, error) {
		if name == "" {
			return nil, fmt.Errorf("workflow name is empty")
		}
		key := "workflows/" + name + ".yaml"
		data, err := config.GetClusterConfig(key)
		if err != nil {
			return nil, fmt.Errorf("minio get %s: %w", key, err)
		}
		if data == nil {
			return nil, fmt.Errorf("workflow %q not found in MinIO", name)
		}
		log.Printf("workflow: loaded %q from MinIO (%d bytes)", name, len(data))
		return data, nil
	}
}
