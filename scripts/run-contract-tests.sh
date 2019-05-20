#!/usr/bin/env bash

usage() {
	echo "$0 <test to run> "
	echo ""
	exit 1
}

if [[ "$#" < "1" ]]; then
	usage
fi

pwd="$(cd $(dirname "$0"); pwd)"
source "$pwd/prepare-env"

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
