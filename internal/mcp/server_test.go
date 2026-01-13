package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ellery/thicc/internal/llmhistory"
)

// testServer creates a test MCP server with a temporary store
func testServer(t *testing.T) (*Server, *llmhistory.Store, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "mcp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := llmhistory.NewStore(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	server := NewServer(store)

	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return server, store, cleanup
}

// sendRequest sends a request to the server and returns the response
func sendRequest(t *testing.T, server *Server, method string, params interface{}) Response {
	t.Helper()

	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("failed to marshal params: %v", err)
		}
		paramsJSON = data
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  paramsJSON,
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Set up input/output buffers
	input := bytes.NewReader(append(reqData, '\n'))
	output := &bytes.Buffer{}

	server.input = input
	server.output = output

	// Process single request
	server.running = true
	go func() {
		// Stop after one request
		time.Sleep(10 * time.Millisecond)
		server.running = false
	}()
	server.Run()

	// Parse response
	var resp Response
	if err := json.NewDecoder(output).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v\nOutput: %s", err, output.String())
	}

	return resp
}

func TestInitialize(t *testing.T) {
	server, _, cleanup := testServer(t)
	defer cleanup()

	resp := sendRequest(t, server, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0",
		},
	})

	if resp.Error != nil {
		t.Fatalf("initialize failed: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", resp.Result)
	}

	if result["protocolVersion"] != ProtocolVersion {
		t.Errorf("unexpected protocol version: %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing serverInfo")
	}

	if serverInfo["name"] != "llm-history" {
		t.Errorf("unexpected server name: %v", serverInfo["name"])
	}
}

func TestToolsList(t *testing.T) {
	server, _, cleanup := testServer(t)
	defer cleanup()

	resp := sendRequest(t, server, "tools/list", nil)

	if resp.Error != nil {
		t.Fatalf("tools/list failed: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", resp.Result)
	}

	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("missing tools array")
	}

	if len(tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(tools))
	}

	// Verify tool names
	expectedTools := map[string]bool{
		"search_history":   false,
		"list_sessions":    false,
		"get_session":      false,
		"get_file_history": false,
	}

	for _, tool := range tools {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := toolMap["name"].(string)
		expectedTools[name] = true
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("tool %s not found in list", name)
		}
	}
}

func TestToolsCallListSessions(t *testing.T) {
	server, store, cleanup := testServer(t)
	defer cleanup()

	// Create test session
	session := &llmhistory.Session{
		ID:         "test-session-001",
		ToolName:   "claude",
		ProjectDir: "/test/project",
		StartTime:  time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	resp := sendRequest(t, server, "tools/call", map[string]interface{}{
		"name":      "list_sessions",
		"arguments": map[string]interface{}{"limit": 10},
	})

	if resp.Error != nil {
		t.Fatalf("tools/call failed: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", resp.Result)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("missing content array")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content item type")
	}

	text, _ := firstContent["text"].(string)
	if !strings.Contains(text, "test-ses") { // ID is truncated to 8 chars
		t.Errorf("response doesn't contain session ID: %s", text)
	}
}

func TestToolsCallGetSession(t *testing.T) {
	server, store, cleanup := testServer(t)
	defer cleanup()

	// Create test session with tool uses
	session := &llmhistory.Session{
		ID:          "get-session-test",
		ToolName:    "aider",
		ToolCommand: "aider --model gpt-4",
		ProjectDir:  "/my/project",
		StartTime:   time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	tu := &llmhistory.ToolUse{
		ID:        "tu-001",
		SessionID: session.ID,
		Timestamp: time.Now(),
		ToolName:  "Edit",
		Input:     "main.go",
		Output:    "file edited",
	}
	if err := store.CreateToolUse(tu); err != nil {
		t.Fatalf("failed to create tool use: %v", err)
	}

	resp := sendRequest(t, server, "tools/call", map[string]interface{}{
		"name":      "get_session",
		"arguments": map[string]interface{}{"session_id": session.ID},
	})

	if resp.Error != nil {
		t.Fatalf("tools/call failed: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", resp.Result)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("missing content array")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content item type")
	}

	text, _ := firstContent["text"].(string)

	// Verify session info is in response
	if !strings.Contains(text, "aider") {
		t.Errorf("response doesn't contain tool name")
	}
	if !strings.Contains(text, "/my/project") {
		t.Errorf("response doesn't contain project dir")
	}
	if !strings.Contains(text, "Edit") {
		t.Errorf("response doesn't contain tool use")
	}
}

func TestToolsCallSearchHistory(t *testing.T) {
	server, store, cleanup := testServer(t)
	defer cleanup()

	// Create session and tool use with searchable content
	session := &llmhistory.Session{
		ID:        "search-test-session",
		ToolName:  "claude",
		StartTime: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	tu := &llmhistory.ToolUse{
		ID:        "search-tu",
		SessionID: session.ID,
		Timestamp: time.Now(),
		ToolName:  "Bash",
		Input:     "npm install typescript",
		Output:    "installed successfully",
	}
	if err := store.CreateToolUse(tu); err != nil {
		t.Fatalf("failed to create tool use: %v", err)
	}

	resp := sendRequest(t, server, "tools/call", map[string]interface{}{
		"name":      "search_history",
		"arguments": map[string]interface{}{"query": "typescript"},
	})

	if resp.Error != nil {
		t.Fatalf("tools/call failed: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", resp.Result)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("missing content array")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content item type")
	}

	text, _ := firstContent["text"].(string)
	if !strings.Contains(text, "typescript") && !strings.Contains(text, "Bash") {
		t.Errorf("response doesn't contain search results: %s", text)
	}
}

func TestToolsCallGetFileHistory(t *testing.T) {
	server, store, cleanup := testServer(t)
	defer cleanup()

	// Create session, tool use, and file touch
	session := &llmhistory.Session{
		ID:        "file-history-session",
		ToolName:  "claude",
		StartTime: time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	tu := &llmhistory.ToolUse{
		ID:        "file-tu",
		SessionID: session.ID,
		Timestamp: time.Now(),
		ToolName:  "Read",
		Input:     "/app/server.go",
		Output:    "file contents here",
	}
	if err := store.CreateToolUse(tu); err != nil {
		t.Fatalf("failed to create tool use: %v", err)
	}

	if err := store.CreateFileTouch("file-tu", "/app/server.go"); err != nil {
		t.Fatalf("failed to create file touch: %v", err)
	}

	resp := sendRequest(t, server, "tools/call", map[string]interface{}{
		"name": "get_file_history",
		"arguments": map[string]interface{}{
			"files": []string{"/app/server.go"},
		},
	})

	if resp.Error != nil {
		t.Fatalf("tools/call failed: %s", resp.Error.Message)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type: %T", resp.Result)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("missing content array")
	}

	firstContent, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected content item type")
	}

	text, _ := firstContent["text"].(string)
	if !strings.Contains(text, "server.go") {
		t.Errorf("response doesn't contain file path: %s", text)
	}
}

func TestMethodNotFound(t *testing.T) {
	server, _, cleanup := testServer(t)
	defer cleanup()

	resp := sendRequest(t, server, "nonexistent/method", nil)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}

	if resp.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("expected error code %d, got %d", ErrCodeMethodNotFound, resp.Error.Code)
	}
}

func TestInvalidToolName(t *testing.T) {
	server, _, cleanup := testServer(t)
	defer cleanup()

	resp := sendRequest(t, server, "tools/call", map[string]interface{}{
		"name":      "nonexistent_tool",
		"arguments": map[string]interface{}{},
	})

	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}

	if resp.Error.Code != ErrCodeInternal {
		t.Errorf("expected error code %d, got %d", ErrCodeInternal, resp.Error.Code)
	}
}

func TestMissingRequiredParam(t *testing.T) {
	server, _, cleanup := testServer(t)
	defer cleanup()

	// search_history requires "query" parameter
	resp := sendRequest(t, server, "tools/call", map[string]interface{}{
		"name":      "search_history",
		"arguments": map[string]interface{}{},
	})

	if resp.Error == nil {
		t.Fatal("expected error for missing required param")
	}
}
