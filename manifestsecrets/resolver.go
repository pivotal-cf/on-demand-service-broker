package manifestsecrets

import (
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

type NoopSecretResolver struct{}

func (r *NoopSecretResolver) ResolveManifestSecrets(manifest []byte, deploymentVariables []boshdirector.Variable, logger *log.Logger) (map[string]string, error) {
	return map[string]string{}, nil
}

type BoshCredHubSecretResolver struct {
	matcher        Matcher
	secretsFetcher BulkGetter
}

func NewResolver(resolveAtBind bool, matcher Matcher, secretsFetcher BulkGetter) broker.ManifestSecretResolver {
	if !resolveAtBind {
		return new(NoopSecretResolver)
	}

	return &BoshCredHubSecretResolver{
		matcher:        matcher,
		secretsFetcher: secretsFetcher,
	}
}

func (r *BoshCredHubSecretResolver) ResolveManifestSecrets(manifest []byte, deploymentVariables []boshdirector.Variable, logger *log.Logger) (map[string]string, error) {
	matches, err := r.matcher.Match(manifest, deploymentVariables)
	if err != nil {
		return nil, err
	}

	secrets, err := r.secretsFetcher.BulkGet(matches, logger)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}
