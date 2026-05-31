# `github.com/owenrumney/go-lsp` — LSP Runtime

**Import paths used:**
- `github.com/owenrumney/go-lsp/lsp` — protocol types (alias: `protocol`)
- `github.com/owenrumney/go-lsp/server` — server lifecycle, client push API
- `github.com/owenrumney/go-lsp/document` — document text store

---

## `server` package

This is the JSON-RPC engine. You never write JSON encoding/decoding yourself.

### Starting the server

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

### Handler interfaces (mix-in pattern)

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

### `server.Client` — server-to-client push

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

## `lsp` package — Protocol Types

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
    Position:    lsp.Position{Line: 2, Character: 20},
    Label:       []lsp.InlayHintLabelPart{{Value: " = {\"foo\":\"bar\"}"}},
    Kind:        lsp.InlayHintKindParameter,
    Tooltip:     &lsp.MarkupContent{Kind: lsp.Markdown, Value: "```json\n...```"},
    PaddingLeft: true,
}

// Code lenses
lsp.CodeLens{
    Range: lsp.Range{...},
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

### Initialize handshake

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

## `document` package — Text Store

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
