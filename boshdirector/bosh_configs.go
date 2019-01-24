package boshdirector

import (
	"fmt"
	"log"

	"github.com/cloudfoundry/bosh-cli/director"
	"github.com/pkg/errors"
)

const (
	boshConfigsLimit = 30
)

type BoshConfig struct {
	Type    string
	Name    string
	Content string
}

func (c *Client) GetConfigs(configName string, logger *log.Logger) ([]BoshConfig, error) {
	var configs []BoshConfig

	logger.Printf("getting configs for %s\n", configName)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return configs, errors.Wrap(err, "Failed to build director")
	}

	boshConfigs, err := d.ListConfigs(boshConfigsLimit, director.ConfigsFilter{Name: configName})
	if err != nil {
		return configs, errors.Wrap(err, fmt.Sprintf(`BOSH error getting configs for "%s"`, configName))
	}

	for _, config := range boshConfigs {
		configs = append(configs, BoshConfig{Type: config.Type, Name: config.Name, Content: config.Content})
	}
	return configs, nil
}

func (c *Client) UpdateConfig(configType, configName string, configContent []byte, logger *log.Logger) error {
	logger.Printf("updating %s config %s\n", configType, configName)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return errors.Wrap(err, "Failed to build director")
	}
	if _, err := d.UpdateConfig(configType, configName, configContent); err != nil {
		return errors.Wrap(err, fmt.Sprintf(`BOSH error updating "%s" config "%s"`, configType, configName))
	}

	return nil
}

func (c *Client) DeleteConfig(configType, configName string, logger *log.Logger) (bool, error) {
	logger.Printf("deleting %s config %s\n", configType, configName)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return false, errors.Wrap(err, "Failed to build director")
	}
	found, err := d.DeleteConfig(configType, configName)
	if err != nil {
		return false, errors.Wrap(err, fmt.Sprintf(`BOSH error deleting "%s" config "%s"`, configType, configName))
	}

	return found, nil
}

func (c *Client) DeleteConfigs(configName string, logger *log.Logger) error {
	logger.Printf("deleting configs for %s\n", configName)
	d, err := c.Director(director.NewNoopTaskReporter())
	if err != nil {
		return errors.Wrap(err, "Failed to build director")
	}

	configs, err := d.ListConfigs(boshConfigsLimit, director.ConfigsFilter{Name: configName})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf(`BOSH error getting configs for "%s"`, configName))
	}

	for _, config := range configs {
		if _, err := d.DeleteConfig(config.Type, config.Name); err != nil {
			return errors.Wrap(err, fmt.Sprintf(`BOSH error deleting "%s" config "%s"`, config.Type, config.Name))
		}
	}

	return nil
}
