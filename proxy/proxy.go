package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type Proxy struct {
	server  *http.Server
	client  *http.Client
	backend *url.URL
}

func NewProxy() *Proxy {
	backend, err := url.Parse("http://localhost:8081")
	if err != nil {
		panic(err)
	}
	return &Proxy{
		server:  &http.Server{Addr: ":8080"},
		client:  &http.Client{},
		backend: backend,
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
	r.RequestURI = ""
	r.URL.Scheme = p.backend.Scheme
	r.URL.Host = p.backend.Host
	r.URL.Path = p.backend.Path + r.URL.Path
	r.Host = ""

	resp, err := p.client.Do(r)
	if err != nil {
		fmt.Println("Error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
