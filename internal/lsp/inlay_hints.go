package lsp

import (
	"context"
	"encoding/json"

	"strings"
	"time"
	"unicode/utf8"

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
		for maxBytes > 0 && !utf8.RuneStart(text[maxBytes]) {
			maxBytes--
		}
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
	if yamlDocument(uri) && !strings.Contains(string(uri), "#bloblang-") {
		host, _ := h.documents.Text(uri)
		out := []protocol.InlayHint{}
		for _, r := range h.regions(uri) {
			cp := *params
			cp.TextDocument.URI = r.uri
			cp.Range = protocol.Range{Start: protocol.Position{}, End: bytePosition(r.text, len(r.text))}
			vals, err := h.InlayHint(ctx, &cp)
			if err != nil {
				continue
			}
			for _, v := range vals {
				v.Position = r.hostPosition(host, v.Position)
				out = append(out, v)
			}
		}
		return out, nil
	}
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

	defer tree.Close()
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
			shortVal, _ := shortValue(sample.Value, h.config.MaxInlineResultBytes)
			hints = append(hints, protocol.InlayHint{
				Position: protocol.Position{
					Line:      int(child.EndPosition().Row),
					Character: int(child.EndPosition().Column),
				},
				Label: rawJSON(" = " + shortVal),
				Tooltip: &protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: h.previewMarkdown(uri, string(rawJSON(sample.Value))),
				},
			})
		}
	}

	// 2. Traverse statements to generate before-assignment and after-assignment inlay hints
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "root_assignment" && child.Kind() != "if_statement" {
			continue
		}

		startRow := int(child.StartPosition().Row)

		// A. Before-Assignment Inlay Hint (at the end of "root" keyword)
		if child.Kind() == "root_assignment" && child.ChildCount() > 0 {
			rootToken := child.Child(0)
			resultBefore, errBefore := h.executor.ExecuteThrough(h.parser, string(uri), sample, text, child.StartByte())
			if errBefore == nil && resultBefore != nil {
				hints = append(hints, protocol.InlayHint{
					Position: protocol.Position{
						Line:      int(rootToken.EndPosition().Row),
						Character: int(rootToken.EndPosition().Column),
					},
					Label: rawJSON(": " + resultBefore.Text),
					Tooltip: &protocol.MarkupContent{
						Kind:  protocol.Markdown,
						Value: h.previewMarkdown(uri, resultBefore.Full),
					},
				})
			}
		}

		// B. After-Assignment Inlay Hint (at the end of statement node)
		resultAfter, errAfter := h.executor.ExecuteThrough(h.parser, string(uri), sample, text, child.EndByte())
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

		if resultAfter != nil {
			hints = append(hints, protocol.InlayHint{
				Position: protocol.Position{
					Line:      int(child.EndPosition().Row),
					Character: int(child.EndPosition().Column),
				},
				Label: rawJSON(" = " + resultAfter.Text),
				Tooltip: &protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: h.previewMarkdown(uri, resultAfter.Full),
				},
			})
		}
	}

	for i := range hints {
		hints[i].Position = rangeUTF16(text, protocol.Range{Start: hints[i].Position, End: hints[i].Position}).Start
	}
	for i := range execErrs {
		execErrs[i].Range = rangeUTF16(text, execErrs[i].Range)
	}
	h.publishExecDiagnostics(ctx, uri, execErrs)
	return hints, nil
}

func (h *Handler) scheduleRefresh(uri protocol.DocumentURI) {
	if h.client != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			if h.refreshInlays {
				_ = h.client.InlayHintRefresh(ctx)
			}
			if h.refreshLenses {
				_ = h.client.CodeLensRefresh(ctx)
			}
		}()
	}
}
