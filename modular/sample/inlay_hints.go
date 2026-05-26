package sample

import (
	"encoding/json"
	"fmt"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/teyfix/bloblang-lsp/modular/ascii"
	"github.com/teyfix/bloblang-lsp/modular/meta"
	"github.com/teyfix/bloblang-lsp/modular/pretty"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ComputeInlayHints calculates standard sample and evaluation overlays.
func (m *Manager) ComputeInlayHints(uri string, text string, tree *tree_sitter.Tree, baseDir string) ([]protocol.InlayHint, []protocol.Diagnostic) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[uri]
	if state == nil || state.sample == nil || text == "" || tree == nil {
		return []protocol.InlayHint{}, nil
	}

	hints := make([]protocol.InlayHint, 0)
	var execErrs []protocol.Diagnostic
	severity := protocol.SeverityWarning
	root := tree.RootNode()

	// 1. Comment overlays
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "comment" {
			break
		}
		commentText := child.Utf8Text([]byte(text))
		lineText := strings.TrimSpace(commentText)
		if strings.HasPrefix(lineText, "#!sample ") || strings.HasPrefix(lineText, "#!sample_from ") {
			shortVal, prettyVal := shortValue(state.sample.Value, m.config.MaxInlineResultBytes)
			hints = append(hints, protocol.InlayHint{
				Position: protocol.Position{
					Line:      int(child.EndPosition().Row),
					Character: int(child.EndPosition().Column),
				},
				Label: rawJSON(" = " + shortVal),
				Tooltip: &protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: fmt.Sprintf("```json\n%s\n```", prettyVal),
				},
			})
		}
	}

	// 2. Assignment overlays
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "root_assignment" {
			continue
		}

		startRow := int(child.StartPosition().Row)

		// Before-Assignment
		if child.ChildCount() > 0 {
			rootToken := child.Child(0)
			resultBefore, errBefore := m.executor.ExecuteCumulative(m.parser, uri, state.sample.Value, text, startRow-1)
			if errBefore == nil && resultBefore != nil {
				hints = append(hints, protocol.InlayHint{
					Position: protocol.Position{
						Line:      int(rootToken.EndPosition().Row),
						Character: int(rootToken.EndPosition().Column),
					},
					Label: rawJSON(": " + resultBefore.Text),
					Tooltip: &protocol.MarkupContent{
						Kind:  protocol.Markdown,
						Value: fmt.Sprintf("```json\n%s\n```", resultBefore.Full),
					},
				})
			}
		}

		// After-Assignment
		resultAfter, errAfter := m.executor.ExecuteCumulative(m.parser, uri, state.sample.Value, text, startRow)
		if errAfter != nil {
			execErrs = append(execErrs, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: startRow, Character: int(child.StartPosition().Column)},
					End:   protocol.Position{Line: int(child.EndPosition().Row), Character: int(child.EndPosition().Column)},
				},
				Severity: &severity,
				Source:   string(meta.ServerName),
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
					Value: fmt.Sprintf("```json\n%s\n```", resultAfter.Full),
				},
			})
		}
	}

	return hints, execErrs
}

// Helpers

func shortValue(val interface{}, maxBytes int) (string, string) {
	encoded, err := json.Marshal(val)
	if err != nil {
		return "", ""
	}
	full := string(encoded)
	text := full
	if maxBytes >= 0 && len(text) > maxBytes {
		text = text[:maxBytes] + ascii.Ellipsis
	}
	prettyPrinted := strings.TrimRight(string(pretty.PrettyOptions(encoded, &pretty.Options{
		Width:    maxBytes,
		Prefix:   "",
		Indent:   "  ",
		SortKeys: false,
	})), "\n")
	return text, prettyPrinted
}

func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func indentMessage(msg string) string {
	msg = strings.Split(msg, "\n")[0]
	parts := strings.Split(msg, ": ")
	var indentParts []string
	for i, part := range parts {
		indent := strings.Repeat("  ", i)
		if i < len(parts)-1 {
			indentParts = append(indentParts, indent+part+":")
		} else {
			indentParts = append(indentParts, indent+part)
		}
	}
	return strings.Join(indentParts, "\n")
}
