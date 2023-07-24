Description: 

System tests are end-to-end tests that validate specific broker behaviours by using a real example of a service adapter and release (Redis or Kafka). 

Running the tests:
- Locally 
  - These tests require cf cli v8, tests may fail under older cli versions.
  - The system tests require a cf-redis-example-app is cloned locally in ~/workspace/cf-redis-example-app.
  - The system tests require a cf-kafka-example-app is cloned locally in ~/workspace/cf-kafka-example-app.
  - Running these tests are time-consuming, and it may be prudent to only run the system tests relevant to the feature you are working on.
  - You should first claim a bosh-lite environment maintained by your CI environment, perhaps with the following commands:
  
     ```
     $ claim-lite
     $ target-lite <env-name>
     ```
  

  - From the `on-demand-service-broker` repo run

     ```
     ./system_tests/run_system_tests_bosh_lite.sh <test-name> 
     ```

  
In CI, these tests are run on bosh-lites on every commit in the [ODB pipeline](https://dedicated-mysql.ci.cf-app.com/teams/main/pipelines/odb).
