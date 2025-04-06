package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

// FetchServer creation test
func TestNewFetchServer(t *testing.T) {
	// Test cases: Creating new FetchServer instances with different parameters
	testCases := []struct {
		name        string
		timeout     int
		userAgent   string
		maxBodySize int64
	}{
		{
			name:        "Default configuration",
			timeout:     30,
			userAgent:   "MCP-Fetch-Server/1.0",
			maxBodySize: 10 * 1024 * 1024,
		},
		{
			name:        "Custom timeout",
			timeout:     60,
			userAgent:   "MCP-Fetch-Server/1.0",
			maxBodySize: 10 * 1024 * 1024,
		},
		{
			name:        "Custom user agent",
			timeout:     30,
			userAgent:   "CustomUserAgent/2.0",
			maxBodySize: 10 * 1024 * 1024,
		},
		{
			name:        "Custom body size limit",
			timeout:     30,
			userAgent:   "MCP-Fetch-Server/1.0",
			maxBodySize: 5 * 1024 * 1024,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create FetchServer instance
			fs := NewFetchServer(tc.timeout, tc.userAgent, tc.maxBodySize)

			// Verification
			assert.NotNil(t, fs, "FetchServer instance should be created")
			assert.Equal(t, tc.userAgent, fs.userAgent, "User-Agent should match")
			assert.Equal(t, tc.maxBodySize, fs.maxBodySize, "Max body size should match")
			assert.NotNil(t, fs.server, "Internal MCPServer should be initialized")
			assert.NotNil(t, fs.client, "HTTP client should be initialized")
			assert.Equal(t, time.Duration(tc.timeout)*time.Second, fs.client.Timeout, "Timeout should match")
		})
	}
}

// Server method test
func TestServer(t *testing.T) {
	// Create FetchServer instance
	fs := NewFetchServer(30, "Test-User-Agent", 1024*1024)
	assert.NotNil(t, fs, "FetchServer instance should be created")

	// Verify Server method returns valid MCPServer instance
	server := fs.Server()
	assert.NotNil(t, server, "Server method should return a valid MCPServer instance")
}

// Mock server helper for testing HTTP requests
func setupMockServer() *httptest.Server {
	handler := http.NewServeMux()

	// GET endpoint
	handler.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"Hello from GET"}`))
	})

	// POST endpoint
	handler.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"Hello from POST"}`))
	})

	// Echo headers endpoint
	handler.HandleFunc("/echo-headers", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Echo-Test", "test-value")

		customHeader := r.Header.Get("X-Custom-Header")
		response := map[string]string{"received-header": customHeader}

		jsonResp, _ := json.Marshal(response)
		w.Write(jsonResp)
	})

	return httptest.NewServer(handler)
}

// Test URL validation
func TestURLValidation(t *testing.T) {
	fs := NewFetchServer(5, "Test-Agent", 1024)

	invalidURLs := []string{
		"ftp://example.com",
		"example.com",
		"//example.com",
		"www.example.com",
	}

	ctx := context.Background()

	for _, url := range invalidURLs {
		t.Run("Invalid URL: "+url, func(t *testing.T) {
			req := mcp.CallToolRequest{
				Params: struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments,omitempty"`
					Meta      *struct {
						ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
					} `json:"_meta,omitempty"`
				}{
					Name:      "fetchURL",
					Arguments: map[string]interface{}{"url": url},
				},
			}

			// Call the handler
			result, err := fs.handleFetchURL(ctx, req)

			// Verify error
			assert.Error(t, err, "Should return error for invalid URL")
			assert.Nil(t, result, "Result should be nil on error")
			assert.Contains(t, err.Error(), "URL must begin with http:// or https://", "Error message should indicate URL format issue")
		})
	}
}

// Test HTTP methods
func TestHTTPMethods(t *testing.T) {
	// Set up a test server
	mockServer := setupMockServer()
	defer mockServer.Close()

	fs := NewFetchServer(5, "Test-Agent", 1024*1024)
	ctx := context.Background()

	// Test GET request
	t.Run("GET request", func(t *testing.T) {
		params := map[string]interface{}{
			"url":    mockServer.URL + "/get",
			"method": "GET",
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "fetchURL",
				Arguments: params,
			},
		}

		result, err := fs.handleFetchURL(ctx, req)

		assert.NoError(t, err, "GET request should not error")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Hello from GET", "Response should contain expected content")
	})

	// Test POST request
	t.Run("POST request", func(t *testing.T) {
		params := map[string]interface{}{
			"url":    mockServer.URL + "/post",
			"method": "POST",
			"body":   `{"test":"data"}`,
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "fetchURL",
				Arguments: params,
			},
		}

		result, err := fs.handleFetchURL(ctx, req)

		assert.NoError(t, err, "POST request should not error")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Hello from POST", "Response should contain expected content")
	})

	// Test default method (GET when not specified)
	t.Run("Default method (GET)", func(t *testing.T) {
		params := map[string]interface{}{
			"url": mockServer.URL + "/get",
			// No method specified, should default to GET
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "fetchURL",
				Arguments: params,
			},
		}

		result, err := fs.handleFetchURL(ctx, req)

		assert.NoError(t, err, "Default GET request should not error")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Hello from GET", "Response should contain expected content")
	})
}

// Test custom headers
func TestCustomHeaders(t *testing.T) {
	// Set up a test server
	mockServer := setupMockServer()
	defer mockServer.Close()

	fs := NewFetchServer(5, "Test-Agent", 1024*1024)
	ctx := context.Background()

	t.Run("Custom headers", func(t *testing.T) {
		headers := map[string]string{
			"X-Custom-Header": "test-value",
		}

		headersJSON, _ := json.Marshal(headers)

		params := map[string]interface{}{
			"url":     mockServer.URL + "/echo-headers",
			"headers": string(headersJSON),
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "fetchURL",
				Arguments: params,
			},
		}

		result, err := fs.handleFetchURL(ctx, req)

		assert.NoError(t, err, "Request with custom headers should not error")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "test-value", "Response should contain the echoed header value")
	})

	// Test invalid headers JSON
	t.Run("Invalid headers JSON", func(t *testing.T) {
		params := map[string]interface{}{
			"url":     mockServer.URL + "/echo-headers",
			"headers": "{invalid json}",
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "fetchURL",
				Arguments: params,
			},
		}

		result, err := fs.handleFetchURL(ctx, req)

		assert.Error(t, err, "Invalid headers JSON should cause an error")
		assert.Nil(t, result, "Result should be nil on error")
		assert.Contains(t, err.Error(), "invalid headers JSON", "Error should mention invalid headers JSON")
	})
}
