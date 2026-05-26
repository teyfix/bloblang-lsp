package bloblang

import (
	"fmt"
	"strings"

	tree_sitter_bloblang "github.com/teyfix/tree-sitter-bloblang/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type parsedTree struct {
	tree *tree_sitter.Tree
	text string
}

// Parser is a stateful incremental tree-sitter parser for Bloblang documents.
// It must be owned by a single goroutine — the FileActor that drives it.
// No locking is needed because the actor processes events sequentially.
type Parser struct {
	parser *tree_sitter.Parser
	trees  map[string]*parsedTree
}

// NewParser creates a Parser with the Bloblang grammar loaded.
func NewParser() (*Parser, error) {
	language := tree_sitter.NewLanguage(tree_sitter_bloblang.Language())
	p := tree_sitter.NewParser()
	if err := p.SetLanguage(language); err != nil {
		return nil, err
	}
	return &Parser{
		parser: p,
		trees:  make(map[string]*parsedTree),
	}, nil
}

// endPoint returns the tree-sitter Point that corresponds to the very end of s.
func endPoint(s string) tree_sitter.Point {
	lines := strings.Split(s, "\n")
	row := len(lines) - 1
	col := len(lines[row])
	return tree_sitter.Point{Row: uint(row), Column: uint(col)}
}

// Parse returns the syntax tree for uri/document, reusing the previous tree
// as an edit base when the text has changed.
//
// The caller must NOT call tree.Close() — the Parser retains ownership of all
// trees and frees them via InvalidateDocument or Close.
func (p *Parser) Parse(uri string, document string) (*tree_sitter.Tree, error) {
	if cached, ok := p.trees[uri]; ok && cached.text == document {
		return cached.tree, nil
	}

	var oldTree *tree_sitter.Tree
	if cached, ok := p.trees[uri]; ok {
		oldTree = cached.tree
		edit := tree_sitter.InputEdit{
			StartByte:      0,
			OldEndByte:     uint(len(cached.text)),
			NewEndByte:     uint(len(document)),
			StartPosition:  tree_sitter.Point{Row: 0, Column: 0},
			OldEndPosition: endPoint(cached.text),
			NewEndPosition: endPoint(document),
		}
		oldTree.Edit(&edit)
	}

	tree := p.parser.Parse([]byte(document), oldTree)
	if tree == nil {
		return nil, fmt.Errorf("could not parse document: tree is nil")
	}

	if oldTree != nil {
		oldTree.Close()
	}

	p.trees[uri] = &parsedTree{tree: tree, text: document}
	return tree, nil
}

// InvalidateDocument frees the retained tree for uri.
func (p *Parser) InvalidateDocument(uri string) {
	if cached, ok := p.trees[uri]; ok {
		if cached.tree != nil {
			cached.tree.Close()
		}
		delete(p.trees, uri)
	}
}

// Close frees all retained trees. Call when the Parser is no longer needed.
func (p *Parser) Close() {
	for k, cached := range p.trees {
		if cached.tree != nil {
			cached.tree.Close()
		}
		delete(p.trees, k)
	}
}
