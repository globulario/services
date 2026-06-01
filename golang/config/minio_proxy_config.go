package config

const (
	MinioProxyAuthModeAccessKey = "accessKey"
	MinioProxyAuthModeFile      = "file"
	MinioProxyAuthModeNone      = "none"
)

// MinioProxyConfig describes the configuration the gateway or other services need
// to contact an external MinIO-compatible object store.
type MinioProxyConfig struct {
	Endpoint     string         `json:"endpoint,omitempty"`
	Bucket       string         `json:"bucket,omitempty"`
	Prefix       string         `json:"prefix,omitempty"`
	Secure       bool           `json:"secure,omitempty"`
	CABundlePath string         `json:"caBundlePath,omitempty"`
	Auth         *MinioProxyAuth `json:"auth,omitempty"`
}

// MinioProxyAuth describes authentication mode / credentials for MinIO.
type MinioProxyAuth struct {
	Mode      string `json:"mode,omitempty"`
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	CredFile  string `json:"credFile,omitempty"`
}
