module github.com/globulario/services/golang

go 1.16

replace github.com/davecourtois/Utility => ../../Utility

require (
	github.com/SebastiaanKlippert/go-wkhtmltopdf v1.7.0
	github.com/alexbrainman/odbc v0.0.0-20200426075526-f0492dfa1575
	github.com/allegro/bigcache v1.2.1
	github.com/davecourtois/GoXapian v0.0.0-20201222213557-81c72bc9e73c
	github.com/davecourtois/Utility v0.0.0-20210515191918-3118f6f72191
	github.com/denisenkom/go-mssqldb v0.10.0
	github.com/dgraph-io/badger/v3 v3.2103.2
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/djimenez/iconv-go v0.0.0-20160305225143-8960e66bd3da
	github.com/emersion/go-imap v1.1.0
	github.com/emersion/go-message v0.14.1
	github.com/emersion/go-smtp v0.15.0
	github.com/emicklei/proto v1.9.0
	github.com/go-ldap/ldap/v3 v3.3.0
	github.com/go-sql-driver/mysql v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.4.0
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/lib/pq v1.10.2
	github.com/mattn/go-sqlite3 v1.14.7
	github.com/mhale/smtpd v0.0.0-20210322105601-438c8edb069c
	github.com/miekg/dns v1.1.42
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/polds/imgbase64 v0.0.0-20140820003345-cb7bf37298b7
	github.com/prometheus/client_golang v1.10.0
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/struCoder/pidusage v0.2.1
	github.com/syndtr/goleveldb v1.0.0
	github.com/tealeg/xlsx v1.0.5
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/txn2/txeh v1.3.0
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.mongodb.org/mongo-driver v1.5.2
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20210525063256-abc453219eb5
	google.golang.org/genproto v0.0.0-20210524171403-669157292da3 // indirect
	google.golang.org/grpc v1.38.0
	google.golang.org/protobuf v1.26.0
	gopkg.in/alexcesaro/quotedprintable.v2 v2.0.0-20150314193201-9b4a113f96b3 // indirect
	gopkg.in/gomail.v1 v1.0.0-20150320132819-11b919ab4933
)
