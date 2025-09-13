package pki

import (
	"encoding/pem"
	"os"
	"path/filepath"
)

type FileStorage struct{}

func (FileStorage) PathJoin(elem ...string) string { return filepath.Join(elem...) }
func (FileStorage) Exists(path string) bool        { return exists(path) }

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func writePEM(path string, block *pem.Block, mode os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(block), mode)
}
