package lsp

import (
	"context"
	"encoding/json"
	"go.yaml.in/yaml/v3"
	"os"
	"path/filepath"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type formatToken struct {
	text       string
	start, end uint
}

func tokens(n *tree_sitter.Node, source []byte, out *[]formatToken) {
	switch n.Kind() {
	case "string", "comment", "number":
		*out = append(*out, formatToken{n.Utf8Text(source), n.StartByte(), n.EndByte()})
		return
	}
	if n.ChildCount() == 0 {
		if n.EndByte() > n.StartByte() {
			*out = append(*out, formatToken{n.Utf8Text(source), n.StartByte(), n.EndByte()})
		}
		return
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		tokens(n.Child(i), source, out)
	}
}
func (h *Handler) formatText(uri protocol.DocumentURI, text string, tabSize int) (string, bool) {
	env := h.benv.WithCustomImporter(func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(h.baseDirForURI(uri), name)) })
	_, originalErr := env.Parse(text)
	tree, err := h.parser.Parse(string(uri)+":format", text)
	if err != nil || tree == nil {
		return text, false
	}
	defer tree.Close()
	if tree.RootNode().HasError() {

		return text, false
	}
	var ts []formatToken
	tokens(tree.RootNode(), []byte(text), &ts)
	if len(ts) == 0 {
		return text, true
	}
	if tabSize <= 0 {
		tabSize = 2
	}
	if tabSize > 8 {
		tabSize = 8
	}
	indent := 0
	var out strings.Builder
	prev := ""
	var end uint
	for _, t := range ts {
		gap := text[end:t.start]
		newlines := strings.Count(gap, "\n")
		if t.text == "}" || t.text == "]" || t.text == ")" {
			if indent > 0 {
				indent--
			}
		}
		if newlines > 0 {
			if newlines > 2 {
				newlines = 2
			}
			out.WriteString(strings.Repeat("\n", newlines))
			out.WriteString(strings.Repeat(" ", indent*tabSize))
		} else if out.Len() > 0 && formatSpace(prev, t.text) {
			out.WriteByte(' ')
		}
		out.WriteString(t.text)
		if t.text == "{" || t.text == "[" || t.text == "(" {
			indent++
		}
		end = t.end
		prev = t.text
	}
	result := out.String() + "\n"
	// Reparse the formatted result before returning an edit.
	if _, err := env.Parse(result); err != nil && originalErr == nil {

		return text, false
	}
	return result, true
}
func formatSpace(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if strings.HasPrefix(b, "#") {
		return true
	}
	if b == "." || a == "." || a == "$" || a == "@" || a == "!" || b == "," || b == ":" || b == ")" || b == "]" {
		return false
	}
	if b == "(" {
		if a == "if" || a == "match" {
			return true
		}
		return !validIdentifier(a)
	}
	if a == "(" || a == "[" {
		return false
	}
	if a == "{" && b == "}" {
		return false
	}
	if a == ":" || a == "," {
		return true
	}
	if b == "}" || a == "{" {
		return true
	}
	return true
}
func (h *Handler) Formatting(_ context.Context, p *protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	text, ok := h.documents.Text(p.TextDocument.URI)
	if !ok {
		return []protocol.TextEdit{}, nil
	}
	if yamlDocument(p.TextDocument.URI) && !strings.Contains(string(p.TextDocument.URI), "#bloblang-") {
		var edits []protocol.TextEdit
		for _, r := range h.regions(p.TextDocument.URI) {
			if r.interpolation {
				continue
			}
			formatted, valid := h.formatText(r.uri, r.text, p.Options.TabSize)
			if !valid || formatted == r.text {
				continue
			}
			var replacement string
			if r.style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
				indent := r.indent + 2 // key column is the scalar header column; content indentation comes from first mapped byte.
				if len(r.offsets) > 0 {
					pos := bytePosition(text, r.offsets[0])
					indent = pos.Character
				}
				body := strings.TrimSuffix(formatted, "\n")
				replacement = "|-\n" + strings.Repeat(" ", indent) + strings.ReplaceAll(body, "\n", "\n"+strings.Repeat(" ", indent)) + "\n"
			} else {
				b, _ := json.Marshal(strings.TrimSuffix(formatted, "\n"))
				replacement = string(b)
			}
			if replacement == text[r.start:r.end] {
				continue
			}
			edits = append(edits, protocol.TextEdit{Range: protocol.Range{Start: bytePosition(text, r.start), End: bytePosition(text, r.end)}, NewText: replacement})
		}
		return edits, nil
	}
	formatted, valid := h.formatText(p.TextDocument.URI, text, p.Options.TabSize)
	if !valid || formatted == text {
		return []protocol.TextEdit{}, nil
	}
	return []protocol.TextEdit{{Range: protocol.Range{Start: protocol.Position{}, End: bytePosition(text, len(text))}, NewText: formatted}}, nil
}
