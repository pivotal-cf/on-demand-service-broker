# on-demand-service-broker
A Cloud Foundry generic on demand service broker.

This is an on-demand broker designed to take advantage of BOSH 2.0 features such
as IP management and global cloud configuration.

In production, this application is deployed via a BOSH release. See its
[repo](https://github.com/pivotal-cf/on-demand-service-broker-release) for more
details.

## User Documentation

User documentation can be found [here](https://docs.pivotal.io/svc-sdk/odb). Documentation is targeted at service authors wishing to deploy their services on-demand and operators wanting to offer services on-demand.

## Development

### How are dependencies managed?
We only deploy this application with BOSH. Its dependencies are vendored as submodules
into the [BOSH release](https://github.com/pivotal-cf/on-demand-service-broker-release).

### Configuration
This app is configured with a config file, the path to which should be supplied on
the command line: `on-demand-broker -configFilePath /some/file.yml`.

An example configuration file is `config/test_assets/good_config.yml`.

You will need to upload a
service release for example a [Redis release](https://github.com/pivotal-cf-experimental/redis-example-service-release)
to your BOSH director.

### Running tests
You can make use of the script in `scripts/run-tests.sh` to run tests skipping system tests.

#### Running Systests
In order to run the systests you must set the environment variable that are
listed in system_tests/.envrc.template. The values for these variables can be
taken from the pipeline.yml and adjusted for your local environment

### Dev / test tools
* go 1.8
* [counterfeiter](https://github.com/maxbrunsfeld/counterfeiter) (for re-generating fakes)
* CF CLI (for system tests. See below)

### What's in that system_tests directory?
Black-box tests designed to be run against this broker, a real BOSH director and
a real Cloud Foundry.

## Guidelines for PRs

First of all, thanks for your contribution! :+1:

Please, make sure to read this few points before opening a new pull request.

1. Make sure that tests run locally running `scripts/run-tests.sh`.
2. Try to keep your pull request as small as possible by focusing on the feature you would like to add.
3. If you see opportunities for refactoring, feel free to let us know by opening an issue.
4. Try to follow the testing style we are using in the file you are modifying. Be aware that there might be inconsistencies among different tests.

