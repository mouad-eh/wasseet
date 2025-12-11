package yaml_test

import (
	"net/url"
	"reflect"
	"testing"

	yamlapi "github.com/mouad-eh/wasseet/api/yaml"
	"github.com/mouad-eh/wasseet/loadbalancer"
	"github.com/mouad-eh/wasseet/proxy/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestValidate_ValidConfig(t *testing.T) {
	yamlContent := `
port: 8080
backend_groups:
  - name: backend1
    load_balancing: round_robin
    servers:
      - localhost:9000
      - localhost:9001
rules:
  - path: /api
    backend_group: backend1
    request_operations:
      - type: add_header
        header: X-Forwarded-For
        value: 127.0.0.1
    response_operations:
      - type: add_header
        header: X-Response-From
        value: proxy
`

	var config yamlapi.Config
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	require.NoError(t, err)

	err = config.Validate()
	require.NoError(t, err)
}

func TestValidate_InvalidConfig(t *testing.T) {
	yamlContent := `
port: 8080
backend_groups:
  - name: backend1
    servers:
      - invalid-server
rules:
  - path: /api
    backend_group: nonexistent_group
`

	var config yamlapi.Config
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	require.NoError(t, err)

	err = config.Validate()
	require.Error(t, err)
}

func TestResolve(t *testing.T) {
	yamlContent := `
port: 8080
backend_groups:
  - name: backend1
    load_balancing: round_robin
    servers:
      - localhost:9000
      - localhost:9001
rules:
  - path: /
    backend_group: backend1
    request_operations:
      - type: add_header
        header: X-Forwarded-For
        value: 127.0.0.1
    response_operations:
      - type: add_header
        header: X-Response-From
        value: proxy
`

	var yamlconfig yamlapi.Config
	err := yaml.Unmarshal([]byte(yamlContent), &yamlconfig)
	require.NoError(t, err)

	// Resolve the config
	resolved := yamlconfig.Resolve()

	// Build the expected proxy.Config manually
	servers := []*url.URL{
		{Scheme: "http", Host: "localhost:9000"},
		{Scheme: "http", Host: "localhost:9001"},
	}

	backendGroup := &config.BackendGroup{
		Name:    "backend1",
		Lb:      loadbalancer.NewRoundRobin(servers),
		Servers: servers,
	}

	requestOps := []config.RequestOperation{
		&config.AddHeaderRequestOperation{
			Header: "X-Forwarded-For",
			Value:  "127.0.0.1",
		},
	}

	responseOps := []config.ResponseOperation{
		&config.AddHeaderResponseOperation{
			Header: "X-Response-From",
			Value:  "proxy",
		},
	}

	rule := &config.Rule{
		Host:               "",
		Path:               "",
		BackendGroup:       backendGroup,
		RequestOperations:  requestOps,
		ResponseOperations: responseOps,
	}

	expected := config.Config{
		Port:          8080,
		BackendGroups: []*config.BackendGroup{backendGroup},
		Rules:         []*config.Rule{rule},
	}

	require.True(t, reflect.DeepEqual(resolved, expected))
}
