module github.com/globulario/services/golang/event/event_client

go 1.16

replace github.com/globulario/services/golang/security => ../../security

replace github.com/globulario/services/golang/resource/resourcepb => ../../resource/resourcepb

replace github.com/globulario/services/golang/event/eventpb => ../eventpb

replace github.com/globulario/services/golang/admin/adminpb => ../../admin/adminpb

require (
	github.com/davecourtois/Utility v0.0.0-20210430205301-666a7d0dc453
	github.com/globulario/services/golang/event/eventpb v0.0.0-00010101000000-000000000000
	github.com/globulario/services/golang/globular_client v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/security v0.0.0-00010101000000-000000000000 // indirect
	google.golang.org/grpc v1.37.0
)
