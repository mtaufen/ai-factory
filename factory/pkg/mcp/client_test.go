package mcp

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestClientConnectNamedPipe(t *testing.T) {
	dir := t.TempDir()
	pipePath := filepath.Join(dir, "mcp-pipe")

	if err := syscall.Mkfifo(pipePath, 0666); err != nil {
		t.Fatalf("failed to create fifo: %v", err)
	}

	client := NewClient(pipePath)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
}

type readWriteCloser struct {
	io.Reader
	io.Writer
	close func() error
}

func (rw *readWriteCloser) Close() error {
	return rw.close()
}

func TestClientCallTool(t *testing.T) {
	cRead, sWrite := io.Pipe()
	sRead, cWrite := io.Pipe()

	clientPipe := &readWriteCloser{
		Reader: cRead,
		Writer: cWrite,
		close: func() error {
			cRead.Close()
			cWrite.Close()
			return nil
		},
	}

	serverPipe := &readWriteCloser{
		Reader: sRead,
		Writer: sWrite,
		close: func() error {
			sRead.Close()
			sWrite.Close()
			return nil
		},
	}

	client := NewClientWithPipe(clientPipe)
	defer client.Close()
	defer serverPipe.Close()

	go func() {
		decoder := json.NewDecoder(serverPipe)
		for {
			var req JSONRPCRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}

			res := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
			}

			if req.Method == "tools/call" {
				res.Result = json.RawMessage(`{"content":[{"type":"text","text":"hello world"}],"isError":false}`)
			} else {
				res.Error = &JSONRPCError{Code: -32601, Message: "Method not found"}
			}

			resBytes, _ := json.Marshal(res)
			resBytes = append(resBytes, '\n')
			serverPipe.Write(resBytes)
		}
	}()

	ctx := context.Background()
	result, err := client.CallTool(ctx, "test_tool", map[string]interface{}{})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if len(result.Content) != 1 || result.Content[0].Text != "hello world" {
		t.Errorf("Unexpected result: %+v", result)
	}
}

func TestClientCallToolError(t *testing.T) {
	cRead, sWrite := io.Pipe()
	sRead, cWrite := io.Pipe()

	clientPipe := &readWriteCloser{
		Reader: cRead,
		Writer: cWrite,
		close: func() error {
			cRead.Close()
			cWrite.Close()
			return nil
		},
	}

	serverPipe := &readWriteCloser{
		Reader: sRead,
		Writer: sWrite,
		close: func() error {
			sRead.Close()
			sWrite.Close()
			return nil
		},
	}

	client := NewClientWithPipe(clientPipe)
	defer client.Close()
	defer serverPipe.Close()

	go func() {
		decoder := json.NewDecoder(serverPipe)
		for {
			var req JSONRPCRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}

			res := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &JSONRPCError{Code: -32603, Message: "Internal error"},
			}

			resBytes, _ := json.Marshal(res)
			resBytes = append(resBytes, '\n')
			serverPipe.Write(resBytes)
		}
	}()

	ctx := context.Background()
	_, err := client.CallTool(ctx, "fail_tool", map[string]interface{}{})
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}

	if err.Error() != "jsonrpc error -32603: Internal error" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestConnectionManager(t *testing.T) {
	dir := t.TempDir()
	pipePath := filepath.Join(dir, "mcp-pipe-cm")

	if err := syscall.Mkfifo(pipePath, 0666); err != nil {
		t.Fatalf("failed to create fifo: %v", err)
	}

	cm := NewConnectionManager()
	defer cm.CloseAll()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	client1, err := cm.GetClient(ctx, "server1", pipePath)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	client2, err := cm.GetClient(ctx, "server1", pipePath)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	if client1 != client2 {
		t.Fatalf("Expected same client instance for the same name")
	}

	if err := cm.CloseAll(); err != nil {
		t.Fatalf("CloseAll failed: %v", err)
	}
}

