package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBinaryStartup(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bloblang-lsp.exe")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "."
	out, err := build.CombinedOutput()
	require.NoError(t, err, string(out))

	cmd := exec.Command(bin)
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	writeMessage(t, stdin, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"processId":    nil,
			"capabilities": map[string]interface{}{},
		},
	})
	response := readMessage(t, stdout)
	require.Contains(t, response, "capabilities")

	writeMessage(t, stdin, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": "shutdown"})
	_ = readMessage(t, stdout)
	require.NoError(t, stdin.Close())

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("server did not exit after stdin closed")
	case <-done:
	}
}

func writeMessage(t *testing.T, w io.Writer, msg map[string]interface{}) {
	t.Helper()
	body, err := json.Marshal(msg)
	require.NoError(t, err)
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	require.NoError(t, err)
}

func readMessage(t *testing.T, r io.Reader) string {
	t.Helper()
	br := bufio.NewReader(r)
	contentLength := 0
	for {
		line, err := br.ReadString('\n')
		require.NoError(t, err)
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		_, _ = fmt.Sscanf(line, "Content-Length: %d", &contentLength)
	}
	require.Positive(t, contentLength)
	body := make([]byte, contentLength)
	_, err := io.ReadFull(br, body)
	require.NoError(t, err)
	return string(body)
}
