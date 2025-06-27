package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ServerName    = "calculator-server"
	ServerVersion = "0.2.0"
)

func main() {
	transport := os.Getenv("TRANSPORT")
	if transport == "" {
		transport = "streamable-http"
	}
	log.Printf("Starting with transport: %s", transport)

	// Create MCP server
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

	// Add calculator tool
	s.AddTool(mcp.NewTool(
		"calculate",
		mcp.WithDescription("Perform mathematical operations including basic arithmetic, parentheses, and proper operator precedence"),
		mcp.WithString("expression",
			mcp.Description("A mathematical expression to evaluate (e.g., '2 + 3', '10 * 5', '(2 + 3) * 4', '15 / 3')"),
			mcp.Required(),
		),
	), handleCalculate)

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
	// Check for PORT environment variable first (common in containers)
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

	log.Printf("MCP endpoints available at: http://localhost:%s/mcp", port)
	log.Printf("Health check available at: http://localhost:%s/health", port)

	// Start HTTP server
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func handleCalculate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract the expression argument
	expression, err := request.RequireString("expression")
	if err != nil {
		return mcp.NewToolResultError("Expression parameter is required"), nil
	}

	// Clean and validate the expression
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return mcp.NewToolResultError("Expression cannot be empty"), nil
	}

	// Evaluate the expression
	result, err := evaluateExpression(expression)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error evaluating expression: %v", err)), nil
	}

	// Format and return the result
	return mcp.NewToolResultText(formatResult(result)), nil
}

// Token types for mathematical expressions
type TokenType int

const (
	TokenNumber TokenType = iota
	TokenPlus
	TokenMinus
	TokenMultiply
	TokenDivide
	TokenOpenParen
	TokenCloseParen
	TokenEOF
)

type Token struct {
	Type  TokenType
	Value string
}

type Tokenizer struct {
	input string
	pos   int
}

func NewTokenizer(input string) *Tokenizer {
	return &Tokenizer{input: input, pos: 0}
}

func (t *Tokenizer) NextToken() Token {
	// Skip whitespace
	for t.pos < len(t.input) && unicode.IsSpace(rune(t.input[t.pos])) {
		t.pos++
	}

	if t.pos >= len(t.input) {
		return Token{TokenEOF, ""}
	}

	char := t.input[t.pos]

	switch char {
	case '+':
		t.pos++
		return Token{TokenPlus, "+"}
	case '-':
		t.pos++
		return Token{TokenMinus, "-"}
	case '*':
		t.pos++
		return Token{TokenMultiply, "*"}
	case '/':
		t.pos++
		return Token{TokenDivide, "/"}
	case '(':
		t.pos++
		return Token{TokenOpenParen, "("}
	case ')':
		t.pos++
		return Token{TokenCloseParen, ")"}
	default:
		if t.isStartOfNumber() {
			return t.readNumber()
		}
		// Invalid character
		t.pos++
		return Token{TokenEOF, ""}
	}
}

func (t *Tokenizer) isStartOfNumber() bool {
	if t.pos >= len(t.input) {
		return false
	}
	char := t.input[t.pos]
	return unicode.IsDigit(rune(char)) || char == '.'
}

func (t *Tokenizer) readNumber() Token {
	start := t.pos
	hasDot := false

	for t.pos < len(t.input) {
		char := t.input[t.pos]
		if unicode.IsDigit(rune(char)) {
			t.pos++
		} else if char == '.' && !hasDot {
			hasDot = true
			t.pos++
		} else {
			break
		}
	}

	return Token{TokenNumber, t.input[start:t.pos]}
}

// Parser with operator precedence
type Parser struct {
	tokenizer *Tokenizer
	current   Token
}

func NewParser(input string) *Parser {
	p := &Parser{tokenizer: NewTokenizer(input)}
	p.advance()
	return p
}

func (p *Parser) advance() {
	p.current = p.tokenizer.NextToken()
}

func (p *Parser) Parse() (float64, error) {
	result, err := p.parseExpression()
	if err != nil {
		return 0, err
	}

	if p.current.Type != TokenEOF {
		return 0, fmt.Errorf("unexpected token: %s", p.current.Value)
	}

	return result, nil
}

// Parse addition and subtraction (lowest precedence)
func (p *Parser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}

	for p.current.Type == TokenPlus || p.current.Type == TokenMinus {
		op := p.current.Type
		p.advance()

		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}

		if op == TokenPlus {
			left = left + right
		} else {
			left = left - right
		}
	}

	return left, nil
}

// Parse multiplication and division (higher precedence)
func (p *Parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}

	for {
		switch p.current.Type {
		case TokenMultiply, TokenDivide:
			op := p.current.Type
			p.advance()

			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}

			if op == TokenMultiply {
				left = left * right
			} else {
				if right == 0 {
					return 0, fmt.Errorf("division by zero")
				}
				left = left / right
			}
		case TokenNumber, TokenOpenParen:
			// Implicit multiplication: 5(2+3) or 5 2 -> 5*2
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			left = left * right
		default:
			return left, nil
		}
	}
}

// Parse numbers and parentheses (highest precedence)
func (p *Parser) parseFactor() (float64, error) {
	if p.current.Type == TokenNumber {
		value := p.current.Value
		p.advance()

		result, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", value)
		}
		return result, nil
	}

	if p.current.Type == TokenOpenParen {
		p.advance() // consume '('

		result, err := p.parseExpression()
		if err != nil {
			return 0, err
		}

		if p.current.Type != TokenCloseParen {
			return 0, fmt.Errorf("expected closing parenthesis")
		}
		p.advance() // consume ')'

		return result, nil
	}

	if p.current.Type == TokenMinus {
		p.advance() // consume '-'

		value, err := p.parseFactor()
		if err != nil {
			return 0, err
		}

		return -value, nil
	}

	return 0, fmt.Errorf("unexpected token: %s", p.current.Value)
}

func evaluateExpression(expr string) (float64, error) {
	parser := NewParser(expr)
	return parser.Parse()
}

func formatResult(result float64) string {
	// Format to remove unnecessary decimal places
	if result == float64(int64(result)) {
		return fmt.Sprintf("%.0f", result)
	}
	return fmt.Sprintf("%g", result)
}
