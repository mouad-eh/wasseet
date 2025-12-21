package proxy

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/mouad-eh/wasseet/api/config"
	"github.com/mouad-eh/wasseet/request"
	"go.uber.org/zap"
)

type HealthChecker struct {
	logger        *zap.SugaredLogger
	client        BackendClient
	backendGroups []*config.BackendGroup
	mu            sync.RWMutex               // protect health map
	health        map[string]map[string]bool // backendGroup -> backend -> healthy
	retries       map[string]map[string]int  // backendGroup -> backend -> retries
}

func NewHealthChecker(backendGroups []*config.BackendGroup, client BackendClient, logger *zap.SugaredLogger) *HealthChecker {
	hc := &HealthChecker{
		logger:        logger,
		client:        client,
		backendGroups: backendGroups,
		health:        make(map[string]map[string]bool),
		retries:       make(map[string]map[string]int),
	}
	for _, bg := range backendGroups {
		hc.health[bg.Name] = make(map[string]bool)
		hc.retries[bg.Name] = make(map[string]int)
	}
	for _, bg := range backendGroups {
		for _, backend := range bg.Servers {
			hc.health[bg.Name][backend.String()] = true
			hc.retries[bg.Name][backend.String()] = 0
		}
	}
	return hc
}

func (hc *HealthChecker) Start(shutdownCh chan struct{}) {
	for _, bg := range hc.backendGroups {
		if bg.HealthCheck == nil {
			continue
		}
		for _, backend := range bg.Servers {
			go hc.checkHealth(bg.Name, backend.String(), bg.HealthCheck, shutdownCh)
		}
	}
}

func (hc *HealthChecker) checkHealth(backendGroup, backend string, params *config.HealthCheck, shutdownCh chan struct{}) {
	ticker := time.NewTicker(params.Interval)
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), params.Timeout)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, "GET", backend+params.Path, nil)
			if err != nil {
				hc.logger.Errorw("Failed to create health check request", "backend", backend, "err", err)
				continue
			}

			resp, err := hc.client.Do(request.ClientRequest{Request: req})
			if err != nil || resp.StatusCode != http.StatusOK {
				hc.retries[backendGroup][backend]++
				if hc.retries[backendGroup][backend] >= params.Retries && hc.getHealthStatus(backendGroup, backend) {
					hc.logger.Warnw("Backend is unhealthy", "backend_group", backendGroup, "backend", backend)
					hc.setHealthStatus(backendGroup, backend, false)
				}
			} else {
				if !hc.getHealthStatus(backendGroup, backend) {
					hc.logger.Infow("Backend is healthy", "backend_group", backendGroup, "backend", backend)
					hc.retries[backendGroup][backend] = 0
					hc.setHealthStatus(backendGroup, backend, true)
				}
			}
		case <-shutdownCh:
			return
		}
	}
}

func (hc *HealthChecker) setHealthStatus(backendGroup, backend string, status bool) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.health[backendGroup][backend] = status
}

func (hc *HealthChecker) getHealthStatus(backendGroup, backend string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.health[backendGroup][backend]
}
