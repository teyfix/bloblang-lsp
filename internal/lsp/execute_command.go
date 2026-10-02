package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
)

func (h *Handler) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (any, error) {

	if params.Command == "bloblang-lsp.openFile" && len(params.Arguments) == 1 {
		var uriStr string
		if err := json.Unmarshal(params.Arguments[0], &uriStr); err != nil {
			return nil, err
		}
		return h.client.ShowDocument(ctx, &protocol.ShowDocumentParams{
			URI:       protocol.URI(uriStr),
			TakeFocus: new(true),
		})
	}

	if params.Command == "bloblang-lsp.showResult" && len(params.Arguments) >= 1 {
		var origin protocol.DocumentURI
		if len(params.Arguments) > 1 {
			_ = json.Unmarshal(params.Arguments[1], &origin)
		}
		lang, body := h.preview(origin, string(params.Arguments[0]))

		// 1. Create a temporary JSON file
		tmpFile, err := os.CreateTemp("", "bloblang-sample-*."+lang)
		if err != nil {
			// Log error and fallback to window/showMessage
			return nil, err
		}

		// 2. Write the raw data
		if _, err := tmpFile.WriteString(body + "\n"); err != nil {
			return nil, err
		}
		tmpFile.Close() // Close it so the editor can read it

		// 3. Convert absolute file path to a proper file:// URI
		// (Note: filepath.ToSlash is important for Windows path compatibility)
		tmpPath := filepath.ToSlash(tmpFile.Name())
		if !strings.HasPrefix(tmpPath, "/") {
			tmpPath = "/" + tmpPath // Ensure leading slash for Windows drives (e.g., /C:/...)
		}

		uri := "file://" + tmpPath

		// 4. Send the showDocument request
		return h.client.ShowDocument(ctx, &protocol.ShowDocumentParams{
			URI:       protocol.URI(uri),
			TakeFocus: new(true),
		})
	}

	return nil, fmt.Errorf("unknown command: %s", params.Command)
}
