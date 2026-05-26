package sample

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize"
	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/teyfix/bloblang-lsp/modular/meta"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ComputeCodeLenses calculates open file and cumulative display actions.
func (m *Manager) ComputeCodeLenses(uri string, text string, tree *tree_sitter.Tree, baseDir string) ([]protocol.CodeLens, []protocol.Diagnostic) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[uri]
	if state == nil || state.sample == nil || text == "" || tree == nil {
		return []protocol.CodeLens{}, nil
	}

	lenses := make([]protocol.CodeLens, 0)
	var execErrs []protocol.Diagnostic
	severity := protocol.SeverityWarning
	root := tree.RootNode()

	// 1. Comment open-sample action lenses
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
				resolved := filepath.Join(baseDir, rel)
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
						Command:   meta.CommandOpenFile,
						Arguments: []json.RawMessage{rawJSON(uriStr)},
					},
				})
			}
		}
	}

	// 2. Statement-level display-inspection lenses
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "root_assignment" {
			continue
		}

		startRow := int(child.StartPosition().Row)

		// Cumulative state before
		resultBefore, errBefore := m.executor.ExecuteCumulative(m.parser, uri, state.sample.Value, text, startRow-1)
		if errBefore == nil && resultBefore != nil && resultBefore.Truncated {
			lenses = append(lenses, protocol.CodeLens{
				Range: protocol.Range{
					Start: protocol.Position{Line: startRow, Character: 0},
					End:   protocol.Position{Line: startRow, Character: 0},
				},
				Command: &protocol.Command{
					Title:     fmt.Sprintf("Show Input (%s)", humanize.Bytes(uint64(len(resultBefore.Full)))),
					Command:   meta.CommandShowResult,
					Arguments: []json.RawMessage{[]byte(resultBefore.Full)},
				},
			})
		}

		// Cumulative state after
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

		if resultAfter != nil && resultAfter.Truncated {
			lenses = append(lenses, protocol.CodeLens{
				Range: protocol.Range{
					Start: protocol.Position{Line: startRow, Character: 0},
					End:   protocol.Position{Line: startRow, Character: 0},
				},
				Command: &protocol.Command{
					Title:     fmt.Sprintf("Show Output (%s)", humanize.Bytes(uint64(len(resultAfter.Full)))),
					Command:   meta.CommandShowResult,
					Arguments: []json.RawMessage{[]byte(resultAfter.Full)},
				},
			})
		}
	}

	return lenses, execErrs
}
