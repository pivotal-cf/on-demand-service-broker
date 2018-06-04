package manifestsecrets

import (
	"errors"
	"regexp"

	yaml "gopkg.in/yaml.v2"
)

type CredHubPathMatcher struct{}

func (m *CredHubPathMatcher) Match(manifest []byte) ([][]byte, error) {
	refs := regexp.MustCompile(`\(\((.*?)\)\)`)
	matches := refs.FindAllSubmatch(manifest, -1)

	varsBlockNames, err := m.NamesFromVarsBlock(manifest)
	if err != nil {
		return nil, err
	}

	ret := [][]byte{}
	for _, match := range matches {
		name := match[1]
		if !varsBlockNames[string(name)] {
			ret = append(ret, match[1])
		}
	}

	return ret, nil
}

func (m *CredHubPathMatcher) NamesFromVarsBlock(manifest []byte) (map[string]bool, error) {
	var manifestObj struct {
		Variables []struct {
			Name string `yaml:"name"`
		} `yaml:"variables"`
	}

	err := yaml.Unmarshal(manifest, &manifestObj)
	if err != nil {
		return nil, err
	}

	ret := map[string]bool{}
	for _, variable := range manifestObj.Variables {
		if len(variable.Name) == 0 {
			return nil, errors.New("variable without name in variables block")
		}
		ret[variable.Name] = true
	}

	return ret, nil
}
