package env_helpers

import (
	"fmt"
	"os"
)

func ValidateEnvVars(vars ...string) error {
	var unsetVariables string
	for _, variableName := range vars {
		e := os.Getenv(variableName)
		if e == "" {
			unsetVariables += " " + variableName
		}
	}

	if unsetVariables != "" {
		return fmt.Errorf("the following required variables weren't set: %s", unsetVariables)
	}

	return nil
}
