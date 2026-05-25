package benthos

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

func (b *Bloblang) Parse(document string) (*tree_sitter.Tree, error) {
	if tree := b.parser.Parse([]byte(document), nil); tree != nil {
		return tree, nil
	}

	return nil, fmt.Errorf("could not parse document: %s", "tree is nil")
}
