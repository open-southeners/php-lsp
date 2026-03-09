package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "project")
}

type lspHarness struct {
	t         *testing.T
	server    *Server
	in        *io.PipeWriter
	out       *io.PipeReader
	nextID    int
	responses chan map[string]interface{}
}

func newHarness(t *testing.T) *lspHarness {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	logger := log.New(io.Discard, "", 0)
	server := NewServer(inR, outW, logger)
	go func() {
		if err := server.Run(); err != nil {
			// Server stopped
		}
	}()
	h := &lspHarness{t: t, server: server, in: inW, out: outR, nextID: 1, responses: make(chan map[string]interface{}, 100)}
	// Background reader: parses all LSP messages from the server and routes
	// responses (messages with an "id") to the responses channel.
	// Notifications are silently discarded.
	go func() {
		reader := bufio.NewReader(outR)
		for {
			var contentLength int
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				line = strings.TrimSpace(line)
				if line == "" {
					break
				}
				if strings.HasPrefix(line, "Content-Length:") {
					fmt.Sscanf(strings.TrimPrefix(line, "Content-Length:"), "%d", &contentLength)
				}
			}
			if contentLength == 0 {
				continue
			}
			body := make([]byte, contentLength)
			if _, err := io.ReadFull(reader, body); err != nil {
				return
			}
			var msg map[string]interface{}
			if json.Unmarshal(body, &msg) != nil {
				continue
			}
			if _, hasID := msg["id"]; hasID {
				h.responses <- msg
			}
		}
	}()
	return h
}

func (h *lspHarness) send(method string, params interface{}) int {
	h.t.Helper()
	id := h.nextID
	h.nextID++
	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	body, _ := json.Marshal(msg)
	fmt.Fprintf(h.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return id
}

func (h *lspHarness) notify(method string, params interface{}) {
	h.t.Helper()
	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	}
	body, _ := json.Marshal(msg)
	fmt.Fprintf(h.in, "Content-Length: %d\r\n\r\n%s", len(body), body)
}

func (h *lspHarness) readResponse(id int) map[string]interface{} {
	h.t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case msg := <-h.responses:
			if respID, ok := msg["id"]; ok {
				var respIDInt int
				switch v := respID.(type) {
				case float64:
					respIDInt = int(v)
				case int:
					respIDInt = v
				}
				if respIDInt == id {
					return msg
				}
			}
		case <-timeout:
			h.t.Fatalf("timeout waiting for response id=%d", id)
			return nil
		}
	}
}

func (h *lspHarness) close() {
	h.in.Close()
}

func TestServerInitialize(t *testing.T) {
	h := newHarness(t)
	defer h.close()

	root := testdataPath()
	id := h.send("initialize", map[string]interface{}{
		"rootUri":      "file://" + root,
		"capabilities": map[string]interface{}{},
		"processId":    nil,
	})

	resp := h.readResponse(id)
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatal("expected result in initialize response")
	}
	caps, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("expected capabilities")
	}
	if caps["hoverProvider"] != true {
		t.Error("expected hoverProvider to be true")
	}
	if caps["completionProvider"] == nil {
		t.Error("expected completionProvider")
	}
	if caps["definitionProvider"] != true {
		t.Error("expected definitionProvider to be true")
	}
	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("expected serverInfo")
	}
	if serverInfo["name"] != "php-lsp" {
		t.Errorf("expected server name 'php-lsp', got %v", serverInfo["name"])
	}
}

func TestServerHoverIntegration(t *testing.T) {
	h := newHarness(t)
	defer h.close()

	root := testdataPath()
	initID := h.send("initialize", map[string]interface{}{
		"rootUri":      "file://" + root,
		"capabilities": map[string]interface{}{},
		"processId":    nil,
	})
	h.readResponse(initID)

	h.notify("initialized", map[string]interface{}{})

	// Wait for async indexing to complete
	time.Sleep(2 * time.Second)

	// Open the Service.php file
	source, err := os.ReadFile(filepath.Join(root, "src", "Service.php"))
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	serviceURI := "file://" + filepath.Join(root, "src", "Service.php")
	h.notify("textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        serviceURI,
			"languageId": "php",
			"version":    1,
			"text":       string(source),
		},
	})

	// Find the position of "info" in "$this->logger->info"
	lines := strings.Split(string(source), "\n")
	var hoverLine, hoverChar int
	for i, line := range lines {
		if strings.Contains(line, "$this->logger->info") {
			hoverLine = i
			hoverChar = strings.Index(line, "info")
			break
		}
	}

	hoverID := h.send("textDocument/hover", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": serviceURI},
		"position":     map[string]interface{}{"line": hoverLine, "character": hoverChar},
	})

	resp := h.readResponse(hoverID)
	result, ok := resp["result"].(map[string]interface{})
	if !ok || result == nil {
		t.Fatal("expected hover result")
	}
	contents, ok := result["contents"].(map[string]interface{})
	if !ok {
		t.Fatal("expected contents in hover result")
	}
	value, ok := contents["value"].(string)
	if !ok {
		t.Fatal("expected value in hover contents")
	}
	if !strings.Contains(value, "function info") {
		t.Errorf("expected method signature in hover, got:\n%s", value)
	}
}

func TestServerShutdown(t *testing.T) {
	h := newHarness(t)
	defer h.close()

	root := testdataPath()
	initID := h.send("initialize", map[string]interface{}{
		"rootUri":      "file://" + root,
		"capabilities": map[string]interface{}{},
		"processId":    nil,
	})
	h.readResponse(initID)

	shutdownID := h.send("shutdown", nil)
	resp := h.readResponse(shutdownID)
	if resp["error"] != nil {
		t.Errorf("expected no error on shutdown, got: %v", resp["error"])
	}
}
