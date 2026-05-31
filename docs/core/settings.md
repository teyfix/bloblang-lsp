# Module Specification: Settings (`settings.go`)

This document specifies the design, properties, and configuration lifecycle of the **Settings** package (`internal/settings/settings.go`).

---

## 1. Overview & Purpose

The `settings` package is the configuration manager of the language server. It loads, unmarshals, and provides a typed configuration structure to the rest of the application. It parses configuration parameters from standard files (`.bloblangrc`) and overrides them with environment variables prefixed with `BLOBLANG_LSP_`.

---

## 2. Configuration Schema & Properties

The `Config` structure defines parameters governing logs, size limits, and execution caching:

| Property | Type | Default Value | Description |
| :--- | :--- | :--- | :--- |
| `BloblangDocsURL` | `string` | `"https://docs.redpanda.com/... "` | Base URL for accessing Bloblang standard method/function documentation. |
| `LogLevel` | `string` | `"info"` | Log level output tier (`debug`, `info`, `warn`, `error`). |
| `MaxInlineDocumentBytes`| `int` | `150000` | Maximum file size allowed for processing inline results. Prevent execution on huge files. |
| `MaxInlineResultBytes` | `int` | `100` | Truncation limit for rendering inline evaluation values in the editor. |
| `PartialExecCacheSize` | `int` | `1000` | Max entries in the cumulative statement execution cache. |
| `PartialExecCacheTTL` | `time.Duration`| `5 * time.Minute` | Time-to-live for cached statement evaluations. |

> [!NOTE]
> **Debouncing Omission**: As execution times across all features take under 5ms, all debouncing settings and options are explicitly omitted from the configuration schema to keep the implementation simple.

---

## 3. Configuration Sources & Precedence

Viper reads configurations from multiple locations, applying the following precedence order (highest to lowest):

1. **Environment Variables**: Overrides any file configuration. Format: `BLOBLANG_LSP_<UPPERCASE_KEY>` (e.g., `BLOBLANG_LSP_LOG_LEVEL`).
2. **Current Directory**: Looking for `.bloblangrc` in the workspace root path where the server process is executed.
3. **XDG Config Directory**: `XDG_CONFIG_HOME/bloblang-lsp/.bloblangrc` (or fallback to `~/.config/bloblang-lsp/.bloblangrc` on Unix).
4. **User Home Directory**: `~/.bloblangrc`.
5. **Static Defaults**: Hardcoded compile-time values declared inside `settings.go`.

---

## 4. Key Constraints & Rules

### What is EXPECTED
* **Read-Only / Thread-Safety**: The settings struct is read-only after initial load at process startup. It is passed as a pointer to other modules, which can read from it concurrently without locking.
* **Deterministic Defaults**: If no `.bloblangrc` or environment variables are provided, the server must fallback to safe, documented default values without throwing an error.

### What is FORBIDDEN
* **No Mutex Locking in Readers**: Since the configuration is immutable after initialization, never wrap reads in mutex locks.
* **No Direct File Refreshes**: Do not support dynamic runtime config reloads unless explicitly requested, avoiding unnecessary filesystem polling or watch-loops.
