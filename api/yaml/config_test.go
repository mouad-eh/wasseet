package yaml

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/mouad-eh/wasseet/loadbalancer"
	"github.com/mouad-eh/wasseet/proxy"
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

	var config Config
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

	var config Config
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

	var config Config
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	require.NoError(t, err)

	// Resolve the config
	resolved := config.Resolve()

	// Build the expected proxy.Config manually
	servers := []*url.URL{
		{Scheme: "http", Host: "localhost:9000"},
		{Scheme: "http", Host: "localhost:9001"},
	}

	backendGroup := &proxy.BackendGroup{
		Name:    "backend1",
		Lb:      loadbalancer.NewRoundRobin(servers),
		Servers: servers,
	}

	requestOps := []proxy.RequestOperation{
		&proxy.AddHeaderRequestOperation{
			Header: "X-Forwarded-For",
			Value:  "127.0.0.1",
		},
	}

	responseOps := []proxy.ResponseOperation{
		&proxy.AddHeaderResponseOperation{
			Header: "X-Response-From",
			Value:  "proxy",
		},
	}

	rule := &proxy.Rule{
		Host:               "",
		Path:               "",
		BackendGroup:       backendGroup,
		RequestOperations:  requestOps,
		ResponseOperations: responseOps,
	}

	expected := proxy.Config{
		Port:          8080,
		BackendGroups: []*proxy.BackendGroup{backendGroup},
		Rules:         []*proxy.Rule{rule},
	}

	require.True(t, reflect.DeepEqual(resolved, expected))
}
