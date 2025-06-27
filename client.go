//go:build client

// mcp-client that can be used to test mcp server.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      *int        `json:"id,omitempty"` // Notifications don't have ID
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Client struct {
	stdin        io.WriteCloser
	responseChan chan string
	messageID    int
	tools        []ToolInfo
	initialized  bool
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run -tags=client client.go <your-mcp-server-binary> [args...]")
		fmt.Println("Example: go run -tags=client client.go ./calculator-server")
		os.Exit(1)
	}

	client := &Client{messageID: 1}

	// Start your MCP server
	args := os.Args[2:] // Skip program name and server binary
	cmd := exec.Command(os.Args[1], args...)

	// Set environment to force stdio transport mode
	cmd.Env = append(os.Environ(), "TRANSPORT=stdio")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("Error creating stdin pipe: %v\n", err)
		return
	}
	client.stdin = stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error creating stdout pipe: %v\n", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error creating stderr pipe: %v\n", err)
		return
	}

	fmt.Printf("Starting MCP server: %s %s\n", os.Args[1], strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		return
	}

	// Handle server logs (stderr)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Printf("Server Log: %s\n", scanner.Text())
		}
	}()

	// Handle server responses (stdout)
	client.responseChan = make(chan string, 10)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) != "" {
				client.responseChan <- line
			}
		}
		close(client.responseChan)
	}()

	// Auto-initialize connection
	fmt.Println("Auto-initializing MCP connection...")
	if err := client.initializeConnection(); err != nil {
		fmt.Printf("Failed to initialize: %v\n", err)
		return
	}

	// Auto-discover tools
	fmt.Println("Discovering available tools...")
	if err := client.discoverTools(); err != nil {
		fmt.Printf("Failed to discover tools: %v\n", err)
		return
	}

	client.runInteractiveMode()

	// Cleanup
	stdin.Close()
	cmd.Process.Kill()
}

func (c *Client) initializeConnection() error {
	// Send initialize request
	initMessage := MCPMessage{
		JSONRPC: "2.0",
		ID:      &c.messageID,
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

	if err := c.sendMessage(initMessage); err != nil {
		return err
	}

	// Wait for initialize response
	response, err := c.waitForResponse(5 * time.Second)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf("initialization failed: %v", response.Error)
	}

	fmt.Println("Received initialize response")

	// Send initialized notification
	initializedNotification := MCPMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
		Params:  map[string]interface{}{},
	}

	if err := c.sendMessage(initializedNotification); err != nil {
		return err
	}

	c.initialized = true
	fmt.Println("MCP connection initialized successfully")
	return nil
}

func (c *Client) discoverTools() error {
	if !c.initialized {
		return fmt.Errorf("connection not initialized")
	}

	listMessage := MCPMessage{
		JSONRPC: "2.0",
		ID:      &c.messageID,
		Method:  "tools/list",
		Params:  map[string]interface{}{},
	}

	if err := c.sendMessage(listMessage); err != nil {
		return err
	}

	response, err := c.waitForResponse(5 * time.Second)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return fmt.Errorf("tools/list failed: %v", response.Error)
	}

	// Parse tools from response
	if result, ok := response.Result.(map[string]interface{}); ok {
		if toolsData, ok := result["tools"].([]interface{}); ok {
			c.tools = nil // Reset tools
			for _, toolData := range toolsData {
				if tool, ok := toolData.(map[string]interface{}); ok {
					toolInfo := ToolInfo{}
					if name, ok := tool["name"].(string); ok {
						toolInfo.Name = name
					}
					if desc, ok := tool["description"].(string); ok {
						toolInfo.Description = desc
					}
					c.tools = append(c.tools, toolInfo)
				}
			}
		}
	}

	fmt.Printf("Discovered %d tool(s):\n", len(c.tools))
	for _, tool := range c.tools {
		fmt.Printf("   - %s: %s\n", tool.Name, tool.Description)
	}

	return nil
}

func (c *Client) runInteractiveMode() {
	reader := bufio.NewReader(os.Stdin)

	for {
		c.printMenu()
		fmt.Print("Enter choice: ")
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			c.testBasicCalculation()
		case "2":
			c.customCalculation(reader)
		case "3":
			c.listTools()
		case "4":
			c.sendCustomMessage(reader)
		case "5":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Println("Invalid choice")
		}
	}
}

func (c *Client) printMenu() {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Println(" MCP Calculator Client - Choose an action:")
	fmt.Println("1. Test basic calculation")
	fmt.Println("2. Custom calculation")
	fmt.Println("3. List available tools")
	fmt.Println("4. Send custom JSON-RPC message")
	fmt.Println("5. Exit")
	fmt.Println(strings.Repeat("=", 70))
}

func (c *Client) testBasicCalculation() {
	fmt.Println("\nRunning calculation test...")

	expr := "((2 + 3) * -4) / 2"
	result, err := c.callCalculator(expr)
	if err != nil {
		fmt.Printf(" %s = ERROR: %v\n", expr, err)
	} else {
		fmt.Printf(" %s = %s\n", expr, result)
	}
}

func (c *Client) customCalculation(reader *bufio.Reader) {
	fmt.Print("Enter mathematical expression: ")
	expr, _ := reader.ReadString('\n')
	expr = strings.TrimSpace(expr)

	if expr == "" {
		fmt.Println(" Empty expression")
		return
	}

	result, err := c.callCalculator(expr)
	if err != nil {
		fmt.Printf(" Error: %v\n", err)
	} else {
		fmt.Printf("Result: %s = %s\n", expr, result)
	}
}

func (c *Client) listTools() {
	fmt.Printf("\nAvailable tools (%d):\n", len(c.tools))
	for i, tool := range c.tools {
		fmt.Printf("%d. %s\n   Description: %s\n", i+1, tool.Name, tool.Description)
	}
}

func (c *Client) sendCustomMessage(reader *bufio.Reader) {
	fmt.Print("Enter JSON-RPC message: ")
	jsonMsg, _ := reader.ReadString('\n')
	jsonMsg = strings.TrimSpace(jsonMsg)

	fmt.Println("\nSENDING RAW JSON:")
	fmt.Println(jsonMsg)

	c.stdin.Write([]byte(jsonMsg + "\n"))

	// Wait for response
	select {
	case response, ok := <-c.responseChan:
		if !ok {
			fmt.Println("Server closed connection")
			return
		}
		c.printResponse(response)
	case <-time.After(5 * time.Second):
		fmt.Println(" Timeout waiting for response")
	}
}

func (c *Client) sendMessage(message MCPMessage) error {
	if message.ID != nil {
		c.messageID++
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return err
	}

	fmt.Printf(" SENDING: %s\n", string(jsonData))
	_, err = c.stdin.Write(append(jsonData, '\n'))
	return err
}

func (c *Client) waitForResponse(timeout time.Duration) (*MCPMessage, error) {
	select {
	case responseStr, ok := <-c.responseChan:
		if !ok {
			return nil, fmt.Errorf("server closed connection")
		}

		c.printResponse(responseStr)

		var response MCPMessage
		if err := json.Unmarshal([]byte(responseStr), &response); err != nil {
			return nil, fmt.Errorf("failed to parse response: %v", err)
		}

		return &response, nil

	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting for response")
	}
}

func (c *Client) printResponse(response string) {
	fmt.Println(" SERVER RESPONSE:")

	// Try to pretty print JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(response), &jsonData); err == nil {
		prettyJSON, _ := json.MarshalIndent(jsonData, "", "  ")
		fmt.Println(string(prettyJSON))
	} else {
		fmt.Printf("Raw response: %s\n", response)
	}
}

func (c *Client) callCalculator(expression string) (string, error) {
	// Find calculator tool
	var toolName string
	for _, tool := range c.tools {
		if strings.Contains(tool.Name, "calculate") {
			toolName = tool.Name
			break
		}
	}

	if toolName == "" {
		return "", fmt.Errorf("calculator tool not found")
	}

	message := MCPMessage{
		JSONRPC: "2.0",
		ID:      &c.messageID,
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": toolName,
			"arguments": map[string]string{
				"expression": expression,
			},
		},
	}

	if err := c.sendMessage(message); err != nil {
		return "", err
	}

	response, err := c.waitForResponse(10 * time.Second)
	if err != nil {
		return "", err
	}

	if response.Error != nil {
		return "", fmt.Errorf("tool call failed: %v", response.Error)
	}

	// Extract result from tool response
	if result, ok := response.Result.(map[string]interface{}); ok {
		if content, ok := result["content"].([]interface{}); ok && len(content) > 0 {
			if item, ok := content[0].(map[string]interface{}); ok {
				if text, ok := item["text"].(string); ok {
					return text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("unexpected response format")
}
