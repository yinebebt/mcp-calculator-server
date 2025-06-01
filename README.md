# MCP Calculator Server

A Model Context Protocol (MCP) server that provides calculator functionality for MCP clients.

## Features

- **Mathematical Operations**: Supports addition (+), subtraction (-), multiplication (*), and division (/)
- **MCP Protocol Compliance**: Fully compatible with Claude Desktop and other MCP clients

## Installation

```bash
go install github.com/yinebebt/mcp-calculator-server
```

## Configuration

### Claude Desktop Configuration

Add this server to your Claude Desktop configuration file:

```json
{
  "mcpServers": {
    "calculator": {
      "command": "/path/to/mcp-calculator-server"
    }
  }
}
```

### Restart Claude Desktop

After updating the configuration, restart Claude Desktop to load the new server.

## Usage

Once configured, you can ask Claude to perform calculations like "Can you calculate 2 + 3?"

## Tool Reference

### calculate

Performs basic mathematical operations.

**Parameters:**
- `expression` (string, required): A mathematical expression to evaluate

**Supported operations:**
- Addition: `+`
- Subtraction: `-`
- Multiplication: `*`
- Division: `/`