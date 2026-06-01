package actions

import (
	"context"
	"errors"
	"fmt"
	"syscall"

	"google.golang.org/protobuf/types/known/structpb"
)

type diskFreeAction struct{}

func (diskFreeAction) Name() string { return "check.disk_free" }

func (diskFreeAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	if args.GetFields()["path"].GetStringValue() == "" {
		return errors.New("path is required")
	}
	if args.GetFields()["min_bytes"].GetNumberValue() <= 0 {
		return errors.New("min_bytes must be positive")
	}
	return nil
}

func (diskFreeAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	path := args.GetFields()["path"].GetStringValue()
	minBytes := uint64(args.GetFields()["min_bytes"].GetNumberValue())
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return "", err
	}
	available := stat.Bavail * uint64(stat.Bsize)
	if available < minBytes {
		return "", fmt.Errorf("disk free %d lower than %d", available, minBytes)
	}
	return fmt.Sprintf("disk ok %d bytes", available), nil
}

func init() {
	Register(diskFreeAction{})
}
