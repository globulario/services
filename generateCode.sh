#!/bin/bash Run that command from inside your globular server.

# It's better to regenerate the whole thing when something change, so
# all the code will be in the same gRpc version. Trust  me I lost one 
# day of my life just like that. But at then end I learn something...

# GO grpc file generation
protoc proto/admin.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/resource.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/rbac.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/log.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/lb.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/ca.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/packages.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/dns.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/echo.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/search.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/event.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/storage.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/file.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/sql.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/ldap.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/mail.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/persistence.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/monitoring.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/spc.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/catalog.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/conversation.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang

# Web-Api generation.
# ** Note that gooleapis /usr/local/include/google/api must exist... (https://github.com/googleapis/googleapis)
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/admin/adminpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true admin.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/resource/resourcepb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true resource.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/rbac/rbacpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true rbac.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/log/logpb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true log.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/packages/packagespb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true packages.proto
protoc -I /usr/local/include -I proto --grpc-gateway_out ./golang/ca/capb --grpc-gateway_opt logtostderr=true --grpc-gateway_opt paths=source_relative --grpc-gateway_opt generate_unbound_methods=true ca.proto
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
mkdir typescript/admin
protoc --js_out=import_style=commonjs:typescript/admin  -I ./proto/ admin.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/admin -I ./proto/ admin.proto
mkdir typescript/resource
protoc --js_out=import_style=commonjs:typescript/resource  -I ./proto/ resource.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/resource -I ./proto/ resource.proto
mkdir typescript/ca
protoc --js_out=import_style=commonjs:typescript/ca  -I ./proto/ ca.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/ca -I ./proto/ ca.proto
mkdir typescript/packages
protoc --js_out=import_style=commonjs:typescript/packages  -I ./proto/ packages.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/packages -I ./proto/ packages.proto
mkdir typescript/lb
protoc --js_out=import_style=commonjs:typescript/lb  -I ./proto/ lb.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/lb -I ./proto/ lb.proto
mkdir typescript/echo
protoc --js_out=import_style=commonjs:typescript/echo  -I ./proto/ echo.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/echo -I ./proto/ echo.proto
mkdir typescript/conversation
protoc --js_out=import_style=commonjs:typescript/conversation  -I ./proto/ conversation.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/conversation -I ./proto/ conversation.proto
mkdir typescript/search
protoc --js_out=import_style=commonjs:typescript/search  -I ./proto/ search.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/search -I ./proto/ search.proto
mkdir typescript/event
protoc --js_out=import_style=commonjs:typescript/event  -I ./proto/ event.proto
protoc --js_out=import_style=commonjs:typescript/event  -I ./proto/ event.proto
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
# on window use C:\msys64\mingw64\bin\grpc_csharp_plugin.exe
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

