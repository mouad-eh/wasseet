package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/mouad-eh/wasseet/api/config"
	"github.com/mouad-eh/wasseet/request"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Proxy struct {
	configManager *ConfigManager
	healthChecker *HealthChecker
	listener      net.Listener
	server        *http.Server
	client        BackendClient
	logger        *zap.SugaredLogger
	shutdownCh    chan struct{}
}

func NewProxy(config config.Source, bc BackendClient) *Proxy {
	loggerConfig := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.DebugLevel),
		Development: true,
		Encoding:    "json",
		EncoderConfig: zapcore.EncoderConfig{
			// TimeKey:      "timestamp",
			LevelKey: "level",
			// CallerKey:    "caller",
			MessageKey:  "msg",
			EncodeLevel: zapcore.LowercaseLevelEncoder,
			// EncodeTime:   zapcore.RFC3339TimeEncoder,
			// EncodeCaller: zapcore.ShortCallerEncoder,
		},
		OutputPaths: []string{"stdout"},
	}

	logger, _ := loggerConfig.Build()
	sugaredLogger := logger.Sugar()
	// healthChecker := NewHealthChecker(config.BackendGroups, bc, sugaredLogger)
	configManager, err := NewConfigManager(config, sugaredLogger)
	if err != nil {
		sugaredLogger.Fatalf("failed to create config manager: %v", err)
	}
	return &Proxy{
		server:        &http.Server{},
		client:        bc,
		configManager: configManager,
		// healthChecker: healthChecker,
		logger:     sugaredLogger,
		shutdownCh: make(chan struct{}),
	}
}

func (p *Proxy) Start() error {
	p.configManager.Start(p.shutdownCh)
	// p.healthChecker.Start(p.shutdownCh)
	// start http server
	defaultServerMux := &http.ServeMux{}
	defaultServerMux.Handle("/", p)
	p.server.Handler = defaultServerMux

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", p.configManager.GetLatestConfig().Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	p.listener = listener

	if err := p.server.Serve(listener); err != nil {
		return err
	}
	return nil
}

func (p *Proxy) GetAddr() string {
	if p.listener != nil {
		return p.listener.Addr().String()
	}
	return ""
}

func (p *Proxy) Stop() error {
	close(p.shutdownCh)
	return p.server.Shutdown(context.Background())
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverReq := request.ServerRequest{r}

	latestConfig := p.configManager.GetLatestConfig()

	rule, err := latestConfig.GetFirstMatchingRule(serverReq)
	if err != nil {
		p.logger.Errorw(err.Error(), "request_type", "server",
			"request_method", r.Method, "request_path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	rule.ApplyRequestOperations(serverReq)

	targetBackend := rule.BackendGroup.Lb.Next()
	// here we assume that at least one backend is healthy
	// TODO: handle case when all backends are unhealthy
	// for !p.healthChecker.getHealthStatus(rule.BackendGroup.Name, targetBackend.String()) {
	// 	targetBackend = rule.BackendGroup.Lb.Next()
	// }

	clientReq := serverReq.ToClientRequest(targetBackend)
	resp, err := p.client.Do(clientReq)
	if err != nil {
		p.logger.Errorw(err.Error(), "request_type", "client",
			"request_method", r.Method, "request_url", r.URL.String())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	rule.ApplyResponseOperations(resp)

	for header, values := range resp.Header {
		w.Header()[header] = values
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
