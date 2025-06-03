//go:build client

// A simple mcp-client that can be used to test the mcp server.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run mcp-client.go <your-mcp-server-binary> [args...]")
		fmt.Println("Example: go run mcp-client.go ./my-mcp-server")
		fmt.Println("Example: go run mcp-client.go node my-server.js")
		os.Exit(1)
	}

	// Start your MCP server
	args := os.Args[2:] // Skip program name and server binary
	cmd := exec.Command(os.Args[1], args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("❌ Error creating stdin pipe: %v\n", err)
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("❌ Error creating stdout pipe: %v\n", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("❌ Error creating stderr pipe: %v\n", err)
		return
	}

	fmt.Printf("Starting MCP server: %s %s\n", os.Args[1], strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		fmt.Printf("❌ Error starting server: %v\n", err)
		return
	}

	// Handle server logs (stderr)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Printf("Server Stderr: %s\n", scanner.Text())
		}
	}()

	// Handle server responses (stdout)
	responseChan := make(chan string)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) != "" {
				responseChan <- line
			}
		}
		close(responseChan)
	}()

	messageID := 1
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println(" MCP Client - Choose an action:")
		fmt.Println("1. Initialize connection")
		fmt.Println("2. List available tools")
		fmt.Println("3. Calculate 3 + 3")
		fmt.Println("4. Calculate custom expression")
		fmt.Println("5. Send custom JSON-RPC message")
		fmt.Println("6. Exit")
		fmt.Println(strings.Repeat("=", 60))

		fmt.Print("Enter choice (1-6): ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		var message MCPMessage

		switch choice {
		case "1":
			message = MCPMessage{
				JSONRPC: "2.0",
				ID:      messageID,
				Method:  "initialize",
				Params: map[string]interface{}{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]interface{}{},
					"clientInfo": map[string]string{
						"name":    "test-client",
						"version": "1.0.0",
					},
				},
			}

		case "2":
			message = MCPMessage{
				JSONRPC: "2.0",
				ID:      messageID,
				Method:  "tools/list",
				Params:  map[string]interface{}{},
			}

		case "3":
			message = MCPMessage{
				JSONRPC: "2.0",
				ID:      messageID,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "local__calculator__calculate",
					"arguments": map[string]string{
						"expression": "3 + 3",
					},
				},
			}

		case "4":
			fmt.Print("Enter mathematical expression: ")
			expr, _ := reader.ReadString('\n')
			expr = strings.TrimSpace(expr)

			message = MCPMessage{
				JSONRPC: "2.0",
				ID:      messageID,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "local__calculator__calculate",
					"arguments": map[string]string{
						"expression": expr,
					},
				},
			}

		case "5":
			fmt.Print("Enter JSON-RPC message: ")
			jsonMsg, _ := reader.ReadString('\n')
			jsonMsg = strings.TrimSpace(jsonMsg)

			// Send raw JSON
			fmt.Println("\nSENDING RAW JSON:")
			fmt.Println(jsonMsg)
			stdin.Write([]byte(jsonMsg + "\n"))

			// Wait for response
			waitForResponse(responseChan)
			continue

		case "6":
			fmt.Println("Shutting down...")
			stdin.Close()
			cmd.Process.Kill()
			return

		default:
			fmt.Println("Invalid choice")
			continue
		}

		// Send message to server
		fmt.Println("\nSENDING TO SERVER:")
		prettyJSON, _ := json.MarshalIndent(message, "", "  ")
		fmt.Println(string(prettyJSON))

		jsonData, _ := json.Marshal(message)
		fmt.Printf("\nRAW JSON PAYLOAD:\n%s\n", string(jsonData))

		stdin.Write(append(jsonData, '\n'))
		messageID++

		// Wait for response
		waitForResponse(responseChan)
	}
}

func waitForResponse(responseChan chan string) {
	fmt.Println("\nWaiting for server response...")

	select {
	case response, ok := <-responseChan:
		if !ok {
			fmt.Println("Server closed connection")
			return
		}

		fmt.Println("\nSERVER RESPONSE:")

		// Try to pretty print JSON
		var jsonData interface{}
		if err := json.Unmarshal([]byte(response), &jsonData); err == nil {
			prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
			fmt.Println(string(prettyJSON))
		} else {
			fmt.Printf("Raw response: %s\n", response)
		}

	case <-time.After(5 * time.Second):
		fmt.Println("Timeout waiting for response")
	}
}
