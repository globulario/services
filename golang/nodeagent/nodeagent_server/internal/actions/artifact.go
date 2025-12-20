package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type artifactFetchAction struct{}
type artifactVerifyAction struct{}

const repositoryAddressEnv = "REPOSITORY_ADDRESS"
const defaultRepositoryAddress = "localhost:10101"

func (artifactFetchAction) Name() string { return "artifact.fetch" }

func (artifactFetchAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	fields := args.GetFields()
	required := []string{"publisher", "name", "version", "platform", "dest"}
	for _, key := range required {
		if v := strings.TrimSpace(fields[key].GetStringValue()); v == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	return nil
}

func (artifactFetchAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	ref := &repositorypb.ArtifactRef{
		PublisherId: strings.TrimSpace(fields["publisher"].GetStringValue()),
		Name:        strings.TrimSpace(fields["name"].GetStringValue()),
		Version:     strings.TrimSpace(fields["version"].GetStringValue()),
		Platform:    strings.TrimSpace(fields["platform"].GetStringValue()),
	}
	dest := strings.TrimSpace(fields["dest"].GetStringValue())
	if dest == "" {
		return "", errors.New("dest is required")
	}

	client, err := newRepositoryClient()
	if err != nil {
		return "", err
	}
	defer client.Close()

	data, err := client.DownloadArtifact(ref)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(dest, data); err != nil {
		return "", err
	}
	return fmt.Sprintf("fetched %s", dest), nil
}

func (artifactVerifyAction) Name() string { return "artifact.verify" }

func (artifactVerifyAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	fields := args.GetFields()
	if strings.TrimSpace(fields["path"].GetStringValue()) == "" {
		return errors.New("path is required")
	}
	if strings.TrimSpace(fields["sha256"].GetStringValue()) == "" {
		return errors.New("sha256 is required")
	}
	return nil
}

func (artifactVerifyAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	path := strings.TrimSpace(fields["path"].GetStringValue())
	expected := strings.ToLower(strings.TrimSpace(fields["sha256"].GetStringValue()))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	actual := hex.EncodeToString(hash[:])
	if actual != expected {
		return "", fmt.Errorf("sha256 mismatch: expected %s got %s", expected, actual)
	}
	return fmt.Sprintf("verified %s", path), nil
}

func newRepositoryClient() (*repository_client.Repository_Service_Client, error) {
	addr := strings.TrimSpace(os.Getenv(repositoryAddressEnv))
	if addr == "" {
		addr = defaultRepositoryAddress
	}
	return repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
}

func writeAtomic(dest string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".artifact-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

func init() {
	Register(artifactFetchAction{})
	Register(artifactVerifyAction{})
}
