#!/usr/bin/env bash

set -euo pipefail

: "${DEV_ENV:=local}"
: "${ODB:=$HOME/workspace/on-demand-service-broker-release}"

usage() {
	echo "$0 <test to run> "
	echo ""
	exit 1
}

if [[ "$#" -lt "1" ]]; then
	usage
fi

WORKSPACE_DIR="$HOME/workspace" source "$HOME/workspace/services-enablement-meta/concourse/odb/scripts/export-env-vars"

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
export DUMMY_RELEASE_URL="https://dummy-bosh-release.s3.amazonaws.com/dummy-release-2%2Bdev.1.tgz"

GO111MODULE=on GOFLAGS="-mod=vendor" go run github.com/onsi/ginkgo/v2/ginkgo \
  --randomize-suites \
  --randomize-all \
  --keep-going \
  -r \
  -cover \
  -trace \
  -race \
  "$@"
