module github.com/globulario/services/golang/ca/ca_client

go 1.16

replace github.com/globulario/services/golang/ca/capb => ../capb
replace github.com/globulario/services/golang/security => ../../security

require (
	github.com/globulario/services/golang/ca/capb v0.0.0-00010101000000-000000000000
	github.com/globulario/services/golang/globular_client v0.0.0-20210430215429-ce20d9b13195
	github.com/globulario/services/golang/security v0.0.0-00010101000000-000000000000 // indirect
	google.golang.org/grpc v1.37.0
)
