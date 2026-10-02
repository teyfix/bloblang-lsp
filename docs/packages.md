> Historical design notes from an abandoned rewrite. The shipped implementation is `internal/lsp`; see [the current README](../README.md) and source for behavior. These notes are not implementation requirements.

# External Package Reference

This directory describes every external package used in `bloblang-lsp`. Because these packages are niche, these docs are the authoritative guide for implementing agents.

## Packages

| Package | File | Role |
|---|---|---|
| `github.com/owenrumney/go-lsp` | [go-lsp.md](packages/go-lsp.md) | LSP runtime — JSON-RPC server, handler interfaces, protocol types, document store |
| `github.com/tree-sitter/go-tree-sitter` | [go-tree-sitter.md](packages/go-tree-sitter.md) | Tree-sitter C library Go bindings — concrete syntax tree parsing and traversal |
| `github.com/teyfix/tree-sitter-bloblang/bindings/go` | [tree-sitter-bloblang.md](packages/tree-sitter-bloblang.md) | Custom Bloblang grammar — provides the `Language()` pointer for the parser |
| `github.com/redpanda-data/benthos/v4/public/bloblang` | [benthos-bloblang.md](packages/benthos-bloblang.md) | Bloblang runtime — environment, semantic parsing, execution, reflection API |
| `github.com/redpanda-data/connect/v4/public/components/*` | [connect-components.md](packages/connect-components.md) | Connect extensions — blank imports that register additional Bloblang functions/methods |

---

## Cross-Package API Usage by Feature

| Feature | Packages Used | Key APIs |
|---|---|---|
| **Diagnostic** | `tree_sitter`, `bloblang` | `root.HasError()` → traverse for `IsError()`/`IsMissing()` nodes; `env.Parse(text)` for semantic errors; iterate `import_statement` nodes for path resolution |
| **Sample** | `tree_sitter`, `bloblang` | Walk `comment` nodes for `#!sample`/`#!sample_from`; `env.Parse(snippet)` + `executor.Query(sample)` for cumulative execution; `node.StartPosition().Row` for line mapping |
| **Hover** | `tree_sitter`, pre-built maps | `root.DescendantForPointRange(point, point)` → check `node.Kind()` and `node.Parent().Kind()`; look up identifier in `functionDocs` / `methodDocs` maps |
| **Completion** | Pre-built `[]lsp.CompletionItem` | Scan current line text for trigger char context; filter pre-built list by `CompletionItemKindMethod` vs `CompletionItemKindFunction` |
| **Handler** | `server`, `lsp`, `document` | `store.Open/Change/Close`; `client.PublishDiagnostics`; `lsp.*Params` / `lsp.*Result` types |
| **Bootstrap** | `bloblang`, `server`, `config` | `NewEnvironment()` → `WalkFunctions/WalkMethods` → build caches → `server.NewServer` → `s.Run` |
