package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
)

func (h *Handler) CodeLens(_ context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	uri := params.TextDocument.URI
	sample := h.getSample(uri)
	if sample == nil {
		return []protocol.CodeLens{}, nil
	}
	text, ok := h.documents.Text(uri)
	if !ok {
		return []protocol.CodeLens{}, nil
	}
	lines := strings.Split(text, "\n")
	lenses := make([]protocol.CodeLens, 0)
	for lineIdx, line := range lines {
		if !rootAssignRe.MatchString(line) {
			continue
		}
		result, err := h.executor.ExecutePartial(string(uri), sample.Value, text, lineIdx)
		if err != nil || result == nil {
			continue
		}
		command := protocol.Command{Title: result.Text}
		if result.Truncated {
			command = protocol.Command{
				Title:   fmt.Sprintf("Show full result (%d chars)", len(result.Full)),
				Command: "bloblang-lsp.showResult",
				Arguments: []json.RawMessage{
					rawJSON(string(uri)),
					rawJSON(lineIdx),
					rawJSON(result.Full),
				},
			}
		}
		lenses = append(lenses, protocol.CodeLens{
			Range: protocol.Range{
				Start: protocol.Position{Line: lineIdx, Character: 0},
				End:   protocol.Position{Line: lineIdx, Character: len(line)},
			},
			Command: &command,
		})
	}
	return lenses, nil
}
