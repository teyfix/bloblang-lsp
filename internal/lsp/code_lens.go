package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	protocol "github.com/owenrumney/go-lsp/lsp"
)

func (h *Handler) CodeLens(ctx context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
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
	var execErrs []protocol.Diagnostic
	severity := protocol.SeverityWarning
	for lineIdx, line := range lines {
		if !rootAssignRe.MatchString(line) {
			continue
		}
		result, err := h.executor.ExecuteCumulative(string(uri), sample.Value, text, lineIdx-1)
		if err != nil {
			execErrs = append(execErrs, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: lineIdx, Character: 0},
					End:   protocol.Position{Line: lineIdx, Character: len(line)},
				},
				Severity: &severity,
				Source:   "bloblang",
				Message:  err.Error(),
			})
			continue
		}
		if result == nil {
			continue
		}
		command := protocol.Command{Title: result.Text}
		if result.Truncated {
			command = protocol.Command{
				Title:     fmt.Sprintf("Show full result (%s)", humanize.Bytes(uint64(len(result.Full)))),
				Command:   "bloblang-lsp.showResult",
				Arguments: []json.RawMessage{[]byte(result.Full)},
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
	h.publishExecDiagnostics(ctx, uri, execErrs)
	return lenses, nil
}
