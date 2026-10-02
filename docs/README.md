# Bloblang LSP

The shipped server is `cmd/bloblang-lsp` and `internal/lsp`. The older `cmd/language-server` command uses the same implementation. One handler tracks open documents and answers LSP requests; it uses the Benthos public parser/runtime and Tree-sitter for source boundaries. The unfinished modular actor rewrite has been removed.

See the repository [README](../README.md) for features, samples, packaging and validation. Source is the reference for behavior. YAML mappings are extracted into independent document regions and mapped back to host positions; the server does not simulate a YAML processor pipeline. Named map definitions and lambda bodies have no unique runtime invocation context, so ambiguous sampled hovers are omitted.

Logs go to stderr. Stdout is reserved for JSON-RPC. Diagnostics publish asynchronously because the go-lsp transport must continue reading client responses. Client refresh requests are sent only when the client advertises support. go-lsp 0.2.5 includes the required `result: null` JSON-RPC response fix.
