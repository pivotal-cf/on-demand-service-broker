#!/usr/bin/env bash

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"${script_dir}/run-tests.sh" --race --fail-on-pending --skip-package=contract_tests,system_tests
