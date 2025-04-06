package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	timeout        int
	userAgent      string
	maxBodySize    int64
	apiKey         string
	searchEngineID string
)

// GoogleSearchResult represents a search result from the Google API
type GoogleSearchResult struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Snippet     string `json:"snippet"`
	DisplayLink string `json:"displayLink,omitempty"`
}

// GoogleApiResponse represents the response from Google Custom Search API
type GoogleApiResponse struct {
	Kind string `json:"kind"`
	URL  *struct {
		Type     string `json:"type"`
		Template string `json:"template"`
	} `json:"url,omitempty"`
	Queries           map[string]interface{} `json:"queries"`
	Context           interface{}            `json:"context,omitempty"`
	SearchInformation *struct {
		SearchTime            float64 `json:"searchTime"`
		FormattedSearchTime   string  `json:"formattedSearchTime"`
		TotalResults          string  `json:"totalResults"`
		FormattedTotalResults string  `json:"formattedTotalResults"`
	} `json:"searchInformation,omitempty"`
	Items []struct {
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
	} `json:"items,omitempty"`
}

// GoogleSearchServer is an MCP server that performs Google searches.
type GoogleSearchServer struct {
	server         *server.MCPServer
	client         *http.Client
	userAgent      string
	maxBodySize    int64
	apiKey         string
	searchEngineID string
}

// NewGoogleSearchServer creates a new GoogleSearchServer instance.
func NewGoogleSearchServer(timeout int, userAgent string, maxBodySize int64, apiKey, searchEngineID string) *GoogleSearchServer {
	log.Printf("GoogleSearchServer created: timeout=%ds, userAgent=%s, maxBodySize=%d", timeout, userAgent, maxBodySize)

	// Create HTTP client with configured timeout
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	s := &GoogleSearchServer{
		client:         client,
		userAgent:      userAgent,
		maxBodySize:    maxBodySize,
		apiKey:         apiKey,
		searchEngineID: searchEngineID,
	}

	mcpServer := server.NewMCPServer(
		"google-search-server", // server name
		"1.0.0",                // version
	)

	// Register searchGoogle tool
	searchTool := mcp.NewTool("searchGoogle",
		mcp.WithDescription("Performs a Google search and returns the results"),
		mcp.WithString("query",
			mcp.Description("The search query string"),
			mcp.Required(),
		),
		mcp.WithNumber("num",
			mcp.Description("Number of search results to return (max 10)"),
			mcp.DefaultNumber(5),
		),
		mcp.WithNumber("start",
			mcp.Description("Index of the first result to return (starts at 1)"),
			mcp.DefaultNumber(1),
		),
		mcp.WithString("language",
			mcp.Description("Language for search results (e.g., 'en', 'ko', 'ja')"),
			mcp.DefaultString("en"),
		),
		mcp.WithString("country",
			mcp.Description("Country code for search context (e.g., 'us', 'kr', 'jp')"),
			mcp.DefaultString("us"),
		),
		mcp.WithBoolean("safeSearch",
			mcp.Description("Whether to filter out adult content"),
			mcp.DefaultBool(true),
		),
	)

	// Register getApiStatus tool to check and validate API configuration
	statusTool := mcp.NewTool("getApiStatus",
		mcp.WithDescription("Checks if the Google API configuration is valid"),
	)

	mcpServer.AddTool(searchTool, s.handleGoogleSearch)
	mcpServer.AddTool(statusTool, s.handleApiStatus)

	s.server = mcpServer
	return s
}

// handleApiStatus checks if the API configuration is valid
func (s *GoogleSearchServer) handleApiStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Println("Checking API configuration")

	var statusMsg string
	if s.apiKey == "" {
		statusMsg = "Error: API key is not configured. Please set the API_KEY environment variable or use the -api-key flag."
	} else if s.searchEngineID == "" {
		statusMsg = "Error: Search Engine ID is not configured. Please set the SEARCH_ENGINE_ID environment variable or use the -search-engine-id flag."
	} else {
		statusMsg = "Google Search API is properly configured and ready to use."
	}

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: statusMsg,
			},
		},
	}

	return result, nil
}

// handleGoogleSearch handles the Google search request.
func (s *GoogleSearchServer) handleGoogleSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Println("Starting Google search request processing")

	// Validate API configuration
	if s.apiKey == "" {
		return nil, fmt.Errorf("API key is not configured")
	}
	if s.searchEngineID == "" {
		return nil, fmt.Errorf("Search Engine ID is not configured")
	}

	var params struct {
		Query      string  `json:"query"`
		Num        float64 `json:"num,omitempty"`
		Start      float64 `json:"start,omitempty"`
		Language   string  `json:"language,omitempty"`
		Country    string  `json:"country,omitempty"`
		SafeSearch bool    `json:"safeSearch,omitempty"`
	}

	args, err := json.Marshal(req.Params.Arguments)
	if err != nil {
		log.Printf("Error: Failed to marshal arguments: %v", err)
		return nil, fmt.Errorf("failed to marshal arguments: %w", err)
	}

	if err := json.Unmarshal(args, &params); err != nil {
		log.Printf("Error: Invalid parameters: %v", err)
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Log request details
	log.Printf("Google search request: Query=%s, Num=%v, Start=%v, Language=%s, Country=%s, SafeSearch=%v",
		params.Query, params.Num, params.Start, params.Language, params.Country, params.SafeSearch)

	// Validate query
	if params.Query == "" {
		errMsg := "Search query cannot be empty"
		log.Printf("Error: %s", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Set defaults if not provided
	if params.Num <= 0 {
		params.Num = 5
	} else if params.Num > 10 {
		params.Num = 10 // Google API limit is 10 results per page
	}

	if params.Start <= 0 {
		params.Start = 1
	}

	// Construct Google Custom Search API URL
	baseURL := "https://www.googleapis.com/customsearch/v1"
	values := url.Values{}
	values.Add("q", params.Query)
	values.Add("key", s.apiKey)
	values.Add("cx", s.searchEngineID)
	values.Add("num", strconv.Itoa(int(params.Num)))
	values.Add("start", strconv.Itoa(int(params.Start)))

	if params.Language != "" {
		values.Add("lr", "lang_"+params.Language)
	}

	if params.Country != "" {
		values.Add("gl", params.Country)
	}

	if params.SafeSearch {
		values.Add("safe", "active")
	} else {
		values.Add("safe", "off")
	}

	searchURL := baseURL + "?" + values.Encode()

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		log.Printf("Error: Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent
	httpReq.Header.Set("User-Agent", s.userAgent)
	httpReq.Header.Set("Accept", "application/json")

	// Send the request
	log.Printf("Sending request to Google Custom Search API")
	resp, err := s.client.Do(httpReq)
	if err != nil {
		log.Printf("Error: Request failed: %v", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response (with size limitation)
	body, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBodySize))
	if err != nil {
		log.Printf("Error: Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error: API returned status code %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API error: %s (status code: %d)", string(body), resp.StatusCode)
	}

	// Parse API response
	var apiResponse GoogleApiResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		log.Printf("Error: Failed to parse API response: %v", err)
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	// Extract search results
	var results []GoogleSearchResult
	if apiResponse.Items != nil {
		for _, item := range apiResponse.Items {
			results = append(results, GoogleSearchResult{
				Title:       item.Title,
				Link:        item.Link,
				Snippet:     item.Snippet,
				DisplayLink: item.DisplayLink,
			})
		}
	}

	// Create summary information
	var totalResults string
	var searchTime string
	if apiResponse.SearchInformation != nil {
		totalResults = apiResponse.SearchInformation.FormattedTotalResults
		searchTime = apiResponse.SearchInformation.FormattedSearchTime
	}

	// Generate response content
	var resultContent strings.Builder
	resultContent.WriteString(fmt.Sprintf("Google Search Results for: %s\n\n", params.Query))
	resultContent.WriteString(fmt.Sprintf("Found approximately %s results in %s seconds\n\n", totalResults, searchTime))

	if len(results) == 0 {
		resultContent.WriteString("No results found.")
	} else {
		for i, result := range results {
			resultContent.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
			resultContent.WriteString(fmt.Sprintf("   URL: %s\n", result.Link))
			resultContent.WriteString(fmt.Sprintf("   %s\n\n", result.Snippet))
		}
	}

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: resultContent.String(),
			},
		},
	}

	log.Println("Google search request completed successfully")
	return result, nil
}

// Server returns the MCPServer - for direct access by mcphost
func (s *GoogleSearchServer) Server() *server.MCPServer {
	return s.server
}

func init() {
	// Define flags
	flag.IntVar(&timeout, "timeout", 30, "HTTP request timeout in seconds")
	flag.StringVar(&userAgent, "user-agent", "MCP-GoogleSearch-Server/1.0", "User-Agent header for requests")
	flag.Int64Var(&maxBodySize, "max-body-size", 10*1024*1024, "Maximum response body size in bytes (default 10MB)")
	flag.StringVar(&apiKey, "api-key", "", "Google Custom Search API key")
	flag.StringVar(&searchEngineID, "search-engine-id", "", "Google Custom Search Engine ID")
}

func main() {
	// Parse flags
	flag.Parse()

	// Set up basic logging
	log.SetPrefix("[GoogleSearchServer] ")
	log.SetFlags(log.Ldate | log.Ltime)

	// Check for environment variables if flags not provided
	if apiKey == "" {
		apiKey = os.Getenv("API_KEY")
	}
	if searchEngineID == "" {
		searchEngineID = os.Getenv("SEARCH_ENGINE_ID")
	}

	log.Printf("Starting Google search server: timeout=%ds, user-agent=%s", timeout, userAgent)
	if apiKey == "" || searchEngineID == "" {
		log.Printf("Warning: API key or Search Engine ID not configured. The server will start but searches will fail.")
	}

	// Create GoogleSearchServer instance
	searchServer := NewGoogleSearchServer(timeout, userAgent, maxBodySize, apiKey, searchEngineID)
	log.Println("GoogleSearchServer instance created successfully, starting server...")

	// Access mcpServer instance using searchServer.Server()
	if err := server.ServeStdio(searchServer.Server()); err != nil {
		log.Printf("Error: Server execution failed: %v", err)
		os.Exit(1)
	}

	log.Println("GoogleSearchServer shutdown")
}
