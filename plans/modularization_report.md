# Architectural Report: Bloblang LSP Modularization Design

This report documents the architectural redesign, design rationales, and modularization blueprints completed for the `bloblang-lsp` project.

---

## 1. Goal & Motivation

The original codebase of the language server (`internal/lsp`) suffered from monolithic coupling, where the core JSON-RPC handlers, Abstract Syntax Tree (AST) parsing, diagnostic mapping, and inline execution evaluations were tightly interwoven. 

Our goal was to **fully modularize and design a decoupled, thread-safe, reactive architecture**. This design encapsulates core infrastructure away from localized feature engines, establishing a highly clean blueprint so that future developer agents can implement or extend capabilities in a plug-and-play fashion without spilling logic or creating import cycles.

---

## 2. Key Design Decisions & "Why"

To achieve these goals, we established several modern system-level patterns:

### 1. Pure Stdio Isolation (`logger.go`)
* **Decision**: All system-level logging uses `log/slog` structured logs directed strictly to `os.Stderr`.
* **Why**: Standard `os.Stdout` is reserved exclusively for the JSON-RPC message exchange. Any stray prints or trace logs outputting to standard output corrupt the editor's protocol parser and crash the integration loop.

### 2. The Mutex-Free Hierarchical Actor Model (`concurrency.md`)
* **Decision**: Eliminate standard reader/writer mutex locks on document buffers. Instead, each open document is assigned a single-threaded **Document Actor** goroutine. All edits and commands are dispatched sequentially via a Go channel mailbox.
* **Why**: Serialization of edits and state checks within a single goroutine completely prevents race conditions, resource starvation, and complex lock contentions.

### 3. Generic Attribute API & Reduction (`generic_api.md`)
* **Decision**: Formulated the `AttributeProducer` interface to decouple state-contributing features (`sample` and `diagnostic`). 
* **Why**: Multiple distinct features often provide overlapping document attributes (e.g., both compile-time checks and sample directives produce diagnostics). By defining a generic API, the `DocumentActor` triggers them concurrently, receives their outputs, and **reduces (merges)** them into a single publish payload. This allows newly added features to be registered dynamically without mutating the orchestrator logic.

### 4. Direct Synchronous Query Listeners (`generic_api.md`)
* **Decision**: Pure synchronous queries (Hover, Completion) bypass the reduction loop and listen directly on dedicated actor-routed query channels.
* **Why**: This provides direct, microsecond execution pathways for time-sensitive editor interactions.

### 5. Nonce Filtering & Zero-Context-Cancellation (`generic_api.md`)
* **Decision**: All operations execute under **5ms** (microseconds on average). We explicitly omitted Go `context.Context` cancellation patterns, relying instead on unique **Nonces** (transaction identifiers).
* **Why**: Cancellation of microsecond functions introduces premature complexity and CPU overhead. Instead, if the user types rapidly, the handler uses transaction nonces to **drop stale responses** quietly, ensuring the editor state is always aligned with the latest request.

### 6. Hierarchical Refactoring (`docs/`)
* **Decision**: Refactored the flat documentation files into a structured directory hierarchy (`docs/core/` and `docs/features/`), indexed by a central `docs/README.md` portal.
* **Why**: Isolates structural system plumbing from specialized capability engines, making it easy for future contributors to navigate the documentation portal.

---

## 3. Accomplishments & Deliverables

During this session, we successfully built and refactored a complete documentation suite:

* **Core Specifications** (`docs/core/`):
  * [main.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/main.md): Entrypoint bootstrapping and dependency loading priority.
  * [settings.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/settings.md): Viper configuration schema and environment bindings.
  * [telemetry.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/telemetry.md): slog structured logger and Stdio isolation.
  * [handler.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/handler.md): Thin gateway JSON-RPC bridging handler.
  * [document.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/document.md): Single-threaded Document Actor state orchestration.
* **Architecture Blueprints** (`docs/`):
  * [concurrency.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/concurrency.md): Mutex-free Go channel hierarchy and nonces.
  * [generic_api.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/generic_api.md): Generic `AttributeProducer` interface and concurrent reduction rules.
* **Feature Specs** (`docs/features/`):
  * [diagnostic.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/features/diagnostic.md): AST syntax and Benthos semantic validation.
  * [sample.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/features/sample.md): Directive extraction, cumulative caching, inlay hints, and lenses.
  * [hover.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/features/hover.md): pre-built standard documentation lookup and live evaluations.
  * [completion.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/features/completion.md): Trigger-context autocompletes.
