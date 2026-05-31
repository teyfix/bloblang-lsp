# Bloblang LSP Documentation Portal

Welcome to the documentation portal for the modular `bloblang-lsp` server. This portal acts as the source of truth for the server's architectural design, core systems, and feature modules.

Before implementing any changes, refactoring, or new features, review the corresponding package blueprints documented below.

---

## 🗺️ Portal Directory Map

### 1. Architectural Foundation & Concurrency
These blueprints specify the core mutex-free concurrency model and generic interfaces of the language server:
* **[docs/concurrency.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/concurrency.md)**: Describes the **hierarchical actor model** ($Handler \rightarrow DocumentActor \rightarrow Features$) and transactional **nonces**.
* **[docs/generic_api.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/generic_api.md)**: Defines the **`AttributeProducer` API**, concurrent reduction, direct sync query channels, and the zero-context-cancellation rules.

---

### 2. Core Modules (`docs/core/`)
These specifications document the structural plumbing and lifecycle bootstrap layers of the server:
* **[docs/core/main.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/main.md)**: Standardizes entrypoint bootstrapping, logger initialization, capability configuration, and startup priority.
* **[docs/core/settings.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/settings.md)**: Defines configuration schema parameters, precedence rules, and defaults.
* **[docs/core/telemetry.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/telemetry.md)**: Establishes `slog` structured logging standards and rules governing Stdio isolation.
* **[docs/core/handler.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/handler.md)**: Captures the thin gateway interface adapter that bridges RPC requests to actor mailboxes, stripped of all feature logic.
* **[docs/core/document.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/core/document.md)**: Specifies the `DocumentActor` structure as the exclusive orchestrator and keeper of active states (`lastDiagnostics`, `lastInlayHints`, `lastCodeLenses`).

---

### 3. Feature Engines (`docs/features/`)
These documents outline the localized execution engines, AST logic, and Benthos reflection rules for the server capabilities:
* **[docs/features/diagnostic.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/features/diagnostic.md)**: Outlines AST syntax checking, Benthos semantic environment parses, and file import warning resolutions.
* **[docs/features/sample.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/features/sample.md)**: Outlines sample directives extraction (`#!sample`/`#!sample_from`), live cumulative execution caching, inlay hints, and command-trigger code lenses.
* **[docs/features/hover.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/features/hover.md)**: Outlines cursor node analysis, pre-built static documentation lookups, and cumulative state evaluations when hovering over `root`.
* **[docs/features/completion.md](file:///home/dixie/git/teyfix/bloblang-lsp/docs/features/completion.md)**: Outlines context-aware auto-completion lists (trigger characters: `.`, `@`, `$`) that distinguish methods from standard functions.
