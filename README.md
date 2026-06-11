# MCP Calculator Server

A Model Context Protocol (MCP) server that provides calculator functionality with robust mathematical operations. Built on the official [go-sdk](https://github.com/modelcontextprotocol/go-sdk).

## Features

### Tools

- **calculate**: Mathematical operations with proper operator precedence, parentheses support, and scientific notation
- **random_number**: Generate random numbers within specified ranges using various probability distributions (uses elicitation when no distribution is given)

Both tools declare tool annotations (read-only, closed-world hints), titles, and structured output schemas.

### Resources

- **math://constants**: Mathematical constants (π, e, φ, √2, ln2, ln10) in JSON format
- **math://constants/{name}**: Resource template — a single constant by name, e.g. `math://constants/pi`
- **server://info**: Server information and capabilities overview

### Prompts

- **math_problem**: Generate mathematical word problems with configurable difficulty and topics
- **explain_calculation**: Step-by-step mathematical expression explanations

### Protocol Features

- **Completions**: Argument completion for prompt arguments (`difficulty`, `topic`) and constant names in the resource template
- **Elicitation**: `random_number` asks the client which distribution to use when not specified
- **Logging**: Tool activity is forwarded to the client as MCP log messages (in addition to stderr)
- **Icons & metadata**: Server and tool icons, titles, website URL, and instructions

### Transport Modes

- **stdio**: For local development and MCP Inspector integration
- **streamable-http**: For web deployments and container environments (with graceful shutdown)

> **Note:** The HTTP transport binds all interfaces with no authentication or Origin validation — intended for demos and trusted networks only. Put it behind a reverse proxy (or use stdio) for anything else.

**Default Behavior:**

- Transport: `streamable-http` (if no TRANSPORT environment variable is set)
- Port: `8080` (configurable via PORT environment variable)

## Installation

Prebuilt binaries for **Linux, macOS, and Windows** (amd64/arm64) are attached to each [GitHub release](https://github.com/yinebebt/mcp-calculator-server/releases).

Or install with Go:

```bash
go install github.com/yinebebt/mcp-calculator-server@latest
```

## Usage

```bash
# Run with stdio
TRANSPORT=stdio mcp-calculator-server

# Run with HTTP (serves MCP at :8080/mcp, health check at /health)
mcp-calculator-server

# Test
go test ./...
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
- **Error detection**: Division by zero, invalid syntax, unmatched parentheses, non-finite results (NaN/Inf)

## Release

Releases are built with [GoReleaser](https://goreleaser.com) via GitHub Actions. Pushing a `vX.Y.Z` tag cross-compiles for linux/darwin/windows × amd64/arm64 and attaches archives and checksums to the GitHub release.
