package proxy

import (
	"fmt"
	"io"
	"net/http"
)

type Proxy struct {
	server *http.Server
	client BackendClient
	config *Config
}

func NewProxy(config *Config, bc BackendClient) *Proxy {
	return &Proxy{
		server: &http.Server{Addr: fmt.Sprintf(":%d", config.Port)},
		client: bc,
		config: config,
	}
}

func (p *Proxy) Start() error {
	defaultServerMux := &http.ServeMux{}
	defaultServerMux.Handle("/", p)
	p.server.Handler = defaultServerMux
	if err := p.server.ListenAndServe(); err != nil {
		return fmt.Errorf("failed to start http server: %w", err)
	}
	return nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	serverReq := ServerRequest{r}
	backendGroup, err := p.getBackendGroup(serverReq)
	if err != nil {
		fmt.Println("Error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	targetBackend := backendGroup.Lb.Next()

	clientReq := serverReq.ToClientRequest(targetBackend)
	resp, err := p.client.Do(clientReq)
	if err != nil {
		fmt.Println("Error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func (p *Proxy) getBackendGroup(r ServerRequest) (*BackendGroup, error) {
	// for now, the proxy handles only one backend group
	return p.config.BackendGroups[0], nil
}
