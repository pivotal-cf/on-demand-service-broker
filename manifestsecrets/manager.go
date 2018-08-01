package manifestsecrets

import (
	"log"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
	"github.com/pivotal-cf/on-demand-service-broker/broker"
)

type NoopSecretManager struct{}

func (r *NoopSecretManager) ResolveManifestSecrets(manifest []byte, deploymentVariables []boshdirector.Variable, logger *log.Logger) (map[string]string, error) {
	return nil, nil
}

type BoshCredHubSecretManager struct {
	matcher        Matcher
	secretsFetcher BulkGetter
}

func BuildManager(resolveAtBind bool, matcher Matcher, secretsFetcher BulkGetter) broker.ManifestSecretManager {
	if !resolveAtBind {
		return new(NoopSecretManager)
	}

	return &BoshCredHubSecretManager{
		matcher:        matcher,
		secretsFetcher: secretsFetcher,
	}
}

func (r *BoshCredHubSecretManager) ResolveManifestSecrets(manifest []byte, deploymentVariables []boshdirector.Variable, logger *log.Logger) (map[string]string, error) {
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
