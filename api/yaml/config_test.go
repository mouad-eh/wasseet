package yaml

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidate_ValidConfig(t *testing.T) {
	data, err := os.ReadFile("testdata/valid_config.yaml")
	if err != nil {
		t.Fatalf("failed to read valid_config.yaml: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to unmarshal valid config: %v", err)
	}

	err = config.Validate()
	if err != nil {
		t.Errorf("expected valid config to pass validation, but got error: %v", err)
	}
}

func TestValidate_InvalidConfig(t *testing.T) {
	data, err := os.ReadFile("testdata/invalid_config.yaml")
	if err != nil {
		t.Fatalf("failed to read invalid_config.yaml: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to unmarshal invalid config: %v", err)
	}

	err = config.Validate()
	if err == nil {
		t.Errorf("expected invalid config to fail validation, but got no error")
	}
}
