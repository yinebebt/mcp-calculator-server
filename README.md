# MCP Calculator Server

A Model Context Protocol (MCP) compliant calculator server.

## Features

- **MCP-compliant**: Follows the [Model Context Protocol specification](https://modelcontextprotocol.io/specification/2025-03-26)
- **Dual modes**: Supports both stdio (for MCP clients) and HTTP (for testing)

## Installation

```bash
# Clone the project
cd mcp-calculator-server

# Build the server
go build -o calculator-server .
```

## MCP Client Integration

### Claude Desktop and Curosr

Add this server to your Claude Desktop or Cursor MCP configuration:

```json
{
  "mcpServers": {
    "calculator": {
      "command": "mcp-calculator-server"
    }
  }
}
```
