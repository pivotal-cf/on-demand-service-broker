package manifestsecrets

import (
	"fmt"
	"strings"

	"github.com/pivotal-cf/on-demand-service-broker/broker"
	"github.com/pivotal-cf/on-demand-services-sdk/serviceadapter"
)

type ODBSecrets struct {
	ServiceOfferingID string
}

func (o ODBSecrets) GenerateSecretPaths(deploymentName, manifest string, secretsMap serviceadapter.ODBManagedSecrets) []broker.ManifestSecret {
	secrets := []broker.ManifestSecret{}
	for name, val := range secretsMap {
		if strings.Contains(manifest, fmt.Sprintf("((%s:%s))", serviceadapter.ODBSecretPrefix, name)) {
			secrets = append(secrets, broker.ManifestSecret{
				Name:  name,
				Value: val,
				Path:  fmt.Sprintf("/odb/%s/%s/%s", o.ServiceOfferingID, deploymentName, name),
			})
		}
	}
	return secrets
}

func (o ODBSecrets) ReplaceODBRefs(manifest string, secrets []broker.ManifestSecret) string {
	newManifest := manifest

	for _, s := range secrets {
		newManifest = strings.Replace(
			newManifest,
			fmt.Sprintf("((%s:%s))", serviceadapter.ODBSecretPrefix, s.Name),
			fmt.Sprintf("((%s))", s.Path),
			-1,
		)
	}

	return newManifest
}
