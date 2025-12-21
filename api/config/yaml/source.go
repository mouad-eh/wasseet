package yaml

import (
	"fmt"
	"os"

	"github.com/mouad-eh/wasseet/api/config"
	"gopkg.in/yaml.v3"
)

type Source struct {
	Path string
}

func (s *Source) Load() (config.Config, error) {
	configBytes, err := os.ReadFile(s.Path)
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlconfig Config
	if err := yaml.Unmarshal(configBytes, &yamlconfig); err != nil {
		return config.Config{}, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	err = yamlconfig.Validate()
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to validate config file: %w", err)
	}

	return yamlconfig.Resolve(), nil
}
