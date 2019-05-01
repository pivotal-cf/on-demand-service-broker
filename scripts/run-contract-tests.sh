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

ginkgo \
  -randomizeSuites=true \
  -randomizeAllSpecs=true \
  -keepGoing=true \
  -r \
  -cover \
  -trace \
  -race \
  "$@"
