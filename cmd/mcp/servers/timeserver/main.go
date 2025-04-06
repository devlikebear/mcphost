package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	defaultTimezone string
)

// TimeServer is an MCP server that provides the current time.
type TimeServer struct {
	server          *server.MCPServer
	defaultTimezone string
}

// NewTimeServer creates a new TimeServer instance.
func NewTimeServer(defaultTimezone string) *TimeServer {
	log.Printf("TimeServer created: default timezone=%s", defaultTimezone)
	s := &TimeServer{
		defaultTimezone: defaultTimezone,
	}

	mcpServer := server.NewMCPServer(
		"time-server", // server name
		"1.0.0",       // version
	)

	// Register getCurrentTime tool
	tool := mcp.NewTool("getCurrentTime",
		mcp.WithDescription("Returns the time for the specified timezone. If a time string is provided, it converts that time; otherwise, it returns the current time."),
		mcp.WithString("timezone",
			mcp.Description("Timezone to query the time for (e.g., Asia/Seoul, UTC)"),
			mcp.Required(),
		),
		mcp.WithString("timeStr",
			mcp.Description("RFC3339 formatted time string to convert (e.g., 2025-04-06T14:30:00Z). If empty, current time is used"),
		),
	)

	mcpServer.AddTool(tool, s.handleGetCurrentTime)
	s.server = mcpServer
	return s
}

// convertTimeToTimezone converts a time to the specified timezone.
// timeStr is an RFC3339 formatted time string (e.g., "2025-04-06T14:30:00Z").
// If timeStr is empty, the current time (time.Now()) is used.
// If requestedTimezone is empty, defaultTimezone is used.
func (s *TimeServer) convertTimeToTimezone(timeStr, requestedTimezone string) (string, time.Time, error) {
	// Use default timezone if parameter is empty
	timezone := requestedTimezone
	if timezone == "" {
		timezone = s.defaultTimezone
		log.Printf("Using default timezone: %s", timezone)
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("Error: Invalid timezone: %s - %v", timezone, err)
		return "", time.Time{}, fmt.Errorf("invalid timezone: %w", err)
	}

	// Check if time string was provided
	var targetTime time.Time
	if timeStr == "" {
		// Use current time if no time string is provided
		targetTime = time.Now().In(loc)
	} else {
		// Parse the time string
		parsedTime, err := time.Parse(time.RFC3339, timeStr)
		if err != nil {
			log.Printf("Error: Invalid time format: %s - %v", timeStr, err)
			return "", time.Time{}, fmt.Errorf("invalid time format (expected RFC3339, e.g. 2025-04-06T14:30:00Z): %w", err)
		}

		// Convert parsed time to requested timezone
		targetTime = parsedTime.In(loc)
	}

	return timezone, targetTime, nil
}

// handleGetCurrentTime handles the current time request.
func (s *TimeServer) handleGetCurrentTime(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Println("Starting time request processing")
	var params struct {
		Timezone string `json:"timezone"`
		TimeStr  string `json:"timeStr,omitempty"` // Optional time string parameter
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
	log.Printf("Parameters: timezone=%s, timeStr=%s", params.Timezone, params.TimeStr)

	// Convert time based on timezone using the separated function
	timezone, now, err := s.convertTimeToTimezone(params.TimeStr, params.Timezone)
	if err != nil {
		return nil, err
	}

	// Create result message
	var resultMsg string
	if params.TimeStr == "" {
		resultMsg = fmt.Sprintf("Current time (%s): %s", timezone, now.Format(time.RFC3339))
	} else {
		resultMsg = fmt.Sprintf("Converted time (%s): %s", timezone, now.Format(time.RFC3339))
	}

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: resultMsg,
			},
		},
	}
	log.Println("Time request processing completed")
	return result, nil
}

// Server returns the MCPServer - for direct access by mcphost
func (s *TimeServer) Server() *server.MCPServer {
	return s.server
}

func init() {
	// Define flags
	flag.StringVar(&defaultTimezone, "timezone", "Asia/Seoul", "Set default timezone")
}

func main() {
	// Parse flags
	flag.Parse()

	// Set up basic logging
	log.SetPrefix("[TimeServer] ")
	log.SetFlags(log.Ldate | log.Ltime)

	// Set default timezone
	log.Printf("Starting time server: default timezone=%s", defaultTimezone)

	// Create TimeServer instance with default timezone
	timeServer := NewTimeServer(defaultTimezone)
	log.Println("TimeServer instance created successfully, starting server...")

	// Access mcpServer instance using timeServer.Server()
	if err := server.ServeStdio(timeServer.Server()); err != nil {
		log.Printf("Error: Server execution failed: %v", err)
		os.Exit(1)
	}

	log.Println("TimeServer shutdown")
}
