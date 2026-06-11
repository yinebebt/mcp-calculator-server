package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	ServerName    = "mcp-calculator-server"
	ServerWebsite = "https://github.com/yinebebt/mcp-calculator-server"
)

// ServerVersion is overridden at release time via
// -ldflags "-X main.ServerVersion=...".
var ServerVersion = "0.6.0"

const serverIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="4" y="2" width="16" height="20" rx="2" fill="#4a90d9"/><rect x="6" y="4" width="12" height="5" rx="1" fill="#fff"/><circle cx="8" cy="13" r="1.3" fill="#fff"/><circle cx="12" cy="13" r="1.3" fill="#fff"/><circle cx="16" cy="13" r="1.3" fill="#fff"/><circle cx="8" cy="17" r="1.3" fill="#fff"/><circle cx="12" cy="17" r="1.3" fill="#fff"/><circle cx="16" cy="17" r="1.3" fill="#fff"/></svg>`

var mathConstants = map[string]float64{
	"pi":    math.Pi,
	"e":     math.E,
	"phi":   math.Phi,
	"sqrt2": math.Sqrt2,
	"ln2":   math.Ln2,
	"ln10":  math.Log(10),
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	transport := os.Getenv("TRANSPORT")
	if transport == "" {
		transport = "streamable-http"
	}
	slog.Info("starting server", "name", ServerName, "version", ServerVersion, "transport", transport)

	s := createMCPServer(logger)

	switch transport {
	case "stdio":
		if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			slog.Error("stdio server error", "error", err)
			os.Exit(1)
		}
	case "streamable-http":
		if err := startHTTPServer(s); err != nil {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	default:
		slog.Error("unsupported transport (supported: stdio or streamable-http)", "transport", transport)
		os.Exit(1)
	}
}

func createMCPServer(logger *slog.Logger) *mcp.Server {
	serverIcons := []mcp.Icon{{
		Source:   "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(serverIconSVG)),
		MIMEType: "image/svg+xml",
		Sizes:    []string{"any"},
	}}

	s := mcp.NewServer(&mcp.Implementation{
		Name:       ServerName,
		Title:      "Calculator",
		Version:    ServerVersion,
		WebsiteURL: ServerWebsite,
		Icons:      serverIcons,
	}, &mcp.ServerOptions{
		Instructions: "Mathematical utility server. Use the calculate tool for arithmetic " +
			"expressions, random_number for random values, and the math://constants resources " +
			"for common mathematical constants.",
		Logger:            logger,
		CompletionHandler: handleCompletion,
	})

	readOnly := &mcp.ToolAnnotations{
		ReadOnlyHint:  true,
		OpenWorldHint: ptr(false),
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "calculate",
		Title:       "Calculator",
		Description: "Perform basic mathematical operations like add, subtract, multiply, and divide",
		Annotations: readOnly,
		Icons:       serverIcons,
	}, handleCalculate)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "random_number",
		Title:       "Random Number Generator",
		Description: "Generate a random number within a specified range using various probability distributions",
		Annotations: readOnly,
	}, handleRandomNumber)

	s.AddResource(&mcp.Resource{
		URI:         "math://constants",
		Name:        "math_constants",
		Title:       "Mathematical Constants",
		Description: "Common mathematical constants and their values",
		MIMEType:    "application/json",
	}, handleMathConstants)

	s.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "math://constants/{name}",
		Name:        "math_constant",
		Title:       "Mathematical Constant",
		Description: "A single mathematical constant by name: pi, e, phi, sqrt2, ln2, ln10",
		MIMEType:    "application/json",
	}, handleMathConstant)

	s.AddResource(&mcp.Resource{
		URI:         "server://info",
		Name:        "server_info",
		Title:       "Server Information",
		Description: "Information about this MCP server",
		MIMEType:    "text/plain",
	}, handleServerInfo)

	s.AddPrompt(&mcp.Prompt{
		Name:        "math_problem",
		Title:       "Math Problem Generator",
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

	s.AddPrompt(&mcp.Prompt{
		Name:        "explain_calculation",
		Title:       "Calculation Explainer",
		Description: "Explain how to solve a mathematical expression step by step",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "expression",
				Description: "Mathematical expression to explain",
				Required:    true,
			},
		},
	}, handleExplainCalculationPrompt)

	slog.Info("MCP server initialization complete",
		"tools", "calculate, random_number",
		"resources", "math://constants, math://constants/{name}, server://info",
		"prompts", "math_problem, explain_calculation")

	return s
}

func ptr[T any](v T) *T { return &v }

func startHTTPServer(s *mcp.Server) error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s
	}, &mcp.StreamableHTTPOptions{
		SessionTimeout: 5 * time.Minute,
		Stateless:      false,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			slog.Error("health check write error", "error", err)
		}
	})
	mux.Handle("/mcp", handler)

	// No ReadTimeout/WriteTimeout: streamable HTTP relies on long-lived SSE streams.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info("HTTP server listening", "port", port)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		slog.Info("shutdown signal received, draining connections")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Long-lived SSE streams never go idle, so Shutdown routinely times
		// out; force-close the remainder instead of reporting a crash.
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Info("forcing close after shutdown timeout", "error", err)
			return srv.Close()
		}
		return nil
	}
}

// textOutput is the structured output returned by both tools.
type textOutput struct {
	Result string `json:"result"`
}

// toolError builds an error CallToolResult without failing the request.
func toolError(msg string) (*mcp.CallToolResult, textOutput, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, textOutput{}, nil
}

// sessionLog forwards a log message to the connected client via MCP logging,
// in addition to the server's own stderr logging.
func sessionLog(ctx context.Context, ss *mcp.ServerSession, msg string, args ...any) {
	if ss == nil {
		return
	}
	slog.New(mcp.NewLoggingHandler(ss, nil)).InfoContext(ctx, msg, args...)
}

type calculateInput struct {
	Expression string `json:"expression" jsonschema:"A mathematical expression to evaluate (e.g., '2 + 3', '10 * 5', '15 / 3')"`
}

func handleCalculate(ctx context.Context, req *mcp.CallToolRequest, input calculateInput) (*mcp.CallToolResult, textOutput, error) {
	expression := input.Expression

	if len(expression) == 0 {
		slog.Warn("calculate: empty expression")
		return toolError("Expression cannot be empty")
	}

	if len(expression) > 500 {
		slog.Warn("calculate: expression too long", "length", len(expression))
		return toolError("Expression too long (maximum 500 characters)")
	}

	const validChars = "0123456789+-*/.()eE "
	for _, char := range expression {
		if !strings.ContainsRune(validChars, char) {
			slog.Warn("calculate: invalid character", "char", string(char))
			return toolError(fmt.Sprintf("Invalid character in expression: '%c'", char))
		}
	}

	result, err := evaluateExpression(expression)
	if err != nil {
		slog.Warn("calculate: evaluation failed", "error", err)
		return toolError(fmt.Sprintf("Calculation error: %v", err))
	}

	if math.IsNaN(result) || math.IsInf(result, 0) {
		slog.Warn("calculate: non-finite result", "expression", expression)
		return toolError("Calculation resulted in a non-finite number (NaN or infinity)")
	}

	resultStr := fmt.Sprintf("Result: %s = %s", expression, formatResult(result))
	slog.Info("calculate", "expression", expression, "result", formatResult(result))
	sessionLog(ctx, req.Session, "calculate", "expression", expression, "result", formatResult(result))
	return nil, textOutput{Result: resultStr}, nil
}

// generateUniform creates a uniform random number in the range [min, max)
func generateUniform(min, max float64) (float64, error) {
	const minSteps = 10000 // at least 4 decimal places of granularity
	scaled := (max - min) * minSteps

	// Clamp to a valid big.Int bound: rand.Int panics on non-positive values,
	// and large ranges overflow int64. A NaN scaled falls through to minSteps.
	var maxInt int64 = minSteps
	switch {
	case scaled >= math.MaxInt64:
		maxInt = math.MaxInt64
	case scaled > minSteps:
		maxInt = int64(scaled)
	}

	randInt, err := rand.Int(rand.Reader, big.NewInt(maxInt))
	if err != nil {
		return 0, err
	}

	// Interpolate rather than offset from min, so spans that overflow float64
	// or exceed int64 precision stay uniform across the whole range.
	frac := float64(randInt.Int64()) / float64(maxInt)
	return min*(1-frac) + max*frac, nil
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

	// generateUniform can return exactly 0; log(0) would yield -Inf.
	if u1 <= 0 {
		u1 = 1e-9
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

type randomNumberInput struct {
	Min          *float64 `json:"min,omitempty" jsonschema:"Minimum value (default: 1)"`
	Max          *float64 `json:"max,omitempty" jsonschema:"Maximum value (default: 100)"`
	Distribution *string  `json:"distribution,omitempty" jsonschema:"Probability distribution: 'uniform' (default), 'normal' (Gaussian/bell curve), or 'exponential' (exponential decay)"`
}

func handleRandomNumber(ctx context.Context, req *mcp.CallToolRequest, input randomNumberInput) (*mcp.CallToolResult, textOutput, error) {
	minm := 1.0
	maxi := 100.0

	if input.Min != nil {
		minm = *input.Min
	}
	if input.Max != nil {
		maxi = *input.Max
	}

	// Validate the range before potentially bothering the user with elicitation.
	if minm >= maxi {
		slog.Warn("random_number: invalid range", "min", minm, "max", maxi)
		return toolError("Minimum value must be less than maximum value")
	}

	var distribution string
	if input.Distribution != nil && *input.Distribution != "" {
		distribution = *input.Distribution
	} else {
		distribution = elicitDistribution(ctx, req.Session)
	}

	if distribution != "uniform" && distribution != "normal" && distribution != "exponential" {
		return toolError(fmt.Sprintf("Unknown distribution: %s. Supported distributions are: uniform, normal, exponential", distribution))
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
		slog.Error("random_number: generation failed", "error", err)
		return toolError("Failed to generate random number")
	}

	resultStr := fmt.Sprintf("Random number (%s distribution) between %.2f and %.2f: %.6f", distribution, minm, maxi, result)
	slog.Info("random_number", "result", result, "distribution", distribution, "min", minm, "max", maxi)
	sessionLog(ctx, req.Session, "random_number", "result", result, "distribution", distribution)
	return nil, textOutput{Result: resultStr}, nil
}

// elicitDistribution asks the client which distribution to use, falling back
// to uniform when elicitation is unavailable, declined, or fails.
func elicitDistribution(ctx context.Context, ss *mcp.ServerSession) string {
	if ss == nil {
		slog.Info("random_number: no session for elicitation, defaulting to uniform")
		return "uniform"
	}

	slog.Info("random_number: distribution not specified, eliciting from client")

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

	elicitResult, err := ss.Elicit(ctx, &mcp.ElicitParams{
		Message:         "Which probability distribution would you like to use for the random number?",
		RequestedSchema: distSchema,
	})
	if err != nil {
		slog.Warn("random_number: elicitation failed, defaulting to uniform", "error", err)
		return "uniform"
	}

	switch elicitResult.Action {
	case "accept":
		if dist, ok := elicitResult.Content["distribution"].(string); ok {
			slog.Info("random_number: user selected distribution", "distribution", dist)
			return dist
		}
		slog.Warn("random_number: invalid distribution in elicitation response, defaulting to uniform")
	case "decline", "cancel":
		slog.Info("random_number: elicitation not accepted, defaulting to uniform", "action", elicitResult.Action)
	default:
		slog.Warn("random_number: unknown elicitation action, defaulting to uniform", "action", elicitResult.Action)
	}
	return "uniform"
}

func handleMathConstants(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	slog.Info("resource access", "uri", req.Params.URI)

	data, err := json.MarshalIndent(mathConstants, "", "  ")
	if err != nil {
		return nil, err
	}

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

func handleMathConstant(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	slog.Info("resource access", "uri", req.Params.URI)

	name, err := url.PathUnescape(strings.TrimPrefix(req.Params.URI, "math://constants/"))
	if err != nil {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	name = strings.ToLower(name)
	value, ok := mathConstants[name]
	if !ok {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}

	data, err := json.Marshal(map[string]float64{name: value})
	if err != nil {
		return nil, err
	}

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
Runtime: %s
Protocol: Model Context Protocol (MCP)
Capabilities:
  - Tools: 2 available (calculate, random_number)
  - Resources: 2 available (math constants, server info)
  - Resource Templates: 1 available (math://constants/{name})
  - Prompts: 2 available (math problem, explain calculation)
  - Completions: prompt argument and constant-name completion
  - Logging: MCP log messages forwarded to the client

Transport Support:
  - stdio: For local development and MCP Inspector
  - streamable-http: For web deployments and containers

This server provides mathematical operations for testing MCP implementations.`,
		ServerName, ServerVersion, runtime.Version())

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

// promptArgumentCompletions lists completion candidates per prompt argument.
var promptArgumentCompletions = map[string]map[string][]string{
	"math_problem": {
		"difficulty": {"easy", "medium", "hard"},
		"topic":      {"addition", "subtraction", "multiplication", "division", "mixed"},
	},
}

func handleCompletion(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	var candidates []string

	switch req.Params.Ref.Type {
	case "ref/prompt":
		candidates = promptArgumentCompletions[req.Params.Ref.Name][req.Params.Argument.Name]
	case "ref/resource":
		if req.Params.Ref.URI == "math://constants/{name}" && req.Params.Argument.Name == "name" {
			for name := range mathConstants {
				candidates = append(candidates, name)
			}
		}
	}

	prefix := strings.ToLower(req.Params.Argument.Value)
	values := []string{}
	for _, c := range candidates {
		if strings.HasPrefix(c, prefix) {
			values = append(values, c)
		}
	}
	// Constant names come from map iteration; keep client-visible order stable.
	slices.Sort(values)

	return &mcp.CompleteResult{
		Completion: mcp.CompletionResultDetails{
			Values: values,
			Total:  len(values),
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

	slog.Info("math_problem prompt", "difficulty", difficulty, "topic", topic)

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
