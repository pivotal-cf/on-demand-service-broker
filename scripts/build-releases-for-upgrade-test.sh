#!/usr/bin/env bash

set -e

upgrade_from_version="0.28.0"

rm -rf /tmp/odb
mkdir -p /tmp/odb/dev-releases/

pushd /tmp/odb
    git clone git@github.com:pivotal-cf/on-demand-service-broker-release.git odb-release
    cd odb-release

    git checkout "v$upgrade_from_version"
    git pullsubs

    bosh create-release \
        --name on-demand-service-broker-prev \
        --version "${upgrade_from_version}" \
        --tarball "/tmp/odb/dev-releases/on-demand-service-broker-prev-${upgrade_from_version}.tgz" \
        --force

    for release in redis-example-service-adapter redis-example-service; do
        pushd "examples/${release}-release"
            git checkout "v${upgrade_from_version}"
            bosh create-release \
                --name "${release}-prev" \
                --version "${upgrade_from_version}" \
                --tarball "/tmp/odb/dev-releases/${release}-prev-${upgrade_from_version}.tgz"
        popd
    done
popd

if ! gsutil cp -r /tmp/odb/dev-releases/* gs://dev-releases; then
    echo "Failed to move releases to s3 bucket. Are you logged in?"
    echo "try 'gcloud init' or 'gcloud auth login'"
fi

echo "All done"