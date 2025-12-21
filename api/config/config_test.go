package config_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mouad-eh/wasseet/api/config"
	"github.com/mouad-eh/wasseet/request"
	"github.com/mouad-eh/wasseet/testutils/mocks"
	"github.com/stretchr/testify/require"
)

func TestRuleMatch(t *testing.T) {
	tests := []struct {
		name     string
		rule     *config.Rule
		request  request.ServerRequest
		expected bool
	}{
		{
			name: "match by path when host is empty",
			rule: &config.Rule{
				Host: "",
				Path: "/api",
			},
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://any.com/api", nil)
				return request.ServerRequest{req}
			}(),
			expected: true,
		},
		{
			name: "match by host when path is empty",
			rule: &config.Rule{
				Host: "example.com",
				Path: "",
			},
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com/any/path", nil)
				return request.ServerRequest{req}
			}(),
			expected: true,
		},
		{
			name: "partial match by host",
			rule: &config.Rule{
				Host: "example.com",
				Path: "/api",
			},
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com/other", nil)
				return request.ServerRequest{req}
			}(),
			expected: false,
		},
		{
			name: "partial match by path",
			rule: &config.Rule{
				Host: "other.com",
				Path: "/api/users",
			},
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://different.com/api/users", nil)
				return request.ServerRequest{req}
			}(),
			expected: false,
		},
		{
			name: "match by both host and path",
			rule: &config.Rule{
				Host: "example.com",
				Path: "/api",
			},
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com/api", nil)
				return request.ServerRequest{req}
			}(),
			expected: true,
		},
		{
			name: "no match by both host and path",
			rule: &config.Rule{
				Host: "example.com",
				Path: "/api",
			},
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://other.com/other", nil)
				return request.ServerRequest{req}
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

func TestApplyRequestOperations(t *testing.T) {
	tests := []struct {
		name                   string
		operations             []config.RequestOperation
		expectedApplyCallCount int
	}{
		{
			name:                   "zero operations",
			operations:             []config.RequestOperation{},
			expectedApplyCallCount: 0,
		},
		{
			name: "multiple operations",
			operations: []config.RequestOperation{
				&mocks.RequestOperationMock{},
				&mocks.RequestOperationMock{},
				&mocks.RequestOperationMock{},
			},
			expectedApplyCallCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com", nil)
			serverReq := request.ServerRequest{req}

			rule := &config.Rule{
				RequestOperations: tt.operations,
			}
			rule.ApplyRequestOperations(serverReq)

			// Count the actual calls made
			callCount := 0
			for _, op := range tt.operations {
				if mock, ok := op.(*mocks.RequestOperationMock); ok {
					callCount += len(mock.ApplyCalls())
				}
			}

			require.Equal(t, tt.expectedApplyCallCount, callCount)
		})
	}
}

func TestApplyResponseOperations(t *testing.T) {
	tests := []struct {
		name              string
		operations        []config.ResponseOperation
		expectedCallCount int
	}{
		{
			name:              "zero operations",
			operations:        []config.ResponseOperation{},
			expectedCallCount: 0,
		},
		{
			name: "multiple operations",
			operations: []config.ResponseOperation{
				&mocks.ResponseOperationMock{},
				&mocks.ResponseOperationMock{},
				&mocks.ResponseOperationMock{},
			},
			expectedCallCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				Header: make(http.Header),
			}

			rule := &config.Rule{
				ResponseOperations: tt.operations,
			}
			rule.ApplyResponseOperations(resp)

			// Count the actual calls made
			callCount := 0
			for _, op := range tt.operations {
				if mock, ok := op.(*mocks.ResponseOperationMock); ok {
					callCount += len(mock.ApplyCalls())
				}
			}

			require.Equal(t, tt.expectedCallCount, callCount)
		})
	}
}

func TestGetFirstMatchingRule(t *testing.T) {
	backendGroup := &config.BackendGroup{
		Name: "backend-1",
	}

	tests := []struct {
		name        string
		config      *config.Config
		request     request.ServerRequest
		expectError bool
		expectRule  *config.Rule
	}{
		{
			name: "returns first matching rule",
			config: &config.Config{
				Rules: []*config.Rule{
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
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com", nil)
				return request.ServerRequest{req}
			}(),
			expectError: false,
			expectRule: &config.Rule{
				Host:         "example.com",
				BackendGroup: backendGroup,
			},
		},
		{
			name: "returns correct rule when first doesn't match",
			config: &config.Config{
				Rules: []*config.Rule{
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
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://second.com/api", nil)
				return request.ServerRequest{req}
			}(),
			expectError: false,
			expectRule: &config.Rule{
				Host:         "second.com",
				Path:         "/api",
				BackendGroup: backendGroup,
			},
		},
		{
			name: "returns error when no rule matches",
			config: &config.Config{
				Rules: []*config.Rule{
					{
						Host:         "example.com",
						Path:         "/api/v1",
						BackendGroup: backendGroup,
					},
				},
			},
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://nomatch.com/other", nil)
				return request.ServerRequest{req}
			}(),
			expectError: true,
		},
		{
			name: "returns error when no rules defined",
			config: &config.Config{
				Rules: []*config.Rule{},
			},
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com", nil)
				return request.ServerRequest{req}
			}(),
			expectError: true,
		},
		{
			name: "respects rule ordering",
			config: &config.Config{
				Rules: []*config.Rule{
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
			request: func() request.ServerRequest {
				req := httptest.NewRequest("GET", "http://example.com/api", nil)
				return request.ServerRequest{req}
			}(),
			expectError: false,
			expectRule: &config.Rule{
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
