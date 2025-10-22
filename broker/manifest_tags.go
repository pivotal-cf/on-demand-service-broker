package broker

import (
	"gopkg.in/yaml.v2"
)

type ManifestWithTags struct {
	Tags map[string]interface{} `yaml:"tags"`
}

func getTagsFromManifest(manifest []byte) map[string]interface{} {
	var manifestWithTags ManifestWithTags
	if err := yaml.Unmarshal(manifest, &manifestWithTags); err != nil {
		return nil
	}
	return manifestWithTags.Tags
}

func toInstanceMetadataLabels(metadata map[string]interface{}) map[string]any {
	if metadata == nil {
		return nil
	}
	labels := make(map[string]any, len(metadata))
	for k, v := range metadata {
		labels[k] = v
	}
	return labels
}
