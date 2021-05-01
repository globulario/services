module github.com/globulario/services/golang/interceptors

go 1.16

replace github.com/globulario/services/golang/security => ../security

replace github.com/globulario/services/golang/resource/resourcepb => ../resource/resourcepb

replace github.com/globulario/services/golang/admin/adminpb => ../admin/adminpb

require (
	github.com/StackExchange/wmi v0.0.0-20210224194228-fe8f1750fd46 // indirect
	github.com/allegro/bigcache v1.2.1 // indirect
	github.com/davecourtois/Utility v0.0.0-20210430205301-666a7d0dc453
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/globulario/services v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/globular_client v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/rbac/rbac_client v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/rbac/rbacpb v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/resource/resource_client v0.0.0-20210430220500-7bdc3b2193ef
	github.com/globulario/services/golang/resource/resourcepb v0.0.0-00010101000000-000000000000 // indirect
	github.com/globulario/services/golang/security v0.0.0-00010101000000-000000000000 // indirect
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/shirou/gopsutil v3.21.3+incompatible
	github.com/syndtr/goleveldb v1.0.0 // indirect
	google.golang.org/grpc v1.37.0
	google.golang.org/protobuf v1.26.0
)
