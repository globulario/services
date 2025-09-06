
#There is the command to build all services at once.
go build  -buildvcs=false -o ./admin/admin_server ./admin/admin_server
chmod +x ./admin/admin_server/admin_server
go build  -buildvcs=false -o ./applications_manager/applications_manager_server ./applications_manager/applications_manager_server
chmod +x ./applications_manager/applications_manager_server/applications_manager_server
go build  -buildvcs=false -o ./services_manager/services_manager_server ./services_manager/services_manager_server
chmod +x ./services_manager/services_manager_server/services_manager_server
go build  -buildvcs=false -o ./authentication/authentication_server ./authentication/authentication_server
chmod +x ./authentication/authentication_server/authentication_server
go build  -buildvcs=false -o ./catalog/catalog_server ./catalog/catalog_server
chmod +x ./catalog/catalog_server/catalog_server    
go build  -buildvcs=false -o ./blog/blog_server ./blog/blog_server
chmod +x ./blog/blog_server/blog_server
go build  -buildvcs=false -o ./conversation/conversation_server ./conversation/conversation_server
chmod +x ./conversation/conversation_server/conversation_server
go build  -buildvcs=false -o ./discovery/discovery_server ./discovery/discovery_server
chmod +x ./discovery/discovery_server/discovery_server
go build  -buildvcs=false -o ./dns/dns_server ./dns/dns_server
chmod +x ./dns/dns_server/dns_server
go build  -buildvcs=false -o ./echo/echo_server ./echo/echo_server
chmod +x ./echo/echo_server/echo_server
go build  -buildvcs=false -o ./event/event_server ./event/event_server
chmod +x ./event/event_server/event_server
go build  -buildvcs=false -o ./file/file_server ./file/file_server
chmod +x ./file/file_server/file_server
go build  -buildvcs=false -o ./media/media_server ./media/media_server
chmod +x ./media/media_server/media_server
go build  -buildvcs=false -o ./ldap/ldap_server ./ldap/ldap_server
chmod +x ./ldap/ldap_server/ldap_server
go build  -buildvcs=false -o ./log/log_server ./log/log_server
chmod +x ./log/log_server/log_server
go build  -buildvcs=false -o ./mail/mail_server ./mail/mail_server
chmod +x ./mail/mail_server/mail_server
go build  -buildvcs=false -o ./monitoring/monitoring_server ./monitoring/monitoring_server
chmod +x ./monitoring/monitoring_server/monitoring_server
go build  -buildvcs=false -o ./persistence/persistence_server ./persistence/persistence_server
chmod +x ./persistence/persistence_server/persistence_server
go build  -buildvcs=false -o ./rbac/rbac_server ./rbac/rbac_server
chmod +x ./rbac/rbac_server/rbac_server
go build  -buildvcs=false -o ./repository/repository_server ./repository/repository_server
chmod +x ./repository/repository_server/repository_server
go build  -buildvcs=false -o ./resource/resource_server ./resource/resource_server
chmod +x ./resource/resource_server/resource_server
go build  -buildvcs=false -o ./sql/sql_server ./sql/sql_server
chmod +x ./sql/sql_server/sql_server
go build  -buildvcs=false -o ./storage/storage_server ./storage/storage_server
chmod +x ./storage/storage_server/storage_server
go build  -buildvcs=false -o ./title/title_server ./title/title_server
chmod +x ./title/title_server/title_server
go build  -buildvcs=false -o ./torrent/torrent_server ./torrent/torrent_server
chmod +x ./torrent/torrent_server/torrent_server
go build  -buildvcs=false -o ./search/search_server ./search/search_server
chmod +x ./search/search_server/search_server   


# start services...
export ServicesRoot=/home/dave/Documents/globulario/services &
./authentication/authentication_server/authentication_server &
./dns/dns_server/dns_server &
./blog/blog_server/blog_server &
./applications_manager/applications_manager_server/applications_manager_server &
./services_manager/services_manager_server/services_manager_server &
./conversation/conversation_server/conversation_server &
./discovery/discovery_server/discovery_server &
./echo/echo_server/echo_server &
./event/event_server/event_server &
./file/file_server/file_server &
./log/log_server/log_server &
./monitoring/monitoring_server/monitoring_server &
./persistence/persistence_server/persistence_server &
./rbac/rbac_server/rbac_server &
./repository/repository_server/repository_server &
./resource/resource_server/resource_server &
./search/search_server/search_server &
./sql/sql_server/sql_server &
./storage/storage_server/storage_server &
./title/title_server/title_server &
./torrent/torrent_server/torrent_server
./catalog/catalog_server/catalog_server &
./ldap/ldap_server/ldap_server &
./mail/mail_server/mail_server
# publish services, that trigger executable update on globule who run that services.
