package proxy

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sync"

	yamlapi "github.com/mouad-eh/wasseet/api/yaml"
	"github.com/mouad-eh/wasseet/proxy/config"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type ConfigManager struct {
	mu             sync.RWMutex // to protect the latestVersion and Configs map
	latestVersion  int
	configFilePath string
	configs        map[int]*config.Config // map of config versions
	logger         *zap.SugaredLogger
}

func NewConfigManager(cfg *config.Config, configFilePath string, logger *zap.SugaredLogger) *ConfigManager {
	cm := &ConfigManager{
		latestVersion:  0,
		configs:        make(map[int]*config.Config),
		configFilePath: configFilePath,
		logger:         logger,
	}
	cm.configs[0] = cfg
	return cm
}

// Start starts a new goroutine that listens for SIGHUP signals and reloads the config
func (cm *ConfigManager) Start(shutdownCh chan struct{}) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP)
	go func() {
		for {
			select {
			case <-shutdownCh:
				return
			case <-sigChan:
				err := cm.LoadConfig()
				if err != nil {
					cm.logger.Error("Failed to load config:", err)
				}
				cm.logger.Info("Config reloaded")

			}
		}
	}()
}

func (cm *ConfigManager) LoadConfig() error {
	configBytes, err := os.ReadFile(cm.configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlConfig yamlapi.Config
	if err := yaml.Unmarshal(configBytes, &yamlConfig); err != nil {
		return fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	err = yamlConfig.Validate()
	if err != nil {
		return fmt.Errorf("failed to validate config file: %w", err)
	}

	config := yamlConfig.Resolve()

	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.latestVersion++
	cm.configs[cm.latestVersion] = &config

	return nil
}

func (cm *ConfigManager) GetLatestConfig() *config.Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configs[cm.latestVersion]
}
