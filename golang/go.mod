module github.com/globulario/services/golang

go 1.21

toolchain go1.21.4

replace github.com/davecourtois/Utility => ../../Utility
replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0

require (
	github.com/SebastiaanKlippert/go-wkhtmltopdf v1.7.0
	github.com/StalkR/httpcache v1.0.0
	github.com/StalkR/imdb v1.0.15
	github.com/alexbrainman/odbc v0.0.0-20200426075526-f0492dfa1575
	github.com/allegro/bigcache/v3 v3.1.0
	github.com/anacrolix/torrent v1.41.0
	github.com/barasher/go-exiftool v1.8.0
	github.com/blevesearch/bleve v1.0.14
	github.com/davecourtois/Utility v0.0.0-20231126190644-a97e89dc06b0
	github.com/denisenkom/go-mssqldb v0.10.0
	github.com/dgraph-io/badger/v3 v3.2103.5
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dhowden/tag v0.0.0-20220618230019-adf36e896086
	github.com/djimenez/iconv-go v0.0.0-20160305225143-8960e66bd3da
	github.com/emersion/go-imap v1.1.0
	github.com/emersion/go-message v0.14.1
	github.com/emersion/go-smtp v0.15.0
	github.com/emicklei/proto v1.12.2
	github.com/fsnotify/fsnotify v1.7.0
	github.com/go-ldap/ldap/v3 v3.4.6
	github.com/go-sql-driver/mysql v1.6.0
	github.com/gocolly/colly/v2 v2.1.0
	github.com/gocql/gocql v1.6.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/improbable-eng/grpc-web v0.15.0
	github.com/jasonlvhit/gocron v0.0.1
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0
	github.com/karmdip-mi/go-fitz v0.0.0-20210702102225-a530a79566e9
	github.com/lib/pq v1.10.2
	github.com/mattn/go-sqlite3 v2.0.2+incompatible
	github.com/mhale/smtpd v0.0.0-20210322105601-438c8edb069c
	github.com/miekg/dns v1.1.42
	github.com/prometheus/client_golang v1.17.0
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/soheilhy/cmux v0.1.5
	github.com/struCoder/pidusage v0.2.1
	github.com/syndtr/goleveldb v1.0.0
	github.com/tealeg/xlsx v1.0.5
	github.com/txn2/txeh v1.5.5
	go.etcd.io/etcd v3.3.27+incompatible
	go.mongodb.org/mongo-driver v1.5.2
	golang.org/x/crypto v0.16.0
	golang.org/x/net v0.19.0
	google.golang.org/grpc v1.59.0
	google.golang.org/protobuf v1.31.0
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/PuerkitoBio/goquery v1.5.1 // indirect
	github.com/andybalholm/cascadia v1.2.0 // indirect
	github.com/antchfx/htmlquery v1.2.3 // indirect
	github.com/antchfx/xmlquery v1.2.4 // indirect
	github.com/antchfx/xpath v1.1.8 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/coreos/etcd v3.3.27+incompatible // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/coreos/pkg v0.0.0-20230601102743-20bbbf26f4d8 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	github.com/jackpal/gateway v1.0.13 // indirect
	github.com/kennygrant/sanitize v1.2.4 // indirect
	github.com/lor00x/goldap v0.0.0-20180618054307-a546dffdd1a3 // indirect
	github.com/matttproud/golang_protobuf_extensions/v2 v2.0.0 // indirect
	github.com/mitchellh/colorstring v0.0.0-20190213212951-d06e56a500db // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polds/imgbase64 v0.0.0-20140820003345-cb7bf37298b7 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/rs/cors v1.10.1 // indirect
	github.com/saintfish/chardet v0.0.0-20120816061221-3af4cd4741ca // indirect
	github.com/schollz/progressbar/v3 v3.14.1 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/stretchr/testify v1.8.4 // indirect
	github.com/temoto/robotstxt v1.1.1 // indirect
	github.com/vjeantet/ldapserver v1.0.1 // indirect
	go.etcd.io/etcd/api/v3 v3.5.10 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.10 // indirect
	go.etcd.io/etcd/client/v3 v3.5.10 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/tools v0.0.0-20190618225709-2cfd321de3ee // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/oauth2 v0.12.0 // indirect
	golang.org/x/term v0.15.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231127180814-3a041ad873d4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231127180814-3a041ad873d4 // indirect
	google.golang.org/grpc/examples v0.0.0-20221006202345-c03925db8d3c // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	honnef.co/go/tools v0.0.1-2020.1.4 // indirect
	nhooyr.io/websocket v1.8.10 // indirect
)

require (
	github.com/Azure/go-ntlmssp v0.0.0-20221128193559-754e69321358 // indirect
	github.com/RoaringBitmap/roaring v0.9.4 // indirect
	github.com/anacrolix/chansync v0.3.0 // indirect
	github.com/anacrolix/confluence v1.9.0 // indirect
	github.com/anacrolix/dht/v2 v2.15.2-0.20220123034220-0538803801cb // indirect
	github.com/anacrolix/envpprof v1.1.1 // indirect
	github.com/anacrolix/go-libutp v1.3.1 // indirect
	github.com/anacrolix/log v0.13.1 // indirect
	github.com/anacrolix/missinggo v1.3.0 // indirect
	github.com/anacrolix/missinggo/perf v1.0.0 // indirect
	github.com/anacrolix/missinggo/v2 v2.5.2 // indirect
	github.com/anacrolix/mmsg v1.0.0 // indirect
	github.com/anacrolix/multiless v0.2.0 // indirect
	github.com/anacrolix/stm v0.3.0 // indirect
	github.com/anacrolix/sync v0.4.0 // indirect
	github.com/anacrolix/upnp v0.1.3-0.20220123035249-922794e51c96 // indirect
	github.com/anacrolix/utp v0.1.0 // indirect
	github.com/aws/aws-sdk-go v1.34.28 // indirect
	github.com/benbjohnson/immutable v0.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.2.0 // indirect
	github.com/blevesearch/go-porterstemmer v1.0.3 // indirect
	github.com/blevesearch/mmap-go v1.0.2 // indirect
	github.com/blevesearch/segment v0.9.0 // indirect
	github.com/blevesearch/snowballstem v0.9.0 // indirect
	github.com/blevesearch/zap/v11 v11.0.14 // indirect
	github.com/blevesearch/zap/v12 v12.0.14 // indirect
	github.com/blevesearch/zap/v13 v13.0.6 // indirect
	github.com/blevesearch/zap/v14 v14.0.5 // indirect
	github.com/blevesearch/zap/v15 v15.0.3 // indirect
	github.com/bradfitz/iter v0.0.0-20191230175014-e8f45d346db8 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/chai2010/webp v1.1.1 // indirect
	github.com/couchbase/vellum v1.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto v0.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/edsrzf/mmap-go v1.0.0 // indirect
	github.com/emersion/go-sasl v0.0.0-20200509203442-7bfe0ed36a21 // indirect
	github.com/emersion/go-textwrapper v0.0.0-20200911093747-65d896831594 // indirect
	github.com/glendc/go-external-ip v0.1.0 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.5 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang/glog v1.2.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/btree v1.0.1 // indirect
	github.com/google/flatbuffers v23.5.26+incompatible // indirect
	github.com/google/uuid v1.4.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kalafut/imohash v1.0.2 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/martinlindhe/base36 v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/go-ps v1.0.0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pion/datachannel v1.5.2 // indirect
	github.com/pion/dtls/v2 v2.1.2 // indirect
	github.com/pion/ice/v2 v2.1.20 // indirect
	github.com/pion/interceptor v0.1.7 // indirect
	github.com/pion/logging v0.2.2 // indirect
	github.com/pion/mdns v0.0.5 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.9 // indirect
	github.com/pion/rtp v1.7.4 // indirect
	github.com/pion/sctp v1.8.2 // indirect
	github.com/pion/sdp/v3 v3.0.4 // indirect
	github.com/pion/srtp/v2 v2.0.5 // indirect
	github.com/pion/stun v0.3.5 // indirect
	github.com/pion/transport v0.13.0 // indirect
	github.com/pion/turn/v2 v2.0.6 // indirect
	github.com/pion/udp v0.1.1 // indirect
	github.com/pion/webrtc/v3 v3.1.24-0.20220208053747-94262c1b2b38 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.45.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20200410134404-eec4a21b6bb0 // indirect
	github.com/rs/dnscache v0.0.0-20210201191234-295bba877686 // indirect
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	github.com/steveyen/gtreap v0.1.0 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tklauser/numcpus v0.3.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/willf/bitset v1.1.11 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.0.2 // indirect
	github.com/xdg-go/stringprep v1.0.2 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/image v0.14.0 // indirect
	golang.org/x/sync v0.5.0
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20231127180814-3a041ad873d4 // indirect
	modernc.org/libc v1.11.82 // indirect
	modernc.org/mathutil v1.4.1 // indirect
	modernc.org/memory v1.0.5 // indirect
	modernc.org/sqlite v1.14.2-0.20211125151325-d4ed92c0a70f // indirect
	zombiezen.com/go/sqlite v0.8.0 // indirect
)
