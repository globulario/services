module github.com/globulario/services/golang/catalog/catalog_client

go 1.16

replace github.com/globulario/services/golang/security => ../../security

replace github.com/globulario/services/golang/catalog/catalogpb => ../catalogpb

require (
	github.com/davecourtois/Utility v0.0.0-20210430205301-666a7d0dc453
	github.com/globulario/services/golang/catalog/catalogpb v0.0.0-00010101000000-000000000000
	github.com/globulario/services/golang/globular_client v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/security v0.0.0-20210430220500-7bdc3b2193ef // indirect
	github.com/golang/protobuf v1.5.2
	golang.org/x/sys v0.0.0-20210426230700-d19ff857e887 // indirect
	google.golang.org/genproto v0.0.0-20210429181445-86c259c2b4ab // indirect
	google.golang.org/grpc v1.37.0
)
