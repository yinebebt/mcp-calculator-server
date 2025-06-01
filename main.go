package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yinebebt/mcp-calculator-server/internal/mcp"
	"github.com/yinebebt/mcp-calculator-server/internal/tools"
)

const (
	serverName    = "mcp-calculator-server"
	serverVersion = "0.1.0"
)

func main() {
	// Parse command line flags
	var (
		httpMode = flag.Bool("http", false, "Run in HTTP mode (default: stdio mode)")
		port     = flag.Int("port", 7000, "Port to listen on in HTTP mode")
		help     = flag.Bool("help", false, "Show help message")
	)
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	// Create MCP server
	server := mcp.NewServer(serverName, serverVersion)

	// Register calculator tool
	calculatorTool := tools.CalculatorTool()
	server.RegisterTool(calculatorTool, tools.CalculatorHandler)

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if *httpMode {
		log.Printf("Starting %s v%s in HTTP mode", serverName, serverVersion)

		errChan := make(chan error, 1)
		go func() {
			errChan <- server.ServeHTTP(*port)
		}()

		// Wait for shutdown signal or error
		select {
		case <-sigChan:
			log.Println("Received shutdown signal, stopping server...")
		case err := <-errChan:
			log.Printf("Server error: %v", err)
		}
	} else {
		log.Printf("Starting %s v%s in stdio mode", serverName, serverVersion)

		errChan := make(chan error, 1)
		go func() {
			errChan <- server.ServeStdio()
		}()

		// Wait for shutdown signal or error
		select {
		case <-sigChan:
			log.Println("Received shutdown signal, stopping server...")
		case err := <-errChan:
			if err != nil {
				log.Printf("Server error: %v", err)
				os.Exit(1)
			}
		}
	}

	log.Println("Server stopped")
}

func printHelp() {
	fmt.Printf(`%s v%s - MCP Calculator Server

USAGE:
    %s [OPTIONS]

OPTIONS:
    -http        Run in HTTP mode for testing (default: stdio mode for MCP clients)
    -port=7000   Port to listen on in HTTP mode (default: 7000)
    -help        Show this help message

MODES:
    Stdio Mode (default):
        Used by MCP clients like Claude Desktop. Communicates via JSON-RPC over stdin/stdout.
    
    HTTP Mode:
        Used for testing with curl or other HTTP clients.

MCP INTEGRATION:
    To use with Claude Desktop, add this server to your MCP configuration:
    
    {
      "mcpServers": {
        "calculator": {
          "command": "/path/to/%s"
        }
      }
    }

TOOL CAPABILITIES:
    Calculator tool supports: add, subtract, multiply, divide
    
    Parameters:
    - operation: "add" | "subtract" | "multiply" | "divide"
    - a: number (first operand)
    - b: number (second operand)

`, serverName, serverVersion, os.Args[0], serverName)
}
