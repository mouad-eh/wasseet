package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
)

type ClientRequest struct {
	*http.Request
}

type ServerRequest struct {
	*http.Request
}

// used for converting the request received by the http server of the proxy
// to a request sent to the backend using the http client
func (r ServerRequest) ToClientRequest(backend *url.URL) ClientRequest {
	r.Request.RequestURI = ""
	r.Request.URL.Scheme = backend.Scheme
	r.Request.URL.Host = backend.Host
	r.Request.URL.Path = backend.Path + r.URL.Path
	r.Request.Host = backend.Host
	return ClientRequest(r)
}

// used for testing purposes by mock backend clients to convert the client request to a server request
func (r ClientRequest) ToServerRequest() ServerRequest {
	handlerCompatibleReq := httptest.NewRequest(r.Method, r.URL.String(), r.Body)
	handlerCompatibleReq.Header = r.Header.Clone()

	return ServerRequest{handlerCompatibleReq}
}
