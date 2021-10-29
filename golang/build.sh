
#There is the command to build all services at once.
go build -o ./admin/admin_server ./admin/admin_server
go build -o ./applications_manager/applications_manager_server ./applications_manager/applications_manager_server
go build -o ./services_manager/services_manager_server ./services_manager/services_manager_server
go build -o ./authentication/authentication_server ./authentication/authentication_server
go build -o ./catalog/catalog_server ./catalog/catalog_server
go build -o ./conversation/conversation_server ./conversation/conversation_server
go build -o ./discovery/discovery_server ./discovery/discovery_server
go build -o ./dns/dns_server ./dns/dns_server
go build -o ./echo/echo_server ./echo/echo_server
go build -o ./event/event_server ./event/event_server
go build -o ./file/file_server ./file/file_server
go build -o ./ldap/ldap_server ./ldap/ldap_server
go build -o ./log/log_server ./log/log_server
go build -o ./mail/mail_server ./mail/mail_server
go build -o ./monitoring/monitoring_server ./monitoring/monitoring_server
go build -o ./persistence/persistence_server ./persistence/persistence_server
go build -o ./rbac/rbac_server ./rbac/rbac_server
go build -o ./repository/repository_server ./repository/repository_server
go build -o ./resource/resource_server ./resource/resource_server
go build -o ./search/search_server ./search/search_server
go build -o ./sql/sql_server ./sql/sql_server
go build -o ./storage/storage_server ./storage/storage_server



