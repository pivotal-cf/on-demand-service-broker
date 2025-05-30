# brokerapi

[![test](https://github.com/cloudfoundry/brokerapi/actions/workflows/run-tests.yml/badge.svg?branch=main)](https://github.com/cloudfoundry/brokerapi/actions/workflows/run-tests.yml?query=branch%3Amain)

https://github.com/cloudfoundry/brokerapi/actions/workflows/run-tests/badge.svg?query=branch%3Amain

A Go package for building [V2 Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker/) compliant Service Brokers.

## [Docs](https://godoc.org/code.cloudfoundry.org/brokerapi/v13)

## Dependencies

- Go
- GNU Make 3.81

## Contributing	

We appreciate and welcome open source contribution. We will try to review the changes as soon as we can.	

## Usage

`brokerapi` defines a
[`ServiceBroker`](https://godoc.org/code.cloudfoundry.org/brokerapi/v13/domain#ServiceBroker/domain#ServiceBroker)
interface. Pass an implementation of this to
[`brokerapi.New`](https://godoc.org/code.cloudfoundry.org/brokerapi/v13#New),
which returns an `http.Handler` that you can use to serve handle HTTP requests.

## Error types

`brokerapi` defines a handful of error types in `service_broker.go` for some
common error cases that your service broker may encounter. Return these from
your `ServiceBroker` methods where appropriate, and `brokerapi` will do the
"right thing" (™), and give Cloud Foundry an appropriate status code, as per
the [Service Broker API
specification](https://docs.cloudfoundry.org/services/api.html).

### Custom Errors

`NewFailureResponse()` allows you to return a custom error from any of the
`ServiceBroker` interface methods which return an error. Within this you must
define an error, a HTTP response status code and a logging key. You can also
use the `NewFailureResponseBuilder()` to add a custom `Error:` value in the
response, or indicate that the broker should return an empty response rather
than the error message.

## Request Context

When provisioning a service `brokerapi` validates the `service_id` and `plan_id`
in the request, attaching the found instances to the request Context. These
values can be retrieved in a `brokerapi.ServiceBroker` implementation using 
utility methods `RetrieveServiceFromContext` and `RetrieveServicePlanFromContext`
as shown below.

```go
func (sb *ServiceBrokerImplementation) Provision(ctx context.Context,
  instanceID string, details brokerapi.ProvisionDetails, asyncAllowed bool) {

  service := brokerapi.RetrieveServiceFromContext(ctx)
  if service == nil {
    // Lookup service
  }

  // [..]
}
```

## Originating Identity

The request context for every request contains the unparsed
`X-Broker-API-Originating-Identity` header under the key
`originatingIdentity`.  More details on how the Open Service Broker API
manages request originating identity is available
[here](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#originating-identity).

## Request Identity

The request context for every request contains the unparsed
`X-Broker-API-Request-Identity` header under the key
`requestIdentity`.  More details on how the Open Service Broker API
manages request originating identity is available
[here](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#request-identity).

## Example Service Broker

You can see the
[cf-redis](https://github.com/pivotal-cf/cf-redis-broker/blob/2f0e9a8ebb1012a9be74bbef2d411b0b3b60352f/broker/broker.go)
service broker uses the BrokerAPI package to create a service broker for Redis.

## Releasing

Releasing steps can be found [here](https://github.com/cloudfoundry/brokerapi/wiki/Releasing-new-BrokerAPI-major-version)

