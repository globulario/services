module github.com/globulario/services/golang/storage/storage_client

go 1.16

replace github.com/globulario/services/golang/security => ../../security

replace github.com/globulario/services/golang/resource/resourcepb => ../../resource/resourcepb

replace github.com/globulario/services/golang/storage/storagepb => ../storagepb

replace github.com/globulario/services/golang/admin/adminpb => ../../admin/adminpb

require (
	github.com/globulario/services/golang/globular_client v0.0.0-20210501011657-2bc6004d4175
	github.com/globulario/services/golang/storage/storagepb v0.0.0-00010101000000-000000000000
	google.golang.org/grpc v1.37.0
)
