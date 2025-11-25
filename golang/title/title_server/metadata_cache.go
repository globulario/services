package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/protobuf/encoding/protojson"
)

func saveVideoMetadataCache(abs string, raw []byte) error {
	return saveMetadataCache(abs, raw)
}

func loadVideoMetadataCache(abs string) (*titlepb.Video, error) {
	data, err := loadMetadataCache(abs)
	if err != nil {
		return nil, err
	}
	video := new(titlepb.Video)
	if err := protojson.Unmarshal(data, video); err != nil {
		return nil, err
	}
	return video, nil
}

func saveTitleMetadataCache(abs string, raw []byte) error {
	return saveMetadataCache(abs, raw)
}

func loadTitleMetadataCache(abs string) (*titlepb.Title, error) {
	data, err := loadMetadataCache(abs)
	if err != nil {
		return nil, err
	}
	title := new(titlepb.Title)
	if err := protojson.Unmarshal(data, title); err != nil {
		return nil, err
	}
	return title, nil
}

func saveMetadataCache(abs string, raw []byte) error {
	if abs == "" || len(raw) == 0 {
		return fmt.Errorf("invalid metadata cache request")
	}
	path := metadataCachePath(abs)
	if path == "" {
		return fmt.Errorf("invalid cache path for %s", abs)
	}
	if err := Utility.CreateIfNotExists(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, raw, 0o664); err != nil {
		return err
	}
	return nil
}

func loadMetadataCache(abs string) ([]byte, error) {
	if abs == "" {
		return nil, fmt.Errorf("invalid cache load request")
	}
	path := metadataCachePath(abs)
	if path == "" {
		return nil, fmt.Errorf("invalid cache path for %s", abs)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func metadataCachePath(abs string) string {
	if abs == "" {
		return ""
	}
	clean := filepath.ToSlash(abs)
	parent := filepath.Dir(clean)
	base := filepath.Base(clean)
	if info, err := os.Stat(clean); err == nil && !info.IsDir() {
		base = strings.TrimSuffix(base, filepath.Ext(base))
	}
	if base == "" {
		return ""
	}
	hiddenDir := filepath.Join(parent, ".hidden", base)
	return filepath.Join(hiddenDir, "metadata.json")
}
