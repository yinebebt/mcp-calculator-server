package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"unicode"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create MCP server
	s := server.NewMCPServer(
		"calculator-server",
		"0.2.0",
	)

	// Add calculator tool
	s.AddTool(mcp.NewTool(
		"calculate",
		mcp.WithDescription("Perform mathematical operations including basic arithmetic, parentheses, and proper operator precedence"),
		mcp.WithString("expression",
			mcp.Description("A mathematical expression to evaluate (e.g., '2 + 3', '10 * 5', '(2 + 3) * 4', '15 / 3')"),
			mcp.Required(),
		),
	), handleCalculate)

	// Serve using stdio
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// handleCalculate handles calculator tool calls
func handleCalculate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract expression
	expression, err := request.RequireString("expression")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Parse and evaluate the expression
	result, err := evaluateExpression(expression)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.TextContent{
					Type: "text",
					Text: fmt.Sprintf("Error: %v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// Return successful result
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: formatResult(result),
			},
		},
	}, nil
}

// TokenType for expression parsing
type TokenType int

const (
	NUMBER TokenType = iota
	OPERATOR
	LPAREN
	RPAREN
	EOF
)

type Token struct {
	Type  TokenType
	Value string
}

// Tokenizer converts expression string into tokens
type Tokenizer struct {
	input string
	pos   int
}

func NewTokenizer(input string) *Tokenizer {
	return &Tokenizer{
		input: strings.ReplaceAll(input, " ", ""), // Remove spaces
		pos:   0,
	}
}

func (t *Tokenizer) NextToken() Token {
	if t.pos >= len(t.input) {
		return Token{Type: EOF}
	}

	char := rune(t.input[t.pos])

	// Handle numbers (including decimals and negative numbers)
	if unicode.IsDigit(char) || char == '.' || (char == '-' && t.isStartOfNumber()) {
		return t.readNumber()
	}

	// Handle operators
	if char == '+' || char == '-' || char == '*' || char == '/' {
		t.pos++
		return Token{Type: OPERATOR, Value: string(char)}
	}

	// Handle parentheses
	if char == '(' {
		t.pos++
		return Token{Type: LPAREN, Value: "("}
	}

	if char == ')' {
		t.pos++
		return Token{Type: RPAREN, Value: ")"}
	}

	// Invalid character
	return Token{Type: EOF}
}

func (t *Tokenizer) isStartOfNumber() bool {
	// Check if '-' is at the beginning or after an operator or opening parenthesis
	if t.pos == 0 {
		return true
	}

	prevChar := rune(t.input[t.pos-1])
	return prevChar == '(' || prevChar == '+' || prevChar == '-' || prevChar == '*' || prevChar == '/'
}

func (t *Tokenizer) readNumber() Token {
	start := t.pos

	// Handle negative sign
	if t.input[t.pos] == '-' {
		t.pos++
	}

	// Read digits and decimal point
	for t.pos < len(t.input) {
		char := rune(t.input[t.pos])
		if unicode.IsDigit(char) || char == '.' {
			t.pos++
		} else {
			break
		}
	}

	return Token{Type: NUMBER, Value: t.input[start:t.pos]}
}

// Parser implements recursive descent parser for mathematical expressions
type Parser struct {
	tokenizer *Tokenizer
	current   Token
}

func NewParser(input string) *Parser {
	tokenizer := NewTokenizer(input)
	return &Parser{
		tokenizer: tokenizer,
		current:   tokenizer.NextToken(),
	}
}

func (p *Parser) advance() {
	p.current = p.tokenizer.NextToken()
}

func (p *Parser) Parse() (float64, error) {
	result, err := p.parseExpression()
	if err != nil {
		return 0, err
	}

	if p.current.Type != EOF {
		return 0, fmt.Errorf("unexpected token: %s", p.current.Value)
	}

	return result, nil
}

// parseExpression handles addition and subtraction (lowest precedence)
func (p *Parser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}

	for p.current.Type == OPERATOR && (p.current.Value == "+" || p.current.Value == "-") {
		op := p.current.Value
		p.advance()

		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}

		switch op {
		case "+":
			left = left + right
		case "-":
			left = left - right
		}
	}

	return left, nil
}

// parseTerm handles multiplication and division (higher precedence)
func (p *Parser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}

	for p.current.Type == OPERATOR && (p.current.Value == "*" || p.current.Value == "/") {
		op := p.current.Value
		p.advance()

		right, err := p.parseFactor()
		if err != nil {
			return 0, err
		}

		switch op {
		case "*":
			left = left * right
		case "/":
			if right == 0 {
				return 0, fmt.Errorf("division by zero is not allowed")
			}
			left = left / right
		}
	}

	return left, nil
}

// parseFactor handles numbers and parentheses (highest precedence)
func (p *Parser) parseFactor() (float64, error) {
	if p.current.Type == NUMBER {
		value, err := strconv.ParseFloat(p.current.Value, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", p.current.Value)
		}
		p.advance()
		return value, nil
	}

	if p.current.Type == LPAREN {
		p.advance() // consume '('
		result, err := p.parseExpression()
		if err != nil {
			return 0, err
		}

		if p.current.Type != RPAREN {
			return 0, fmt.Errorf("expected closing parenthesis")
		}
		p.advance() // consume ')'
		return result, nil
	}

	return 0, fmt.Errorf("unexpected token: %s", p.current.Value)
}

// evaluateExpression evaluates a mathematical expression with proper precedence
func evaluateExpression(expr string) (float64, error) {
	if strings.TrimSpace(expr) == "" {
		return 0, fmt.Errorf("empty expression")
	}

	parser := NewParser(expr)
	return parser.Parse()
}

// formatResult formats the calculation result
func formatResult(result float64) string {
	// Use Go's smart formatting - removes unnecessary decimal places
	return strconv.FormatFloat(result, 'g', -1, 64)
}
