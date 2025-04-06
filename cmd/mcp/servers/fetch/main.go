package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	timeout     int
	userAgent   string
	maxBodySize int64
)

// FetchServer is an MCP server that performs HTTP/HTTPS requests.
type FetchServer struct {
	server      *server.MCPServer
	client      *http.Client
	userAgent   string
	maxBodySize int64
}

// NewFetchServer creates a new FetchServer instance.
func NewFetchServer(timeout int, userAgent string, maxBodySize int64) *FetchServer {
	log.Printf("FetchServer created: timeout=%ds, userAgent=%s, maxBodySize=%d", timeout, userAgent, maxBodySize)

	// Create HTTP client with configured timeout
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	s := &FetchServer{
		client:      client,
		userAgent:   userAgent,
		maxBodySize: maxBodySize,
	}

	mcpServer := server.NewMCPServer(
		"fetch-server", // server name
		"1.0.0",        // version
	)

	// Register fetchURL tool
	tool := mcp.NewTool("fetchURL",
		mcp.WithDescription("Fetches data from a URL using HTTP/HTTPS. Supports GET, POST, PUT, DELETE, PATCH methods."),
		mcp.WithString("url",
			mcp.Description("The URL to fetch data from (must be a valid HTTP/HTTPS URL)"),
			mcp.Required(),
		),
		mcp.WithString("method",
			mcp.Description("HTTP method to use (GET, POST, PUT, DELETE, PATCH). Defaults to GET if not specified."),
		),
		mcp.WithString("body",
			mcp.Description("Request body for POST, PUT, PATCH requests"),
		),
		mcp.WithString("contentType",
			mcp.Description("Content-Type header for the request. For POST requests with a body, defaults to application/json"),
		),
		mcp.WithString("headers",
			mcp.Description("JSON string containing additional headers to send with the request"),
		),
	)

	mcpServer.AddTool(tool, s.handleFetchURL)
	s.server = mcpServer
	return s
}

// handleFetchURL handles the URL fetch request.
func (s *FetchServer) handleFetchURL(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Println("Starting fetch request processing")

	var params struct {
		URL         string `json:"url"`
		Method      string `json:"method,omitempty"`
		Body        string `json:"body,omitempty"`
		ContentType string `json:"contentType,omitempty"`
		Headers     string `json:"headers,omitempty"`
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

	// Log request details (without sensitive information)
	log.Printf("Fetch request: URL=%s, Method=%s", params.URL, params.Method)

	// Validate URL
	if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
		errMsg := "URL must begin with http:// or https://"
		log.Printf("Error: %s", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Use GET as default method if not specified
	method := params.Method
	if method == "" {
		method = "GET"
	}

	// Create request
	var reqBody io.Reader
	if params.Body != "" {
		reqBody = strings.NewReader(params.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, params.URL, reqBody)
	if err != nil {
		log.Printf("Error: Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent
	httpReq.Header.Set("User-Agent", s.userAgent)

	// Set Content-Type if provided
	if params.ContentType != "" {
		httpReq.Header.Set("Content-Type", params.ContentType)
	} else if params.Body != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		// Default to application/json for POST/PUT/PATCH with body
		httpReq.Header.Set("Content-Type", "application/json")
	}

	// Add custom headers if provided
	if params.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(params.Headers), &headers); err != nil {
			log.Printf("Error: Invalid headers JSON: %v", err)
			return nil, fmt.Errorf("invalid headers JSON: %w", err)
		}

		for key, value := range headers {
			httpReq.Header.Set(key, value)
		}
	}

	// Send the request
	log.Printf("Sending %s request to %s", method, params.URL)
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

	// Prepare headers response
	headerMap := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			headerMap[key] = values[0]
		}
	}

	// Create response structure
	responseDetails := struct {
		StatusCode int               `json:"status_code"`
		Headers    map[string]string `json:"headers"`
		Body       string            `json:"body"`
		URL        string            `json:"url"`
		Method     string            `json:"method"`
	}{
		StatusCode: resp.StatusCode,
		Headers:    headerMap,
		Body:       string(body),
		URL:        params.URL,
		Method:     method,
	}

	// Marshal response to JSON
	responseJSON, err := json.MarshalIndent(responseDetails, "", "  ")
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return nil, fmt.Errorf("error marshaling response: %w", err)
	}

	// Create result message
	resultMsg := fmt.Sprintf("Response from %s (status: %d):\n%s", params.URL, resp.StatusCode, string(responseJSON))

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: resultMsg,
			},
		},
	}

	log.Printf("Fetch request completed with status: %d", resp.StatusCode)
	return result, nil
}

// Server returns the MCPServer - for direct access by mcphost
func (s *FetchServer) Server() *server.MCPServer {
	return s.server
}

func init() {
	// Define flags
	flag.IntVar(&timeout, "timeout", 30, "HTTP request timeout in seconds")
	flag.StringVar(&userAgent, "user-agent", "MCP-Fetch-Server/1.0", "User-Agent header for requests")
	flag.Int64Var(&maxBodySize, "max-body-size", 10*1024*1024, "Maximum response body size in bytes (default 10MB)")
}

func main() {
	// Parse flags
	flag.Parse()

	// Set up basic logging
	log.SetPrefix("[FetchServer] ")
	log.SetFlags(log.Ldate | log.Ltime)

	log.Printf("Starting fetch server: timeout=%ds, user-agent=%s, max-body-size=%d", timeout, userAgent, maxBodySize)

	// Create FetchServer instance
	fetchServer := NewFetchServer(timeout, userAgent, maxBodySize)
	log.Println("FetchServer instance created successfully, starting server...")

	// Access mcpServer instance using fetchServer.Server()
	if err := server.ServeStdio(fetchServer.Server()); err != nil {
		log.Printf("Error: Server execution failed: %v", err)
		os.Exit(1)
	}

	log.Println("FetchServer shutdown")
}
