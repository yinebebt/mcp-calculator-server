package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

// Server represents an MCP server
type Server struct {
	tools        map[string]Tool
	toolHandlers map[string]ToolHandler
	initialized  bool
	capabilities ServerCapabilities
	serverInfo   ServerInfo
}

// ToolHandler represents a function that handles tool calls
type ToolHandler func(args map[string]interface{}) (*CallToolResponse, error)

// NewServer creates a new MCP server
func NewServer(name, version string) *Server {
	return &Server{
		tools:        make(map[string]Tool),
		toolHandlers: make(map[string]ToolHandler),
		capabilities: ServerCapabilities{
			Tools: &ToolsCapability{
				ListChanged: false,
			},
		},
		serverInfo: ServerInfo{
			Name:    name,
			Version: version,
		},
	}
}

// RegisterTool registers a tool with the server
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.tools[tool.Name] = tool
	s.toolHandlers[tool.Name] = handler
}

// HandleRequest processes a JSON-RPC request and returns a response
func (s *Server) HandleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    MethodNotFound,
				Message: fmt.Sprintf("Unknown method: %s", req.Method),
			},
		}
	}
}

func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	var initReq InitializeRequest
	if err := json.Unmarshal(req.Params, &initReq); err != nil {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Invalid initialize parameters",
				Data:    err.Error(),
			},
		}
	}

	// Validate protocol version
	if initReq.ProtocolVersion != MCPVersion {
		log.Printf("Warning: Client requested protocol version %s, server supports %s",
			initReq.ProtocolVersion, MCPVersion)
	}

	s.initialized = true

	response := InitializeResponse{
		ProtocolVersion: MCPVersion,
		Capabilities:    s.capabilities,
		ServerInfo:      s.serverInfo,
	}

	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  response,
	}
}

func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	if !s.initialized {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    InternalError,
				Message: "Server not initialized",
			},
		}
	}

	tools := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}

	response := ListToolsResponse{
		Tools: tools,
	}

	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  response,
	}
}

func (s *Server) handleToolsCall(req *JSONRPCRequest) *JSONRPCResponse {
	if !s.initialized {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    InternalError,
				Message: "Server not initialized",
			},
		}
	}

	var callReq CallToolRequest
	if err := json.Unmarshal(req.Params, &callReq); err != nil {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    InvalidParams,
				Message: "Invalid tool call parameters",
				Data:    err.Error(),
			},
		}
	}

	handler, exists := s.toolHandlers[callReq.Name]
	if !exists {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    MethodNotFound,
				Message: fmt.Sprintf("Unknown tool: %s", callReq.Name),
			},
		}
	}

	result, err := handler(callReq.Arguments)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Error: &RPCError{
				Code:    InternalError,
				Message: "Tool execution failed",
				Data:    err.Error(),
			},
		}
	}

	return &JSONRPCResponse{
		JSONRPC: JSONRPCVersion,
		ID:      req.ID,
		Result:  result,
	}
}

// ServeStdio runs the server in stdio mode (for MCP clients)
func (s *Server) ServeStdio() error {
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			response := &JSONRPCResponse{
				JSONRPC: JSONRPCVersion,
				ID:      nil,
				Error: &RPCError{
					Code:    ParseError,
					Message: "Parse error",
					Data:    err.Error(),
				},
			}
			if err := encoder.Encode(response); err != nil {
				log.Printf("Failed to encode error response: %v", err)
			}
			continue
		}

		response := s.HandleRequest(&req)
		if err := encoder.Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stdin scan error: %w", err)
	}

	return nil
}
