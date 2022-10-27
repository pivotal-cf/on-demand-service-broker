Description: 

System tests are end-to-end tests that validate specific broker behaviours by using a real example of a service adapter and release (Redis or Kafka). 

Running the tests:
- Locally 
  - These tests require cf cli v6, tests may fail under newer cli versions.
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
  
These tests are run on bosh-lites on every commit in the [ODB pipeline](https://dedicated-mysql.ci.cf-app.com/teams/main/pipelines/odb).
