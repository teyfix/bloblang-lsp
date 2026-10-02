> Historical design notes from an abandoned rewrite. The shipped implementation is `internal/lsp`; see [the current README](../README.md) and source for behavior. These notes are not implementation requirements.

# `github.com/redpanda-data/connect/v4/public/components/*` — Connect Extensions

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

> [!IMPORTANT]
> These are blank imports — **do not call any functions from these packages directly**. They exist solely to trigger `init()` registration. The registered functions then appear automatically in `env.WalkFunctions()` and `env.WalkMethods()` output.

---

## What Each Component Registers

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

> [!NOTE]
> When a user writes a Bloblang mapping that uses `.vector()` or `hash()`, the LSP server validates it correctly because these components are registered. If a component import is removed, that function disappears from completion, hover docs, and semantic validation — causing false "undefined function" diagnostics.

---

## Why They Must Be in `env.go`

Go's `init()` functions run once at program startup per package, in import order. The connect component packages register their functions/methods into the Benthos global registry during their `init()`. `bloblang.GlobalEnvironment()` then returns a snapshot of that fully-populated registry.

If the blank imports are in a different file that is compiled after `env.go`, the registration may or may not have happened by the time `GlobalEnvironment()` is called — this is undefined behaviour. **Always co-locate these imports with the `GlobalEnvironment()` call.**

---

## Adding or Removing Components

To add a new Redpanda Connect component (e.g., `components/kafka`):
1. Add `_ "github.com/redpanda-data/connect/v4/public/components/kafka"` to `env.go`.
2. Run `go mod tidy` to update `go.sum`.
3. The new component's functions/methods will automatically appear in completion, hover, and validation on the next server startup — no other code changes needed.

To remove a component, delete its blank import. Any Bloblang functions it registered will immediately disappear from all LSP features.
