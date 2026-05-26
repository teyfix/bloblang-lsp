package bloblang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// SyntaxDiagnostics checks the document for tree-sitter syntax errors.
func SyntaxDiagnostics(text string, tree *tree_sitter.Tree) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic
	if text != "" && tree != nil {
		root := tree.RootNode()
		if root.HasError() {
			severity := protocol.SeverityError
			source := "bloblang (syntax)"

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
				for i := 0; i < int(node.ChildCount()); i++ {
					child := node.Child(uint(i))
					if child.HasError() {
						collectErrors(child)
					}
				}
			}
			collectErrors(root)
		}
	}
	return diagnostics
}

// EnvDiagnostics parses the text with Benthos's Bloblang environment compiler to find compilation errors.
func EnvDiagnostics(text string, baseDir string) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic
	if text != "" {
		env := bloblang.NewEnvironment().WithCustomImporter(func(name string) ([]byte, error) {
			return os.ReadFile(filepath.Join(baseDir, name))
		})
		_, err := env.Parse(text)
		if err != nil {
			diagnostics = append(diagnostics, convertErrorToDiagnostics(err)...)
		}
	}
	return diagnostics
}

// ImportHintDiagnostics generates Info-level diagnostics reporting resolved path details for import statements.
func ImportHintDiagnostics(uri string, text string, tree *tree_sitter.Tree, baseDir string) []protocol.Diagnostic {
	var diagnostics []protocol.Diagnostic
	if text != "" && tree != nil {
		severity := protocol.SeverityInformation
		source := "bloblang"
		root := tree.RootNode()

		for i := 0; i < int(root.ChildCount()); i++ {
			child := root.Child(uint(i))
			if child.Kind() == "import_statement" {
				var pathStr string
				for j := 0; j < int(child.ChildCount()); j++ {
					gc := child.Child(uint(j))
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
				resolved := filepath.Join(baseDir, pathStr)
				row := int(child.StartPosition().Row)
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range:    protocol.Range{Start: protocol.Position{Line: row, Character: 0}, End: protocol.Position{Line: row, Character: 0}},
					Severity: &severity,
					Source:   source,
					Message:  fmt.Sprintf("Importing from %s", resolved),
				})
			}
		}

		if strings.HasPrefix(uri, "untitled:") {
			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range:    protocol.Range{Start: protocol.Position{Line: 0, Character: 0}, End: protocol.Position{Line: 0, Character: 0}},
				Severity: &severity,
				Source:   source,
				Message:  fmt.Sprintf("Import base directory: %s", baseDir),
			})
		}
	}
	return diagnostics
}

func convertErrorToDiagnostics(err error) []protocol.Diagnostic {
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
