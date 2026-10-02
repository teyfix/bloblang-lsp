package lsp

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"strings"
	"unicode/utf8"
)

// layout groups choose a flat representation if it fits the remaining columns.
// Hard lines separate statements and keep line comments from swallowing code.
type layout struct {
	text              string
	parts             []layout
	line, hard, group bool
	indent            int
}

func literal(s string) layout          { return layout{text: s} }
func soft(s string) layout             { return layout{text: s, line: true} }
func hardline() layout                 { return layout{line: true, hard: true} }
func concat(ds ...layout) layout       { return layout{parts: ds} }
func nested(d layout, size int) layout { return layout{parts: []layout{d}, indent: size} }
func grouped(d layout) layout          { return layout{parts: []layout{d}, group: true} }
func flatLayout(d layout) (string, bool) {
	if d.hard {
		return "", false
	}
	if d.line || d.parts == nil {
		return d.text, !strings.Contains(d.text, "\n")
	}
	var b strings.Builder
	for _, p := range d.parts {
		s, ok := flatLayout(p)
		if !ok {
			return "", false
		}
		b.WriteString(s)
	}
	return b.String(), true
}
func renderLayout(d layout, width int) string {
	var b strings.Builder
	column := 0
	var render func(layout, int, bool)
	render = func(d layout, indent int, flat bool) {
		indent += d.indent
		if d.group && !flat {
			if s, ok := flatLayout(d); ok && column+utf8.RuneCountInString(s) <= width {
				b.WriteString(s)
				column += utf8.RuneCountInString(s)
				return
			}
		}
		if d.line {
			if flat && !d.hard {
				b.WriteString(d.text)
				column += utf8.RuneCountInString(d.text)
			} else {
				b.WriteByte('\n')
				b.WriteString(strings.Repeat(" ", indent))
				column = indent
			}
			return
		}
		if d.parts == nil {
			b.WriteString(d.text)
			if i := strings.LastIndex(d.text, "\n"); i >= 0 {
				column = utf8.RuneCountInString(d.text[i+1:])
			} else {
				column += utf8.RuneCountInString(d.text)
			}
			return
		}
		for _, p := range d.parts {
			render(p, indent, flat)
		}
	}
	render(d, 0, false)
	return b.String()
}
func edgeTokens(n *tree_sitter.Node, source []byte) (string, string) {
	var ts []formatToken
	tokens(n, source, &ts)
	if len(ts) == 0 {
		return "", ""
	}
	return ts[0].text, ts[len(ts)-1].text
}
func nodeLayout(n *tree_sitter.Node, source []byte, size int) layout {
	if n.ChildCount() == 0 || n.Kind() == "string" || n.Kind() == "comment" || n.Kind() == "number" {
		return literal(n.Utf8Text(source))
	}
	var children []*tree_sitter.Node
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c.EndByte() > c.StartByte() {
			children = append(children, c)
		}
	}
	if len(children) == 0 {
		return literal("")
	}
	kind := n.Kind()
	list := kind == "object_literal" || kind == "array_literal" || kind == "parenthesized_expr" || kind == "call_expr" || kind == "method_call"
	block := kind == "statement_block" || kind == "map_declaration" || kind == "match_expr" || kind == "if_expr"
	var parts []layout
	for i, c := range children {
		if i > 0 {
			prev := children[i-1]
			_, last := edgeTokens(prev, source)
			first, _ := edgeTokens(c, source)
			gap := string(source[prev.EndByte():c.StartByte()])
			separator := literal("")
			if prev.Kind() == "comment" {
				separator = hardline()
			} else if kind == "source" {
				separator = hardline()
				if strings.Count(gap, "\n") > 1 {
					separator = concat(hardline(), hardline())
				}
			} else if block && (last == "{" || first == "}" || benthosStatement(c.Kind()) || c.Kind() == "match_case") {
				separator = hardline()
			} else if list && last == "," {
				separator = soft(" ")
			} else if list && (last == "{" || last == "[" || last == "(") {
				flat := ""
				if last == "{" && first != "}" {
					flat = " "
				}
				separator = soft(flat)
			} else if list && (first == "}" || first == "]" || first == ")") {
				flat := ""
				if first == "}" && last != "{" {
					flat = " "
				}
				separator = soft(flat)
			} else if kind == "binary_expr" && i == 2 {
				separator = soft(" ")
			} else if kind == "unary_expr" {
				separator = literal("")
			} else if c.Kind() == "comment" {
				separator = literal(" ")
				if strings.Contains(gap, "\n") {
					separator = hardline()
				}
			} else if !list && strings.Contains(gap, "\n") && kind != "binary_expr" && kind != "field_access" {
				separator = hardline()
			} else if formatSpace(last, first) {
				separator = literal(" ")
			}
			// Closing delimiters align with the group; interiors indent after opening.
			if (list || block) && (first == "}" || first == "]" || first == ")") {
				parts = append(parts, separator)
			} else if list || block {
				parts = append(parts, nested(separator, size))
			} else {
				parts = append(parts, separator)
			}
		}
		d := nodeLayout(c, source, size)
		first, _ := edgeTokens(c, source)
		if (list || block) && i > 0 && first != "}" && first != "]" && first != ")" {
			d = nested(d, size)
		}
		parts = append(parts, d)
	}
	d := concat(parts...)
	if list || kind == "binary_expr" || kind == "field_access" || kind == "unary_expr" {
		d = grouped(d)
	}
	return d
}
