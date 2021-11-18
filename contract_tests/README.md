Contract tests test specific interactions with external APIs. The APIs that ODB use are CF, BOSH, Credhub, UAA and Telemetry. You can find a list of endpoints that are used from each [here](https://docs.pivotal.io/svc-sdk/odb/0-42/security.html).
The aim of these tests is to check that the APIs interfaces that ODB relies on do not change.

Running the contract tests:
- These tests make real HTTP calls to the collaborating APIs and therefore you need a bosh-lite environment
  - You can get a new environment by running:
    ```
    claim-lite
    target-lite <env-name>
    ```
- From the `on-demand-service-broker` repo run
  ```
  ./scripts/run-contract-tests-on-lite <test-name> 
  ```
