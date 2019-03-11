#!/usr/bin/env bash
usage() {
	echo "$0 <env name> <test to run>"
	exit 1
}

if [ "$#" != "2" ]; then
	usage
fi

env_name="${1:-$(cat $ENV_PATH/name)}"
shift

source $HOME/workspace/services-enablement-lites-pool/bosh-lites/**/${env_name}

$META/concourse/bosh-lites-pool/tasks/make-broker-deployment-vars.sh "${env_name}" > /tmp/broker-${env_name}.yml

export BOSH_DEPLOYMENT_VARS="/tmp/broker-${env_name}.yml"
export BROKER_SYSTEM_DOMAIN="$BOSH_LITE_DOMAIN"
export CONSUL_REQUIRED=false
export EXAMPLE_SI_API_PATH=~/workspace/example-service-instances-api
export KAFKA_EXAMPLE_APP_PATH=~/workspace/kafka-example-app
export KAFKA_SERVICE_ADAPTER_RELEASE_NAME=kafka-example-service-adapter
export KAFKA_SERVICE_RELEASE_NAME=kafka-example-service
export ODB_RELEASE_TEMPLATES_PATH=~/workspace/on-demand-service-broker-release/examples/deployment
export ODB_VERSION=latest
export REDIS_EXAMPLE_APP_PATH=~/workspace/cf-redis-example-app
export REDIS_SERVICE_ADAPTER_RELEASE_NAME=redis-example-service-adapter
export REDIS_SERVICE_RELEASE_NAME=redis-service
export SI_API_PATH="$HOME/workspace/example-service-instances-api/"

echo -e "$BOSH_GW_PRIVATE_KEY_CONTENTS" > /tmp/jb-$env_name.pem
chmod 700 /tmp/jb-$env_name.pem
export BOSH_GW_PRIVATE_KEY=/tmp/jb-$env_name.pem

export BOSH_NON_INTERACTIVE=true

export CF_URL="$(bosh int --path /cf/api_url "$BOSH_DEPLOYMENT_VARS")"
export CF_CLIENT_ID="$(bosh int --path /cf/client_credentials/client_id "$BOSH_DEPLOYMENT_VARS" 2>/dev/null)"
export CF_CLIENT_SECRET="$(bosh int --path /cf/client_credentials/client_secret "$BOSH_DEPLOYMENT_VARS" 2>/dev/null)"
export CF_USERNAME="$(bosh int --path /cf/user_credentials/username "$BOSH_DEPLOYMENT_VARS" 2>/dev/null)"
export CF_PASSWORD="$(bosh int --path /cf/user_credentials/password "$BOSH_DEPLOYMENT_VARS" 2>/dev/null)"
export CF_ORG="$(bosh int --path /cf/org "$BOSH_DEPLOYMENT_VARS")"
export CF_SPACE="$(bosh int --path /cf/space "$BOSH_DEPLOYMENT_VARS")"
export DOPPLER_ADDRESS=wss://doppler.$BOSH_LITE_DOMAIN


bosh create-release --name on-demand-service-broker-$DEV_ENV --dir $ODB --force
bosh upload-release --name on-demand-service-broker-$DEV_ENV --dir $ODB --rebase

bosh create-release --name redis-example-service-adapter-$DEV_ENV --dir $ODB/examples/redis-example-service-adapter-release --force
bosh upload-release --name redis-example-service-adapter-$DEV_ENV --dir $ODB/examples/redis-example-service-adapter-release --rebase

bosh create-release --name redis-service-$DEV_ENV --dir $ODB/examples/redis-example-service-release --force
bosh upload-release --name redis-service-$DEV_ENV --dir $ODB/examples/redis-example-service-release --rebase

./run_system_tests.sh "$@"
