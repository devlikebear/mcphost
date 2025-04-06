package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

// GoogleSearchServer creation test
func TestNewGoogleSearchServer(t *testing.T) {
	// Test cases: Creating new GoogleSearchServer instances with different parameters
	testCases := []struct {
		name           string
		timeout        int
		userAgent      string
		maxBodySize    int64
		apiKey         string
		searchEngineID string
	}{
		{
			name:           "Default configuration",
			timeout:        30,
			userAgent:      "MCP-GoogleSearch-Server/1.0",
			maxBodySize:    10 * 1024 * 1024,
			apiKey:         "test-api-key",
			searchEngineID: "test-search-engine-id",
		},
		{
			name:           "Custom timeout",
			timeout:        60,
			userAgent:      "MCP-GoogleSearch-Server/1.0",
			maxBodySize:    10 * 1024 * 1024,
			apiKey:         "test-api-key",
			searchEngineID: "test-search-engine-id",
		},
		{
			name:           "Custom user agent",
			timeout:        30,
			userAgent:      "CustomUserAgent/2.0",
			maxBodySize:    10 * 1024 * 1024,
			apiKey:         "test-api-key",
			searchEngineID: "test-search-engine-id",
		},
		{
			name:           "Missing API credentials",
			timeout:        30,
			userAgent:      "MCP-GoogleSearch-Server/1.0",
			maxBodySize:    10 * 1024 * 1024,
			apiKey:         "",
			searchEngineID: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create GoogleSearchServer instance
			gs := NewGoogleSearchServer(tc.timeout, tc.userAgent, tc.maxBodySize, tc.apiKey, tc.searchEngineID)

			// Verification
			assert.NotNil(t, gs, "GoogleSearchServer instance should be created")
			assert.Equal(t, tc.userAgent, gs.userAgent, "User-Agent should match")
			assert.Equal(t, tc.maxBodySize, gs.maxBodySize, "Max body size should match")
			assert.Equal(t, tc.apiKey, gs.apiKey, "API key should match")
			assert.Equal(t, tc.searchEngineID, gs.searchEngineID, "Search Engine ID should match")
			assert.NotNil(t, gs.server, "Internal MCPServer should be initialized")
			assert.NotNil(t, gs.client, "HTTP client should be initialized")
			assert.Equal(t, time.Duration(tc.timeout)*time.Second, gs.client.Timeout, "Timeout should match")
		})
	}
}

// Server method test
func TestServer(t *testing.T) {
	// Create GoogleSearchServer instance
	gs := NewGoogleSearchServer(30, "Test-User-Agent", 1024*1024, "test-key", "test-cx")
	assert.NotNil(t, gs, "GoogleSearchServer instance should be created")

	// Verify Server method returns valid MCPServer instance
	server := gs.Server()
	assert.NotNil(t, server, "Server method should return a valid MCPServer instance")
}

// Mock server helper for testing Google Search API
func setupMockGoogleAPI() *httptest.Server {
	handler := http.NewServeMux()

	// Mock search endpoint
	handler.HandleFunc("/customsearch/v1", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")

		if query == "" {
			http.Error(w, "Missing query parameter", http.StatusBadRequest)
			return
		}

		// Check for API key
		if r.URL.Query().Get("key") == "" {
			http.Error(w, "Missing API key", http.StatusUnauthorized)
			return
		}

		// Check for search engine ID
		if r.URL.Query().Get("cx") == "" {
			http.Error(w, "Missing cx parameter", http.StatusBadRequest)
			return
		}

		// Create simple mock response
		mockResponse := GoogleApiResponse{
			Kind: "customsearch#search",
			URL: &struct {
				Type     string `json:"type"`
				Template string `json:"template"`
			}{
				Type:     "application/json",
				Template: "https://www.googleapis.com/customsearch/v1?q={searchTerms}&num={count?}&start={startIndex?}",
			},
			Queries: map[string]interface{}{
				"request": []map[string]interface{}{
					{
						"searchTerms": query,
						"count":       10,
						"startIndex":  1,
					},
				},
			},
			SearchInformation: &struct {
				SearchTime            float64 `json:"searchTime"`
				FormattedSearchTime   string  `json:"formattedSearchTime"`
				TotalResults          string  `json:"totalResults"`
				FormattedTotalResults string  `json:"formattedTotalResults"`
			}{
				SearchTime:            0.2,
				FormattedSearchTime:   "0.2",
				TotalResults:          "1000",
				FormattedTotalResults: "1,000",
			},
		}

		// Add mock search results
		if query != "no-results" {
			mockResponse.Items = []struct {
				Kind        string `json:"kind"`
				Title       string `json:"title"`
				HTMLTitle   string `json:"htmlTitle"`
				Link        string `json:"link"`
				DisplayLink string `json:"displayLink"`
				Snippet     string `json:"snippet"`
				HTMLSnippet string `json:"htmlSnippet"`
				CacheID     string `json:"cacheId,omitempty"`
				Mime        string `json:"mime,omitempty"`
				FileFormat  string `json:"fileFormat,omitempty"`
			}{
				{
					Kind:        "customsearch#result",
					Title:       "Test Result 1 for " + query,
					HTMLTitle:   "Test Result 1 for " + query,
					Link:        "https://example.com/1",
					DisplayLink: "example.com",
					Snippet:     "This is a test snippet for the first result about " + query,
					HTMLSnippet: "This is a test snippet for the first result about " + query,
				},
				{
					Kind:        "customsearch#result",
					Title:       "Test Result 2 for " + query,
					HTMLTitle:   "Test Result 2 for " + query,
					Link:        "https://example.com/2",
					DisplayLink: "example.com",
					Snippet:     "This is a test snippet for the second result about " + query,
					HTMLSnippet: "This is a test snippet for the second result about " + query,
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	})

	return httptest.NewServer(handler)
}

// Test API status with missing credentials
func TestApiStatusMissingCredentials(t *testing.T) {
	// Create server with missing credentials
	gs := NewGoogleSearchServer(5, "Test-Agent", 1024, "", "")
	ctx := context.Background()

	req := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      "getApiStatus",
			Arguments: map[string]interface{}{},
		},
	}

	// Call the handler
	result, err := gs.handleApiStatus(ctx, req)

	// Verify result
	assert.NoError(t, err, "API status check should not error even with missing credentials")
	assert.NotNil(t, result, "Result should not be nil")
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "API key is not configured", "Should report that API key is missing")
}

// Test API status with valid credentials
func TestApiStatusWithCredentials(t *testing.T) {
	// Create server with valid credentials
	gs := NewGoogleSearchServer(5, "Test-Agent", 1024, "valid-key", "valid-cx")
	ctx := context.Background()

	req := mcp.CallToolRequest{
		Params: struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments,omitempty"`
			Meta      *struct {
				ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
			} `json:"_meta,omitempty"`
		}{
			Name:      "getApiStatus",
			Arguments: map[string]interface{}{},
		},
	}

	// Call the handler
	result, err := gs.handleApiStatus(ctx, req)

	// Verify result
	assert.NoError(t, err, "API status check should not error")
	assert.NotNil(t, result, "Result should not be nil")
	assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "properly configured", "Should report that configuration is valid")
}

// Test Google search with valid query
func TestGoogleSearch(t *testing.T) {
	// Setup mock Google API server
	mockServer := setupMockGoogleAPI()
	defer mockServer.Close()

	// Create GoogleSearchServer and override the base URL to point to the mock server
	gs := NewGoogleSearchServer(5, "Test-Agent", 1024*1024, "test-key", "test-cx")
	ctx := context.Background()

	t.Run("Basic search", func(t *testing.T) {
		params := map[string]interface{}{
			"query": "test search",
			"num":   float64(2),
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "searchGoogle",
				Arguments: params,
			},
		}

		// Temporarily replace the base URL for testing
		originalBaseURL := "https://www.googleapis.com/customsearch/v1"
		baseURL := mockServer.URL + "/customsearch/v1"

		// Hijack the HTTP request to use our mock server
		originalClient := gs.client
		gs.client = &http.Client{
			Transport: &mockTransport{
				originalURL: originalBaseURL,
				mockURL:     baseURL,
			},
		}

		result, err := gs.handleGoogleSearch(ctx, req)

		// Restore the original client
		gs.client = originalClient

		assert.NoError(t, err, "Search should not error with valid parameters")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Google Search Results for: test search", "Response should contain the search query")
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "Test Result 1", "Response should contain search results")
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "https://example.com/", "Response should contain result URLs")
	})

	t.Run("No results search", func(t *testing.T) {
		params := map[string]interface{}{
			"query": "no-results",
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "searchGoogle",
				Arguments: params,
			},
		}

		// Temporarily replace the base URL for testing
		originalBaseURL := "https://www.googleapis.com/customsearch/v1"
		baseURL := mockServer.URL + "/customsearch/v1"

		// Hijack the HTTP request to use our mock server
		originalClient := gs.client
		gs.client = &http.Client{
			Transport: &mockTransport{
				originalURL: originalBaseURL,
				mockURL:     baseURL,
			},
		}

		result, err := gs.handleGoogleSearch(ctx, req)

		// Restore the original client
		gs.client = originalClient

		assert.NoError(t, err, "Search should not error with valid parameters")
		assert.NotNil(t, result, "Result should not be nil")
		assert.Contains(t, result.Content[0].(mcp.TextContent).Text, "No results found", "Response should indicate no results found")
	})

	t.Run("Missing query", func(t *testing.T) {
		params := map[string]interface{}{
			"query": "",
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "searchGoogle",
				Arguments: params,
			},
		}

		result, err := gs.handleGoogleSearch(ctx, req)

		assert.Error(t, err, "Search should error with empty query")
		assert.Nil(t, result, "Result should be nil on error")
		assert.Contains(t, err.Error(), "Search query cannot be empty", "Error should indicate missing query")
	})

	t.Run("Missing API credentials", func(t *testing.T) {
		// Create new server with missing credentials
		gsWithoutCreds := NewGoogleSearchServer(5, "Test-Agent", 1024*1024, "", "")

		params := map[string]interface{}{
			"query": "test",
		}

		req := mcp.CallToolRequest{
			Params: struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments,omitempty"`
				Meta      *struct {
					ProgressToken mcp.ProgressToken `json:"progressToken,omitempty"`
				} `json:"_meta,omitempty"`
			}{
				Name:      "searchGoogle",
				Arguments: params,
			},
		}

		result, err := gsWithoutCreds.handleGoogleSearch(ctx, req)

		assert.Error(t, err, "Search should error with missing API credentials")
		assert.Nil(t, result, "Result should be nil on error")
		assert.Contains(t, err.Error(), "API key is not configured", "Error should indicate missing API key")
	})
}

// Mock HTTP transport to redirect requests to our test server
type mockTransport struct {
	originalURL string
	mockURL     string
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace the host with our mock server's host
	if strings.HasPrefix(req.URL.String(), t.originalURL) {
		newURL := strings.Replace(req.URL.String(), t.originalURL, t.mockURL, 1)
		newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		if err != nil {
			return nil, err
		}

		// Copy headers
		for k, v := range req.Header {
			for _, vv := range v {
				newReq.Header.Add(k, vv)
			}
		}

		// Send the request to the mock server
		client := &http.Client{}
		return client.Do(newReq)
	}

	// For any other requests, use the default transport
	return http.DefaultTransport.RoundTrip(req)
}
