package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
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

	lenses := make([]protocol.CodeLens, 0)
	var execErrs []protocol.Diagnostic
	severity := protocol.SeverityWarning

	tree, parseErr := h.parser.Parse(string(uri), text)
	if parseErr != nil || tree == nil {
		return []protocol.CodeLens{}, nil
	}

	root := tree.RootNode()

	// 1. Traverse comments to find sample_from directives
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "comment" {
			break
		}
		commentText := child.Utf8Text([]byte(text))
		lineText := strings.TrimSpace(commentText)
		if strings.HasPrefix(lineText, "#!sample_from ") {
			rel := strings.TrimSpace(strings.TrimPrefix(lineText, "#!sample_from "))
			if rel != "" {
				resolved := filepath.Join(h.baseDirForURI(uri), rel)
				uriStr := "file://" + filepath.ToSlash(resolved)
				if !strings.HasPrefix(uriStr, "file:///") {
					uriStr = "file:///" + strings.TrimPrefix(filepath.ToSlash(resolved), "/")
				}
				row := int(child.StartPosition().Row)
				lenses = append(lenses, protocol.CodeLens{
					Range: protocol.Range{
						Start: protocol.Position{Line: row, Character: 0},
						End:   protocol.Position{Line: row, Character: 0},
					},
					Command: &protocol.Command{
						Title:     "[Open Sample]",
						Command:   "bloblang-lsp.openFile",
						Arguments: []json.RawMessage{rawJSON(uriStr)},
					},
				})
			}
		}
	}

	// 2. Traverse statements to generate statement level Show Input / Show Output code lenses
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "root_assignment" {
			continue
		}

		startRow := int(child.StartPosition().Row)

		// Calculate cumulative state BEFORE this assignment
		resultBefore, errBefore := h.executor.ExecuteCumulative(h.parser, string(uri), sample.Value, text, startRow-1)
		if errBefore != nil {
			continue
		}

		if resultBefore != nil && resultBefore.Truncated {
			lenses = append(lenses, protocol.CodeLens{
				Range: protocol.Range{
					Start: protocol.Position{Line: startRow, Character: 0},
					End:   protocol.Position{Line: startRow, Character: 0},
				},
				Command: &protocol.Command{
					Title:     fmt.Sprintf("Show Input (%s)", humanize.Bytes(uint64(len(resultBefore.Full)))),
					Command:   "bloblang-lsp.showResult",
					Arguments: []json.RawMessage{[]byte(resultBefore.Full)},
				},
			})
		}

		// Calculate cumulative state AFTER this assignment
		resultAfter, errAfter := h.executor.ExecuteCumulative(h.parser, string(uri), sample.Value, text, startRow)
		if errAfter != nil {
			execErrs = append(execErrs, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: startRow, Character: int(child.StartPosition().Column)},
					End:   protocol.Position{Line: int(child.EndPosition().Row), Character: int(child.EndPosition().Column)},
				},
				Severity: &severity,
				Source:   "bloblang",
				Message:  indentMessage(errAfter.Error()),
			})
			continue
		}

		if resultAfter != nil && resultAfter.Truncated {
			lenses = append(lenses, protocol.CodeLens{
				Range: protocol.Range{
					Start: protocol.Position{Line: startRow, Character: 0},
					End:   protocol.Position{Line: startRow, Character: 0},
				},
				Command: &protocol.Command{
					Title:     fmt.Sprintf("Show Output (%s)", humanize.Bytes(uint64(len(resultAfter.Full)))),
					Command:   "bloblang-lsp.showResult",
					Arguments: []json.RawMessage{[]byte(resultAfter.Full)},
				},
			})
		}
	}

	h.publishExecDiagnostics(ctx, uri, execErrs)
	return lenses, nil
}
