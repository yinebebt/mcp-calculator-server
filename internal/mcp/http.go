package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// ServeHTTP runs the server in HTTP mode (for testing)
func (s *Server) ServeHTTP(port int) error {
	mux := http.NewServeMux()

	// MCP endpoints
	mux.HandleFunc("/mcp/tools/list", s.httpToolsList)
	mux.HandleFunc("/mcp/tools/call", s.httpToolsCall)

	// Metadata endpoints
	mux.HandleFunc("/mcp/.well-known/ai-plugin.json", s.httpAIPlugin)
	mux.HandleFunc("/mcp/openapi.json", s.httpOpenAPI)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("MCP Calculator Server starting on http://localhost%s", addr)
	log.Printf("Test endpoints:")
	log.Printf("  GET  http://localhost%s/health", addr)
	log.Printf("  GET  http://localhost%s/mcp/.well-known/ai-plugin.json", addr)
	log.Printf("  GET  http://localhost%s/mcp/openapi.json", addr)
	log.Printf("  POST http://localhost%s/mcp/tools/list", addr)
	log.Printf("  POST http://localhost%s/mcp/tools/call", addr)
	log.Printf("  POST http://localhost%s/mcp/tools/calculate/invoke", addr)

	return http.ListenAndServe(addr, mux)
}

func (s *Server) httpToolsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Auto-initialize if not already done
	if !s.initialized {
		s.initialized = true
	}

	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/list",
	}

	response := s.HandleRequest(req)
	s.writeJSONResponse(w, response)
}

func (s *Server) httpToolsCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Auto-initialize if not already done
	if !s.initialized {
		s.initialized = true
	}

	var callReq CallToolRequest
	if err := json.NewDecoder(r.Body).Decode(&callReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	params, _ := json.Marshal(callReq)
	req := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  params,
	}

	response := s.HandleRequest(req)
	s.writeJSONResponse(w, response)
}

func (s *Server) httpAIPlugin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	plugin := map[string]interface{}{
		"schema_version":        "v1",
		"name_for_model":        "calculator",
		"name_for_human":        "Calculator",
		"description_for_model": "A calculator that can perform basic arithmetic operations",
		"description_for_human": "Perform basic math operations like add, subtract, multiply, divide",
		"auth": map[string]string{
			"type": "none",
		},
		"api": map[string]string{
			"type": "openapi",
			"url":  "/mcp/openapi.json",
		},
		"logo_url":       "",
		"contact_email":  "",
		"legal_info_url": "",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plugin)
}

func (s *Server) httpOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	openapi := map[string]interface{}{
		"openapi": "3.0.1",
		"info": map[string]interface{}{
			"title":       "MCP Calculator Server",
			"description": "A calculator server implementing the Model Context Protocol",
			"version":     s.serverInfo.Version,
		},
		"servers": []map[string]string{
			{"url": "http://localhost:7000"},
		},
		"paths": map[string]interface{}{
			"/mcp/tools/calculate/invoke": map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     "Perform a calculation",
					"description": "Execute a calculator operation",
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"operation": map[string]interface{}{
											"type": "string",
											"enum": []string{"add", "subtract", "multiply", "divide"},
										},
										"a": map[string]interface{}{
											"type": "number",
										},
										"b": map[string]interface{}{
											"type": "number",
										},
									},
									"required": []string{"operation", "a", "b"},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Calculation result",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(openapi)
}

func (s *Server) writeJSONResponse(w http.ResponseWriter, response *JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")

	// Return appropriate HTTP status code based on JSON-RPC response
	if response.Error != nil {
		switch response.Error.Code {
		case ParseError, InvalidRequest, InvalidParams:
			w.WriteHeader(http.StatusBadRequest)
		case MethodNotFound:
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	json.NewEncoder(w).Encode(response)
}
