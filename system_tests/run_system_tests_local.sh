#!/usr/bin/env bash

usage() {
  echo "Usage: $(basename "$0") [vars.yml] [system tests]"
  echo
  echo 'Command Arguments:'
  echo '  vars.yml: path to broker deployment variables [$BROKER_DEPLOYMENT_VARS]'
  echo '  system tests: comma separated list of system tests to run. Defaults to all.'
  echo 'Environment variables:'
  echo '  $ODB_BROKER_RELEASE_PATH: path to on-demand-broker-release directory'
  echo '  $SI_API_PATH: path to example-service-instance-api app directory (https://github.com/pivotal-cf-experimental/example-service-instances-api)'
  exit
}

if [ -n "$BROKER_DEPLOYMENT_VARS" ]; then
  broker_dep_vars="$BROKER_DEPLOYMENT_VARS"
else
  if [ "$#" -lt "1" ]; then
    usage
  fi
  broker_dep_vars="$1"
  shift
fi

cwd="$(dirname "$0")"
export BOSH_ENVIRONMENT="$(bosh int --path /bosh/url "$broker_dep_vars")"
export BOSH_CLIENT="$(bosh int --path /bosh/authentication/username "$broker_dep_vars")"
export BOSH_CLIENT_SECRET="$(bosh int --path /bosh/authentication/password "$broker_dep_vars")"
export BOSH_NON_INTERACTIVE=true
export BOSH_CA_CERT="$(bosh int --path /bosh/root_ca_cert "$broker_dep_vars")"
export CF_URL="$(bosh int --path /cf/api_url "$broker_dep_vars")"
export CF_CLIENT_ID="$(bosh int --path /cf/client_credentials/client_id "$broker_dep_vars" 2>/dev/null)"
export CF_CLIENT_SECRET="$(bosh int --path /cf/client_credentials/client_secret "$broker_dep_vars" 2>/dev/null)"
export CF_USERNAME="$(bosh int --path /cf/user_credentials/username "$broker_dep_vars" 2>/dev/null)"
export CF_PASSWORD="$(bosh int --path /cf/user_credentials/password "$broker_dep_vars" 2>/dev/null)"
export CF_ORG="$(bosh int --path /cf/org "$broker_dep_vars")"
export CF_SPACE="$(bosh int --path /cf/space "$broker_dep_vars")"
export DEV_ENV=local
export SERVICE_RELEASE_NAME=redis-service
export BROKER_SYSTEM_DOMAIN="$(bosh int --path /cf/system_domain "$broker_dep_vars")"
set -x
odb_release_path="$ODB_BROKER_RELEASE_PATH"
if [ -z "$odb_release_path" ]; then
  odb_release_path="$(cd $cwd/../../../../..; pwd)"
fi
export ODB_RELEASE_TEMPLATES_PATH="$odb_release_path/examples/deployment"
if [ ! -d "$ODB_RELEASE_TEMPLATES_PATH" ]; then
  echo -e "\nODB_BROKER_RELEASE_PATH not properly set\n"
  usage
fi
export BOSH_DEPLOYMENT_VARS="$broker_dep_vars"

export SI_API_PATH="${SI_API_PATH:-"$HOME/workspace/example-service-instances-api/"}"

create_and_upload_releases() {
  $odb_release_path/vbox/create-and-upload-releases.sh $odb_release_path
  $odb_release_path/vbox/create-and-upload-releases.sh $odb_release_path/examples/redis-example-service-adapter-release
  $odb_release_path/vbox/create-and-upload-releases.sh $odb_release_path/examples/redis-example-service-release
}

if [ -z "${SKIP_UPLOAD_RELEASES:-""}" ]; then
    create_and_upload_releases
fi

"$(dirname $0)"/run_system_tests.sh "$@"
