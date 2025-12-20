package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/types/known/structpb"
)

type fileWriteAction struct{}

func (fileWriteAction) Name() string { return "file.write_atomic" }

func (fileWriteAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	if path := strings.TrimSpace(args.GetFields()["path"].GetStringValue()); path == "" {
		return errors.New("path is required")
	}
	if args.GetFields()["content"].GetStringValue() == "" {
		return errors.New("content is required")
	}
	return nil
}

func (fileWriteAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	path := strings.TrimSpace(args.GetFields()["path"].GetStringValue())
	content := args.GetFields()["content"].GetStringValue()
	if path == "" {
		return "", errors.New("path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	newHash := sha256.Sum256([]byte(content))
	if existing, err := os.ReadFile(path); err == nil {
		if hex.EncodeToString(newHash[:]) == hashBytes(existing) {
			return "content unchanged", nil
		}
	}
	tmp, err := os.CreateTemp(dir, ".write-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.WriteString(tmp, content); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return "", err
	}
	return "file updated", nil
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func init() {
	Register(fileWriteAction{})
}
