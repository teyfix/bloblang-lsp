> Historical design notes from an abandoned rewrite. The shipped implementation is `internal/lsp`; see [the current README](../README.md) and source for behavior. These notes are not implementation requirements.

# `github.com/teyfix/tree-sitter-bloblang/bindings/go` — Bloblang Grammar

**Import path:** `github.com/teyfix/tree-sitter-bloblang/bindings/go` (alias: `tree_sitter_bloblang`)

This is a custom Tree-sitter grammar for the Bloblang language, compiled to CGo bindings. It provides exactly one function:

```go
// Returns a raw C pointer to the TSLanguage struct for the Bloblang grammar.
// Pass to tree_sitter.NewLanguage() to get a *tree_sitter.Language.
tree_sitter_bloblang.Language() unsafe.Pointer
```

**Usage (always in this exact pattern):**
```go
language := tree_sitter.NewLanguage(tree_sitter_bloblang.Language())
parser := tree_sitter.NewParser()
if err := parser.SetLanguage(language); err != nil {
    return nil, err
}
```

**This is the only API this package exposes.** All node kind knowledge comes from the grammar itself — documented in [modular/ast/kind.go](file:///home/dixie/git/teyfix/bloblang-lsp/modular/ast/kind.go) and [internal/benthos/parser/README.md](file:///home/dixie/git/teyfix/bloblang-lsp/internal/benthos/parser/README.md).

---

## Node Kind Reference

For the full node kind table, see [go-tree-sitter.md](go-tree-sitter.md) and the bloblang grammar README. Quick summary of the kinds used most often across features:

**Statements (direct children of `source`):**

| Kind | Example |
|---|---|
| `root_assignment` | `root.doc.id = this.id` |
| `let_assignment` | `let x = this.value` |
| `meta_assignment` | `meta topic = @original` |
| `map_declaration` | `map normalize { root = this }` |
| `import_statement` | `import "./common.blobl"` |
| `comment` | `# text` — also used for `#!sample` directives |

**Expressions (recursive):**

| Kind | Example |
|---|---|
| `method_call` | `this.name.uppercase()` |
| `call_expr` | `uuid_v4()` |
| `field_access` | `this.user.id` |
| `if_expr` | `if this.ok { "yes" } else { "no" }` |
| `match_expr` | `match this.type { "a" => ... }` |
| `binary_expr` | `this.x > 0 && !this.deleted` |

**Atoms:**

| Kind | Example |
|---|---|
| `identifier` | bare names, function names |
| `this_ref` | `this` |
| `variable_ref` | `$my_var` |
| `meta_ref` | `@kafka_topic` |
| `string` | `"hello"` |
| `number` | `42`, `3.14` |
| `boolean` | `true`, `false` |

> [!NOTE]
> The grammar is maintained at `github.com/teyfix/tree-sitter-bloblang`. To update the grammar or add new node kinds, modify that repository and bump the dependency version here.
