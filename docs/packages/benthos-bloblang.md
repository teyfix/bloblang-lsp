# `github.com/redpanda-data/benthos/v4/public/bloblang` — Bloblang Runtime

**Import path:** `github.com/redpanda-data/benthos/v4/public/bloblang`

This is the official Benthos Bloblang runtime. It provides: the global function/method registry, semantic parsing and execution, reflection APIs for introspection, and the importer interface for file-based imports.

---

## Environment

`bloblang.Environment` is the entry point for all runtime operations. **Always start from `GlobalEnvironment()`** — it contains the built-in function/method registry. The connect component side-effect imports (see [connect-components.md](connect-components.md)) extend this global registry before `GlobalEnvironment()` is called.

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

## Semantic Parsing & Execution

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

`env.Parse` performs **full semantic validation** — it catches undefined functions, type mismatches, and unresolved map references that the AST alone cannot detect.

---

## Reflection API (Completion & Hover)

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
```

### Parameter introspection

```go
// data.Params.Definitions  []TemplateParamData
// data.Params.Variadic     bool
//
// Each TemplateParamData:
// p.Name               string
// p.ValueType          string  ("string", "integer", "float", "bool", "array", "object", "query expression")
// p.IsOptional         bool
// p.DefaultMarshalled  string  (JSON-encoded default value, empty if none)
// p.Description        string
```

### Example data

```go
// ex.Summary    string
// ex.Mapping    string  (the Bloblang code)
// ex.Results    [][2]string  {input, output} pairs
// ex.SkipTesting bool
```

### Status filter rules

- Skip `data.Status == "hidden"` — internal, never surface to users.
- Skip `data.Status == "deprecated"` unless `includeDeprecated = true`.
- Surface `"stable"`, `"beta"`, `"experimental"` with appropriate UI badges.

| Status | Completion label prefix | Sort priority |
|---|---|---|
| `stable` / `""` | _(none)_ | `0_name` |
| `beta` | `[β] ` | `1_name` |
| `experimental` | `[⚗] ` | `1_name` |
| `deprecated` | `[⚠] ` | `2_name` |
| `hidden` | — | Never shown |
