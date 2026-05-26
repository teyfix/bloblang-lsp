package lsp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func (h *Handler) validateDocument(ctx context.Context, uri protocol.DocumentURI) {
	text, ok := h.documents.Text(uri)
	if !ok {
		return
	}

	diagnostics := append([]protocol.Diagnostic{}, h.sampleDiagnosticsFor(uri)...)
	if text != "" {
		hasSyntaxError := false
		tree, parseErr := h.parser.Parse(string(uri), text)
		if parseErr == nil && tree != nil {
			root := tree.RootNode()
			if root.HasError() {
				hasSyntaxError = true
				severity := protocol.SeverityError
				source := "bloblang"

				var collectErrors func(*tree_sitter.Node)
				collectErrors = func(node *tree_sitter.Node) {
					if node.IsError() {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: int(node.StartPosition().Row), Character: int(node.StartPosition().Column)},
								End:   protocol.Position{Line: int(node.EndPosition().Row), Character: int(node.EndPosition().Column)},
							},
							Severity: &severity,
							Source:   source,
							Message:  "Syntax error",
						})
						return
					}
					if node.IsMissing() {
						diagnostics = append(diagnostics, protocol.Diagnostic{
							Range: protocol.Range{
								Start: protocol.Position{Line: int(node.StartPosition().Row), Character: int(node.StartPosition().Column)},
								End:   protocol.Position{Line: int(node.EndPosition().Row), Character: int(node.EndPosition().Column)},
							},
							Severity: &severity,
							Source:   source,
							Message:  fmt.Sprintf("Missing %s", node.Kind()),
						})
						return
					}
					for i := uint(0); i < node.ChildCount(); i++ {
						child := node.Child(i)
						if child.HasError() {
							collectErrors(child)
						}
					}
				}
				collectErrors(root)
			}
		}

		if !hasSyntaxError {
			env := h.benv.WithCustomImporter(func(name string) ([]byte, error) {
				return os.ReadFile(filepath.Join(h.baseDirForURI(uri), name))
			})
			_, err := env.Parse(text)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				diagnostics = append(diagnostics, convertErrorToDiagnostics(text, err)...)
			}
		}
	}

	diagnostics = append(diagnostics, h.importHintDiagnostics(uri, text)...)
	if ctx.Err() != nil {
		return
	}
	h.publishDiagnostics(ctx, uri, diagnostics)
	if h.validationHook != nil {
		h.validationHook(uri, diagnostics)
	}
}

func convertErrorToDiagnostics(_ string, err error) []protocol.Diagnostic {
	severity := protocol.SeverityError
	source := "bloblang"

	rawErr := ""
	if prettyErr, ok := err.(*bloblang.ParseError); ok {
		rawErr = prettyErr.ErrorMultiline()
	} else {
		rawErr = err.Error()
	}

	var line, char int
	idx := strings.Index(rawErr, "line ")
	if idx == -1 {
		return []protocol.Diagnostic{{
			Range:    protocol.Range{Start: protocol.Position{Line: 0, Character: 0}, End: protocol.Position{Line: 0, Character: 0}},
			Severity: &severity,
			Message:  rawErr,
			Source:   source,
		}}
	}

	if _, scanErr := fmt.Sscanf(rawErr[idx:], "line %d char %d", &line, &char); scanErr != nil {
		return []protocol.Diagnostic{{
			Range:    protocol.Range{Start: protocol.Position{Line: 0, Character: 0}, End: protocol.Position{Line: 0, Character: 0}},
			Severity: &severity,
			Message:  scanErr.Error(),
			Source:   source,
		}}
	}

	cleanMessage := rawErr
	if parts := strings.SplitN(rawErr, ": ", 2); len(parts) == 2 {
		cleanMessage = parts[1]
	}
	if parts := strings.SplitN(cleanMessage, ": ", 2); strings.HasPrefix(cleanMessage, "line ") && len(parts) == 2 {
		cleanMessage = parts[1]
	}
	cleanMessage = strings.Split(cleanMessage, "\n")[0]

	if line > 0 {
		line--
	}
	if char > 0 {
		char--
	}

	return []protocol.Diagnostic{{
		Range: protocol.Range{
			Start: protocol.Position{Line: line, Character: char},
			End:   protocol.Position{Line: line, Character: char},
		},
		Severity: &severity,
		Message:  cleanMessage,
		Source:   source,
	}}
}

func (h *Handler) publishDiagnostics(ctx context.Context, uri protocol.DocumentURI, diagnostics []protocol.Diagnostic) {
	if diagnostics == nil {
		diagnostics = []protocol.Diagnostic{}
	}
	h.lastDiagnosticsMu.Lock()
	h.lastDiagnostics[uri] = append([]protocol.Diagnostic(nil), diagnostics...)
	h.lastDiagnosticsMu.Unlock()
	if h.client == nil {
		return
	}
	if err := h.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{URI: uri, Diagnostics: diagnostics}); err != nil {
		h.logger.Debug("publish diagnostics failed", "uri", uri, "err", err)
	}
}

func uriToPath(uri protocol.DocumentURI) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	p, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", err
	}
	if os.PathSeparator == '\\' && len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	return filepath.FromSlash(p), nil
}

func (h *Handler) importHintDiagnostics(uri protocol.DocumentURI, text string) []protocol.Diagnostic {
	severity := protocol.SeverityInformation
	source := "bloblang"
	var diagnostics []protocol.Diagnostic

	tree, parseErr := h.parser.Parse(string(uri), text)
	if parseErr == nil && tree != nil {
		root := tree.RootNode()
		for i := uint(0); i < root.ChildCount(); i++ {
			child := root.Child(i)
			if child.Kind() == "import_statement" {
				var pathStr string
				for j := uint(0); j < child.ChildCount(); j++ {
					gc := child.Child(j)
					if gc.Kind() == "string" {
						pathStr = gc.Utf8Text([]byte(text))
						break
					}
				}
				if pathStr == "" {
					continue
				}
				pathStr = strings.Trim(pathStr, "\"`")
				if strings.HasPrefix(pathStr, `"""`) && strings.HasSuffix(pathStr, `"""`) {
					pathStr = pathStr[3 : len(pathStr)-3]
				}
				resolved := filepath.Join(h.baseDirForURI(uri), pathStr)
				row := int(child.StartPosition().Row)
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range:    protocol.Range{Start: protocol.Position{Line: row, Character: 0}, End: protocol.Position{Line: row, Character: 0}},
					Severity: &severity,
					Source:   source,
					Message:  fmt.Sprintf("Importing from %s", resolved),
				})
			}
		}
	}

	if strings.HasPrefix(string(uri), "untitled:") {
		diagnostics = append(diagnostics, protocol.Diagnostic{
			Range:    protocol.Range{Start: protocol.Position{Line: 0, Character: 0}, End: protocol.Position{Line: 0, Character: 0}},
			Severity: &severity,
			Source:   source,
			Message:  fmt.Sprintf("Import base directory: %s", h.baseDirForURI(uri)),
		})
	}
	return diagnostics
}
