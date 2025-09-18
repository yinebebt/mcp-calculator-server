package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
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
	ServerVersion = "0.5.1"
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
		log.Println("starting server with stdio transport")
		if err := server.ServeStdio(s); err != nil {
			log.Fatalf("Stdio server error: %v", err)
		}
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

	log.Println("Loaded tools: calculate, random_number")

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

	log.Println("Loaded resources: math://constants, server://info")

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

	log.Println("Loaded prompts: math_problem, explain_calculation")
	log.Println("MCP server initialization complete")

	return s
}

func startHTTPServer(s *server.MCPServer) {
	port := "8080"
	if portStr := os.Getenv("PORT"); portStr != "" {
		port = portStr
	}

	log.Printf("starting server with streamable-http transport on port %s", port)

	// Create streamable HTTP server
	streamServer := server.NewStreamableHTTPServer(s)

	// Create HTTP mux for additional endpoints
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Health check write error: %v", err)
		}
	})

	// Mount the MCP streamable HTTP handler at /mcp
	mux.Handle("/mcp", streamServer)

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

	// Validate expression length and characters
	if len(expression) == 0 {
		log.Printf("Calculate error - empty expression")
		return mcp.NewToolResultError("Expression cannot be empty"), nil
	}

	if len(expression) > 500 {
		log.Printf("Calculate error - expression too long: %d characters", len(expression))
		return mcp.NewToolResultError("Expression too long (maximum 500 characters)"), nil
	}

	// Check for valid characters
	validChars := "0123456789+-*/.()eE "
	for _, char := range expression {
		if !strings.ContainsRune(validChars, char) {
			log.Printf("Calculate error - invalid character: %c", char)
			return mcp.NewToolResultError(fmt.Sprintf("Invalid character in expression: '%c'", char)), nil
		}
	}

	result, err := evaluateExpression(expression)
	if err != nil {
		log.Printf("Calculate error - evaluation failed: %v", err)
		return mcp.NewToolResultError(fmt.Sprintf("Calculation error: %v", err)), nil
	}

	// Check for special float values, NaN check
	if math.IsNaN(result) {
		log.Printf("Calculate error - result is NaN")
		return mcp.NewToolResultError("Calculation resulted in an invalid number (NaN)"), nil
	}

	log.Printf("Calculate result: %s = %s", expression, formatResult(result))
	return mcp.NewToolResultText(fmt.Sprintf("Result: %s = %s", expression, formatResult(result))), nil
}

func handleRandomNumber(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	minm := 1.0
	maxi := 100.0

	if minVal, ok := args["min"].(float64); ok {
		minm = minVal
	}
	if maxVal, ok := args["max"].(float64); ok {
		maxi = maxVal
	}

	if minm >= maxi {
		log.Printf("Random number error - invalid range: min=%.2f, max=%.2f", minm, maxi)
		return mcp.NewToolResultError("Minimum value must be less than maximum value"), nil
	}

	// Generate random number
	diff := maxi - minm

	precision := int64(10000) // 4 decimal places
	maxInt := int64(diff * float64(precision))

	if maxInt <= 0 {
		maxInt = 1
	}

	randInt, err := rand.Int(rand.Reader, big.NewInt(maxInt))
	if err != nil {
		log.Printf("Random number generation error: %v", err)
		return mcp.NewToolResultError("Failed to generate random number"), nil
	}

	result := minm + (float64(randInt.Int64()) / float64(precision))

	log.Printf("Generated random number: %.4f (range: %.2f-%.2f)", result, minm, maxi)
	return mcp.NewToolResultText(fmt.Sprintf("Random number between %.2f and %.2f: %.6f", minm, maxi, result)), nil
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

// evaluateExpression evaluates a mathematical expression with proper operator precedence and parentheses support
func evaluateExpression(expr string) (float64, error) {
	expr = removeSpaces(expr)
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}
	return parseExpression(expr)
}

// parseExpression handles the main parsing logic
func parseExpression(expr string) (float64, error) {
	result, remaining, err := parseAddSubWithRemaining(expr)
	if err != nil {
		return 0, err
	}
	// Check for leftover characters (e.g., unmatched closing parentheses)
	if len(remaining) > 0 {
		return 0, fmt.Errorf("unexpected character '%c' at position %d", remaining[0], len(expr)-len(remaining))
	}
	return result, nil
}

// parseAddSubWithRemaining handles addition and subtraction and returns remaining string
func parseAddSubWithRemaining(expr string) (float64, string, error) {
	left, remaining, err := parseMulDiv(expr)
	if err != nil {
		return 0, "", err
	}

	for len(remaining) > 0 {
		if remaining[0] != '+' && remaining[0] != '-' {
			break
		}
		op := remaining[0]
		remaining = remaining[1:]

		// Check for consecutive operators
		if len(remaining) == 0 {
			return 0, "", fmt.Errorf("operator '%c' at end of expression", op)
		}

		right, newRemaining, err := parseMulDiv(remaining)
		if err != nil {
			return 0, "", err
		}
		remaining = newRemaining

		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}

	return left, remaining, nil
}

// parseMulDiv handles multiplication and division (higher precedence)
func parseMulDiv(expr string) (float64, string, error) {
	left, remaining, err := parseUnary(expr)
	if err != nil {
		return 0, "", err
	}

	for len(remaining) > 0 {
		if remaining[0] != '*' && remaining[0] != '/' {
			break
		}
		op := remaining[0]
		remaining = remaining[1:]

		// Check for consecutive operators
		if len(remaining) == 0 {
			return 0, "", fmt.Errorf("operator '%c' at end of expression", op)
		}

		right, newRemaining, err := parseUnary(remaining)
		if err != nil {
			return 0, "", err
		}
		remaining = newRemaining

		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, "", fmt.Errorf("division by zero is not allowed")
			}
			left /= right
		}
	}

	return left, remaining, nil
}

// parseUnary handles unary operators (+ and -)
func parseUnary(expr string) (float64, string, error) {
	if len(expr) == 0 {
		return 0, "", fmt.Errorf("unexpected end of expression")
	}

	if expr[0] == '+' {
		return parseUnary(expr[1:])
	}

	if expr[0] == '-' {
		val, remaining, err := parseUnary(expr[1:])
		return -val, remaining, err
	}

	return parseFactor(expr)
}

// parseFactor handles numbers and parentheses (highest precedence)
func parseFactor(expr string) (float64, string, error) {
	if len(expr) == 0 {
		return 0, "", fmt.Errorf("unexpected end of expression")
	}

	// Handle parentheses
	if expr[0] == '(' {
		// Find matching closing parenthesis
		parenCount := 1
		i := 1
		for i < len(expr) && parenCount > 0 {
			switch expr[i] {
			case '(':
				parenCount++
			case ')':
				parenCount--
			}
			i++
		}

		if parenCount > 0 {
			return 0, "", fmt.Errorf("mismatched parentheses: missing closing parenthesis")
		}

		// Evaluate expression inside parentheses
		innerExpr := expr[1 : i-1]
		if len(innerExpr) == 0 {
			return 0, "", fmt.Errorf("empty parentheses are not allowed")
		}
		result, err := parseExpression(innerExpr)
		if err != nil {
			return 0, "", err
		}

		return result, expr[i:], nil
	}

	// Handle numbers
	return parseNumber(expr)
}

// parseNumber extracts and parses a number from the beginning of the expression
func parseNumber(expr string) (float64, string, error) {
	if len(expr) == 0 {
		return 0, "", fmt.Errorf("unexpected end of expression")
	}

	i := 0
	// Skip digits, decimal point, and scientific notation
	for i < len(expr) {
		c := expr[i]
		if (c >= '0' && c <= '9') || c == '.' {
			i++
		} else if c == 'e' || c == 'E' {
			// Handle scientific notation
			i++
			if i < len(expr) && (expr[i] == '+' || expr[i] == '-') {
				i++
			}
			// Continue parsing digits after the exponent
			for i < len(expr) && expr[i] >= '0' && expr[i] <= '9' {
				i++
			}
		} else {
			break
		}
	}

	if i == 0 {
		return 0, "", fmt.Errorf("expected number but found '%c'", expr[0])
	}

	numberStr := expr[:i]
	val, err := strconv.ParseFloat(numberStr, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid number format: %s", numberStr)
	}

	return val, expr[i:], nil
}

func removeSpaces(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

func formatResult(result float64) string {
	return fmt.Sprintf("%.10g", result)
}
