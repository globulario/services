module github.com/globulario/services/golang/admin/admin_client

go 1.16

replace github.com/globulario/services/golang/security => ../../security
replace github.com/globulario/services/golang/admin/adminpb v0.0.0 => ../adminpb

require (
	github.com/globulario/services/golang/globular_client v0.0.0-20210430215429-ce20d9b13195
	github.com/globulario/services/golang/security v0.0.0
	github.com/polds/imgbase64 v0.0.0-20140820003345-cb7bf37298b7
	google.golang.org/grpc v1.37.0
    github.com/davecourtois/Utility v0.0.0-20210430205301-666a7d0dc453
 github.com/globulario/services/golang/admin/adminpb v0.0.0
)

