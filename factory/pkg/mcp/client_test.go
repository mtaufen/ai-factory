package mcp

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestClient_LifecycleEdgeCases(t *testing.T) {
	cRead, _ := io.Pipe()
	client := NewClientWithPipe(&readWriteCloser{
		Reader: cRead, Writer: io.Discard, close: func() error { return nil },
	})

	// 97-99: Connect already connected
	if err := client.Connect(context.Background()); err != nil {
		t.Fatalf("unexpected error connecting already-connected client: %v", err)
	}

	// Close normally
	client.Close()

	// 119: Close already closed
	if err := client.Close(); err != nil {
		t.Fatalf("unexpected error closing already-closed client: %v", err)
	}

	// 163-165: CallTool on disconnected client
	if _, err := client.CallTool(context.Background(), "test", nil); err == nil {
		t.Fatal("expected error calling tool on disconnected client")
	}
}

func TestClient_ConnectFailure(t *testing.T) {
	// 103-105: Connect broken pipe path
	client := NewClient("/non/existent/path/fifo")
	if err := client.Connect(context.Background()); err == nil {
		t.Fatal("expected Connect() to fail on invalid pipe path")
	}
}

func TestClient_CallToolMarshalError(t *testing.T) {
	cRead, _ := io.Pipe()
	client := NewClientWithPipe(&readWriteCloser{
		Reader: cRead, Writer: io.Discard, close: func() error { return nil },
	})
	defer client.Close()

	// 194-196: Unserializable argument
	_, err := client.CallTool(context.Background(), "test", map[string]interface{}{
		"fn": func() {},
	})
	if err == nil {
		t.Fatal("expected CallTool() to fail on unserializable arguments")
	}
}

func TestClient_CallToolContextCancel(t *testing.T) {
	cRead, _ := io.Pipe()
	client := NewClientWithPipe(&readWriteCloser{
		Reader: cRead, Writer: io.Discard, close: func() error { return nil },
	})
	defer client.Close()

	// 209-210: cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.CallTool(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected CallTool() to fail on canceled context")
	}
}

func TestClient_ReadLoopError(t *testing.T) {
	cRead, sWrite := io.Pipe()
	client := NewClientWithPipe(&readWriteCloser{
		Reader: cRead, Writer: io.Discard, close: func() error { return nil },
	})
	defer client.Close()

	// Send malformed JSON to trigger readLoop error (137-139)
	sWrite.Write([]byte("malformed json payload\n"))

	// Wait briefly for readLoop to process it
	time.Sleep(100 * time.Millisecond)

	// 167-169: CallTool should now fail immediately loading readErr
	_, err := client.CallTool(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected CallTool() to fail due to readLoop error")
	}
}

func TestClient_WriteError(t *testing.T) {
	cRead, _ := io.Pipe()
	sRead, cWrite := io.Pipe()

	// Close the write side immediately so writing fails
	sRead.Close()
	cWrite.Close()

	client := NewClientWithPipe(&readWriteCloser{
		Reader: cRead, Writer: cWrite, close: func() error { return nil },
	})
	defer client.Close()

	// 204-206: Write should fail
	_, err := client.CallTool(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected CallTool() to fail due to closed writer")
	}
}

func TestClient_ResultUnmarshalError(t *testing.T) {
	cRead, sWrite := io.Pipe()
	sRead, cWrite := io.Pipe()

	client := NewClientWithPipe(&readWriteCloser{
		Reader: cRead, Writer: cWrite, close: func() error { return nil },
	})
	defer client.Close()

	go func() {
		decoder := json.NewDecoder(sRead)
		var req JSONRPCRequest
		decoder.Decode(&req)

		// Send valid JSON literal for Result that fails ToolResult unmarshaling (216-218)
		res := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  json.RawMessage(`"valid json but not a tool result"`),
		}
		bytes, _ := json.Marshal(res)
		sWrite.Write(append(bytes, '\n'))
	}()

	_, err := client.CallTool(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected CallTool() to fail on malformed tool result")
	}
}

func TestConnectionManager_GetClientFailure(t *testing.T) {
	cm := NewConnectionManager()
	// 250-252: invalid pipe path causes Connect() to fail inside GetClient
	_, err := cm.GetClient(context.Background(), "test", "/invalid/fifo/path")
	if err == nil {
		t.Fatal("expected GetClient() to fail when Connect() fails")
	}
}

type errorClosingClient struct {
	Client
}

func (e *errorClosingClient) Close() error {
	return fmt.Errorf("close error")
}

func TestConnectionManager_CloseAllError(t *testing.T) {
	cm := NewConnectionManager().(*connectionManager)
	cm.clients["broken"] = &errorClosingClient{}

	if err := cm.CloseAll(); err == nil {
		t.Fatal("expected CloseAll() to return an error")
	}
}


