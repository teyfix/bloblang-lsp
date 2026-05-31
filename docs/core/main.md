# Module Specification: Language Server Entrypoint (`main.go`)

This document specifies the design, responsibilities, constraints, and sequence flows of the language server entrypoint (`cmd/language-server/main.go`).

---

## 1. Overview & Purpose

`main.go` is the bootstrap entrypoint for the `bloblang-lsp` modular server. Its sole purpose is to load configuration, set up structural logging, construct the core LSP handler, declare top-level capabilities, and launch the JSON-RPC communication channel.

---

## 2. Bootstrapping Sequence Flow

```mermaid
sequenceDiagram
    autonumber
    main.go->>Config: LoadConfig()
    Config-->>main.go: config.Config struct
    main.go->>slog: NewTextHandler(os.Stderr)
    Note over main.go: Construct slog.Logger
    main.go->>lsphandler: NewHandler(config, logger)
    lsphandler-->>main.go: Handler instance
    main.go->>server: NewServer(Handler, Capabilities, Logger)
    server-->>main.go: Server instance
    main.go->>server: Run(context, RunStdio())
```

---

## 3. Package Dependencies & Initialization Order

For `main.go` to successfully boot the LSP server, three core packages must be resolved and initialized in the following strict sequential order:

1. **Settings** (`internal/settings/settings.go`):
   * **Why it's first**: The server configuration must be loaded before any other component is initialized, as both the logger and handler require configuration parameters (e.g. log levels, timeouts, limits).
   * **Initialization**: Triggered by calling `settings.Load()`.
2. **Logger** (`internal/telemetry/logger.go`):
   * **Why it's second**: Logging is the primary telemetry interface. It requires configuration from the Settings package to determine log levels and formats, and must be operational before instantiating the handler to record diagnostic lifecycle logs.
   * **Initialization**: Instantiated using `telemetry.NewLogger(cfg)`.
3. **Handler** (`internal/handler/handler.go`):
   * **Why it's third**: The outer JSON-RPC bridging handler requires references to the resolved configuration (Settings) and structural logging interface (Logger) to coordinate all underlying features and events.
   * **Initialization**: Constructed by calling `handler.NewHandler(cfg, logger)`.

---

## 4. Responsibilities

The entrypoint must perform the following tasks:

### 1. Configuration Loading
* Invoke `config.LoadConfig()`. If an error occurs, print it and terminate with `log.Fatal`.
* Standard environment variables (`BLOBLANG_LSP_*`) and `.bloblangrc` are processed implicitly.

### 2. Structured Logging Setup
* Extract the log level from `config.LogLevel` (supporting `debug`, `info`, `warn`, `error`).
* Instantiate a `slog.Logger` wrapping `slog.NewTextHandler(os.Stderr)`.

### 3. Outer Handler Construction
* Construct the `lsphandler` with `lsphandler.NewHandler(cfg, logger)`.

### 4. Capabilities Announcement
* Instantiate `server.NewServer` specifying standard JSON-RPC capabilities:
  * **Trigger Characters**: Define character triggers for completions (e.g. `"."`, `"@"`, `"$"`).
  * **Execute Commands**: Announce the command namespace IDs (e.g. `"bloblang/showResult"` and `"bloblang/openFile"`).

### 5. Transport execution
* Trigger the Stdio RPC transport stream via `s.Run(context.Background(), server.RunStdio())`.

---

## 5. Key Constraints & Rules

### What is EXPECTED
* **Pure Stdio separation**: All server logs must go to `os.Stderr`. Any prints to `os.Stdout` will corrupt the JSON-RPC standard protocol parsing and crash client-server synchronization.
* **Separation of concerns**: Keep the main loop clean. The entrypoint should never reference AST nodes, sample parsing, or feature packages directly.

### What is FORBIDDEN
* **No Feature-Specific Logic**: Never import features (like autocomplete, validation, or code lenses) in `main.go`. They must be handled inside their respective modules.
* **No Global Variables**: Never define package-level mutable variables. State must reside cleanly within initialized structs.
