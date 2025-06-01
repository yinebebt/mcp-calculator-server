package tools

import (
	"fmt"
	"github.com/yinebebt/mcp-calculator-server/internal/mcp"
)

// CalculatorTool creates and returns the calculator tool definition
func CalculatorTool() mcp.Tool {
	return mcp.Tool{
		Name:        "calculator",
		Description: "Perform basic math operations like add, subtract, multiply, divide",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "The mathematical operation to perform",
					"enum":        []string{"add", "subtract", "multiply", "divide"},
				},
				"a": map[string]interface{}{
					"type":        "number",
					"description": "The first number",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "The second number",
				},
			},
			Required: []string{"operation", "a", "b"},
		},
	}
}

// CalculatorHandler handles calculator tool calls
func CalculatorHandler(args map[string]interface{}) (*mcp.CallToolResponse, error) {
	// Extract operation
	opInterface, ok := args["operation"]
	if !ok {
		return &mcp.CallToolResponse{
			Content: []mcp.ToolContent{{
				Type: "text",
				Text: "Error: operation parameter is required",
			}},
			IsError: true,
		}, nil
	}

	operation, ok := opInterface.(string)
	if !ok {
		return &mcp.CallToolResponse{
			Content: []mcp.ToolContent{{
				Type: "text",
				Text: "Error: operation must be a string",
			}},
			IsError: true,
		}, nil
	}

	// Extract first number
	aInterface, ok := args["a"]
	if !ok {
		return &mcp.CallToolResponse{
			Content: []mcp.ToolContent{{
				Type: "text",
				Text: "Error: parameter 'a' is required",
			}},
			IsError: true,
		}, nil
	}

	var a float64
	switch v := aInterface.(type) {
	case float64:
		a = v
	case int:
		a = float64(v)
	default:
		return &mcp.CallToolResponse{
			Content: []mcp.ToolContent{{
				Type: "text",
				Text: "Error: parameter 'a' must be a number",
			}},
			IsError: true,
		}, nil
	}

	// Extract second number
	bInterface, ok := args["b"]
	if !ok {
		return &mcp.CallToolResponse{
			Content: []mcp.ToolContent{{
				Type: "text",
				Text: "Error: parameter 'b' is required",
			}},
			IsError: true,
		}, nil
	}

	var b float64
	switch v := bInterface.(type) {
	case float64:
		b = v
	case int:
		b = float64(v)
	default:
		return &mcp.CallToolResponse{
			Content: []mcp.ToolContent{{
				Type: "text",
				Text: "Error: parameter 'b' must be a number",
			}},
			IsError: true,
		}, nil
	}

	// Perform calculation
	var result float64
	var err error

	switch operation {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return &mcp.CallToolResponse{
				Content: []mcp.ToolContent{{
					Type: "text",
					Text: "Error: division by zero is not allowed",
				}},
				IsError: true,
			}, nil
		}
		result = a / b
	default:
		return &mcp.CallToolResponse{
			Content: []mcp.ToolContent{{
				Type: "text",
				Text: fmt.Sprintf("Error: unsupported operation '%s'. Supported operations: add, subtract, multiply, divide", operation),
			}},
			IsError: true,
		}, nil
	}

	// Return successful result
	return &mcp.CallToolResponse{
		Content: []mcp.ToolContent{{
			Type: "text",
			Text: fmt.Sprintf("%.10g", result), // Use %g to avoid unnecessary decimal places
		}},
	}, err
}
