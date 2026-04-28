package actions

import (
	"github.com/globulario/services/golang/config"
	"github.com/minio/minio-go/v7"
)

// ObjectstoreLayout exposes the bucket/prefix layout for external consumers
// (e.g. the webroot sync workflow).
type ObjectstoreLayout struct {
	UsersBucket   string
	WebrootBucket string
	UsersPrefix   string
	WebrootPrefix string
	Domain        string
}

// ResolveContractPathPublic exposes the default contract path.
func ResolveContractPathPublic() string {
	return resolveContractPath()
}

// LoadMinioConfigPublic exposes loadMinioConfig.
func LoadMinioConfigPublic(path string, strict bool) (*config.MinioProxyConfig, string, error) {
	return loadMinioConfig(path, strict)
}

// BuildMinioClientPublic exposes buildMinioClient.
func BuildMinioClientPublic(cfg *config.MinioProxyConfig) (*minio.Client, error) {
	return buildMinioClient(cfg)
}

// DeriveMinioLayoutPublic exposes deriveMinioLayoutForNodeAgent with a public return type.
func DeriveMinioLayoutPublic(cfg *config.MinioProxyConfig, domain string) (ObjectstoreLayout, error) {
	l, err := deriveMinioLayoutForNodeAgent(cfg, domain)
	if err != nil {
		return ObjectstoreLayout{}, err
	}
	return ObjectstoreLayout{
		UsersBucket:   l.usersBucket,
		WebrootBucket: l.webrootBucket,
		UsersPrefix:   l.usersPrefix,
		WebrootPrefix: l.webrootPrefix,
		Domain:        l.domain,
	}, nil
}
