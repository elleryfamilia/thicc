package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ellery/thicc/internal/llmhistory"
)

// Protocol version
const ProtocolVersion = "2024-11-05"

// Message types
const (
	TypeRequest      = "request"
	TypeResponse     = "response"
	TypeNotification = "notification"
)

// Request represents an MCP JSON-RPC request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard error codes
const (
	ErrCodeParse          = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
)

// Server implements the MCP protocol over stdio
type Server struct {
	store   *llmhistory.Store
	tools   *Tools
	input   io.Reader
	output  io.Writer
	running bool
}

// NewServer creates a new MCP server
func NewServer(store *llmhistory.Store) *Server {
	return &Server{
		store:  store,
		tools:  NewTools(store),
		input:  os.Stdin,
		output: os.Stdout,
	}
}

// Run starts the server, reading from stdin and writing to stdout
func (s *Server) Run() error {
	s.running = true
	scanner := bufio.NewScanner(s.input)

	// Increase buffer size for large messages
	const maxScanTokenSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxScanTokenSize)

	log.Println("MCP: Server started, waiting for requests...")

	for scanner.Scan() && s.running {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse request
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, ErrCodeParse, "Parse error", err.Error())
			continue
		}

		// Handle request
		s.handleRequest(&req)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// handleRequest dispatches a request to the appropriate handler
func (s *Server) handleRequest(req *Request) {
	log.Printf("MCP: Received request: %s", req.Method)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed
		log.Println("MCP: Client initialized")
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(req)
	case "shutdown":
		s.handleShutdown(req)
	default:
		s.sendError(req.ID, ErrCodeMethodNotFound, "Method not found", req.Method)
	}
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize(req *Request) {
	result := map[string]interface{}{
		"protocolVersion": ProtocolVersion,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "llm-history",
			"version": "0.1.0",
		},
	}
	s.sendResult(req.ID, result)
}

// handleToolsList handles the tools/list request
func (s *Server) handleToolsList(req *Request) {
	tools := s.tools.List()
	result := map[string]interface{}{
		"tools": tools,
	}
	s.sendResult(req.ID, result)
}

// ToolCallParams represents the params for tools/call
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// handleToolsCall handles the tools/call request
func (s *Server) handleToolsCall(req *Request) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, ErrCodeInvalidParams, "Invalid params", err.Error())
		return
	}

	result, err := s.tools.Call(params.Name, params.Arguments)
	if err != nil {
		s.sendError(req.ID, ErrCodeInternal, "Tool error", err.Error())
		return
	}

	// MCP tools/call expects content array
	response := map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": result,
			},
		},
	}
	s.sendResult(req.ID, response)
}

// handleShutdown handles the shutdown request
func (s *Server) handleShutdown(req *Request) {
	s.running = false
	s.sendResult(req.ID, nil)
}

// sendResult sends a successful response
func (s *Server) sendResult(id interface{}, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.send(resp)
}

// sendError sends an error response
func (s *Server) sendError(id interface{}, code int, message, data string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.send(resp)
}

// send writes a response to stdout
func (s *Server) send(resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("MCP: Failed to marshal response: %v", err)
		return
	}
	fmt.Fprintln(s.output, string(data))
}
