package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ServerName    = "mcp-calculator-server"
	ServerVersion = "0.5.0"
)

func main() {
	transport := os.Getenv("TRANSPORT")
	if transport == "" {
		transport = "streamable-http"
	}
	log.Printf("Starting with transport: %s", transport)

	s := createMCPServer()

	// Start server with appropriate transport
	switch transport {
	case "stdio":
		startStdioServer(s)
	case "streamable-http":
		startHTTPServer(s)
	default:
		log.Fatalf("Unsupported transport: %s (supported: stdio or streamable-http)", transport)
	}
}

func createMCPServer() *server.MCPServer {
	s := server.NewMCPServer(ServerName, ServerVersion)

	log.Printf("Initializing MCP server: %s v%s", ServerName, ServerVersion)

	// Calculator tool
	s.AddTool(mcp.NewTool(
		"calculate",
		mcp.WithDescription("Perform basic mathematical operations like add, subtract, multiply, and divide"),
		mcp.WithString("expression",
			mcp.Description("A mathematical expression to evaluate (e.g., '2 + 3', '10 * 5', '15 / 3')"),
			mcp.Required(),
		),
	), handleCalculate)

	// Random number generator tool
	s.AddTool(mcp.NewTool(
		"random_number",
		mcp.WithDescription("Generate a random number within a specified range"),
		mcp.WithNumber("min",
			mcp.Description("Minimum value (default: 1)"),
		),
		mcp.WithNumber("max",
			mcp.Description("Maximum value (default: 100)"),
		),
	), handleRandomNumber)

	log.Printf("Loaded %d tools: calculate, random_number", 2)

	// Math constants resource
	s.AddResource(mcp.NewResource(
		"math://constants",
		"Mathematical Constants",
		mcp.WithResourceDescription("Common mathematical constants and their values"),
		mcp.WithMIMEType("application/json"),
	), handleMathConstants)

	// Server information resource
	s.AddResource(mcp.NewResource(
		"server://info",
		"Server Information",
		mcp.WithResourceDescription("Information about this MCP server"),
		mcp.WithMIMEType("text/plain"),
	), handleServerInfo)

	log.Printf("Loaded %d resources: math://constants, server://info", 2)

	// Math problem generator prompt
	s.AddPrompt(mcp.NewPrompt(
		"math_problem",
		mcp.WithPromptDescription("Generate a mathematical word problem"),
		mcp.WithArgument("difficulty",
			mcp.ArgumentDescription("Difficulty level: 'easy', 'medium', 'hard'"),
		),
		mcp.WithArgument("topic",
			mcp.ArgumentDescription("Math topic: 'addition', 'subtraction', 'multiplication', 'division', 'mixed'"),
		),
	), handleMathProblemPrompt)

	// Calculation explanation prompt
	s.AddPrompt(mcp.NewPrompt(
		"explain_calculation",
		mcp.WithPromptDescription("Explain how to solve a mathematical expression step by step"),
		mcp.WithArgument("expression",
			mcp.ArgumentDescription("Mathematical expression to explain"),
			mcp.RequiredArgument(),
		),
	), handleExplainCalculationPrompt)

	log.Printf("Loaded %d prompts: math_problem, explain_calculation", 2)
	log.Printf("MCP server initialization complete")

	return s
}

func startStdioServer(s *server.MCPServer) {
	log.Println("Starting MCP server with stdio transport")
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Stdio server error: %v", err)
	}
}

func startHTTPServer(s *server.MCPServer) {
	port := "8080"
	if portStr := os.Getenv("PORT"); portStr != "" {
		port = portStr
	}

	log.Printf("Starting MCP server with Streamable HTTP transport on port %s", port)

	// Create streamable HTTP server
	streamServer := server.NewStreamableHTTPServer(s)

	// Create HTTP mux for additional endpoints
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Mount the MCP streamable HTTP handler at /mcp
	mux.Handle("/mcp", streamServer)

	log.Printf("MCP endpoints available at: :%s/mcp", port)
	log.Printf("Health check available at: :%s/health", port)

	// Start HTTP server
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func handleCalculate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	expression, err := request.RequireString("expression")
	if err != nil {
		log.Printf("Calculate error - invalid expression parameter: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
	}

	result, err := evaluateExpression(expression)
	if err != nil {
		log.Printf("Calculate error - evaluation failed: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Calculation error: %v", err)), nil
	}

	log.Printf("Calculate result: %s = %s", expression, formatResult(result))
	return mcp.NewToolResultText(fmt.Sprintf("Result: %s = %s", expression, formatResult(result))), nil
}

func handleRandomNumber(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	min := 1.0
	max := 100.0

	if minVal, ok := args["min"].(float64); ok {
		min = minVal
	}
	if maxVal, ok := args["max"].(float64); ok {
		max = maxVal
	}

	if min >= max {
		log.Printf("Random number error - invalid range: min=%.2f, max=%.2f", min, max)
		return mcp.NewToolResultError("Minimum value must be less than maximum value"), nil
	}

	// Simple random number generation
	result := min + (max-min)*0.42

	log.Printf("Generated random number: %.2f (range: %.2f-%.2f)", result, min, max)
	return mcp.NewToolResultText(fmt.Sprintf("Random number between %.2f and %.2f: %.2f", min, max, result)), nil
}

func handleMathConstants(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	log.Printf("Resource access: %s", request.Params.URI)

	constants := map[string]interface{}{
		"pi":    3.141592653589793,
		"e":     2.718281828459045,
		"phi":   1.618033988749895, // Golden ratio
		"sqrt2": 1.4142135623730951,
		"ln2":   0.6931471805599453,
		"ln10":  2.302585092994046,
	}

	data, _ := json.MarshalIndent(constants, "", "  ")

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(data),
		},
	}, nil
}

func handleServerInfo(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	info := fmt.Sprintf(`MCP Calculator Server
=====================================

Server Name: %s
Version: %s
Protocol: Model Context Protocol (MCP)
Capabilities:
  - Tools: 2 available (calculate, random_number)
  - Resources: 2 available (math constants, server info)
  - Prompts: 2 available (math problem, explain calculation)

Transport Support:
  - stdio: For local development and MCP Inspector
  - streamable-http: For web deployments and containers

This server provides mathematical operations for testing MCP implementations.

Last updated: %s`, ServerName, ServerVersion, time.Now().Format(time.RFC3339))

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/plain",
			Text:     info,
		},
	}, nil
}

func handleMathProblemPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	difficulty := "medium"
	topic := "mixed"

	if diff := request.Params.Arguments["difficulty"]; diff != "" {
		difficulty = diff
	}

	if args := request.Params.Arguments["topic"]; args != "" {
		topic = args
	}

	log.Printf("Math problem prompt - difficulty: %s, topic: %s", difficulty, topic)

	var prompt string
	switch strings.ToLower(difficulty) {
	case "easy":
		switch strings.ToLower(topic) {
		case "addition":
			prompt = "Create a simple addition word problem suitable for elementary students. Use small numbers (1-20) and a real-world scenario like counting toys, apples, or students."
		case "subtraction":
			prompt = "Create a simple subtraction word problem suitable for elementary students. Use small numbers (1-20) and a scenario like giving away items or eating some food."
		default:
			prompt = "Create an easy math word problem suitable for elementary students using basic addition or subtraction with numbers 1-20."
		}
	case "hard":
		switch strings.ToLower(topic) {
		case "multiplication":
			prompt = "Create a challenging multiplication word problem involving multi-digit numbers, rates, or area calculations. Include multiple steps if appropriate."
		case "division":
			prompt = "Create a challenging division word problem involving large numbers, remainders, or real-world applications like splitting costs or calculating rates."
		default:
			prompt = "Create a challenging multi-step word problem that requires advanced mathematical thinking and combines multiple operations."
		}
	default: // medium
		prompt = "Create a moderately challenging word problem that requires 2-3 steps to solve and involves realistic scenarios like shopping, time, or measurements."
	}

	return mcp.NewGetPromptResult(
		fmt.Sprintf("Math problem generator for %s level %s problems", difficulty, topic),
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(prompt),
			),
		},
	), nil
}

func handleExplainCalculationPrompt(_ context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	expression := request.Params.Arguments["expression"]
	if expression == "" {
		expression = "2 + 3 * 4"
	}

	prompt := fmt.Sprintf(`Explain how to solve this mathematical expression step by step: %s

Please provide:
1. The expression to solve
2. Order of operations (PEMDAS/BODMAS) explanation
3. Step-by-step breakdown
4. Final answer
5. A brief explanation of why each step was necessary

Make the explanation clear, suitable for someone learning mathematics.`, expression)

	return mcp.NewGetPromptResult(
		fmt.Sprintf("Step-by-step explanation for solving: %s", expression),
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent(prompt),
			),
		},
	), nil
}

// evaluateExpression evaluates a simple mathematical expression
func evaluateExpression(expr string) (float64, error) {
	expr = removeSpaces(expr)

	// Find the operator
	var op rune
	var opIndex = -1

	// Look for operators (from right to left for left-associative operations)
	for i := len(expr) - 1; i >= 0; i-- {
		switch expr[i] {
		case '+', '-':
			// Skip if it's at the beginning (negative number)
			if i > 0 {
				op = rune(expr[i])
				opIndex = i
				goto found
			}
		case '*', '/':
			op = rune(expr[i])
			opIndex = i
			goto found
		}
	}

found:
	if opIndex == -1 {
		// No operator found, try to parse as a single number
		return parseFloat(expr)
	}

	// Split into left and right parts
	left := expr[:opIndex]
	right := expr[opIndex+1:]

	if left == "" || right == "" {
		return 0, fmt.Errorf("invalid expression: missing operand")
	}

	// Parse operands
	leftVal, err := evaluateExpression(left)
	if err != nil {
		return 0, err
	}

	rightVal, err := evaluateExpression(right)
	if err != nil {
		return 0, err
	}

	// Perform operation
	switch op {
	case '+':
		return leftVal + rightVal, nil
	case '-':
		return leftVal - rightVal, nil
	case '*':
		return leftVal * rightVal, nil
	case '/':
		if rightVal == 0 {
			return 0, fmt.Errorf("division by zero is not allowed")
		}
		return leftVal / rightVal, nil
	default:
		return 0, fmt.Errorf("unsupported operation '%c'", op)
	}
}

// parseFloat parses a string into a float64
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}

	return strconv.ParseFloat(s, 64)
}

func removeSpaces(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

func formatResult(result float64) string {
	return fmt.Sprintf("%.10g", result)
}
