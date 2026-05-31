package sample

import (
	"sync"

	"github.com/owenrumney/go-lsp/lsp"
	"github.com/teyfix/bloblang-lsp/modular/ast"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type SampleResult struct {
	CodeLens   []lsp.CodeLens
	InlayHint  []lsp.InlayHint
	Diagnostic []lsp.Diagnostic
}

type Snippet struct {
	nodes []tree_sitter.Node
}

func parseSnippet() {

}

func removeMetaAccess(node tree_sitter.Node, diags *[]lsp.Diagnostic) tree_sitter.Node {

	return node
}

func Sample(text string, tree *tree_sitter.Tree) (*SampleResult, error) {
	var wg sync.WaitGroup

	diags := []lsp.Diagnostic{}

	snippet := Snippet{}
	snippets := make([]Snippet, 0)

	for _, node := range tree.RootNode().Children(tree.RootNode().Walk()) {
		snippet.nodes = append(snippet.nodes, removeMetaAccess(node, &diags))

		if node.Kind() == string(ast.RootAssignment) {
			snippets = append(snippets, snippet)
			snippet = Snippet{}
		}
	}

	snippets = append(snippets, snippet)

	return nil, nil
}
