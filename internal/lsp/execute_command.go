package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	pretty "github.com/teyfix/bloblang-lsp/internal/tidwall"
)

func (h *Handler) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (any, error) {

	if params.Command == "bloblang-lsp.showResult" && len(params.Arguments) == 1 {
		json := pretty.PrettyOptions(params.Arguments[0], &pretty.Options{
			Width:    h.config.MaxInlineResultBytes,
			Prefix:   "",
			Indent:   "  ",
			SortKeys: false,
		})

		// 1. Create a temporary JSON file
		tmpFile, err := os.CreateTemp("", "bloblang-sample-*.json")
		if err != nil {
			// Log error and fallback to window/showMessage
			return nil, err
		}

		// 2. Write the raw data
		if _, err := tmpFile.Write(json); err != nil {
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
