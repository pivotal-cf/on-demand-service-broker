module github.com/pivotal-cf/on-demand-service-broker

go 1.13

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	code.cloudfoundry.org/credhub-cli v0.0.0-20190923163340-a6d1ba3b23bd
	code.cloudfoundry.org/lager v1.1.1-0.20191025172104-c49605531964
	github.com/apoydence/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/bmatcuk/doublestar v1.1.5 // indirect
	github.com/charlievieth/fs v0.0.0-20170613215519-7dc373669fa1 // indirect
	github.com/cloudfoundry-community/go-uaa v0.3.1
	github.com/cloudfoundry/bosh-cli v6.4.0+incompatible
	github.com/cloudfoundry/bosh-utils v0.0.0-20191026100324-0b6803ec5382
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/noaa v2.1.0+incompatible
	github.com/cloudfoundry/socks5-proxy v0.2.0 // indirect
	github.com/cloudfoundry/sonde-go v0.0.0-20171206171820-b33733203bb4
	github.com/cppforlife/go-semi-semantic v0.0.0-20160921010311-576b6af77ae4 // indirect
	github.com/craigfurman/herottp v0.0.0-20190418132442-c546d62f2a8d
	github.com/elazarl/goproxy v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20191011121108-aa519ddbe484 // indirect
	github.com/gogo/protobuf v0.0.0-20171007142547-342cbe0a0415 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.2.0 // indirect
	github.com/hashicorp/go-version v0.0.0-20170914154128-fc61389e27c7 // indirect
	github.com/mailru/easyjson v0.0.0-20171120080333-32fa128f234d // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.2.3
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d // indirect
	github.com/onsi/ginkgo v1.15.0
	github.com/onsi/gomega v1.10.5
	github.com/pborman/uuid v1.2.1
	github.com/pivotal-cf/brokerapi/v7 v7.4.0
	github.com/pivotal-cf/on-demand-services-sdk v0.40.1-0.20200916152143-812ef1f3436b
	github.com/pivotal-cf/paraphernalia v0.0.0-20171027171623-4272315231ce // indirect
	github.com/pkg/errors v0.9.1
	github.com/poy/eachers v0.0.0-20181020210610-23942921fe77 // indirect
	github.com/square/certstrap v1.2.0 // indirect
	github.com/stretchr/testify v1.5.1 // indirect
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	github.com/urfave/negroni v1.0.0
	github.com/xeipuuv/gojsonschema v1.2.0
	gopkg.in/yaml.v2 v2.3.0
)

replace gopkg.in/fsnotify.v1 v1.4.7 => gopkg.in/fsnotify/fsnotify.v1 v1.4.7
