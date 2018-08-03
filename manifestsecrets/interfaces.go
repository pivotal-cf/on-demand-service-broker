package manifestsecrets

import (
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

//go:generate counterfeiter -o fakes/fake_credhub_operator.go . CredhubOperator

type CredhubOperator interface {
	BulkGet(map[string]boshdirector.Variable, *log.Logger) (map[string]string, error)
	FindNameLike(name string, logger *log.Logger) ([]string, error)
	BulkDelete(paths []string, logger *log.Logger) error
}

//go:generate counterfeiter -o fakes/fake_matcher.go . Matcher

type Matcher interface {
	Match(manifest []byte, deploymentVariables []boshdirector.Variable) (map[string]boshdirector.Variable, error)
}
