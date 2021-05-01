module globulario/services/golang/packages/packages_client

go 1.16

replace github.com/globulario/services/golang/security => ../../security

replace github.com/globulario/services/golang/resource/resourcepb => ../../resource/resourcepb

replace github.com/globulario/services/golang/packages/packagespb => ../packagespb

replace github.com/globulario/services/golang/admin/adminpb => ../../admin/adminpb

require (
	github.com/davecourtois/Utility v0.0.0-20210430205301-666a7d0dc453
	github.com/globulario/services/golang/globular_client v0.0.0-20210501011657-2bc6004d4175
	github.com/globulario/services/golang/packages/packagespb v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.37.0
)
