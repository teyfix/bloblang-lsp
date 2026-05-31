# External Package Reference

This document describes every external package used in `bloblang-lsp`, explaining what APIs to use, why each package is imported, and how they interact. Because these packages are niche, this is the authoritative guide for implementing agents.

---

## 1. `github.com/owenrumney/go-lsp` — LSP Runtime

**Import paths used:**
- `github.com/owenrumney/go-lsp/lsp` — protocol types (alias: `protocol`)
- `github.com/owenrumney/go-lsp/server` — server lifecycle, client push API
- `github.com/owenrumney/go-lsp/document` — document text store

---

### 1.1 `server` package

This is the JSON-RPC engine. You never write JSON encoding/decoding yourself.

#### Starting the server

```go
s := server.NewServer(handler,
    server.WithLogger(logger),
    server.WithCapabilityOptions(server.CapabilityOptions{
        Completion: &lsp.CompletionOptions{
            TriggerCharacters: []string{".", "@", "$"},
        },
        ExecuteCommand: &lsp.ExecuteCommandOptions{
            Commands: []string{"bloblang/showResult", "bloblang/openFile"},
        },
    }),
)
if err := s.Run(ctx, server.RunStdio()); err != nil {
    log.Fatal(err)
}
```

`server.RunStdio()` returns an `io.ReadWriteCloser` wired to `os.Stdin` / `os.Stdout`. **Never write to stdout yourself — it corrupts the RPC stream.**

#### Handler interfaces (mix-in pattern)

`go-lsp` uses interface detection at dispatch time — the server checks which interfaces your handler implements at runtime and routes accordingly. You do NOT need to implement all of them. The only **mandatory** interface is:

```go
// Must be implemented by all servers.
type LifecycleHandler interface {
    Initialize(ctx context.Context, params *lsp.InitializeParams) (*lsp.InitializeResult, error)
    Shutdown(ctx context.Context) error
}
```

All other capabilities are opt-in by implementing their interface:

| Interface | Method(s) | LSP method(s) |
|---|---|---|
| `ClientHandler` | `SetClient(*server.Client)` | Called before first dispatch |
| `SetTraceHandler` | `SetTrace` | `$/setTrace` |
| `TextDocumentSyncHandler` | `DidOpen`, `DidChange`, `DidClose` | `textDocument/did*` |
| `CompletionHandler` | `Completion` | `textDocument/completion` |
| `HoverHandler` | `Hover` | `textDocument/hover` |
| `InlayHintHandler` | `InlayHint` | `textDocument/inlayHint` |
| `CodeLensHandler` | `CodeLens` | `textDocument/codeLens` |
| `ExecuteCommandHandler` | `ExecuteCommand` | `workspace/executeCommand` |

**Implement `ClientHandler`** — it gives you the `*server.Client` for pushing async diagnostics.

#### `server.Client` — server-to-client push

The client is injected via `SetClient`. It provides async push notifications:

```go
// Push diagnostics asynchronously (call from any goroutine)
err := client.PublishDiagnostics(ctx, &lsp.PublishDiagnosticsParams{
    URI:         uri,
    Diagnostics: []lsp.Diagnostic{...},
})

// Trigger a refresh of inlay hints on the client side
err := client.InlayHintRefresh(ctx)
```

> [!IMPORTANT]
> `PublishDiagnostics` is the **only** way to push diagnostics. The `Diagnostic` handler interfaces are for pull-based diagnostics — which this server does not use.

---

### 1.2 `lsp` package — Protocol Types

All LSP protocol structs live here. Key types used across all features:

```go
// Core identification
lsp.DocumentURI    // string alias — always use this, never plain string for URIs

// Positions (all 0-indexed)
lsp.Position{Line: 0, Character: 0}
lsp.Range{Start: lsp.Position{...}, End: lsp.Position{...}}

// Diagnostics
lsp.Diagnostic{
    Range:    lsp.Range{...},
    Severity: &severity,           // *lsp.DiagnosticSeverity
    Source:   "bloblang",
    Message:  "Syntax error",
}
// Severity values:
lsp.SeverityError       // 1
lsp.SeverityWarning     // 2
lsp.SeverityInformation // 3
lsp.SeverityHint        // 4

// Hover response
lsp.Hover{
    Contents: lsp.MarkupContent{
        Kind:  lsp.Markdown,   // or lsp.PlainText
        Value: "# title\n\nMarkdown body",
    },
    Range: &lsp.Range{...},   // highlights token in editor; optional but recommended
}

// Completion
lsp.CompletionList{Items: []lsp.CompletionItem{...}}
lsp.CompletionItem{
    Label:            "functionName",
    Kind:             &kind,              // *lsp.CompletionItemKind
    Detail:           "fn(a, b) — Function",
    Documentation:    &lsp.MarkupContent{Kind: lsp.Markdown, Value: "..."},
    InsertText:       "functionName(${1:arg})",
    InsertTextFormat: &snippetFormat,     // lsp.InsertTextFormat(2) for snippets
    SortText:         "0_functionName",
    Tags:             []lsp.CompletionItemTag{lsp.CompletionItemTag(1)}, // deprecated
}
lsp.CompletionItemKindFunction  // for functions
lsp.CompletionItemKindMethod    // for methods

// Inlay hints
lsp.InlayHint{
    Position: lsp.Position{Line: 2, Character: 20},
    Label:    []lsp.InlayHintLabelPart{{Value: " = {\"foo\":\"bar\"}"}},
    Kind:     lsp.InlayHintKindParameter,
    Tooltip:  &lsp.MarkupContent{Kind: lsp.Markdown, Value: "```json\n...```"},
    PaddingLeft: true,
}

// Code lenses
lsp.CodeLens{
    Range:   lsp.Range{...},
    Command: &lsp.Command{
        Title:     "Open Sample",
        Command:   "bloblang/openFile",
        Arguments: []any{absolutePath},
    },
}

// Execute command
lsp.ExecuteCommandParams{
    Command:   "bloblang/showResult",
    Arguments: []json.RawMessage{...},
}
```

#### Initialize handshake

```go
func (h *Handler) Initialize(_ context.Context, params *lsp.InitializeParams) (*lsp.InitializeResult, error) {
    // Extract workspace root from params
    if len(params.WorkspaceFolders) > 0 {
        // use params.WorkspaceFolders[0].URI
    } else if params.RootURI != nil {
        // use *params.RootURI
    }

    openClose := true
    return &lsp.InitializeResult{
        Capabilities: lsp.ServerCapabilities{
            TextDocumentSync: &lsp.TextDocumentSyncOptions{
                OpenClose: &openClose,
                Change:    lsp.SyncFull,  // receive full document text on every change
            },
            HoverProvider:     new(true),
            InlayHintProvider: &lsp.InlayHintOptions{},
            CodeLensProvider:  &lsp.CodeLensOptions{},
            // Completion and ExecuteCommand declared via server.WithCapabilityOptions
        },
        ServerInfo: &lsp.ServerInfo{Name: "bloblang-lsp", Version: "v0.0.1"},
    }, nil
}
```

---

### 1.3 `document` package — Text Store

`document.Store` is a thread-safe map of open documents. The `DocumentActor` uses this to access raw text snapshots.

```go
store := document.NewStore()

// In DidOpen handler:
doc, err := store.Open(params)          // registers the doc; returns snapshot
text := doc.Text()                      // full document text

// In DidChange handler:
doc, err := store.Change(params)        // applies SyncFull replacement; returns snapshot
text := doc.Text()

// In DidClose handler:
store.Close(params)                     // removes from store

// Direct text access (used by actor on demand):
text, ok := store.Text(uri)

// Direct get (returns snapshot):
doc, ok := store.Get(uri)
```

> [!NOTE]
> `store.Get` / `store.Text` are goroutine-safe (uses internal `sync.RWMutex`). Since we use `SyncFull`, every `DidChange` delivers the complete new document text in `params.ContentChanges[0].Text` — no incremental patching needed.

---

## 2. `github.com/tree-sitter/go-tree-sitter` — Tree-sitter Go Bindings

**Import path:** `github.com/tree-sitter/go-tree-sitter` (alias: `tree_sitter`)

Tree-sitter is a C library wrapped with CGo. It parses source code into a concrete syntax tree (CST) incrementally.

---

### 2.1 Core Types

```
Parser   →  (parse text)  →  Tree   →  (root node)  →  Node
```

#### `Parser`

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

#### `Tree`

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

#### `Node` — the primary type for feature logic

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

#### `Point`

```go
tree_sitter.Point{Row: uint(line), Column: uint(char)}
// Row and Column are 0-indexed, matching LSP Position.Line and Position.Character
```

---

### 2.2 Key Patterns for This Project

**Error traversal (diagnostic feature):**
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

**Finding node under cursor (hover/completion feature):**
```go
point := tree_sitter.Point{Row: uint(line), Column: uint(col)}
node := root.DescendantForPointRange(point, point)
if node == nil { return nil, nil }
// node.Kind() identifies what the cursor is on
```

**Walking top-level statements (executor/sample feature):**
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

---

## 3. `github.com/teyfix/tree-sitter-bloblang/bindings/go` — Bloblang Grammar

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

**This is the only API this package exposes.** All node kind knowledge comes from the grammar itself — documented in `modular/ast/kind.go` and `docs/` feature specs.

> [!NOTE]
> The grammar is maintained at `github.com/teyfix/tree-sitter-bloblang`. For the full node kind reference, see [modular/ast/kind.go](file:///home/dixie/git/teyfix/bloblang-lsp/modular/ast/kind.go) and [internal/benthos/parser/README.md](file:///home/dixie/git/teyfix/bloblang-lsp/internal/benthos/parser/README.md).

---

## 4. `github.com/redpanda-data/benthos/v4/public/bloblang` — Bloblang Runtime

**Import path:** `github.com/redpanda-data/benthos/v4/public/bloblang`

This is the official Benthos Bloblang runtime. It provides: the global function/method registry, semantic parsing and execution, reflection APIs for introspection, and the importer interface for file-based imports.

---

### 4.1 Environment

`bloblang.Environment` is the entry point for all runtime operations. **Always start from `GlobalEnvironment()`** — it contains the built-in function/method registry. The connect component side-effect imports (see §5) extend this global registry before `GlobalEnvironment()` is called.

```go
// Build the environment (call once at server startup):
env := bloblang.GlobalEnvironment()

// Wrap with a custom importer (controls how `import "..."` statements resolve):
env = env.WithCustomImporter(func(name string) ([]byte, error) {
    // name is the import path string from the Bloblang source
    // Return file bytes or an error
    return os.ReadFile(filepath.Join(baseDir, name))
})
```

> [!IMPORTANT]
> The `NewEnvironment()` in `env.go` creates an environment with an **error-returning importer** — this is intentional for the global shared environment used for semantic validation and completion cache building. A per-document environment with the correct `baseDir` importer must be created in the `diagnostic` feature at parse time.

---

### 4.2 Semantic Parsing & Execution

```go
// Compile a Bloblang mapping string into an executor.
// Returns an error if the mapping has syntax or semantic errors.
executor, err := env.Parse(mappingText)
if err != nil {
    // err.Error() contains the error message, typically in the form:
    // "line X char Y: <message>"
    // Parse the line/char from the error string for LSP Diagnostic positioning.
}

// Execute the compiled mapping against a sample input.
// sample is any JSON-compatible Go value (map[string]any, etc.)
result, err := executor.Query(sample)
// result is any — marshal to JSON for display
```

**Important**: `env.Parse` performs full semantic validation — it catches undefined functions, type mismatches, and unresolved map references that the AST alone cannot detect.

---

### 4.3 Reflection API (Completion & Hover)

The reflection API lets you enumerate all registered functions and methods at runtime. This is how the completion cache and hover documentation are built at server startup.

```go
// Walk all registered functions:
env.WalkFunctions(func(name string, view *bloblang.FunctionView) {
    data := view.TemplateData()  // bloblang.TemplateFunctionData
    // data.Name        string
    // data.Description string   (Markdown)
    // data.Params      bloblang.TemplateParamsData
    // data.Examples    []bloblang.TemplateExampleData
    // data.Version     string   (Benthos version when introduced)
    // data.Status      string   ("stable", "beta", "experimental", "deprecated", "hidden")
    // data.Category    string
})

// Walk all registered methods:
env.WalkMethods(func(name string, view *bloblang.MethodView) {
    data := view.TemplateData()  // bloblang.TemplateMethodData
    // data.Name        string
    // data.Description string
    // data.Params      bloblang.TemplateParamsData
    // data.Examples    []bloblang.TemplateExampleData
    // data.Version     string
    // data.Status      string
    // data.Categories  []bloblang.TemplateMethodCategoryData
})

// Parameter introspection:
// data.Params.Definitions  []TemplateParamData
// data.Params.Variadic     bool
//
// Each TemplateParamData:
// p.Name               string
// p.ValueType          string  ("string", "integer", "float", "bool", "array", "object", "query expression")
// p.IsOptional         bool
// p.DefaultMarshalled  string  (JSON-encoded default value, empty if none)
// p.Description        string

// Example data:
// ex.Summary    string
// ex.Mapping    string  (the Bloblang code)
// ex.Results    [][2]string  {input, output} pairs
// ex.SkipTesting bool
```

**Filter rules:**
- Skip `data.Status == "hidden"` — internal, never surface.
- Skip `data.Status == "deprecated"` unless `includeDeprecated = true`.

---

## 5. `github.com/redpanda-data/connect/v4/public/components/*` — Connect Extensions

**Import paths** (all imported as blank `_` for side effects only):

```go
_ "github.com/redpanda-data/connect/v4/public/components/crypto"
_ "github.com/redpanda-data/connect/v4/public/components/ffi"
_ "github.com/redpanda-data/connect/v4/public/components/io"
_ "github.com/redpanda-data/connect/v4/public/components/msgpack"
_ "github.com/redpanda-data/connect/v4/public/components/pure"
_ "github.com/redpanda-data/connect/v4/public/components/pure/extended"
_ "github.com/redpanda-data/connect/v4/public/components/sql/base"
_ "github.com/redpanda-data/connect/v4/public/components/text"
```

These imports **must appear in the same file that calls `bloblang.GlobalEnvironment()`** (`env.go`). Each package's `init()` function registers additional Bloblang functions and methods into the global registry. If any are missing, those functions won't appear in completion, hover, or semantic validation.

### What each component adds

| Import | What it registers |
|---|---|
| `components/pure` | Core standard library — string manipulation, math, JSON, `this`, `root`, `deleted()`, `if/match`, etc. **Required baseline.** |
| `components/pure/extended` | Additional pure functions — `parse_yaml`, `parse_xml`, `format_timestamp`, etc. |
| `components/crypto` | Cryptographic functions: `hash`, `hmac`, `encrypt_aes`, `decrypt_aes`, JWT utilities. |
| `components/io` | I/O-related functions: `file` (reads files), `env` (env var access), `hostname`. |
| `components/ffi` | Foreign function interface — allows calling external processes from Bloblang. |
| `components/msgpack` | MessagePack encode/decode: `pack`, `unpack`. |
| `components/sql/base` | SQL-adjacent methods including `.vector()` — transforms a value into a vector embedding format for database insertion (e.g., pgvector). |
| `components/text` | Text processing: `re_find_all`, `re_find_all_submatch`, `parse_csv`, `format_csv`, NLP utilities. |

> [!IMPORTANT]
> These are blank imports — **do not call any functions from these packages directly**. They exist solely to trigger `init()` registration. The registered functions then appear automatically in `env.WalkFunctions()` and `env.WalkMethods()` output.

> [!NOTE]
> When a user writes a Bloblang mapping that uses `.vector()` or `hash()`, the LSP server validates it correctly because these components are registered. If a component import is removed, that function disappears from completion, hover docs, and semantic validation — causing false "undefined function" diagnostics.

---

## 6. Cross-Package API Usage by Feature

| Feature | Packages Used | Key APIs |
|---|---|---|
| **Diagnostic** | `tree_sitter`, `bloblang` | `root.HasError()` → traverse for `IsError()`/`IsMissing()` nodes; `env.Parse(text)` for semantic errors; iterate `import_statement` nodes for path resolution |
| **Sample** | `tree_sitter`, `bloblang` | Walk `comment` nodes for `#!sample`/`#!sample_from`; `env.Parse(snippet)` + `executor.Query(sample)` for cumulative execution; `node.StartPosition().Row` for line mapping |
| **Hover** | `tree_sitter`, pre-built maps | `root.DescendantForPointRange(point, point)` → check `node.Kind()` and `node.Parent().Kind()`; look up identifier in `functionDocs` / `methodDocs` maps |
| **Completion** | Pre-built `[]lsp.CompletionItem` | Scan current line text for trigger char context; filter pre-built list by `CompletionItemKindMethod` vs `CompletionItemKindFunction` |
| **Handler** | `server`, `lsp`, `document` | `store.Open/Change/Close`; `client.PublishDiagnostics`; `lsp.*Params` / `lsp.*Result` types |
| **Bootstrap** | `bloblang`, `server`, `config` | `NewEnvironment()` → `WalkFunctions/WalkMethods` → build caches → `server.NewServer` → `s.Run` |
