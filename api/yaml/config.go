package yaml

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mouad-eh/wasseet/loadbalancer"
	"github.com/mouad-eh/wasseet/proxy/config"
)

type Config struct {
	Port          int            `yaml:"port"`
	BackendGroups []BackendGroup `yaml:"backend_groups"`
	Rules         []Rule         `yaml:"rules"`
}

type BackendGroup struct {
	Name          string            `yaml:"name"`
	LoadBalancing LoadBalancingType `yaml:"load_balancing"` // Optional
	Servers       []string          `yaml:"servers"`
	HealthCheck   *HealthCheck      `yaml:"health_check"` // Optional
}

type HealthCheck struct {
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
	Retries  int    `yaml:"retries"`
}

type LoadBalancingType string

const (
	RoundRobin               LoadBalancingType = "round_robin"
	DefaultLoadBalancingType LoadBalancingType = RoundRobin
)

var validLoadBalancingTypes = map[LoadBalancingType]bool{
	RoundRobin: true,
}

type Rule struct {
	Host               string                     `yaml:"host"` // Optional if Path is specified
	Path               string                     `yaml:"path"` // Optional if Host is specified
	BackendGroup       string                     `yaml:"backend_group"`
	RequestOperations  []RequestOperationWrapper  `yaml:"request_operations"`  // Optional
	ResponseOperations []ResponseOperationWrapper `yaml:"response_operations"` // Optional
}

func (c *Config) Resolve() config.Config {
	// Build a map of backend groups by name for easy lookup
	proxyBGMap := make(map[string]*config.BackendGroup)

	for _, bg := range c.BackendGroups {
		// Parse server URLs
		servers := make([]*url.URL, len(bg.Servers))
		for i, server := range bg.Servers {
			serverURL := server
			if !strings.HasPrefix(server, "http://") {
				serverURL = "http://" + server
			}
			u, _ := url.Parse(serverURL)
			servers[i] = u
		}

		// Create the appropriate load balancer based on type
		var lb loadbalancer.LoadBalancer
		switch bg.LoadBalancing {
		case RoundRobin:
			lb = loadbalancer.NewRoundRobin(servers)
		}

		// Resolve health check
		var healthCheck *config.HealthCheck
		if bg.HealthCheck != nil {
			// we are sure that ParseDuration will not fail because
			// we already checked that during validation.
			interval, _ := time.ParseDuration(bg.HealthCheck.Interval)
			timeout, _ := time.ParseDuration(bg.HealthCheck.Timeout)

			healthCheck = &config.HealthCheck{
				Path:     bg.HealthCheck.Path,
				Interval: interval,
				Timeout:  timeout,
				Retries:  bg.HealthCheck.Retries,
			}
		}

		proxyBG := &config.BackendGroup{
			Name:        bg.Name,
			Lb:          lb,
			Servers:     servers,
			HealthCheck: healthCheck,
		}
		proxyBGMap[bg.Name] = proxyBG
	}

	// Convert rules
	proxyRules := make([]*config.Rule, len(c.Rules))
	for i, rule := range c.Rules {
		// Convert request operations
		requestOps := make([]config.RequestOperation, len(rule.RequestOperations))
		for j, op := range rule.RequestOperations {
			requestOps[j] = op.Operation.Resolve()
		}

		// Convert response operations
		responseOps := make([]config.ResponseOperation, len(rule.ResponseOperations))
		for j, op := range rule.ResponseOperations {
			responseOps[j] = op.Operation.Resolve()
		}

		path := rule.Path
		if path == "/" {
			path = ""
		}
		proxyRules[i] = &config.Rule{
			Host:               rule.Host,
			Path:               path,
			BackendGroup:       proxyBGMap[rule.BackendGroup],
			RequestOperations:  requestOps,
			ResponseOperations: responseOps,
		}
	}

	// Build resolved backend groups slice in the same order as input
	proxyBGs := make([]*config.BackendGroup, len(c.BackendGroups))
	for i, bg := range c.BackendGroups {
		proxyBGs[i] = proxyBGMap[bg.Name]
	}

	return config.Config{
		Port:          c.Port,
		BackendGroups: proxyBGs,
		Rules:         proxyRules,
	}
}

func (c *Config) Validate() error {
	// We consider port 0 as a valid port as it will let the OS choose
	// any available port which can be useful for testing.
	// TODO: Port 0 on prod is not allowed. We should consider adding
	// environment flag (test/prod).
	if !isValidPort(c.Port, true) {
		return fmt.Errorf("port must be between 0 and 65535")
	}
	if len(c.BackendGroups) == 0 {
		return fmt.Errorf("at least one backend group must be defined")
	}
	if len(c.Rules) == 0 {
		return fmt.Errorf("at least one rule must be defined")
	}

	// Validate each backend group
	for i, bg := range c.BackendGroups {
		if err := bg.Validate(); err != nil {
			return fmt.Errorf("backend group %d: %w", i, err)
		}
	}

	// Validate each rule
	for i, rule := range c.Rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("rule %d: %w", i, err)
		}

		// Check if referenced backend group exists
		found := false
		for _, bg := range c.BackendGroups {
			if bg.Name == rule.BackendGroup {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("rule %d: backend group %q not found", i, rule.BackendGroup)
		}
	}

	return nil
}

func (bg BackendGroup) Validate() error {
	if bg.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(bg.Servers) == 0 {
		return fmt.Errorf("at least one server must be defined")
	}
	// Validate servers
	for j, server := range bg.Servers {
		serverToValidate := strings.TrimPrefix(server, "http://")
		if !isValidDNSOrIPWithPort(serverToValidate) {
			return fmt.Errorf("server %d %q must be in format [hostname|IP:port]", j, server)
		}
	}
	// Validate load balancing type
	if !isValidLoadBalancingType(bg.LoadBalancing) {
		return fmt.Errorf("invalid load balancing type %q", bg.LoadBalancing)
	}

	if bg.HealthCheck != nil {
		if err := bg.HealthCheck.Validate(); err != nil {
			return fmt.Errorf("health check: %w", err)
		}
	}

	return nil
}

func (hc HealthCheck) Validate() error {
	if !strings.HasPrefix(hc.Path, "/") {
		return fmt.Errorf("path %q must start with /", hc.Path)
	}

	interval, err := time.ParseDuration(hc.Interval)
	if err != nil {
		return fmt.Errorf("invalid interval %q: %w", hc.Interval, err)
	}
	if interval <= 0 {
		return fmt.Errorf("invalid interval %q: must be greater than 0", hc.Interval)
	}
	timeout, err := time.ParseDuration(hc.Timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %w", hc.Timeout, err)
	}
	if timeout <= 0 {
		return fmt.Errorf("invalid timeout %q: must be greater than 0", hc.Timeout)
	}
	if timeout >= interval {
		return fmt.Errorf("invalid timeout %q: must be less than interval %q", hc.Timeout, hc.Interval)
	}

	return nil
}

func (rule *Rule) Validate() error {
	if rule.Host == "" && rule.Path == "" {
		return fmt.Errorf("either host or path must be specified")
	}

	if rule.Host != "" && !isValidDNSOrIPWithPort(rule.Host) {
		return fmt.Errorf("host %q must be in format [hostname|IP:port]", rule.Host)
	}

	if rule.Path != "" && !strings.HasPrefix(rule.Path, "/") {
		return fmt.Errorf("path must start with /")
	}

	if rule.BackendGroup == "" {
		return fmt.Errorf("backend_group is required")
	}

	for i, op := range rule.RequestOperations {
		if err := op.Operation.Validate(); err != nil {
			return fmt.Errorf("request operation %d: %w", i, err)
		}
	}

	for i, op := range rule.ResponseOperations {
		if err := op.Operation.Validate(); err != nil {
			return fmt.Errorf("response operation %d: %w", i, err)
		}
	}

	return nil
}
