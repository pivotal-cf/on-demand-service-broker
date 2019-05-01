#!/usr/bin/env bash

usage() {
	echo "$0 <test to run> "
	echo ""
	echo "Options"
	echo "-skip-upload-releases : Will not create and upload broker, service and service adapter releases"
	exit 1
}

if [[ "$#" < "1" ]]; then
	usage
fi

pwd="$(cd $(dirname "$0"); pwd)"
source "$pwd/../scripts/prepare-env"

uploadReleases(){
    bosh create-release --name on-demand-service-broker-$DEV_ENV --dir $ODB --force
    bosh upload-release --name on-demand-service-broker-$DEV_ENV --dir $ODB --rebase

    bosh create-release --name redis-example-service-adapter-$DEV_ENV --dir $ODB/examples/redis-example-service-adapter-release --force
    bosh upload-release --name redis-example-service-adapter-$DEV_ENV --dir $ODB/examples/redis-example-service-adapter-release --rebase

    bosh create-release --name redis-service-$DEV_ENV --dir $ODB/examples/redis-example-service-release --force
    bosh upload-release --name redis-service-$DEV_ENV --dir $ODB/examples/redis-example-service-release --rebase

    bosh create-release --name kafka-example-service-adapter-$DEV_ENV --dir $ODB/examples/kafka-example-service-adapter-release --force
    bosh upload-release --name kafka-example-service-adapter-$DEV_ENV --dir $ODB/examples/kafka-example-service-adapter-release --rebase

    bosh create-release --name kafka-example-service-$DEV_ENV --dir $ODB/examples/kafka-example-service-release --force
    bosh upload-release --name kafka-example-service-$DEV_ENV --dir $ODB/examples/kafka-example-service-release --rebase

}
if [ "$2" != "-skip-upload-releases" ]; then
    uploadReleases
fi

"$pwd/run_system_tests.sh" "$@"
