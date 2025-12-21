package integration_tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mouad-eh/wasseet/api/config"
	"github.com/mouad-eh/wasseet/loadbalancer"
	"github.com/mouad-eh/wasseet/proxy"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinLoadBalancing(t *testing.T) {
	// start multiple backend servers
	numBackends := 3
	backendServers := make([]*httptest.Server, numBackends)
	backendURLs := make([]*url.URL, numBackends)
	for i := 0; i < numBackends; i++ {
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("backend%d", i)))
		}))
		backendServers[i] = backendServer
		backendURLs[i], _ = url.Parse(backendServer.URL)
		defer backendServer.Close()
	}

	// create a proxy config including the started backend servers
	backendGroup := &config.BackendGroup{
		Lb:      loadbalancer.NewRoundRobin(backendURLs),
		Servers: backendURLs,
	}

	proxyConfig := &config.Config{
		Port: 0, // let OS assign an available port
		BackendGroups: []*config.BackendGroup{
			backendGroup,
		},
		Rules: []*config.Rule{
			{
				Path:         "",
				BackendGroup: backendGroup,
			},
		},
	}

	// start the proxy
	proxyServer := proxy.NewProxy(proxyConfig, &proxy.HttpClient{Client: &http.Client{}})
	proxyURL := startProxyAndGetURL(t, proxyServer)
	defer proxyServer.Stop()

	// send requests to proxy and check if they are load balanced
	numRequests := 8
	for i := 0; i < numRequests; i++ {
		resp, err := http.Get(proxyURL + "/")
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, fmt.Sprintf("backend%d", i%numBackends), string(body))
	}
}

func TestRoutingToMultipleBackendGroups(t *testing.T) {
	// start multiple backend servers
	numBackendGroups := 2
	backendServers := make([]*httptest.Server, numBackendGroups)
	backendURLs := make([]*url.URL, numBackendGroups)
	for i := 0; i < numBackendGroups; i++ {
		backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("backend%d", i)))
		}))
		backendServers[i] = backendServer
		backendURLs[i], _ = url.Parse(backendServer.URL)
		defer backendServer.Close()
	}

	// create backend groups where each group has one backend server
	backendGroups := make([]*config.BackendGroup, numBackendGroups)
	for i := 0; i < numBackendGroups; i++ {
		backendGroups[i] = &config.BackendGroup{
			Lb:      loadbalancer.NewRoundRobin([]*url.URL{backendURLs[i]}),
			Servers: []*url.URL{backendURLs[i]},
		}
	}

	// create routing rules for each backend group
	rules := make([]*config.Rule, numBackendGroups)
	for i := 0; i < numBackendGroups; i++ {
		rules[i] = &config.Rule{
			Path:         fmt.Sprintf("/api%d", i),
			BackendGroup: backendGroups[i],
		}
	}

	// create proxy config with routing rules for different paths
	proxyConfig := &config.Config{
		Port:          0,
		BackendGroups: backendGroups,
		Rules:         rules,
	}

	// start the proxy
	proxyServer := proxy.NewProxy(proxyConfig, &proxy.HttpClient{Client: &http.Client{}})
	proxyURL := startProxyAndGetURL(t, proxyServer)
	defer proxyServer.Stop()

	// test all valid routes + an invalid route
	for i := 0; i <= numBackendGroups; i++ {
		resp, err := http.Get(proxyURL + fmt.Sprintf("/api%d", i))
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		if i == numBackendGroups { // invalid route
			require.Equal(t, http.StatusNotFound, resp.StatusCode)
		} else { // valid route
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, fmt.Sprintf("backend%d", i), string(body))
		}
	}
}

func TestAddHeaderRequestOperation(t *testing.T) {
	testHeader := "X-Custom-Header"
	testHeaderValue := "test-header-value"

	// start a backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// echo back the test header that was added by the proxy
		w.Write([]byte(r.Header.Get(testHeader)))
	}))
	backendURL, _ := url.Parse(backendServer.URL)
	defer backendServer.Close()

	// create proxy config with a AddHeaderRequest operation
	backendGroup := &config.BackendGroup{
		Lb:      loadbalancer.NewRoundRobin([]*url.URL{backendURL}),
		Servers: []*url.URL{backendURL},
	}
	proxyConfig := &config.Config{
		Port: 0,
		BackendGroups: []*config.BackendGroup{
			backendGroup,
		},
		Rules: []*config.Rule{
			{
				Path:         "",
				BackendGroup: backendGroup,
				RequestOperations: []config.RequestOperation{
					&config.AddHeaderRequestOperation{
						Header: testHeader,
						Value:  testHeaderValue,
					},
				},
			},
		},
	}

	// start the proxy
	proxyServer := proxy.NewProxy(proxyConfig, &proxy.HttpClient{Client: &http.Client{}})
	proxyURL := startProxyAndGetURL(t, proxyServer)
	defer proxyServer.Stop()

	// send request to proxy and verify the custom header was added
	resp, err := http.Get(proxyURL + "/")
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, testHeaderValue, string(body))
}

func TestAddHeaderResponseOperation(t *testing.T) {
	// start a backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	backendURL, _ := url.Parse(backendServer.URL)
	defer backendServer.Close()

	// create a backend group with one backend server
	backendGroup := &config.BackendGroup{
		Lb:      loadbalancer.NewRoundRobin([]*url.URL{backendURL}),
		Servers: []*url.URL{backendURL},
	}

	// create a rule with an AddHeaderResponseOperation
	testHeader := "X-Response-Header"
	testHeaderValue := "response-header-value"
	proxyConfig := &config.Config{
		Port: 0,
		BackendGroups: []*config.BackendGroup{
			backendGroup,
		},
		Rules: []*config.Rule{
			{
				Path:         "",
				BackendGroup: backendGroup,
				ResponseOperations: []config.ResponseOperation{
					&config.AddHeaderResponseOperation{
						Header: testHeader,
						Value:  testHeaderValue,
					},
				},
			},
		},
	}

	// start the proxy
	proxyServer := proxy.NewProxy(proxyConfig, &proxy.HttpClient{Client: &http.Client{}})
	proxyURL := startProxyAndGetURL(t, proxyServer)
	defer proxyServer.Stop()

	// send request to proxy and verify the response header was added
	resp, err := http.Get(proxyURL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, testHeaderValue, resp.Header.Get(testHeader))
}

func TestHealthChecks(t *testing.T) {
	t.Skip("health checks feature has been commented out in order to easily refactor config loading")
	// start two backend servers
	responseBodyServer1 := "backend1"
	backendServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write([]byte(responseBodyServer1))
	}))
	backendURL1, _ := url.Parse(backendServer1.URL)
	defer backendServer1.Close()

	responseBodyServer2 := "backend2"
	healthyHandlerServer2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Write([]byte(responseBodyServer2))
	})
	backendServer2 := httptest.NewServer(healthyHandlerServer2)
	backendURL2, _ := url.Parse(backendServer2.URL)
	defer backendServer2.Close()

	backendURLs := []*url.URL{backendURL1, backendURL2}

	backendGroup := &config.BackendGroup{
		Name:    "test-group",
		Lb:      loadbalancer.NewRoundRobin(backendURLs),
		Servers: backendURLs,
		HealthCheck: &config.HealthCheck{
			Path:     "/health",
			Interval: 20 * time.Millisecond,
			Timeout:  5 * time.Millisecond,
			Retries:  3,
		},
	}

	proxyConfig := &config.Config{
		Port: 0, // let OS assign an available port
		BackendGroups: []*config.BackendGroup{
			backendGroup,
		},
		Rules: []*config.Rule{
			{
				Path:         "",
				BackendGroup: backendGroup,
			},
		},
	}

	// start the proxy
	proxyServer := proxy.NewProxy(proxyConfig, &proxy.HttpClient{Client: &http.Client{}})
	proxyURL := startProxyAndGetURL(t, proxyServer)
	defer proxyServer.Stop()

	// send requests to proxy when two servers are healthy
	numRequests := 5
	numOfRespFromServer1 := 0
	numOfRespFromServer2 := 0
	for i := 0; i < numRequests; i++ {
		resp, err := http.Get(proxyURL + "/")
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		if string(body) == responseBodyServer1 {
			numOfRespFromServer1++
		} else if string(body) == responseBodyServer2 {
			numOfRespFromServer2++
		}
	}
	require.GreaterOrEqual(t, numOfRespFromServer1, 1)
	require.GreaterOrEqual(t, numOfRespFromServer2, 1)

	// simulate server 2 going down
	unhealthyHandlerServer2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	backendServer2.Config.Handler = unhealthyHandlerServer2

	// wait for the proxy to detect that backend server 2 is down
	time.Sleep(100 * time.Millisecond)

	// send requests to proxy when only one server is healthy
	numRequests = 5
	numOfRespFromServer1 = 0
	numOfRespFromServer2 = 0
	for i := 0; i < numRequests; i++ {
		resp, err := http.Get(proxyURL + "/")
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		if string(body) == responseBodyServer1 {
			numOfRespFromServer1++
		} else if string(body) == responseBodyServer2 {
			numOfRespFromServer2++
		}
	}
	require.Equal(t, numOfRespFromServer1, numRequests)
	require.Equal(t, numOfRespFromServer2, 0)

	// bring back server 2 to life
	backendServer2.Config.Handler = healthyHandlerServer2
	time.Sleep(100 * time.Millisecond)

	// send requests to proxy after server 2 is back up
	numRequests = 5
	numOfRespFromServer1 = 0
	numOfRespFromServer2 = 0
	for i := 0; i < numRequests; i++ {
		resp, err := http.Get(proxyURL + "/")
		require.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		if string(body) == responseBodyServer1 {
			numOfRespFromServer1++
		} else if string(body) == responseBodyServer2 {
			numOfRespFromServer2++
		}
	}
	require.GreaterOrEqual(t, numOfRespFromServer1, 1)
	require.GreaterOrEqual(t, numOfRespFromServer2, 1)
}

func TestYamlConfigLoading(t *testing.T) {
	bodyContent := "Hello World!"
	// start a backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(bodyContent))
	}))
	backendURL, _ := url.Parse(backendServer.URL)
	defer backendServer.Close()

	// fill in config template with backend url
	templatePath := filepath.Join("testdata", t.Name(), "config.yaml")
	tempConfigFile := createTempConfigFile(t, templatePath, backendURL)
	defer os.Remove(tempConfigFile.Name())

	// load proxy from config file
	proxy, err := proxy.NewProxyFromConfigFile(tempConfigFile.Name(), &proxy.HttpClient{Client: &http.Client{}})
	require.NoError(t, err)

	proxyURL := startProxyAndGetURL(t, proxy)
	defer proxy.Stop()

	// send request to proxy and verify the response body
	resp, err := http.Get(proxyURL + "/")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, bodyContent, string(body))
}

func TestYamlConfigReloading(t *testing.T) {
	// start a backend server
	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	backendURL, _ := url.Parse(backendServer.URL)
	defer backendServer.Close()

	// fill in v0 config template with backend url
	templatePath_v0 := filepath.Join("testdata", t.Name(), "config_v0.yaml")
	tempConfigFile_v0 := createTempConfigFile(t, templatePath_v0, backendURL)
	defer os.Remove(tempConfigFile_v0.Name())

	// fill in v1 config template with backend url
	templatePath_v1 := filepath.Join("testdata", t.Name(), "config_v1.yaml")
	tempConfigFile_v1 := createTempConfigFile(t, templatePath_v1, backendURL)

	// load proxy from v0 config file
	proxy, err := proxy.NewProxyFromConfigFile(tempConfigFile_v0.Name(), &proxy.HttpClient{Client: &http.Client{}})
	require.NoError(t, err)

	proxyURL := startProxyAndGetURL(t, proxy)
	defer proxy.Stop()

	// send requests to proxy using v0 config
	resp_v0, err := http.Get(proxyURL + "/v0")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp_v0.StatusCode)
	defer resp_v0.Body.Close()

	resp_v1, err := http.Get(proxyURL + "/v1")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp_v1.StatusCode)
	defer resp_v1.Body.Close()

	// mv config_v1 to config_v0
	os.Rename(tempConfigFile_v1.Name(), tempConfigFile_v0.Name())

	// send SIGHUP signal to reload config
	p, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = p.Signal(syscall.SIGHUP)
	require.NoError(t, err)

	// wait for config to reload
	time.Sleep(100 * time.Millisecond)

	// send requests to proxy using v1 config
	resp_v0, err = http.Get(proxyURL + "/v0")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp_v0.StatusCode)
	defer resp_v0.Body.Close()

	resp_v1, err = http.Get(proxyURL + "/v1")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp_v1.StatusCode)
	defer resp_v1.Body.Close()
}

// startProxyAndGetURL starts the proxy in a separate goroutine and waits for its address to be available
func startProxyAndGetURL(t *testing.T, proxyServer *proxy.Proxy) string {
	go func() {
		if err := proxyServer.Start(); err != nil && err != http.ErrServerClosed {
			t.Logf("Proxy server error: %v", err)
		}
	}()

	var proxyURL string
	for i := 0; i < 50; i++ {
		if addr := proxyServer.GetAddr(); addr != "" {
			proxyURL = fmt.Sprintf("http://%s", addr)
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.NotEmpty(t, proxyURL, "proxy failed to start")
	return proxyURL
}

// createTempConfigFile creates a temporary config file from the given template and backend URL
func createTempConfigFile(t *testing.T, templatePath string, backendURL *url.URL) *os.File {
	template, err := os.ReadFile(templatePath)
	require.NoError(t, err)

	// replace placeholder with actual backend URL
	configContent := strings.ReplaceAll(string(template), "{{BACKEND_URL}}", backendURL.String())

	// write to temporary file
	tempConfigFile, err := os.CreateTemp("", "proxy_config_*.yaml")
	require.NoError(t, err)

	_, err = tempConfigFile.WriteString(configContent)
	require.NoError(t, err)
	tempConfigFile.Close()

	return tempConfigFile
}
