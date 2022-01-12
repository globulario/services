#!/bin/bash Run that command from inside your globular server.

# It's better to regenerate the whole thing when something change, so
# all the code will be in the same gRpc version. Trust  me I lost one 
# day of my life just like that. But at then end I learn something...

# GO grpc file generation
protoc proto/admin.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/resource.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/rbac.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/log.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/dns.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/echo.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/search.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/event.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/storage.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/file.proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/sql.proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/ldap.proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/mail.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/persistence.proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/monitoring.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/spc.proto -I proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/catalog.proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/conversation.proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/blog.proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/applications_manager.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/authentication.proto -I proto  --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/services_manager.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/discovery.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/repository.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/config.proto -I proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang

# Web-Api generation.
# ** Note that gooleapis /usr/local/include/google/api must exist... (https://github.com/googleapis/googleapis)
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/admin/adminpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true admin.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/resource/resourcepb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true resource.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/rbac/rbacpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true rbac.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/log/logpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true log.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/packages/packagespb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true packages.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/dns/dnspb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true dns.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/echo/echopb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true echo.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/search/searchpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true search.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/event/eventpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true event.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/storage/storagepb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true storage.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/file/filepb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true file.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/ldap/ldappb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true ldap.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/mail/mailpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true mail.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/monitoring/monitoringpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true monitoring.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/spc/spcpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true spc.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/catalog/catalogpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true catalog.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/conversation/conversationpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true conversation.proto

# TypeScript grpc files generation.
mkdir typescript/config_manager
protoc --js_out=import_style=commonjs:typescript/config_manager  -I ./proto/ config_manager.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/config_manager -I ./proto/ config_manager.proto
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

# CSharp grpc files generation
# on window use C:\Users\mm006819\Documents\exec\grpc_dist\bin\grpc_csharp_plugin.exe
# on linux use /usr/local/bin/grpc_csharp_plugin
protoc --grpc_out=./csharp/event/GlobularEventClient --csharp_out=./csharp/event/GlobularEventClient --csharp_opt=file_extension=.g.cs proto/event.proto --plugin="protoc-gen-grpc=/usr/local/bin/grpc_csharp_plugin"
protoc --grpc_out=./csharp/persistence/GlobularPersistenceClient --csharp_out=./csharp/persistence/GlobularPersistenceClient --csharp_opt=file_extension=.g.cs proto/persistence.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/resource/GlobularResourceClient --csharp_out=./csharp/resource/GlobularResourceClient --csharp_opt=file_extension=.g.cs proto/resource.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/echo/GlobularEchoServer --csharp_out=./csharp/echo/GlobularEchoServer --csharp_opt=file_extension=.g.cs proto/echo.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/lb/GlobularLoadBalancingClient --csharp_out=./csharp/lb/GlobularLoadBalancingClient --csharp_opt=file_extension=.g.cs proto/lb.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/log/GlobularLogClient --csharp_out=./csharp/log/GlobularLogClient --csharp_opt=file_extension=.g.cs proto/log.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"
protoc --grpc_out=./csharp/rbac/GlobularRbacClient --csharp_out=./csharp/rbac/GlobularRbacClient --csharp_opt=file_extension=.g.cs proto/rbac.proto --plugin="protoc-gen-grpc=C:/msys64/mingw64/bin/grpc_csharp_plugin.exe"


# C++ grpc files generation.
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

