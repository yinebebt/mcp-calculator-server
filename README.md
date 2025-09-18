# MCP Calculator Server

A Model Context Protocol (MCP) server that provides calculator functionality with robust mathematical operations.

## Features

### Tools
- **calculate**: Mathematical operations with proper operator precedence, parentheses support, and scientific notation
- **random_number**: Generate random numbers within specified ranges

### Resources
- **math://constants**: Mathematical constants (π, e, φ, √2, ln2, ln10) in JSON format
- **server://info**: Server information and capabilities overview

### Prompts
- **math_problem**: Generate mathematical word problems with configurable difficulty and topics
- **explain_calculation**: Step-by-step mathematical expression explanations

### Transport Modes
- **stdio**: For local development and MCP Inspector integration
- **streamable-http**: For web deployments and container environments

**Default Behavior:**
- Transport: `streamable-http` (if no TRANSPORT environment variable is set)
- Port: `8080` (configurable via PORT environment variable)

## Usage

```bash
# Install
go install github.com/yinebebt/mcp-calculator-server@latest

# Run with stdio
TRANSPORT=stdio mcp-calculator-server

# Run with HTTP
mcp-calculator-server

# Test
go test
```

### MCP Client Configuration
For testing with MCP Client:

```json
{
  "command": "/path/to/mcp-calculator-server",
  "env": {
    "TRANSPORT": "stdio"
  }
}
```

## Mathematical Features

- **Operator precedence**: `2+3*4 = 14`
- **Parentheses**: `(2+3)*4 = 20`
- **Scientific notation**: `1e2 = 100`
- **Error detection**: Division by zero, invalid syntax, unmatched parentheses