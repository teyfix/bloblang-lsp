package lsp

import (
	"bytes"

	protocol "github.com/owenrumney/go-lsp/lsp"
	pretty "github.com/teyfix/bloblang-lsp/internal/tidwall"
	"go.yaml.in/yaml/v3"
	"strings"
)

func (h *Handler) preview(uri protocol.DocumentURI, raw string) (string, string) {
	c := h.workspaceConfig(uri)
	if c.Preview.Format == "json" {
		return "json", strings.TrimSpace(string(pretty.PrettyOptions([]byte(raw), &pretty.Options{Width: c.Formatter.PrintWidth, Indent: "  "})))
	}
	var n yaml.Node
	// YAML accepts JSON and retains numeric lexemes without float64 rounding.
	if yaml.Unmarshal([]byte(raw), &n) != nil {
		return "text", raw
	}
	var compact func(*yaml.Node, int)
	compact = func(n *yaml.Node, depth int) {
		n.Style = 0
		for _, ch := range n.Content {
			compact(ch, depth+1)
		}
		if n.Kind != yaml.MappingNode && n.Kind != yaml.SequenceNode {
			return
		}
		n.Style = yaml.FlowStyle
		b, e := yaml.Marshal(n)
		if e != nil || strings.Contains(strings.TrimSpace(string(b)), "\n") || len(strings.TrimSpace(string(b)))+2*depth > c.Formatter.PrintWidth {
			n.Style = 0
		}
	}
	compact(&n, 0)
	var b bytes.Buffer
	e := yaml.NewEncoder(&b)
	e.SetIndent(2)
	if e.Encode(&n) != nil {
		return "text", raw
	}
	_ = e.Close()
	return "yaml", strings.TrimSpace(b.String())
}
func (h *Handler) previewMarkdown(uri protocol.DocumentURI, raw string) string {
	lang, body := h.preview(uri, raw)
	return "```" + lang + "\n" + body + "\n```"
}
