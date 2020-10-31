#!/bin/bash Run that command from inside your globular server.

# It's better to regenerate the whole thing when something change, so
# all the code will be in the same gRpc version. Trust  me I lost one 
# day of my life just like that. But at then end I learn something...

# GO grpc file generation
protoc proto/admin.proto --go_out=plugins=grpc:./golang
protoc proto/ressource.proto --go_out=plugins=grpc:./golang
protoc proto/ca.proto --go_out=plugins=grpc:./golang
protoc proto/lb.proto --go_out=plugins=grpc:./golang
protoc proto/services.proto --go_out=plugins=grpc:./golang
protoc proto/dns.proto --go_out=plugins=grpc:./golang
protoc proto/echo.proto --go_out=plugins=grpc:./golang
protoc proto/search.proto --go_out=plugins=grpc:./golang
protoc proto/event.proto --go_out=plugins=grpc:./golang
protoc proto/storage.proto --go_out=plugins=grpc:./golang
protoc proto/file.proto --go_out=plugins=grpc:./golang
protoc proto/sql.proto --go_out=plugins=grpc:./golang
protoc proto/ldap.proto --go_out=plugins=grpc:./golang
protoc proto/mail.proto --go_out=plugins=grpc:./golang
protoc proto/persistence.proto --go_out=plugins=grpc:./golang
protoc proto/monitoring.proto --go_out=plugins=grpc:./golang
protoc proto/plc.proto --go_out=plugins=grpc:./golang
protoc proto/spc.proto --go_out=plugins=grpc:./golang
protoc proto/catalog.proto --go_out=plugins=grpc:./golang
protoc proto/plc_link.proto --go_out=plugins=grpc:./golang

# TypeScript grpc files generation.
mkdir typescript/admin
protoc --js_out=import_style=commonjs:typescript/admin  -I ./proto/ admin.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/admin -I ./proto/ admin.proto
mkdir typescript/lb
protoc --js_out=import_style=commonjs:typescript/lb  -I ./proto/ lb.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/lb -I ./proto/ lb.proto
mkdir typescript/ressource
protoc --js_out=import_style=commonjs:typescript/ressource  -I ./proto/ ressource.proto
protoc --grpc-web_out=import_style=commonjs+dts,mode=grpcwebtext:typescript/ressource -I ./proto/ ressource.proto
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

# CSharp grpc files generation
mkdir csharp/event/eventpb
protoc --grpc_out=./csharp/event/eventpb --csharp_out=./csharp/event/eventpb --csharp_opt=file_extension=.g.cs proto/event.proto --plugin="protoc-gen-grpc=C:/Users/mm006819/grpc/.build/grpc_csharp_plugin.exe"
mkdir csharp/persistence/persistencepb
protoc --grpc_out=./csharp/persistence/persistencepb --csharp_out=./csharp/persistence/persistencepb --csharp_opt=file_extension=.g.cs proto/persistence.proto --plugin="protoc-gen-grpc=C:/Users/mm006819/grpc/.build/grpc_csharp_plugin.exe"
mkdir csharp/ressource/ressourcepb
protoc --grpc_out=./csharp/ressource/ressourcepb --csharp_out=./csharp/ressource/ressourcepb --csharp_opt=file_extension=.g.cs proto/ressource.proto --plugin="protoc-gen-grpc=C:/Users/mm006819/grpc/.build/grpc_csharp_plugin.exe"
mkdir csharp/echo/echopb
protoc --grpc_out=./csharp/echo/echopb --csharp_out=./csharp/echo/echopb --csharp_opt=file_extension=.g.cs proto/echo.proto --plugin="protoc-gen-grpc=C:/Users/mm006819/grpc/.build/grpc_csharp_plugin.exe"

# C++ grpc files generation.
mkdir cpp/ressource/ressourcepb
protoc --plugin="protoc-gen-grpc=C://Users//mm006819//grpc//.build//grpc_cpp_plugin.exe" --grpc_out=./cpp/ressource/ressourcepb -I proto ressource.proto
protoc --plugin="protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin" --grpc_out=./cpp/ressource/ressourcepb  -I proto ressource.proto
protoc --cpp_out=./cpp/ressource/ressourcepb -I proto ressource.proto
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

