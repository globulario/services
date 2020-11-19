module github.com/globulario/services

go 1.15

replace github.com/globulario/Globular => ../Globular

require (
	github.com/allegro/bigcache v1.2.1
	github.com/davecourtois/Utility v0.0.0-20201022131821-ab9db56292ab
	github.com/globulario/Globular v0.0.0-00010101000000-000000000000
	github.com/golang/protobuf v1.4.3
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/polds/imgbase64 v0.0.0-20140820003345-cb7bf37298b7 // indirect
	github.com/syndtr/goleveldb v1.0.0
	github.com/tealeg/xlsx v1.0.5 // indirect
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.25.0
)
