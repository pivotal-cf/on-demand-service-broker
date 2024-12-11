#!/usr/bin/env bash

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

: "${ENVIRONMENT_LOCK_METADATA:?ENVIRONMENT_LOCK_METADATA not set. Are you targeting a Shepherd v2 bosh-lite environment?}"
: "${ODB:=$HOME/workspace/on-demand-service-broker-release}"
: "${SKIP_UPLOAD_RELEASES:=false}"

usage() {
  echo "$0 <test to run> "
  echo ""
  echo "Options"
  echo "-skip-upload-releases : Will not create and upload broker, service and service adapter releases"
  exit 1
}

if [[ "$#" -lt 1 ]]; then
  usage
fi

WORKSPACE_DIR="$HOME/workspace" source "$HOME/workspace/services-enablement-meta/concourse/odb/scripts/export-env-vars"

upload_releases() {
  bosh create-release --dir "$ODB" --force --timestamp-version
  bosh upload-release --dir "$ODB"

  bosh create-release --dir "$ODB/examples/redis-example-service-adapter-release" --force --timestamp-version
  bosh upload-release --dir "$ODB/examples/redis-example-service-adapter-release"

  bosh create-release --dir "$ODB/examples/redis-example-service-release" --force --timestamp-version
  bosh upload-release --dir "$ODB/examples/redis-example-service-release"

  bosh create-release --dir "$ODB/examples/kafka-example-service-adapter-release" --force --timestamp-version
  bosh upload-release --dir "$ODB/examples/kafka-example-service-adapter-release"

  bosh create-release --dir "$ODB/examples/kafka-example-service-release" --force --timestamp-version
  bosh upload-release --dir "$ODB/examples/kafka-example-service-release"
}

if [[ $SKIP_UPLOAD_RELEASES != "true" ]]; then
  upload_releases
fi

"${script_dir}/run_system_tests.sh" "$@"
