package proxy

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sync"

	"github.com/mouad-eh/wasseet/api/config"
	"go.uber.org/zap"
)

type ConfigManager struct {
	mu            sync.RWMutex // to protect the latestVersion and Configs map
	latestVersion int
	configSrc     config.Source
	configs       map[int]*config.Config // map of config versions
	logger        *zap.SugaredLogger
}

func NewConfigManager(src config.Source, logger *zap.SugaredLogger) (*ConfigManager, error) {
	cm := &ConfigManager{
		latestVersion: 0,
		configs:       make(map[int]*config.Config),
		configSrc:     src,
		logger:        logger,
	}
	cfg, err := src.Load()
	if err != nil {
		logger.Error("Failed to load config:", err)
		return nil, err
	}
	cm.configs[0] = &cfg
	return cm, nil
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
				err := cm.reloadConfig()
				if err != nil {
					cm.logger.Error("Failed to load config:", err)
				}
				cm.logger.Info("Config reloaded")

			}
		}
	}()
}

func (cm *ConfigManager) reloadConfig() error {
	cfg, err := cm.configSrc.Load()
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.latestVersion++
	cm.configs[cm.latestVersion] = &cfg

	return nil
}

func (cm *ConfigManager) GetLatestConfig() *config.Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configs[cm.latestVersion]
}
