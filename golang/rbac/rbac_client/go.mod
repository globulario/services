module github.com/globulario/services/golang/rbac/rbac_client

go 1.16

replace github.com/globulario/services/golang/security => ../../security

replace github.com/globulario/services/rbac/rbacpb => ../rbacpb

replace github.com/globulario/services/golang/resource/resourcepb => ../../resource/resourcepb

require (
	github.com/globulario/services/golang/globular_client v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/rbac/rbacpb v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/resource/resource_client v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/resource/resourcepb v0.0.0-00010101000000-000000000000
	github.com/globulario/services/golang/security v0.0.0-20210430220500-7bdc3b2193ef // indirect
	google.golang.org/grpc v1.37.0
)
