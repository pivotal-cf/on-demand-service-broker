package manifestsecrets

import (
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

//go:generate counterfeiter -o fakes/fake_bulk_getter.go . BulkGetter

type BulkGetter interface {
	BulkGet(map[string]boshdirector.Variable, *log.Logger) (map[string]string, error)
}

//go:generate counterfeiter -o fakes/fake_matcher.go . Matcher

type Matcher interface {
	Match(manifest []byte, deploymentVariables []boshdirector.Variable) (map[string]boshdirector.Variable, error)
}
