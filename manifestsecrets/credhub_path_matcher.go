package manifestsecrets

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pivotal-cf/on-demand-service-broker/boshdirector"
)

type CredHubPathMatcher struct{}

func (m *CredHubPathMatcher) Match(manifest []byte, deploymentVariables []boshdirector.Variable) (map[string]boshdirector.Variable, error) {
	refs := regexp.MustCompile(`(\(\(.*?\)\))`)
	matches := refs.FindAllSubmatch(manifest, -1)

	ret := map[string]boshdirector.Variable{}

	for _, match := range matches {
		key := string(match[1])
		name := strings.Trim(key, "()")
		if strings.HasPrefix(name, "/") {
			ret[key] = findAbsolutePath(name, deploymentVariables)
		} else {
			ret[key] = findRelativePath(name, deploymentVariables)
		}
	}

	return ret, nil
}

func findAbsolutePath(name string, deploymentVariables []boshdirector.Variable) boshdirector.Variable {
	for _, v := range deploymentVariables {
		if v.Path == name {
			return v
		}
	}
	return boshdirector.Variable{Path: name}
}

func findRelativePath(name string, deploymentVariables []boshdirector.Variable) boshdirector.Variable {
	for _, v := range deploymentVariables {
		namePieces := strings.Split(name, ".")
		if strings.HasSuffix(v.Path, fmt.Sprintf("/%s", namePieces[0])) {
			return v
		}
	}
	return boshdirector.Variable{Path: name}
}
