module github.com/pivotal-cf/on-demand-service-broker

go 1.13

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	code.cloudfoundry.org/credhub-cli v0.0.0-20190923163340-a6d1ba3b23bd
	code.cloudfoundry.org/lager v1.1.1-0.20191025172104-c49605531964
	code.cloudfoundry.org/workpool v0.0.0-20200131000409-2ac56b354115 // indirect
	github.com/apoydence/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/aws/aws-sdk-go v1.31.7 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/bmatcuk/doublestar v1.1.5 // indirect
	github.com/charlievieth/fs v0.0.0-20170613215519-7dc373669fa1 // indirect
	github.com/cheggaaa/pb v2.0.7+incompatible // indirect
	github.com/cloudfoundry-community/go-uaa v0.3.1
	github.com/cloudfoundry/bosh-agent v2.319.0+incompatible // indirect
	github.com/cloudfoundry/bosh-cli v6.2.1+incompatible
	github.com/cloudfoundry/bosh-davcli v0.0.44 // indirect
	github.com/cloudfoundry/bosh-gcscli v0.0.18 // indirect
	github.com/cloudfoundry/bosh-s3cli v0.0.95 // indirect
	github.com/cloudfoundry/bosh-utils v0.0.0-20191026100324-0b6803ec5382
	github.com/cloudfoundry/config-server v0.1.21 // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/noaa v2.1.0+incompatible
	github.com/cloudfoundry/socks5-proxy v0.2.0 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20171206171820-b33733203bb4
	github.com/cppforlife/go-patch v0.2.0 // indirect
	github.com/cppforlife/go-semi-semantic v0.0.0-20160921010311-576b6af77ae4 // indirect
	github.com/craigfurman/herottp v0.0.0-20190418132442-c546d62f2a8d
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/gogo/protobuf v0.0.0-20171007142547-342cbe0a0415 // indirect
	github.com/gorilla/mux v1.7.4
	github.com/gorilla/websocket v1.2.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0 // indirect
	github.com/hashicorp/go-version v0.0.0-20170914154128-fc61389e27c7 // indirect
	github.com/jessevdk/go-flags v1.4.0 // indirect
	github.com/mailru/easyjson v0.0.0-20171120080333-32fa128f234d // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.2.3
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.12.3
	github.com/onsi/gomega v1.10.1
	github.com/pborman/uuid v1.2.0
	github.com/pivotal-cf/brokerapi/v7 v7.3.0
	github.com/pivotal-cf/on-demand-services-sdk v0.40.0
	github.com/pivotal-cf/paraphernalia v0.0.0-20171027171623-4272315231ce // indirect
	github.com/pkg/errors v0.9.1
	github.com/poy/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/square/certstrap v1.2.0 // indirect
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	github.com/urfave/negroni v1.0.0
	github.com/vito/go-interact v1.0.0 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0
	golang.org/x/crypto v0.0.0-20200302210943-78000ba7a073 // indirect
	gopkg.in/VividCortex/ewma.v1 v1.1.1 // indirect
	gopkg.in/cheggaaa/pb.v2 v2.0.7 // indirect
	gopkg.in/yaml.v2 v2.3.0
)

replace gopkg.in/fsnotify.v1 v1.4.7 => gopkg.in/fsnotify/fsnotify.v1 v1.4.7
