# Completion

The handler builds function and method snippets and documentation from the Benthos environment at startup. Completion triggers are `.`, `@`, and `$`.

A dot selects method completions. When a valid sample provides a reliable receiver value, the server filters type-specific methods and can add object-field suggestions. Generic methods remain available. Without a reliable sample result, all static method completions remain available.

`$` suggests visible local variables using AST scopes and declaration order. `@` suggests keys from the sample metadata. Other expression contexts return function completions. Embedded mappings forward requests through their mapped document regions; source is in [`completion.go`](../../internal/lsp/completion.go) and [`Handler.Completion`](../../internal/lsp/handler.go).
