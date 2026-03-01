package actions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/types/known/structpb"
)

type fileBackupAction struct{}

func (fileBackupAction) Name() string { return "file.backup" }

func (fileBackupAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	if args.GetFields()["path"].GetStringValue() == "" {
		return errors.New("path is required")
	}
	return nil
}

func (fileBackupAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	path := args.GetFields()["path"].GetStringValue()
	if path == "" {
		return "", errors.New("path is required")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "nothing to backup", nil
	}
	src, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()
	tmpPath := path + ".bak"
	if err := os.MkdirAll(filepath.Dir(tmpPath), 0o755); err != nil {
		return "", err
	}
	dst, err := os.CreateTemp(filepath.Dir(tmpPath), ".backup-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(dst.Name())
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return "", err
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		return "", err
	}
	if err := dst.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(dst.Name(), tmpPath); err != nil {
		return "", err
	}
	return fmt.Sprintf("backup %s", tmpPath), nil
}

type fileRestoreAction struct{}

func (fileRestoreAction) Name() string { return "file.restore_backup" }

func (fileRestoreAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	if args.GetFields()["path"].GetStringValue() == "" {
		return errors.New("path is required")
	}
	return nil
}

func (fileRestoreAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	path := args.GetFields()["path"].GetStringValue()
	if path == "" {
		return "", errors.New("path is required")
	}
	tmpPath := path + ".bak"
	src, err := os.Open(tmpPath)
	if err != nil {
		return "", err
	}
	defer src.Close()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".restore-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, src); err != nil {
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
	return fmt.Sprintf("restored %s", path), nil
}

func init() {
	Register(fileBackupAction{})
	Register(fileRestoreAction{})
}
