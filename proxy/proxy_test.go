package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mouad-eh/wasseet/proxy"
	"github.com/mouad-eh/wasseet/proxy/config"
	"github.com/mouad-eh/wasseet/proxy/request"
	"github.com/mouad-eh/wasseet/testutils/mocks"
	"github.com/stretchr/testify/require"
)

func TestNoRuleMatchesRequest(t *testing.T) {
	backend := &url.URL{Scheme: "http", Host: "backend.io"}

	backendGroup := &config.BackendGroup{
		Lb:      &mocks.LoadBalancerMock{NextFunc: func() *url.URL { return backend }},
		Servers: []*url.URL{backend},
	}

	config := &config.Config{
		BackendGroups: []*config.BackendGroup{backendGroup},
		Rules: []*config.Rule{
			{
				Path:         "/baz",
				BackendGroup: backendGroup,
			},
		},
	}

	beClient := NewBackendClientMock(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(backend.String()))
		},
	)
	p := proxy.NewProxy(config, beClient)

	req := httptest.NewRequest("GET", "http://proxy.io/foo", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRuleMatchesRequest(t *testing.T) {
	backend := &url.URL{Scheme: "http", Host: "backend.io"}

	loadBalancer := &mocks.LoadBalancerMock{NextFunc: func() *url.URL { return backend }}

	backendGroup := &config.BackendGroup{
		Lb:      loadBalancer,
		Servers: []*url.URL{backend},
	}

	requestOperation := &mocks.RequestOperationMock{}
	responseOperation := &mocks.ResponseOperationMock{}
	config := &config.Config{
		BackendGroups: []*config.BackendGroup{backendGroup},
		Rules: []*config.Rule{
			{
				Path:         "/foo",
				BackendGroup: backendGroup,
				RequestOperations: []config.RequestOperation{
					requestOperation,
				},
				ResponseOperations: []config.ResponseOperation{
					responseOperation,
				},
			},
		},
	}

	beClient := NewBackendClientMock(
		func(w http.ResponseWriter, r *http.Request) {
			// set specific headers to allow request-response binding
			w.Header().Set("Request-Method", r.Method)
			w.Header().Set("Request-Path", r.URL.Path)

			w.Write([]byte(backend.String()))
		},
	)

	p := proxy.NewProxy(config, beClient)

	req := httptest.NewRequest("GET", "http://proxy.io/foo", nil)
	w := httptest.NewRecorder()
	p.ServeHTTP(w, req)

	require.Equal(t, 1, len(loadBalancer.NextCalls()))

	require.Equal(t, 1, len(requestOperation.ApplyCalls()))
	require.Equal(t, "/foo", requestOperation.ApplyCalls()[0].Req.URL.Path)

	require.Equal(t, 1, len(beClient.DoCalls()))
	require.Equal(t, backend.Host, beClient.DoCalls()[0].ClientRequest.URL.Host)

	require.Equal(t, 1, len(responseOperation.ApplyCalls()))
	require.Equal(t, req.Method, responseOperation.ApplyCalls()[0].Resp.Header["Request-Method"][0])
	require.Equal(t, req.URL.Path, responseOperation.ApplyCalls()[0].Resp.Header["Request-Path"][0])

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, string(body), backend.String())
}

//TODO: After implementing backend healthchecks, add test for http client error

func NewBackendClientMock(handler http.HandlerFunc) *mocks.BackendClientMock {
	return &mocks.BackendClientMock{
		DoFunc: func(clientRequest request.ClientRequest) (*http.Response, error) {
			w := httptest.NewRecorder()

			serverReq := clientRequest.ToServerRequest()
			handlerCompatibleReq := serverReq.Request

			handler(w, handlerCompatibleReq)

			return w.Result(), nil
		},
	}
}
