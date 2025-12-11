package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

	yamlapi "github.com/mouad-eh/wasseet/api/yaml"
	"github.com/mouad-eh/wasseet/proxy/config"
	"github.com/mouad-eh/wasseet/proxy/request"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type Proxy struct {
	config   *config.Config
	listener net.Listener
	server   *http.Server
	client   BackendClient
	logger   *zap.SugaredLogger
}

func NewProxyFromConfigFile(configFilePath string, bc BackendClient) (*Proxy, error) {
	configBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var yamlConfig yamlapi.Config
	if err := yaml.Unmarshal(configBytes, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	err = yamlConfig.Validate()
	if err != nil {
		return nil, fmt.Errorf("failed to validate config file: %w", err)
	}

	config := yamlConfig.Resolve()

	return NewProxy(&config, bc), nil
}

func NewProxy(config *config.Config, bc BackendClient) *Proxy {
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
	return &Proxy{
		server: &http.Server{},
		client: bc,
		config: config,
		logger: logger.Sugar(),
	}
}

func (p *Proxy) Start() error {
	defaultServerMux := &http.ServeMux{}
	defaultServerMux.Handle("/", p)
	p.server.Handler = defaultServerMux

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", p.config.Port))
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
	return p.server.Shutdown(context.Background())
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverReq := request.ServerRequest{r}

	rule, err := p.config.GetFirstMatchingRule(serverReq)
	if err != nil {
		p.logger.Errorw(err.Error(), "request_type", "server",
			"request_method", r.Method, "request_path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	rule.ApplyRequestOperations(serverReq)

	targetBackend := rule.BackendGroup.Lb.Next()

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
