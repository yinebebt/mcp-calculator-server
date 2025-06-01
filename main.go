package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create MCP server
	s := server.NewMCPServer(
		"calculator-server",
		"0.1.0",
	)

	// Add calculator tool
	s.AddTool(mcp.NewTool(
		"calculate",
		mcp.WithDescription("Perform basic mathematical operations like add, subtract, multiply, and divide on two numbers"),
		mcp.WithString("expression",
			mcp.Description("A mathematical expression to evaluate (e.g., '2 + 3', '10 * 5', '15 / 3')"),
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

// evaluateExpression evaluates a simple mathematical expression
func evaluateExpression(expr string) (float64, error) {
	// Remove spaces
	expr = removeSpaces(expr)

	// Find the operator
	var op rune
	var opIndex int = -1

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

	var result float64
	var decimal float64 = 1
	var sign float64 = 1
	var hasDecimal bool = false

	i := 0
	if s[0] == '-' {
		sign = -1
		i = 1
	} else if s[0] == '+' {
		i = 1
	}

	for ; i < len(s); i++ {
		char := s[i]
		if char >= '0' && char <= '9' {
			digit := float64(char - '0')
			if hasDecimal {
				decimal *= 10
				result += digit / decimal
			} else {
				result = result*10 + digit
			}
		} else if char == '.' && !hasDecimal {
			hasDecimal = true
		} else {
			return 0, fmt.Errorf("invalid number: %s", s)
		}
	}

	return result * sign, nil
}

// removeSpaces removes all spaces from a string
func removeSpaces(s string) string {
	var result []rune
	for _, char := range s {
		if char != ' ' {
			result = append(result, char)
		}
	}
	return string(result)
}

// formatResult formats the calculation result
func formatResult(result float64) string {
	return fmt.Sprintf("%.10g", result)
}
