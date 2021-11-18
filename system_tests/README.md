Description: 

System tests are end-to-end tests that validate specific broker behaviours by using a real example of a service adapter and release (Redis or Kafka). 

Running the tests:
- Locally 
  - These are time-consuming and should be run locally only when making changes on a specific
  - You should first claim a bosh-lite environment by running:
  
  ```
  claim-lite
  target-lite <env-name>
  ```
  - From the `on-demand-service-broker` repo run

  ```
  ./system_tests/run_system_tests_bosh_lite.sh <test-name> 
  ```

- In CI
  
These tests are run on bosh-lites on every commit in the [ODB pipeline](https://hush-house.pivotal.io/teams/services-enablement/pipelines/odb) and are run nightly on a PCF toolsmith environment in the different versions of the [PCF pipelines](https://hush-house.pivotal.io/teams/services-enablement/pipelines/pcf-2.11.lts2-tests). 
