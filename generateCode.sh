#!/bin/bash Run that command from inside your globular server.

# It's better to regenerate the whole thing when something change, so
# all the code will be in the same gRpc version. Trust  me I lost one 
# day of my life just like that. But at then end I learn something...

# GO grpc file generation
protoc proto/admin.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/resource.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/rbac.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/log.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/ca.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/services.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
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
protoc proto/plc_link.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang
protoc proto/plc.proto --go-grpc_out=require_unimplemented_servers=false:./golang --go_out=./golang

# TypeScript grpc files generation.
mkdir typescript\admin
protoc --js_out=import_style=commonjs:typescript/admin  -I ./proto/ admin.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/admin -I ./proto/ admin.proto
mkdir typescript/resource
protoc --js_out=import_style=commonjs:typescript/resource  -I ./proto/ resource.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/resource -I ./proto/ resource.proto
mkdir typescript/ca
protoc --js_out=import_style=commonjs:typescript/ca  -I ./proto/ ca.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/ca -I ./proto/ ca.proto
mkdir typescript/services
protoc --js_out=import_style=commonjs:typescript/services  -I ./proto/ services.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/services -I ./proto/ services.proto
mkdir typescript/echo
protoc --js_out=import_style=commonjs:typescript/echo  -I ./proto/ echo.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/echo -I ./proto/ echo.proto
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
mkdir typescript/plc
protoc --js_out=import_style=commonjs:typescript/plc  -I ./proto/ plc.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/plc -I ./proto/ plc.proto
mkdir typescript/plc_link
protoc --js_out=import_style=commonjs:typescript/plc_link  -I ./proto/ plc_link.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/plc_link -I ./proto/ plc_link.proto
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
mkdir csharp/event/eventpb
protoc --grpc_out=./csharp/event/eventpb --csharp_out=./csharp/event/eventpb --csharp_opt=file_extension=.g.cs proto/event.proto --plugin="protoc-gen-grpc=C:/Users/mm006819/grpc/.build/grpc_csharp_plugin.exe"
mkdir csharp/persistence/persistencepb
protoc --grpc_out=./csharp/persistence/persistencepb --csharp_out=./csharp/persistence/persistencepb --csharp_opt=file_extension=.g.cs proto/persistence.proto --plugin="protoc-gen-grpc=C:/Users/mm006819/grpc/.build/grpc_csharp_plugin.exe"
mkdir csharp/resource/resourcepb
protoc --grpc_out=./csharp/resource/resourcepb --csharp_out=./csharp/resource/resourcepb --csharp_opt=file_extension=.g.cs proto/resource.proto --plugin="protoc-gen-grpc=C:/Users/mm006819/grpc/.build/grpc_csharp_plugin.exe"
mkdir csharp/echo/echopb
protoc --grpc_out=./csharp/echo/echopb --csharp_out=./csharp/echo/echopb --csharp_opt=file_extension=.g.cs proto/echo.proto --plugin="protoc-gen-grpc=C:/Users/mm006819/grpc/.build/grpc_csharp_plugin.exe"

# C++ grpc files generation.
mkdir cpp/resource/resourcepb
protoc --plugin="protoc-gen-grpc=C://Users//mm006819//grpc//.build//grpc_cpp_plugin.exe" --grpc_out=./cpp/resource/resourcepb -I proto resource.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/resource/resourcepb  -I proto resource.proto
protoc --cpp_out=./cpp/resource/resourcepb -I proto resource.proto
mkdir cpp/echo/echopb
protoc --plugin="protoc-gen-grpc=C://Users//mm006819//grpc//.build//grpc_cpp_plugin.exe" --grpc_out=./cpp/echo/echopb -I proto/ echo.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/echo/echopb -I proto/ echo.proto
protoc --cpp_out=./cpp/echo/echopb  -I proto/ echo.proto
mkdir cpp/plc/plcpb
protoc --plugin="protoc-gen-grpc=C://Users//mm006819//grpc//.build//grpc_cpp_plugin.exe" --grpc_out=./cpp/plc/plcpb -I proto/ plc.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/plc/plcpb -I proto/ plc.proto
protoc --cpp_out=./cpp/plc/plcpb  -I proto/ plc.proto
mkdir cpp/spc/spcpb
protoc --plugin="protoc-gen-grpc=C://Users//mm006819//grpc//.build//grpc_cpp_plugin.exe" --grpc_out=./cpp/spc/spcpb -I proto/ spc.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/spc/spcpb -I proto/ spc.proto
protoc --cpp_out=./cpp/spc/spcpb  -I proto/ spc.proto

