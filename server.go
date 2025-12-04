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

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "mcp-calculator-server"
	ServerVersion = "0.5.4"
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
		if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatalf("Stdio server error: %v", err)
		}
	case "streamable-http":
		startHTTPServer(s)
	default:
		log.Fatalf("Unsupported transport: %s (supported: stdio or streamable-http)", transport)
	}
}

func createMCPServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    ServerName,
		Version: ServerVersion,
	}, nil)

	log.Printf("Initializing MCP server: %s v%s", ServerName, ServerVersion)

	// Calculator tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "calculate",
		Description: "Perform basic mathematical operations like add, subtract, multiply, and divide",
	}, handleCalculate)

	// Random number generator tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "random_number",
		Description: "Generate a random number within a specified range using various probability distributions",
	}, handleRandomNumber)

	log.Println("Loaded tools: calculate, random_number")

	// Math constants resource
	s.AddResource(&mcp.Resource{
		URI:         "math://constants",
		Name:        "Mathematical Constants",
		Description: "Common mathematical constants and their values",
		MIMEType:    "application/json",
	}, handleMathConstants)

	// Server information resource
	s.AddResource(&mcp.Resource{
		URI:         "server://info",
		Name:        "Server Information",
		Description: "Information about this MCP server",
		MIMEType:    "text/plain",
	}, handleServerInfo)

	log.Println("Loaded resources: math://constants, server://info")

	// Math problem generator prompt
	s.AddPrompt(&mcp.Prompt{
		Name:        "math_problem",
		Description: "Generate a mathematical word problem",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "difficulty",
				Description: "Difficulty level: 'easy', 'medium', 'hard'",
				Required:    false,
			},
			{
				Name:        "topic",
				Description: "Math topic: 'addition', 'subtraction', 'multiplication', 'division', 'mixed'",
				Required:    false,
			},
		},
	}, handleMathProblemPrompt)

	// Calculation explanation prompt
	s.AddPrompt(&mcp.Prompt{
		Name:        "explain_calculation",
		Description: "Explain how to solve a mathematical expression step by step",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "expression",
				Description: "Mathematical expression to explain",
				Required:    true,
			},
		},
	}, handleExplainCalculationPrompt)

	log.Println("Loaded prompts: math_problem, explain_calculation")
	log.Println("MCP server initialization complete")

	return s
}

func startHTTPServer(s *mcp.Server) {
	port := "8080"
	if portStr := os.Getenv("PORT"); portStr != "" {
		port = portStr
	}

	log.Printf("starting server with streamable-http transport on port %s", port)

	// Create streamable HTTP handler
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: 5 * time.Minute,
		Stateless:      false,
	})

	// Create HTTP mux for additional endpoints
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			log.Printf("Health check write error: %v", err)
		}
	})

	// Mount the MCP streamable HTTP handler at /mcp
	mux.Handle("/mcp", handler)

	// Start HTTP server
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func handleCalculate(ctx context.Context, req *mcp.CallToolRequest, input struct {
	Expression string `json:"expression" jsonschema:"A mathematical expression to evaluate (e.g., '2 + 3', '10 * 5', '15 / 3')"`
}) (*mcp.CallToolResult, struct {
	Result string `json:"result"`
}, error) {
	expression := input.Expression

	// Validate expression length and characters
	if len(expression) == 0 {
		log.Printf("Calculate error - empty expression")
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Expression cannot be empty"},
				},
			}, struct {
				Result string `json:"result"`
			}{}, nil
	}

	if len(expression) > 500 {
		log.Printf("Calculate error - expression too long: %d characters", len(expression))
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Expression too long (maximum 500 characters)"},
				},
			}, struct {
				Result string `json:"result"`
			}{}, nil
	}

	// Check for valid characters
	validChars := "0123456789+-*/.()eE "
	for _, char := range expression {
		if !strings.ContainsRune(validChars, char) {
			log.Printf("Calculate error - invalid character: %c", char)
			return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{
						&mcp.TextContent{Text: fmt.Sprintf("Invalid character in expression: '%c'", char)},
					},
				}, struct {
					Result string `json:"result"`
				}{}, nil
		}
	}

	result, err := evaluateExpression(expression)
	if err != nil {
		log.Printf("Calculate error - evaluation failed: %v", err)
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Calculation error: %v", err)},
				},
			}, struct {
				Result string `json:"result"`
			}{}, nil
	}

	// Check for special float values, NaN check
	if math.IsNaN(result) {
		log.Printf("Calculate error - result is NaN")
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Calculation resulted in an invalid number (NaN)"},
				},
			}, struct {
				Result string `json:"result"`
			}{}, nil
	}

	resultStr := fmt.Sprintf("Result: %s = %s", expression, formatResult(result))
	log.Printf("Calculate result: %s = %s", expression, formatResult(result))
	return nil, struct {
		Result string `json:"result"`
	}{
		Result: resultStr,
	}, nil
}

// generateUniform creates a uniform random number in the range [min, max)
func generateUniform(min, max float64) (float64, error) {
	diff := max - min
	precision := int64(10000) // 4 decimal places
	maxInt := int64(diff * float64(precision))

	if maxInt <= 0 {
		maxInt = 1
	}

	randInt, err := rand.Int(rand.Reader, big.NewInt(maxInt))
	if err != nil {
		return 0, err
	}

	return min + (float64(randInt.Int64()) / float64(precision)), nil
}

// generateNormal creates a normal (Gaussian) random number in the range [min, max]
func generateNormal(min, max float64) (float64, error) {
	// Use Box-Muller transform to generate normal distribution
	// Mean = (min + max) / 2, StdDev = (max - min) / 6 (so ~99.7% within range)
	mean := (min + max) / 2
	stdDev := (max - min) / 6

	// Generate two uniform random numbers
	u1, err := generateUniform(0, 1)
	if err != nil {
		return 0, err
	}
	u2, err := generateUniform(0, 1)
	if err != nil {
		return 0, err
	}

	// Box-Muller transform
	z0 := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
	result := mean + z0*stdDev

	// Clamp to range
	if result < min {
		result = min
	}
	if result > max {
		result = max
	}

	return result, nil
}

// generateExponential creates an exponential random number in the range [min, max]
func generateExponential(min, max float64) (float64, error) {
	// Lambda = 1 / ((max - min) / 3) to get reasonable distribution in range
	lambda := 3.0 / (max - min)

	u, err := generateUniform(0, 1)
	if err != nil {
		return 0, err
	}

	// Inverse transform method: -ln(1-u) / lambda
	result := min + (-math.Log(1-u) / lambda)

	// Clamp to range
	if result > max {
		result = max
	}

	return result, nil
}

func handleRandomNumber(ctx context.Context, req *mcp.CallToolRequest, input struct {
	Min          *float64 `json:"min,omitempty" jsonschema:"Minimum value (default: 1)"`
	Max          *float64 `json:"max,omitempty" jsonschema:"Maximum value (default: 100)"`
	Distribution *string  `json:"distribution,omitempty" jsonschema:"Probability distribution: 'uniform' (default), 'normal' (Gaussian/bell curve), or 'exponential' (exponential decay)"`
}) (*mcp.CallToolResult, struct {
	Result string `json:"result"`
}, error) {
	minm := 1.0
	maxi := 100.0

	if input.Min != nil {
		minm = *input.Min
	}
	if input.Max != nil {
		maxi = *input.Max
	}

	var distribution string
	if input.Distribution != nil && *input.Distribution != "" {
		distribution = *input.Distribution
	} else {
		// If no distribution specified, trigger elicitation
		if req.Session != nil {
			log.Println("Distribution not specified, triggering elicitation")

			// Create elicitation request using official SDK
			distSchema := &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"distribution": {
						Type:        "string",
						Enum:        []any{"uniform", "normal", "exponential"},
						Description: "Probability distribution type:\n- uniform: Even spread across the range\n- normal: Bell curve (Gaussian) centered in the range\n- exponential: Exponential decay from minimum",
					},
				},
				Required: []string{"distribution"},
			}

			elicitResult, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
				Message:         "Which probability distribution would you like to use for the random number?",
				RequestedSchema: distSchema,
			})

			if err != nil {
				log.Printf("Elicitation request failed: %v, using default uniform distribution", err)
				distribution = "uniform"
			} else {
				switch elicitResult.Action {
				case "accept":
					// ElicitResult.Content is already map[string]any
					if dist, ok := elicitResult.Content["distribution"].(string); ok {
						distribution = dist
						log.Printf("User selected distribution: %s", distribution)
					} else {
						log.Println("Invalid distribution in response, using default uniform")
						distribution = "uniform"
					}
				case "decline", "cancel":
					log.Printf("User %s the elicitation request, using default uniform distribution", elicitResult.Action)
					distribution = "uniform"
				default:
					log.Printf("Unknown elicitation response action: %s, using default uniform", elicitResult.Action)
					distribution = "uniform"
				}
			}
		} else {
			log.Println("No session available for elicitation, using default uniform distribution")
			distribution = "uniform"
		}
	}

	// Validate distribution
	if distribution != "uniform" && distribution != "normal" && distribution != "exponential" {
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Unknown distribution: %s. Supported distributions are: uniform, normal, exponential", distribution)},
				},
			}, struct {
				Result string `json:"result"`
			}{}, nil
	}

	// Generate random number
	if minm >= maxi {
		log.Printf("Random number error - invalid range: min=%.2f, max=%.2f", minm, maxi)
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Minimum value must be less than maximum value"},
				},
			}, struct {
				Result string `json:"result"`
			}{}, nil
	}

	var result float64
	var err error

	switch distribution {
	case "uniform":
		result, err = generateUniform(minm, maxi)
	case "normal":
		result, err = generateNormal(minm, maxi)
	case "exponential":
		result, err = generateExponential(minm, maxi)
	}

	if err != nil {
		log.Printf("Random number generation error: %v", err)
		return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{Text: "Failed to generate random number"},
				},
			}, struct {
				Result string `json:"result"`
			}{}, nil
	}

	resultStr := fmt.Sprintf("Random number (%s distribution) between %.2f and %.2f: %.6f", distribution, minm, maxi, result)
	log.Printf("Generated random number: %.4f (distribution: %s, range: %.2f-%.2f)", result, distribution, minm, maxi)
	return nil, struct {
		Result string `json:"result"`
	}{
		Result: resultStr,
	}, nil
}

func handleMathConstants(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	log.Printf("Resource access: %s", req.Params.URI)

	constants := map[string]interface{}{
		"pi":    3.141592653589793,
		"e":     2.718281828459045,
		"phi":   1.618033988749895, // Golden ratio
		"sqrt2": 1.4142135623730951,
		"ln2":   0.6931471805599453,
		"ln10":  2.302585092994046,
	}

	data, _ := json.MarshalIndent(constants, "", "  ")

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "application/json",
				Text:     string(data),
			},
		},
	}, nil
}

func handleServerInfo(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
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

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "text/plain",
				Text:     info,
			},
		},
	}, nil
}

func handleMathProblemPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	difficulty := "medium"
	topic := "mixed"

	if diff := req.Params.Arguments["difficulty"]; diff != "" {
		difficulty = diff
	}

	if args := req.Params.Arguments["topic"]; args != "" {
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

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Math problem generator for %s level %s problems", difficulty, topic),
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: prompt},
			},
		},
	}, nil
}

func handleExplainCalculationPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	expression := req.Params.Arguments["expression"]
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

	return &mcp.GetPromptResult{
		Description: fmt.Sprintf("Step-by-step explanation for solving: %s", expression),
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: prompt},
			},
		},
	}, nil
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
