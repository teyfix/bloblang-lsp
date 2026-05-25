package benthos

import (
	"fmt"
	"sync"

	tree_sitter_bloblang "github.com/teyfix/tree-sitter-bloblang/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Bloblang struct {
	parser *tree_sitter.Parser
	trees  map[string]*tree_sitter.Tree
	mu     sync.Mutex
}

func NewBloblang() (*Bloblang, error) {
	language := tree_sitter.NewLanguage(tree_sitter_bloblang.Language())
	parser := tree_sitter.NewParser()

	if err := parser.SetLanguage(language); err != nil {
		return nil, err
	}

	return &Bloblang{
		parser: parser,
		trees:  make(map[string]*tree_sitter.Tree), // initialize the map
	}, nil
}

// Parse takes a Bloblang document string and returns its syntax tree.
//
// The returned tree's root node has kind "source" and contains zero or more
// statement nodes as direct children. Callers must call tree.Close() when done.
//
// Statement node kinds (direct children of source):
//
//   - root_assignment   root[.path]* = <expr>
//   - let_assignment    let <name> = <expr>
//   - meta_assignment   meta [key] = <expr>
//   - map_declaration   map <name> { <statement>* }
//   - import_statement  import "<path>"
//
// Expression node kinds (appear recursively as field "value" or sub-expressions):
//
//   - if_expr            if <cond> { <expr> } [else { <expr> } | else <if_expr>]
//   - match_expr         match [<expr>] { <match_case>* }
//   - match_case           <pattern|catch_all> => <expr>
//   - binary_expr        <expr> (> < >= <= == != && || + - * / % |) <expr>
//   - unary_expr         (! | -) <expr>
//   - method_call        <expr>.<method>(<args...>)
//   - field_access       <expr>.<field>
//   - call_expr          <fn>(<args...>)
//   - lambda             <param> -> <expr>
//   - object_literal     { <pair>* }   where pair holds fields "key" and "value"
//   - array_literal      [ <expr>* ]
//   - parenthesized_expr ( <expr> )
//
// Atom / primary node kinds:
//
//   - this_ref      "this"
//   - meta_ref      @<identifier>
//   - bare_meta_ref "@" with no identifier
//   - variable_ref  $<identifier>
//   - deleted       deleted()
//   - identifier    bare name (field, function, map name, …)
//   - string        "…" or """…"""
//   - number        integer or float
//   - boolean       true | false
//   - catch_all     _ (only inside match_case)
//   - comment       # … (attached as an extra, can appear anywhere)
//
// Named fields on key nodes:
//
//	root_assignment : path*, value
//	let_assignment  : name, value
//	meta_assignment : key (optional), value
//	map_declaration : name
//	if_expr         : condition, consequence, alternative (optional)
//	match_expr      : condition (optional)
//	match_case      : pattern, result
//	binary_expr     : left, operator, right
//	unary_expr      : operator, argument
//	method_call     : object, method
//	field_access    : object, field
//	call_expr       : function
//	lambda          : param, body
//	pair            : key, value
func (b *Bloblang) Parse(uri string, document string) (*tree_sitter.Tree, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	tree := b.parser.Parse([]byte(document), b.trees[uri])
	if tree == nil {
		return nil, fmt.Errorf("could not parse document: tree is nil")
	}

	if old := b.trees[uri]; old != nil {
		// free the C-allocated old tree before replacing
		old.Close()
	}

	b.trees[uri] = tree
	return tree, nil
}

func (b *Bloblang) InvalidateDocument(uri string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if tree := b.trees[uri]; tree != nil {
		tree.Close()
	}
	delete(b.trees, uri)
}

// Close frees all retained trees. Call when the Bloblang instance is no longer needed.
func (b *Bloblang) Close(uri string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for uri, tree := range b.trees {
		tree.Close()
		delete(b.trees, uri)
	}
}
