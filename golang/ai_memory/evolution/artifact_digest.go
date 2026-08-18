package evolution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// DigestFiles returns one deterministic digest over the ordered contents of all
// supplied files. Each file is length-delimited before hashing so concatenation
// ambiguities cannot produce the same digest.
func DigestFiles(paths ...string) (string, error) {
	h := sha256.New()
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("open proof artifact %q: %w", path, err)
		}
		info, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return "", fmt.Errorf("stat proof artifact %q: %w", path, err)
		}
		if _, err := fmt.Fprintf(h, "%d\x00", info.Size()); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("hash proof artifact length %q: %w", path, err)
		}
		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("hash proof artifact %q: %w", path, err)
		}
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("close proof artifact %q: %w", path, err)
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
