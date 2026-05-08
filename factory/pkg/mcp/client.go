package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

// JSONRPCRequest represents a JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      uint64      `json:"id"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
	ID      uint64          `json:"id"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

// CallToolRequestParams represents the parameters for tools/call.
type CallToolRequestParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolResult represents the result payload of a tool call.
type ToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ToolContent represents a single content item in a tool result.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Client interface for MCP.
type Client interface {
	Connect(ctx context.Context) error
	Close() error
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error)
}

type client struct {
	pipePath string
	file     io.ReadWriteCloser
	mu       sync.Mutex
	nextID   uint64

	pending map[uint64]chan *JSONRPCResponse
	pMu     sync.Mutex

	readErr atomic.Value
}

// NewClient creates a new MCP Client using a named pipe.
func NewClient(pipePath string) Client {
	return &client{
		pipePath: pipePath,
		pending:  make(map[uint64]chan *JSONRPCResponse),
	}
}

// NewClientWithPipe creates a new MCP Client with a provided pipe (useful for testing).
func NewClientWithPipe(pipe io.ReadWriteCloser) Client {
	c := &client{
		file:    pipe,
		pending: make(map[uint64]chan *JSONRPCResponse),
	}
	go c.readLoop()
	return c
}

func (c *client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.file != nil {
		return nil // already connected
	}

	// Open the named pipe for reading and writing.
	f, err := os.OpenFile(c.pipePath, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		return fmt.Errorf("failed to open pipe %s: %w", c.pipePath, err)
	}
	c.file = f
	go c.readLoop()
	return nil
}

func (c *client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.file != nil {
		err := c.file.Close()
		c.file = nil
		return err
	}
	return nil
}

func (c *client) readLoop() {
	c.mu.Lock()
	file := c.file
	c.mu.Unlock()

	if file == nil {
		return
	}

	decoder := json.NewDecoder(file)
	for {
		var res JSONRPCResponse
		if err := decoder.Decode(&res); err != nil {
			if err != io.EOF {
				c.mu.Lock()
				if c.file != nil {
					c.readErr.Store(fmt.Errorf("read error: %w", err))
				}
				c.mu.Unlock()
			}
			break
		}

		c.pMu.Lock()
		ch, ok := c.pending[res.ID]
		if ok {
			delete(c.pending, res.ID)
		}
		c.pMu.Unlock()

		if ok {
			ch <- &res
		}
	}
}

func (c *client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolResult, error) {
	c.mu.Lock()
	file := c.file
	c.mu.Unlock()

	if file == nil {
		return nil, fmt.Errorf("not connected")
	}

	if errVal := c.readErr.Load(); errVal != nil {
		return nil, errVal.(error)
	}

	id := atomic.AddUint64(&c.nextID, 1)
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params: CallToolRequestParams{
			Name:      name,
			Arguments: args,
		},
		ID: id,
	}

	ch := make(chan *JSONRPCResponse, 1)
	c.pMu.Lock()
	c.pending[id] = ch
	c.pMu.Unlock()

	defer func() {
		c.pMu.Lock()
		delete(c.pending, id)
		c.pMu.Unlock()
	}()

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	// json-rpc messages over standard input/output are separated by newlines, or handled by an encoder.
	reqBytes = append(reqBytes, '\n')

	c.mu.Lock()
	_, err = file.Write(reqBytes)
	c.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("write error: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Error != nil {
			return nil, res.Error
		}
		var toolRes ToolResult
		if err := json.Unmarshal(res.Result, &toolRes); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tool result: %w", err)
		}
		return &toolRes, nil
	}
}

// ConnectionManager manages long-lived MCP client connections.
type ConnectionManager interface {
	GetClient(ctx context.Context, name string, pipePath string) (Client, error)
	CloseAll() error
}

type connectionManager struct {
	mu      sync.Mutex
	clients map[string]Client
}

// NewConnectionManager creates a new ConnectionManager.
func NewConnectionManager() ConnectionManager {
	return &connectionManager{
		clients: make(map[string]Client),
	}
}

func (m *connectionManager) GetClient(ctx context.Context, name string, pipePath string) (Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.clients[name]; ok {
		return c, nil
	}

	c := NewClient(pipePath)
	if err := c.Connect(ctx); err != nil {
		return nil, err
	}

	m.clients[name] = c
	return c, nil
}

func (m *connectionManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var lastErr error
	for _, c := range m.clients {
		if err := c.Close(); err != nil {
			lastErr = err
		}
	}
	m.clients = make(map[string]Client)
	return lastErr
}
