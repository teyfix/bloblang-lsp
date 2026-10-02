# Hovers and previews

[`Handler.Hover`](../../internal/lsp/handler.go) uses Tree-sitter to identify the selected token and Benthos to evaluate sample values. Function and method names show documentation, examples, and a documentation link. `deleted()` has documentation even though it is a special grammar node; its marker removes a field or filters a message according to where it is assigned. Expression-only YAML fields such as `check: !errored()` also support documentation.

With a valid sample, hovers can show fields, expressions, variable references, metadata, and the value assigned by a `let` declaration. Hovering the left-hand `root` of an assignment shows the preceding output state. Before any assignment, that preview shows the input or explicit sample root. Right-hand `root` retains Benthos's actual output semantics and may be uninitialized.

Reached conditional branches preserve preceding statements and conditions. Unreached branches and named-map or lambda contexts without an unambiguous runtime invocation omit dynamic values. Missing or invalid samples leave documentation available.

Values, inlay tooltips, and Show Input/Output use the same renderer in [`preview.go`](../../internal/lsp/preview.go). YAML is the default; `preview.format: "json"` selects JSON. Both use `formatter.printWidth` to collapse small collections and expand larger values. Inline labels have a separate `max_inline_result_bytes` startup limit. Show Input/Output opens a temporary file with the selected format's extension.

Positions use LSP UTF-16 columns and embedded YAML regions map ranges back to the host document. See [sample configuration](sample.md) and [workspace settings](../core/settings.md).
