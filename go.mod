module github.com/globulario/services

go 1.16

replace github.com/globulario/services/golang/security => ./golang/security

replace github.com/globulario/services/golang/resource/resourcepb => ./golang/resource/resourcepb

replace github.com/globulario/services/golang/admin/adminpb => ./golang/admin/adminpb

replace github.com/globulario/services/golang/globular_client => ./golang/globular_client

replace github.com/globulario/services/golang/lb/lbpb => ./golang/lb/lbpb

replace github.com/globulario/services/golang/lb/load_balancing_client => ./golang/lb/load_balancing_client

replace github.com/globulario/services/golang/persistence/persistencepb => ./golang/persistence/persistencepb

replace github.com/globulario/services/golang/persistence/persistence_client => ./golang/persistence/persistence_client

replace github.com/globulario/services/golang/mail/mailpb => ./golang/mail/mailpb

replace github.com/globulario/services/golang/mail/mail_client => ./golang/mail/mail_client

replace github.com/globulario/services/golang/spc/spcpb => ./golang/spcpb

replace github.com/globulario/services/golang/spc/spc_client => ./golang/spc_client

replace github.com/globulario/services/golang/log/logpb => ./golang/log/logpb

replace github.com/globulario/services/golang/log/log_client => ./golang/log/log_client

replace github.com/globulario/services/golang/pacakges/pacakgespb => ./golang/pacakges/pacakgespb

replace github.com/globulario/services/golang/pacakges/pacakges_client => ./golang/pacakges/pacakges_client

replace github.com/globulario/services/golang/search/searchpb => ./golang/search/searchpb

replace github.com/globulario/services/golang/search/search_client => ./golang/search/search_client

replace github.com/globulario/services/golang/storage/storagepb => ./golang/storage/storagepb

replace github.com/globulario/services/golang/storage/store => ./golang/storage/store

replace github.com/globulario/services/golang/storage/storage_client => ./golang/storage/storage_client

require (
	github.com/davecourtois/GoXapian v0.0.0-20201222213557-81c72bc9e73c
	github.com/davecourtois/Utility v0.0.0-20210430205301-666a7d0dc453
	github.com/globulario/services/golang/search/searchpb v0.0.0-00010101000000-000000000000
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.4.0
	google.golang.org/grpc v1.37.0
	google.golang.org/protobuf v1.26.0
)
