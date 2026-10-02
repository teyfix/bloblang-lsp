# Diagnostics

[`diagnostics.go`](../../internal/lsp/diagnostics.go) combines Benthos parse/import errors, sample issues, sample execution warnings, and AST lint findings. YAML external mapping links also receive path diagnostics. Embedded mapping ranges are converted back to host YAML coordinates; compiler columns are converted to LSP UTF-16 positions.

IDE Benthos validation uses the same isolated environment fixtures as sample execution: `env()` reads a whole-sample `env` override or an empty string, and never the language server process environment. This prevents constant-folding concatenations with absent environment variables into editor parse errors. Production runtime semantics and environment fallback lint warnings remain unchanged. See [environment fixtures](sample.md#environment-fixtures).

Validation runs after document changes. A version check discards stale validation results. Editing clears prior execution diagnostics, and clients receive an empty `diagnostics: []` notification when errors are repaired or documents close. Valid comment-only sample directives do not produce an unexpected-end mapping error.

Missing automatic samples are informational. Missing explicitly selected files, malformed samples, and bad directives are errors. Sample failures leave static completion, documentation, navigation, formatting, and linting available. Successful compilation can still produce a warning when evaluation against the selected sample fails.

Lint findings use stable slash-separated diagnostic codes and source `bloblang lint`. Rule severities, disabling, and next-line suppression are configured as described in [linting](lint.md). Invalid workspace settings fall back to defaults; their schema validation belongs to the editor's JSON support.
