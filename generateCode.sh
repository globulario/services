#!/bin/bash Run that command from inside your globular server.

# push a new version
git tag -a golang/v0.1.4 -m "golang v0.1.4"
git push origin golang/v0.1.4

# It's better to regenerate the whole thing when something change, so
# all the code will be in the same gRpc version. Trust  me I lost one 
# day of my life just like that. But at then end I learn something...

# GO grpc file generation
protoc proto/admin.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/admin/adminpb --go_out=paths=source_relative:./golang/admin/adminpb
protoc proto/resource.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/resource/resourcepb --go_out=paths=source_relative:./golang/resource/resourcepb
protoc proto/rbac.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/rbac/rbacpb --go_out=paths=source_relative:./golang/rbac/rbacpb
protoc proto/log.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/log/logpb --go_out=paths=source_relative:./golang/log/logpb
protoc proto/dns.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/dns/dnspb --go_out=paths=source_relative:./golang/dns/dnspb
protoc proto/echo.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/echo/echopb --go_out=paths=source_relative:./golang/echo/echopb
protoc proto/media.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/media/mediapb --go_out=paths=source_relative:./golang/media/mediapb
protoc proto/search.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/search/searchpb --go_out=paths=source_relative:./golang/search/searchpb
protoc proto/event.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/event/eventpb --go_out=paths=source_relative:./golang/event/eventpb
protoc proto/storage.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/storage/storagepb --go_out=paths=source_relative:./golang/storage/storagepb
protoc proto/file.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/file/filepb --go_out=paths=source_relative:./golang/file/filepb
protoc proto/sql.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/sql/sqlpb --go_out=paths=source_relative:./golang/sql/sqlpb
protoc proto/ldap.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/ldap/ldappb --go_out=paths=source_relative:./golang/ldap/ldappb
protoc proto/mail.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/mail/mailpb --go_out=paths=source_relative:./golang/mail/mailpb
protoc proto/persistence.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/persistence/persistencepb --go_out=paths=source_relative:./golang/persistence/persistencepb
protoc proto/monitoring.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/monitoring/monitoringpb --go_out=paths=source_relative:./golang/monitoring/monitoringpb
protoc proto/spc.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/spc/spcpb --go_out=paths=source_relative:./golang/spc/spcpb
protoc proto/catalog.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/catalog/catalogpb --go_out=paths=source_relative:./golang/catalog/catalogpb
protoc proto/conversation.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/conversation/conversationpb --go_out=paths=source_relative:./golang/conversation/conversationpb
protoc proto/blog.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/blog/blogpb --go_out=paths=source_relative:./golang/blog/blogpb
protoc proto/applications_manager.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/applications_manager/applications_managerpb --go_out=paths=source_relative:./golang/applications_manager/applications_managerpb
protoc proto/authentication.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/authentication/authenticationpb --go_out=paths=source_relative:./golang/authentication/authenticationpb
protoc proto/services_manager.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/services_manager/services_managerpb --go_out=paths=source_relative:./golang/services_manager/services_managerpb
protoc proto/title.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/title/titlepb --go_out=paths=source_relative:./golang/title/titlepb
protoc proto/torrent.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/torrent/torrentpb --go_out=paths=source_relative:./golang/torrent/torrentpb
protoc proto/discovery.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/discovery/discoverypb --go_out=paths=source_relative:./golang/discovery/discoverypb
protoc proto/repository.proto -I proto --go-grpc_out=require_unimplemented_servers=false,paths=source_relative:./golang/repository/repositorypb --go_out=paths=source_relative:./golang/repository/repositorypb

# TypeScript grpc files generation.
mkdir typescript/applications_manager
protoc --js_out=import_style=commonjs:typescript/applications_manager  -I ./proto/ applications_manager.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/applications_manager -I ./proto/ applications_manager.proto
mkdir typescript/services_manager
protoc --js_out=import_style=commonjs:typescript/services_manager  -I ./proto/ services_manager.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/services_manager -I ./proto/ services_manager.proto
mkdir typescript/authentication
protoc --js_out=import_style=commonjs:typescript/authentication  -I ./proto/ authentication.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/authentication -I ./proto/ authentication.proto
mkdir typescript/admin
protoc --js_out=import_style=commonjs:typescript/admin  -I ./proto/ admin.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/admin -I ./proto/ admin.proto
mkdir typescript/resource
protoc --js_out=import_style=commonjs:typescript/resource  -I ./proto/ resource.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/resource -I ./proto/ resource.proto
mkdir typescript/repository
protoc --js_out=import_style=commonjs:typescript/repository  -I ./proto/ repository.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/repository -I ./proto/ repository.proto
mkdir typescript/discovery
protoc --js_out=import_style=commonjs:typescript/discovery  -I ./proto/ discovery.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/discovery -I ./proto/ discovery.proto
mkdir typescript/echo
protoc --js_out=import_style=commonjs:typescript/echo  -I ./proto/ echo.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/echo -I ./proto/ echo.proto
mkdir typescript/media
protoc --js_out=import_style=commonjs:typescript/media  -I ./proto/ media.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/media -I ./proto/ media.proto
mkdir typescript/blog
protoc --js_out=import_style=commonjs:typescript/blog  -I ./proto/ blog.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/blog -I ./proto/ blog.proto
mkdir typescript/conversation
protoc --js_out=import_style=commonjs:typescript/conversation  -I ./proto/ conversation.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/conversation -I ./proto/ conversation.proto
mkdir typescript/search
protoc --js_out=import_style=commonjs:typescript/search  -I ./proto/ search.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/search -I ./proto/ search.proto
mkdir typescript/event
protoc --js_out=import_style=commonjs:typescript/event  -I ./proto/ event.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/event -I ./proto/ event.proto
mkdir typescript/storage
protoc --js_out=import_style=commonjs:typescript/storage  -I ./proto/ storage.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/storage -I ./proto/ storage.proto
mkdir typescript/file
protoc --js_out=import_style=commonjs:typescript/file  -I ./proto/ file.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/file -I ./proto/ file.proto
mkdir typescript/sql
protoc --js_out=import_style=commonjs:typescript/sql  -I ./proto/ sql.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/sql -I ./proto/ sql.proto
mkdir typescript/ldap
protoc --js_out=import_style=commonjs:typescript/ldap  -I ./proto/ ldap.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/ldap -I ./proto/ ldap.proto
mkdir typescript/mail
protoc --js_out=import_style=commonjs:typescript/mail  -I ./proto/ mail.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/mail -I ./proto/ mail.proto
mkdir typescript/persistence
protoc --js_out=import_style=commonjs:typescript/persistence  -I ./proto/ persistence.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/persistence -I ./proto/ persistence.proto
mkdir typescript/spc
protoc --js_out=import_style=commonjs:typescript/spc  -I ./proto/ spc.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/spc -I ./proto/ spc.proto
mkdir typescript/monitoring
protoc --js_out=import_style=commonjs:typescript/monitoring  -I ./proto/ monitoring.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/monitoring -I ./proto/ monitoring.proto
mkdir typescript/catalog
protoc --js_out=import_style=commonjs:typescript/catalog  -I ./proto/ catalog.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/catalog -I ./proto/ catalog.proto
mkdir typescript/log
protoc --js_out=import_style=commonjs:typescript/log  -I ./proto/ log.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/log -I ./proto/ log.proto
mkdir typescript/rbac
protoc --js_out=import_style=commonjs:typescript/rbac  -I ./proto/ rbac.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/rbac -I ./proto/ rbac.proto
mkdir typescript/title
protoc --js_out=import_style=commonjs:typescript/title  -I ./proto/ title.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/title -I ./proto/ title.proto
mkdir typescript/torrent
protoc --js_out=import_style=commonjs:typescript/torrent  -I ./proto/ torrent.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/torrent -I ./proto/ torrent.proto
mkdir typescript/dns
protoc --js_out=import_style=commonjs:typescript/dns  -I ./proto/ dns.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/dns -I ./proto/ dns.proto

# CSharp grpc files generation
# on window use C:\Users\account_name\Documents\exec\grpc_dist\bin\grpc_csharp_plugin.exe
# on linux use /usr/local/bin/grpc_csharp_plugin
protoc --grpc_out=./csharp/config/GlobularConfigClient --csharp_out=./csharp/config/GlobularConfigClient --csharp_opt=file_extension=.g.cs proto/config.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/event/GlobularEventClient --csharp_out=./csharp/event/GlobularEventClient --csharp_opt=file_extension=.g.cs proto/event.proto --plugin="protoc-gen-grpc=/usr/local/bin/grpc_csharp_plugin"
protoc --grpc_out=./csharp/persistence/GlobularPersistenceClient --csharp_out=./csharp/persistence/GlobularPersistenceClient --csharp_opt=file_extension=.g.cs proto/persistence.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/resource/GlobularResourceClient --csharp_out=./csharp/resource/GlobularResourceClient --csharp_opt=file_extension=.g.cs proto/resource.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/echo/GlobularEchoServer --csharp_out=./csharp/echo/GlobularEchoServer --csharp_opt=file_extension=.g.cs proto/echo.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/lb/GlobularLoadBalancingClient --csharp_out=./csharp/lb/GlobularLoadBalancingClient --csharp_opt=file_extension=.g.cs proto/lb.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/log/GlobularLogClient --csharp_out=./csharp/log/GlobularLogClient --csharp_opt=file_extension=.g.cs proto/log.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/rbac/GlobularRbacClient --csharp_out=./csharp/rbac/GlobularRbacClient --csharp_opt=file_extension=.g.cs proto/rbac.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"


# C++ grpc files generation.
mkdir cpp/config/configpb
protoc --plugin="protoc-gen-grpc=C:\msys64\mingw64\bin\grpc_cpp_plugin.exe" --grpc_out=./cpp/config/configpb -I proto config.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/config/configpb  -I proto config.proto
protoc --cpp_out=./cpp/config/configpb -I proto config.proto
mkdir cpp/rbac/rbacpb
protoc --plugin="protoc-gen-grpc=C:\msys64\mingw64\bin\grpc_cpp_plugin.exe" --grpc_out=./cpp/rbac/rbacpb -I proto rbac.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/rbac/rbacpb  -I proto rbac.proto
protoc --cpp_out=./cpp/rbac/rbacpb -I proto rbac.proto
mkdir cpp/resource/resourcepb
protoc --plugin="protoc-gen-grpc=C:\msys64\mingw64\bin\grpc_cpp_plugin.exe" --grpc_out=./cpp/resource/resourcepb -I proto resource.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/resource/resourcepb  -I proto resource.proto
protoc --cpp_out=./cpp/resource/resourcepb -I proto resource.proto
mkdir cpp/echo/echopb
protoc --plugin="protoc-gen-grpc=C:\msys64\mingw64\bin\grpc_cpp_plugin.exe" --grpc_out=./cpp/echo/echopb -I proto/ echo.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/echo/echopb -I proto/ echo.proto
protoc --cpp_out=./cpp/echo/echopb  -I proto/ echo.proto
mkdir cpp/spc/spcpb
protoc --plugin="protoc-gen-grpc=C:\msys64\mingw64\bin\grpc_cpp_plugin.exe" --grpc_out=./cpp/spc/spcpb -I proto/ spc.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/spc/spcpb -I proto/ spc.proto
protoc --cpp_out=./cpp/spc/spcpb  -I proto/ spc.proto