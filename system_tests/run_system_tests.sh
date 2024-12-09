#!/usr/bin/env bash

# Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
# This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

set -eu

if [[ $# -eq 0 ]]; then
  echo "About to run ALL the system tests."
  echo "Ctrl-C now to terminate..."
  sleep 5
fi

cf api ${CF_URL} --skip-ssl-validation
cf logout
cf auth $CF_USERNAME $CF_PASSWORD
cf target -o ${CF_ORG} -s ${CF_SPACE} # must already exist

go run github.com/onsi/ginkgo/v2/ginkgo -r \
  --randomize-suites \
  --randomize-all \
  --keep-going \
  --race \
  --fail-on-pending \
  --skip-package=upgrade_deployment_tests \
  "$@"
