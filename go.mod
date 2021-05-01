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

require (
	github.com/alexbrainman/odbc v0.0.0-20200426075526-f0492dfa1575
	github.com/allegro/bigcache v1.2.1
	github.com/davecourtois/GoXapian v0.0.0-20201222213557-81c72bc9e73c
	github.com/davecourtois/Utility v0.0.0-20210430205301-666a7d0dc453
	github.com/denisenkom/go-mssqldb v0.10.0
	github.com/emersion/go-imap v1.0.6
	github.com/emersion/go-message v0.14.1
	github.com/emersion/go-smtp v0.15.0 // indirect
	github.com/emersion/go-smtp-mta v0.0.0-20170206201558-f9b2f2fd6e9a
	github.com/globulario/services/golang/globular_client v0.0.0-20210501011657-2bc6004d4175
	github.com/globulario/services/golang/globular_service v0.0.0-20210501011657-2bc6004d4175
	github.com/globulario/services/golang/interceptors v0.0.0-20210501011657-2bc6004d4175
	github.com/globulario/services/golang/lb/load_balancing_client v0.0.0-00010101000000-000000000000 // indirect
	github.com/globulario/services/golang/mail/mail_client v0.0.0-00010101000000-000000000000
	github.com/globulario/services/golang/mail/mailpb v0.0.0-00010101000000-000000000000
	github.com/globulario/services/golang/persistence/persistence_client v0.0.0-00010101000000-000000000000
	github.com/globulario/services/golang/persistence/persistence_store v0.0.0-20210501011657-2bc6004d4175
	github.com/globulario/services/golang/persistence/persistencepb v0.0.0-00010101000000-000000000000
	github.com/globulario/services/golang/resource/resource_client v0.0.0-20210501011657-2bc6004d4175
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.4.0
	github.com/lib/pq v1.10.1
	github.com/mattn/go-sqlite3 v1.14.7
	github.com/mhale/smtpd v0.0.0-20210322105601-438c8edb069c
	github.com/prometheus/client_golang v1.10.0
	github.com/syndtr/goleveldb v1.0.0
	google.golang.org/grpc v1.37.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/alexcesaro/quotedprintable.v2 v2.0.0-20150314193201-9b4a113f96b3 // indirect
	gopkg.in/gomail.v1 v1.0.0-20150320132819-11b919ab4933
)
