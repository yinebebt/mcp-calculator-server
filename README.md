# MCP Calculator Server

A Model Context Protocol (MCP) server that provides calculator functionality with support for mathematical expressions.

## Features

- **Mathematical Operations**: Addition (+), subtraction (-), multiplication (*), division (/)
- **Expression Parsing**: Proper operator precedence and parentheses support
- **Dual Transport Modes**:
  - **stdio** for local development and MCP Inspector
  - **streamable-http** for web deployments and containers
- **Health Check Endpoint**: Health monitoring for HTTP deployments

## Transport Configuration

Simple environment-based configuration:

```bash
TRANSPORT=stdio        # For local development/debugging
TRANSPORT=streamable-http  # For web deployments (default)
```

Default Behavior

- **Default**: `streamable-http` (if no TRANSPORT is set)
- **Port**: 8080 (configurable via PORT environment variable)

## Installation

Build the server:

```bash
go build -o mcp-calculator-server server.go
```

## Usage

### Local Development (stdio)

```bash
TRANSPORT=stdio ./mcp-calculator-server
```

### Web Deployment (streamable-http, default)

```bash
./mcp-calculator-server
# or explicitly:
TRANSPORT=streamable-http PORT=8080 ./mcp-calculator-server
```

### MCP Inspector Configuration

```bash
Command: /path/to/calculator-server
Environment: TRANSPORT=stdio
```

### Testing with Client

```bash
go run -tags=client client.go ./calculator-server
```

## HTTP Endpoints (streamable-http mode)

- **MCP Endpoint**: `http://localhost:8080/mcp`
- **Health Check**: `http://localhost:8080/health`

## Tool Reference

### calculate

Performs mathematical operations with proper operator precedence and implicit multiplication.

**Parameters:**

- `expression` (string, required): Mathematical expression to evaluate

**Supported Operations:**

- Addition: `+`
- Subtraction: `-`
- Multiplication: `*`
- Division: `/`
- Parentheses: `()` for grouping
- Negative numbers: `-5`, `(-3 + 2)`
