# `github.com/tree-sitter/go-tree-sitter` — Tree-sitter Go Bindings

**Import path:** `github.com/tree-sitter/go-tree-sitter` (alias: `tree_sitter`)

Tree-sitter is a C library wrapped with CGo. It parses source code into a concrete syntax tree (CST) incrementally.

---

## Core Types

```
Parser   →  (parse text)  →  Tree   →  (root node)  →  Node
```

### `Parser`

```go
parser := tree_sitter.NewParser()
if err := parser.SetLanguage(language); err != nil { ... }

// Parse full text (no old tree = fresh parse):
tree := parser.Parse([]byte(text), nil)

// Incremental reparse (pass old tree after calling tree.Edit()):
tree := parser.Parse([]byte(newText), oldTree)
```

> [!IMPORTANT]
> `tree_sitter.Parser` is **NOT goroutine-safe**. One `Parser` instance must be used from a single goroutine at a time. In this project, the `ast.Bloblang` wrapper owns one parser per document actor — ensuring single-goroutine access.

### `Tree`

```go
root := tree.RootNode()    // *Node — top-level "source" node

// MUST call Close() to free C memory:
defer tree.Close()

// Incremental reparse: edit the old tree to reflect a text change first
tree.Edit(&tree_sitter.InputEdit{
    StartByte:      0,
    OldEndByte:     uint(len(oldText)),
    NewEndByte:     uint(len(newText)),
    StartPosition:  tree_sitter.Point{Row: 0, Column: 0},
    OldEndPosition: endPoint(oldText),
    NewEndPosition: endPoint(newText),
})
newTree := parser.Parse([]byte(newText), tree) // pass edited old tree
tree.Close()  // close the old tree after reparse
```

### `Node` — the primary type for feature logic

```go
node.Kind()          // string — e.g. "root_assignment", "identifier", "comment"
node.IsNamed()       // false for anonymous tokens (keywords, punctuation)
node.IsError()       // true if this node is a syntax error
node.IsMissing()     // true if the parser inserted this node to recover
node.HasError()      // true if this node or any descendant has an error

// Positions (0-indexed row/column):
node.StartPosition() // tree_sitter.Point{Row, Column}
node.EndPosition()   // tree_sitter.Point{Row, Column}
node.StartByte()     // uint — byte offset
node.EndByte()       // uint — byte offset

// Extract text:
node.Utf8Text([]byte(documentText))  // string — the source text this node spans

// Tree navigation:
node.Parent()                         // *Node or nil
node.Child(i)                         // *Node — i-th child (0-indexed)
node.ChildCount()                     // uint
node.NamedChildCount()                // uint — excludes anonymous children
node.NamedChild(i)                    // *Node — i-th named child
node.ChildByFieldName("value")        // *Node — by grammar field name

// Cursor-based children iteration (no allocation):
cursor := node.Walk()
node.Children(cursor)                 // []Node — all children

// Point-based lookup (for hover/completion cursor position):
node.DescendantForPointRange(
    tree_sitter.Point{Row: line, Column: col},
    tree_sitter.Point{Row: line, Column: col},
)  // *Node — deepest node covering the given point range
```

### `Point`

```go
tree_sitter.Point{Row: uint(line), Column: uint(char)}
// Row and Column are 0-indexed, matching LSP Position.Line and Position.Character
```

---

## Key Patterns for This Project

### Error traversal (diagnostic feature)

```go
root := tree.RootNode()
if root.HasError() {
    var traverse func(n *tree_sitter.Node)
    traverse = func(n *tree_sitter.Node) {
        if n.IsError() {
            // generate Diagnostic: severity=Error, message="Syntax error"
        }
        if n.IsMissing() {
            // generate Diagnostic: message="Missing <n.Kind()>"
        }
        for i := uint(0); i < n.ChildCount(); i++ {
            traverse(n.Child(i))
        }
    }
    traverse(root)
}
```

### Finding node under cursor (hover/completion feature)

```go
point := tree_sitter.Point{Row: uint(line), Column: uint(col)}
node := root.DescendantForPointRange(point, point)
if node == nil { return nil, nil }
// node.Kind() identifies what the cursor is on
```

### Walking top-level statements (executor/sample feature)

```go
root := tree.RootNode()
for i := uint(0); i < root.ChildCount(); i++ {
    child := root.Child(i)
    if child.StartPosition().Row > throughLine { break }
    switch child.Kind() {
    case "root_assignment", "let_assignment", "meta_assignment":
        // collect for cumulative execution
    case "import_statement":
        // extract path for diagnostic
    case "comment":
        // scan for #!sample directives
    }
}
```
