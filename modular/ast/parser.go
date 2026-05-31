package ast

import (
	"fmt"

	tree_sitter_bloblang "github.com/teyfix/tree-sitter-bloblang/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Bloblang struct {
	parser *tree_sitter.Parser
}

func NewBloblang() (*Bloblang, error) {
	language := tree_sitter.NewLanguage(tree_sitter_bloblang.Language())
	parser := tree_sitter.NewParser()

	if err := parser.SetLanguage(language); err != nil {
		return nil, err
	}

	return &Bloblang{parser: parser}, nil
}

// Parse takes a Bloblang document string and returns its syntax tree.
//
// The returned tree's root node has kind "source" and contains zero or more
// statement nodes as direct children. Callers must call tree.Close() when done.
func (b *Bloblang) Parse(uri string, document string) (*tree_sitter.Tree, error) {
	tree := b.parser.Parse([]byte(document), nil)
	if tree == nil {
		return nil, fmt.Errorf("could not parse document: tree is nil")
	}
	return tree, nil
}
