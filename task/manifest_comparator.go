package task

import (
	"fmt"
	"reflect"

	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

func ManifestsAreTheSame(generateManifest, oldManifest []byte) (bool, error) {
	regeneratedManifest, err := marshalBoshManifest(generateManifest)
	if err != nil {
		return false, err
	}
	ignoreUpdateBlock(&regeneratedManifest)

	boshManifest, err := marshalBoshManifest(oldManifest)
	if err != nil {
		return false, err
	}
	ignoreUpdateBlock(&boshManifest)

	manifestsSame := reflect.DeepEqual(regeneratedManifest, boshManifest)

	return manifestsSame, nil
}

func marshalBoshManifest(rawManifest []byte) (bosh.BoshManifest, error) {
	var boshManifest bosh.BoshManifest
	err := yaml.Unmarshal(rawManifest, &boshManifest)
	if err != nil {
		return bosh.BoshManifest{}, fmt.Errorf("error detecting change in manifest, unable to unmarshal manifest: %s", err)
	}
	return boshManifest, nil
}

func ignoreUpdateBlock(manifest *bosh.BoshManifest) {
	manifest.Update = nil
	for i := range manifest.InstanceGroups {
		manifest.InstanceGroups[i].Update = nil
	}
}
