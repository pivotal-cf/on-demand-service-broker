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

func (r *NoopSecretManager) DeleteSecretsForInstance(instanceID string, logger *log.Logger) error {
	panic("not implemented")
	return nil
}

type BoshCredHubSecretManager struct {
	matcher  Matcher
	operator CredhubOperator
}

func BuildManager(resolveAtBind bool, matcher Matcher, secretsFetcher CredhubOperator) broker.ManifestSecretManager {
	if !resolveAtBind {
		return new(NoopSecretManager)
	}

	return &BoshCredHubSecretManager{
		matcher:  matcher,
		operator: secretsFetcher,
	}
}

func (r *BoshCredHubSecretManager) ResolveManifestSecrets(manifest []byte, deploymentVariables []boshdirector.Variable, logger *log.Logger) (map[string]string, error) {
	matches, err := r.matcher.Match(manifest, deploymentVariables)
	if err != nil {
		return nil, err
	}

	secrets, err := r.operator.BulkGet(matches, logger)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}

func (r *BoshCredHubSecretManager) DeleteSecretsForInstance(instanceID string, logger *log.Logger) error {
	paths, err := r.operator.FindNameLike(instanceID, logger)
	if err != nil {
		return err
	}

	return r.operator.BulkDelete(paths, logger)
}
