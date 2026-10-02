# Bloblang Language Server

A single Go LSP binary for Bloblang and mappings embedded in Redpanda Connect YAML. The VS Code extension in `../vscode-bloblang` bundles this binary and supplies language highlighting.

## Editor features

- Benthos syntax and semantic diagnostics, plus nonblocking sample problems.
- Formatting that preserves tokens, comments and string content; malformed syntax is left unchanged.
- Function and method documentation, sampled field/expression values, before-assignment `root` previews, and output inlay tooltips.
- Definition and references for local variables and named maps (including literal `.apply("name")` calls), import paths and YAML `mapping: from "path"` links.
- Function/method snippets, local variable/metadata completion, and sampled receiver guidance. Method filtering applies only when a valid sample yields a reliable receiver value; otherwise all static completions remain available.
- YAML literal, folded, plain and quoted mappings under `mapping`, `request_map`, `result_map`, `args_mapping`, `fields_mapping`, `check` and `bloblang`, plus `${! ... }` expressions. Host YAML formatting is preserved; use the extension's explicit **Format Embedded Mappings** command.

Each embedded mapping is evaluated independently. A sample describes that mapping's input; the server does not infer intermediate processor inputs from a pipeline. Hover inside reached `if` branches retains branch conditions and preceding statements. Named maps and per-element/lambda contexts with no selected invocation do not show invented runtime values.

## Samples

For `mapping.blobl`, the default is a sibling `mapping.sample.json`, `.yaml` or `.yml`. Files must have a `$bloblang` envelope:

```json
{"$bloblang":{"input":{"name":"Ada"},"meta":{"topic":"people"}}}
```

```yaml
$bloblang:
  input:
    name: Ada
  meta:
    topic: people
```

`input` is required and may be `null`. `meta` is optional and must be an object. Multiple siblings are ambiguous; select an entire sample explicitly. Missing default files produce an Information issue suggesting a sample; missing explicit files, malformed samples and bad directives produce Error issues. These issues disable dynamic previews and leave static diagnostics, formatting, completions and navigation available.

Leading comment directives are dispatched through an internal handler registry:

```bloblang
#!input {"name":"Ada"}
#!meta {"topic":"people"}
root.name = this.name.uppercase()
```

`#!sample {input: {name: Ada}, meta: {topic: people}}` supplies an entire inline sample, without the `$bloblang` envelope. It overrides automatic sibling discovery, including invalid or ambiguous default files. Partial `input` and `meta` directives override their fields in a valid default sample, in document order.

`input_from`, `meta_from`, `sample_from` and `root_from` load JSON/YAML files. **Every `_from` file requires the `$bloblang` envelope**; each handler extracts its corresponding field. Paths resolve relative to the mapping or host YAML file. Sample/import changes refresh open mappings.

Multiline bodies remain valid Bloblang comments:

```bloblang
#!sample |
#| input:
#|   name: Ada
#| meta:
#|   topic: people
root.name = this.name
```

A body ends when `#|` continuation lines end. Empty arguments, unknown directives and duplicate directive names are errors at the header. Directives belong before mapping statements.

For `result_map` overlays, optional `$bloblang.root` or `#!root` seeds a separate output target. This selects Benthos `BloblangMutateFrom`, while `this` continues to read `input`:

```bloblang
#!input {"result":{"video":"ready"}}
#!root {"provider_file_ref":"video"}
root.status = this.result.get(root.provider_file_ref)
```

Metadata enters a public `service.Message`, so `meta("topic")` and `@topic` work. Metadata assignments update preview output metadata. Each prefix starts from a fresh sample. The first assignment's left-hand `root` tooltip shows the input (or explicit target); later left-hand tooltips show the prior prefix output. Right-hand `root` follows Benthos's current output semantics and can be uninitialized before the first assignment. `this` retains the input within that mapping.

Migration from the old sample syntax: change `#!sample {"name":"Ada"}` to `#!input {"name":"Ada"}`, or `#!sample {"input":{"name":"Ada"}}`. Wrap old raw sample files under `$bloblang.input`.

## Build and package

Requires Go 1.26.3+, a C compiler, and the sibling `../tree-sitter-bloblang` checkout. The relative `go.mod` replacement picks up the current grammar. Tree-sitter's Go binding still uses generated C; the bundled extension binary needs no source checkout or compiler at runtime.

```sh
go test ./...
go build -a -o target/bloblang-lsp ./cmd/bloblang-lsp
```

Use `-a` after regenerating the sibling grammar: its included `parser.c` lives outside the Go package directory and Go's normal package cache can miss the change. In `../vscode-bloblang`, run `bun run package` to build a platform VSIX with the binary included. The extension can also launch a configured external binary for development.

To verify a private mapping corpus without changing files:

```sh
BLOBLANG_CORPUS_LIST=/path/to/newline-delimited-files.txt go test ./internal/lsp -run TestFormatterCorpus -v
```

The server implements full document sync, completion, hover, formatting, definition, references, diagnostics, inlay hints and code lenses. Configuration uses `.bloblangrc` and `BLOBLANG_LSP_*` variables; see `internal/config/config.go`. Both CLI entrypoints write logs to stderr and speak LSP on stdin/stdout.
