#!/usr/bin/env bash

set -euo pipefail

usage() {
	echo "$0 <test to run> "
	echo ""
	exit 1
}

if [[ "$#" -lt "1" ]]; then
	usage
fi

pwd="$(cd $(dirname "$0"); pwd)"
source "$pwd/prepare-env"

uploadReleases(){
    bosh create-release --name on-demand-service-broker-$DEV_ENV --dir $ODB --force
    bosh upload-release --name on-demand-service-broker-$DEV_ENV --dir $ODB --rebase

    bosh create-release --name redis-example-service-adapter-$DEV_ENV --dir $ODB/examples/redis-example-service-adapter-release --force
    bosh upload-release --name redis-example-service-adapter-$DEV_ENV --dir $ODB/examples/redis-example-service-adapter-release --rebase

    bosh create-release --name redis-example-service-$DEV_ENV --dir $ODB/examples/redis-example-service-release --force
    bosh upload-release --name redis-example-service-$DEV_ENV --dir $ODB/examples/redis-example-service-release --rebase
}

if [ "${2:-""}" != "-skip-upload-releases" ]; then
    uploadReleases
fi

export DUMMY_RELEASE_SHA="02ffb94879f11518a91aedff8507fe7a28deb6fa"
export DUMMY_RELEASE_URL="https://s3.amazonaws.com/cf-services-internal-builds/dummy-bosh-release/dummy-release-2%2Bdev.1.tgz"

ginkgo \
  -randomizeSuites=true \
  -randomizeAllSpecs=true \
  -keepGoing=true \
  -r \
  -cover \
  -trace \
  -race \
  "$@"
