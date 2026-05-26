package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	protocol "github.com/owenrumney/go-lsp/lsp"
	pretty "github.com/teyfix/bloblang-lsp/internal/tidwall"
)

func shortValue(val interface{}, maxBytes int) (string, string) {
	encoded, err := json.Marshal(val)
	if err != nil {
		return "", ""
	}
	full := string(encoded)
	text := full
	if maxBytes >= 0 && len(text) > maxBytes {
		text = text[:maxBytes] + "…"
	}
	prettyPrinted := strings.TrimRight(string(pretty.PrettyOptions(encoded, &pretty.Options{
		Width:    maxBytes,
		Prefix:   "",
		Indent:   "  ",
		SortKeys: false,
	})), "\n")
	return text, prettyPrinted
}

func (h *Handler) InlayHint(ctx context.Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	uri := params.TextDocument.URI
	sample := h.getSample(uri)
	if sample == nil {
		return []protocol.InlayHint{}, nil
	}
	text, ok := h.documents.Text(uri)
	if !ok {
		return []protocol.InlayHint{}, nil
	}

	hints := make([]protocol.InlayHint, 0)
	var execErrs []protocol.Diagnostic
	severity := protocol.SeverityWarning

	tree, parseErr := h.parser.Parse(string(uri), text)
	if parseErr != nil || tree == nil {
		return []protocol.InlayHint{}, nil
	}

	root := tree.RootNode()

	// 1. Traverse direct children of root to find sample comment nodes
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "comment" {
			// A non-comment node means we reached statements; no sample comments allowed after statements
			break
		}
		commentText := child.Utf8Text([]byte(text))
		lineText := strings.TrimSpace(commentText)
		if strings.HasPrefix(lineText, "#!sample ") || strings.HasPrefix(lineText, "#!sample_from ") {
			shortVal, prettyVal := shortValue(sample.Value, h.config.MaxInlineResultBytes)
			hints = append(hints, protocol.InlayHint{
				Position: protocol.Position{
					Line:      int(child.EndPosition().Row),
					Character: int(child.EndPosition().Column),
				},
				Label:    rawJSON(" = " + shortVal),
				Tooltip: &protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: fmt.Sprintf("```json\n%s\n```", prettyVal),
				},
			})
		}
	}

	// 2. Traverse statements to generate before-assignment and after-assignment inlay hints
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "root_assignment" {
			continue
		}

		startRow := int(child.StartPosition().Row)

		// A. Before-Assignment Inlay Hint (at the end of "root" keyword)
		if child.ChildCount() > 0 {
			rootToken := child.Child(0)
			resultBefore, errBefore := h.executor.ExecuteCumulative(h.parser, string(uri), sample.Value, text, startRow-1)
			if errBefore == nil && resultBefore != nil {
				hints = append(hints, protocol.InlayHint{
					Position: protocol.Position{
						Line:      int(rootToken.EndPosition().Row),
						Character: int(rootToken.EndPosition().Column),
					},
					Label:    rawJSON(": " + resultBefore.Text),
					Tooltip: &protocol.MarkupContent{
						Kind:  protocol.Markdown,
						Value: fmt.Sprintf("```json\n%s\n```", resultBefore.Full),
					},
				})
			}
		}

		// B. After-Assignment Inlay Hint (at the end of statement node)
		resultAfter, errAfter := h.executor.ExecuteCumulative(h.parser, string(uri), sample.Value, text, startRow)
		if errAfter != nil {
			execErrs = append(execErrs, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: startRow, Character: int(child.StartPosition().Column)},
					End:   protocol.Position{Line: int(child.EndPosition().Row), Character: int(child.EndPosition().Column)},
				},
				Severity: &severity,
				Source:   "bloblang",
				Message:  errAfter.Error(),
			})
			continue
		}

		if resultAfter != nil {
			hints = append(hints, protocol.InlayHint{
				Position: protocol.Position{
					Line:      int(child.EndPosition().Row),
					Character: int(child.EndPosition().Column),
				},
				Label:    rawJSON(" = " + resultAfter.Text),
				Tooltip: &protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: fmt.Sprintf("```json\n%s\n```", resultAfter.Full),
				},
			})
		}
	}

	h.publishExecDiagnostics(ctx, uri, execErrs)
	return hints, nil
}

func (h *Handler) scheduleRefresh(uri protocol.DocumentURI) {
	h.inlayLensMu.Lock()
	if cancel, ok := h.inlayCancel[uri]; ok {
		cancel()
	}
	if cancel, ok := h.lensCancel[uri]; ok {
		cancel()
	}
	inlayCtx, inlayCancel := context.WithCancel(context.Background())
	lensCtx, lensCancel := context.WithCancel(context.Background())
	h.inlayCancel[uri] = inlayCancel
	h.lensCancel[uri] = lensCancel
	h.inlayLensMu.Unlock()

	go h.refreshAfter(inlayCtx, true)
	go h.refreshAfter(lensCtx, false)
}

func (h *Handler) refreshAfter(ctx context.Context, inlay bool) {
	select {
	case <-time.After(h.config.InlineResultDebounce):
		if h.client == nil || ctx.Err() != nil {
			return
		}
		if inlay {
			_ = h.client.InlayHintRefresh(ctx)
		} else {
			_ = h.client.CodeLensRefresh(ctx)
		}
	case <-ctx.Done():
	}
}

func (h *Handler) cancelRefresh(uri protocol.DocumentURI) {
	h.inlayLensMu.Lock()
	defer h.inlayLensMu.Unlock()
	if cancel, ok := h.inlayCancel[uri]; ok {
		cancel()
		delete(h.inlayCancel, uri)
	}
	if cancel, ok := h.lensCancel[uri]; ok {
		cancel()
		delete(h.lensCancel, uri)
	}
}
