
#There is the command to build all services at once.
go build  -buildvcs=false -o ./admin/admin_server ./admin/admin_server
go build  -buildvcs=false -o ./applications_manager/applications_manager_server ./applications_manager/applications_manager_server
go build  -buildvcs=false -o ./services_manager/services_manager_server ./services_manager/services_manager_server
go build  -buildvcs=false -o ./authentication/authentication_server ./authentication/authentication_server
go build  -buildvcs=false -o ./catalog/catalog_server ./catalog/catalog_server
go build  -buildvcs=false -o ./blog/blog_server ./blog/blog_server
go build  -buildvcs=false -o ./conversation/conversation_server ./conversation/conversation_server
go build  -buildvcs=false -o ./discovery/discovery_server ./discovery/discovery_server
go build  -buildvcs=false -o ./dns/dns_server ./dns/dns_server
go build  -buildvcs=false -o ./echo/echo_server ./echo/echo_server
go build  -buildvcs=false -o ./event/event_server ./event/event_server
go build  -buildvcs=false -o ./file/file_server ./file/file_server
go build  -buildvcs=false -o ./ldap/ldap_server ./ldap/ldap_server
go build  -buildvcs=false -o ./log/log_server ./log/log_server
go build  -buildvcs=false -o ./mail/mail_server ./mail/mail_server
go build  -buildvcs=false -o ./monitoring/monitoring_server ./monitoring/monitoring_server
go build  -buildvcs=false -o ./persistence/persistence_server ./persistence/persistence_server
go build  -buildvcs=false -o ./rbac/rbac_server ./rbac/rbac_server
go build  -buildvcs=false -o ./repository/repository_server ./repository/repository_server
go build  -buildvcs=false -o ./resource/resource_server ./resource/resource_server
go build  -buildvcs=false -o ./sql/sql_server ./sql/sql_server
go build  -buildvcs=false -o ./storage/storage_server ./storage/storage_server
go build  -buildvcs=false -o ./config/config_server ./config/config_server
go build  -buildvcs=false -o ./title/title_server ./title/title_server
go build  -buildvcs=false -o ./torrent/torrent_server ./torrent/torrent_server
go build  -buildvcs=false -o ./search/search_server ./search/search_server

# start services...
export ServicesRoot=/home/dave/globulario/services
./admin/admin_server/admin_server &
./blog/blog_server/blog_server &
./applications_manager/applications_manager_server/applications_manager_server &
./services_manager/services_manager_server/services_manager_server &
./authentication/authentication_server/authentication_server &
./catalog/catalog_server/catalog_server &
./conversation/conversation_server/conversation_server &
./discovery/discovery_server/discovery_server &
./dns/dns_server/dns_server &
./echo/echo_server/echo_server &
./event/event_server/event_server &
./file/file_server/file_server &
./ldap/ldap_server/ldap_server &
./log/log_server/log_server &
./mail/mail_server/mail_server &
./monitoring/monitoring_server/monitoring_server &
./persistence/persistence_server/persistence_server &
./rbac/rbac_server/rbac_server &
./repository/repository_server/repository_server &
./resource/resource_server/resource_server &
./search/search_server/search_server &
./sql/sql_server/sql_server &
./storage/storage_server/storage_server &
./config/config_server/config_server &
./title/title_server/title_server &
./torrent/torrent_server/torrent_server

# publish services, that trigger executable update on globule who run that services.
