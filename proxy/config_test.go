package proxy_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mouad-eh/wasseet/proxy"
	"github.com/stretchr/testify/require"
)

func TestRuleMatch(t *testing.T) {
	tests := []struct {
		name     string
		rule     *proxy.Rule
		request  proxy.ServerRequest
		expected bool
	}{
		{
			name: "match by path when host is empty",
			rule: &proxy.Rule{
				Host: "",
				Path: "/api",
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://any.com/api", nil)
				return proxy.ServerRequest{req}
			}(),
			expected: true,
		},
		{
			name: "match by host when path is empty",
			rule: &proxy.Rule{
				Host: "example.com",
				Path: "",
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com/any/path", nil)
				return proxy.ServerRequest{req}
			}(),
			expected: true,
		},
		{
			name: "partial match by host",
			rule: &proxy.Rule{
				Host: "example.com",
				Path: "/api",
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com/other", nil)
				return proxy.ServerRequest{req}
			}(),
			expected: false,
		},
		{
			name: "partial match by path",
			rule: &proxy.Rule{
				Host: "other.com",
				Path: "/api/users",
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://different.com/api/users", nil)
				return proxy.ServerRequest{req}
			}(),
			expected: false,
		},
		{
			name: "match by both host and path",
			rule: &proxy.Rule{
				Host: "example.com",
				Path: "/api",
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com/api", nil)
				return proxy.ServerRequest{req}
			}(),
			expected: true,
		},
		{
			name: "no match by both host and path",
			rule: &proxy.Rule{
				Host: "example.com",
				Path: "/api",
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://other.com/other", nil)
				return proxy.ServerRequest{req}
			}(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rule.Match(tt.request)
			require.Equal(t, tt.expected, result)
		})
	}
}

type mockRequestOperation struct {
	callCount int
}

func (m *mockRequestOperation) Apply(req proxy.ServerRequest) {
	m.callCount++
}

func TestApplyRequestOperations(t *testing.T) {
	tests := []struct {
		name              string
		operations        []proxy.RequestOperation
		expectedCallCount int
	}{
		{
			name:              "zero operations",
			operations:        []proxy.RequestOperation{},
			expectedCallCount: 0,
		},
		{
			name: "multiple operations",
			operations: []proxy.RequestOperation{
				&mockRequestOperation{},
				&mockRequestOperation{},
				&mockRequestOperation{},
			},
			expectedCallCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			serverReq := proxy.ServerRequest{req}

			rule := &proxy.Rule{
				RequestOperations: tt.operations,
			}
			rule.ApplyRequestOperations(serverReq)

			// Count the actual calls made
			callCount := 0
			for _, op := range tt.operations {
				if mock, ok := op.(*mockRequestOperation); ok {
					callCount += mock.callCount
				}
			}

			require.Equal(t, tt.expectedCallCount, callCount)
		})
	}
}

type mockResponseOperation struct {
	callCount int
}

func (m *mockResponseOperation) Apply(resp *http.Response) {
	m.callCount++
}

func TestApplyResponseOperations(t *testing.T) {
	tests := []struct {
		name              string
		operations        []proxy.ResponseOperation
		expectedCallCount int
	}{
		{
			name:              "zero operations",
			operations:        []proxy.ResponseOperation{},
			expectedCallCount: 0,
		},
		{
			name: "multiple operations",
			operations: []proxy.ResponseOperation{
				&mockResponseOperation{},
				&mockResponseOperation{},
				&mockResponseOperation{},
			},
			expectedCallCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}

			rule := &proxy.Rule{
				ResponseOperations: tt.operations,
			}
			rule.ApplyResponseOperations(resp)

			// Count the actual calls made
			callCount := 0
			for _, op := range tt.operations {
				if mock, ok := op.(*mockResponseOperation); ok {
					callCount += mock.callCount
				}
			}

			require.Equal(t, tt.expectedCallCount, callCount)
		})
	}
}

func TestGetFirstMatchingRule(t *testing.T) {
	backendGroup := &proxy.BackendGroup{
		Name: "backend-1",
	}

	tests := []struct {
		name        string
		config      *proxy.Config
		request     proxy.ServerRequest
		expectError bool
		expectRule  *proxy.Rule
	}{
		{
			name: "returns first matching rule",
			config: &proxy.Config{
				Rules: []*proxy.Rule{
					{
						Host:         "example.com",
						BackendGroup: backendGroup,
					},
					{
						Host:         "other.com",
						BackendGroup: backendGroup,
					},
				},
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com", nil)
				return proxy.ServerRequest{req}
			}(),
			expectError: false,
			expectRule: &proxy.Rule{
				Host:         "example.com",
				BackendGroup: backendGroup,
			},
		},
		{
			name: "returns correct rule when first doesn't match",
			config: &proxy.Config{
				Rules: []*proxy.Rule{
					{
						Host:         "first.com",
						Path:         "/other",
						BackendGroup: backendGroup,
					},
					{
						Host:         "second.com",
						Path:         "/api",
						BackendGroup: backendGroup,
					},
				},
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://second.com/api", nil)
				return proxy.ServerRequest{req}
			}(),
			expectError: false,
			expectRule: &proxy.Rule{
				Host:         "second.com",
				Path:         "/api",
				BackendGroup: backendGroup,
			},
		},
		{
			name: "returns error when no rule matches",
			config: &proxy.Config{
				Rules: []*proxy.Rule{
					{
						Host:         "example.com",
						Path:         "/api/v1",
						BackendGroup: backendGroup,
					},
				},
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://nomatch.com/other", nil)
				return proxy.ServerRequest{req}
			}(),
			expectError: true,
		},
		{
			name: "returns error when no rules defined",
			config: &proxy.Config{
				Rules: []*proxy.Rule{},
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com", nil)
				return proxy.ServerRequest{req}
			}(),
			expectError: true,
		},
		{
			name: "respects rule ordering",
			config: &proxy.Config{
				Rules: []*proxy.Rule{
					{
						Path:         "/api",
						BackendGroup: backendGroup,
					},
					{
						Host:         "example.com",
						Path:         "/api",
						BackendGroup: backendGroup,
					},
				},
			},
			request: func() proxy.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com/api", nil)
				return proxy.ServerRequest{req}
			}(),
			expectError: false,
			expectRule: &proxy.Rule{
				Path:         "/api",
				BackendGroup: backendGroup,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := tt.config.GetFirstMatchingRule(tt.request)

			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, rule)
			} else {
				require.NoError(t, err)
				require.NotNil(t, rule)
				require.Equal(t, tt.expectRule.Host, rule.Host)
				require.Equal(t, tt.expectRule.Path, rule.Path)
				require.Equal(t, tt.expectRule.BackendGroup, rule.BackendGroup)
			}
		})
	}
}
