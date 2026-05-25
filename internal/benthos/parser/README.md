# tree-sitter-bloblang

A [Tree-sitter](https://tree-sitter.github.io/tree-sitter/) grammar for [Bloblang](https://www.benthos.dev/docs/guides/bloblang/about), the data mapping language used by [Benthos](https://www.benthos.dev/) / [Redpanda Connect](https://docs.redpanda.com/redpanda-connect/about/).

## Language Coverage

The grammar supports the full Bloblang surface area:

| Construct | Example |
|---|---|
| Root assignment | `root.doc.id = this.id` |
| Let variable | `let header = this.title` |
| Meta assignment | `meta kafka_topic = @original_topic` |
| Map declaration | `map normalize_user { root = this }` |
| Import | `import "./common_maps.blobl"` |
| If / else | `if this.foo == 1 { "yes" } else { "no" }` |
| Match | `match this.type { "article" => this, _ => deleted() }` |
| Method chain | `this.name.uppercase().trim()` |
| Lambda | `this.roles.map_each(role -> role.uppercase())` |
| Object literal | `{ "id": this.id, "name": this.name }` |
| Array literal | `[this.a, this.b, this.c]` |
| Binary / unary | `this.count > 0 && !this.deleted` |
| Coalescing | `this.foo \| this.bar \| "default"` |
| Variable ref | `$my_var` |
| Meta ref | `@kafka_topic` |
| Deleted | `deleted()` |
| Comments | `# this is a comment` |

## Go Bindings

### Installation

```bash
go get github.com/teyfix/tree-sitter-bloblang
go get github.com/tree-sitter/go-tree-sitter
```

### Usage

```go
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

// Parse takes a Bloblang document string and returns its syntax tree.
// The returned *tree_sitter.Tree can be traversed via tree.RootNode().
// Callers are responsible for calling tree.Close() when done.
func (b *Bloblang) Parse(document string) (*tree_sitter.Tree, error) {
    if tree := b.parser.Parse([]byte(document), nil); tree != nil {
        return tree, nil
    }

    return nil, fmt.Errorf("could not parse document: %s", "tree is nil")
}
```

### Traversing the Tree

```go
b, _ := NewBloblang()
tree, _ := b.Parse(`root.user.id = this.id.string()`)
defer tree.Close()

root := tree.RootNode()

// Walk all children of the root node
for i := range root.ChildCount() {
    child := root.Child(i)
    fmt.Printf("kind=%s  text=%q\n", child.Kind(), child.Utf8Text([]byte(src)))
}
```

### Node Kinds Reference

The following node kinds are produced by the grammar:

**Statements**

| Kind | Description |
|---|---|
| `source` | Top-level document root |
| `root_assignment` | `root[.path]* = <expr>` |
| `let_assignment` | `let <name> = <expr>` |
| `meta_assignment` | `meta [key] = <expr>` |
| `map_declaration` | `map <name> { ... }` |
| `import_statement` | `import "<path>"` |

**Expressions**

| Kind | Description |
|---|---|
| `if_expr` | `if <cond> { ... } else { ... }` |
| `match_expr` | `match [<expr>] { <case>* }` |
| `match_case` | `<pattern> => <result>` |
| `catch_all` | `_` wildcard in match |
| `binary_expr` | `<left> <op> <right>` |
| `unary_expr` | `!<expr>` or `-<expr>` |
| `method_call` | `<expr>.<method>(args...)` |
| `field_access` | `<expr>.<field>` |
| `call_expr` | `<fn>(args...)` |
| `lambda` | `<param> -> <expr>` |
| `object_literal` | `{ <pair>* }` |
| `array_literal` | `[ <expr>* ]` |
| `parenthesized_expr` | `( <expr> )` |

**Primaries / Atoms**

| Kind | Description |
|---|---|
| `this_ref` | `this` |
| `meta_ref` | `@<identifier>` |
| `bare_meta_ref` | `@` |
| `variable_ref` | `$<identifier>` |
| `deleted` | `deleted()` |
| `identifier` | Names, fields, function names |
| `string` | `"..."` or `"""..."""` |
| `number` | Integer or float |
| `boolean` | `true` / `false` |
| `comment` | `# ...` |

## License

MIT — see [LICENSE](./LICENSE) for details.