package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
)

type Proxy struct {
	config   *Config
	listener net.Listener
	server   *http.Server
	client   BackendClient
}

func NewProxy(config *Config, bc BackendClient) *Proxy {
	return &Proxy{
		server: &http.Server{},
		client: bc,
		config: config,
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
		return fmt.Errorf("failed to start http server: %w", err)
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
	serverReq := ServerRequest{r}

	rule, err := p.config.GetFirstMatchingRule(serverReq)
	if err != nil {
		fmt.Println("Error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rule.ApplyRequestOperations(serverReq)

	targetBackend := rule.BackendGroup.Lb.Next()

	clientReq := serverReq.ToClientRequest(targetBackend)
	resp, err := p.client.Do(clientReq)
	if err != nil {
		fmt.Println("Error:", err)
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
