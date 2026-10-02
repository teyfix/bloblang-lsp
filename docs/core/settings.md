# Configuration

The server has two configuration readers: startup settings in [`internal/config`](../../internal/config/config.go) and workspace editor settings in [`internal/editorconfig`](../../internal/editorconfig/config.go). The VS Code `.bloblangrc.json` schema describes workspace editor settings.

## Workspace editor settings

Place `.bloblangrc.json` at a workspace root. The server chooses the longest matching workspace root for the document, falling back to the first workspace root, or the document's parent directory when no workspace root is available. Embedded mappings use their host YAML document's directory. It does not search upward for nested configuration files.

| Setting | Default | Accepted values |
| --- | --- | --- |
| `formatter.printWidth` | `80` | Integer from 20 through 1000 |
| `preview.format` | `"yaml"` | `"yaml"` or `"json"` |
| `lint.enabled` | `true` | Boolean |
| `lint.rules` | Registry defaults | Flat slash-separated rule IDs |

Rules accept a severity string or an object containing `severity`. Severities are `off`, `hint`, `info`, `warn`, and `error`. Only `style/assignments/prefer-grouped` accepts `minAssignments`, an integer of at least 3. See [lint configuration](../features/lint.md) and the [generated rule reference](../features/lint-rules.md).

The file is read on demand. Client file-watch events revalidate diagnostics and refresh previews; requests also see current settings without restarting. Invalid JSON, unknown properties or rule IDs, and invalid values cause the entire workspace editor configuration to fall back to defaults. The extension associates the generated schema for completion and validation. Invalid editor configuration does not prevent server startup.

Registry metadata drives both the schema and generated rule reference:

```sh
go run ./cmd/config-schema > schemas/bloblangrc.schema.json
go run ./cmd/config-schema --rules > docs/features/lint-rules.md
```

## Legacy server startup settings

Viper searches for the `.bloblangrc` base name with supported file extensions, for example `.bloblangrc.yaml`, in the process working directory, home directory, then `$XDG_CONFIG_HOME/bloblang-lsp` (or `~/.config/bloblang-lsp`). The first discovered file is loaded; files from those locations are not merged. Environment variables override file values and defaults.

```yaml
# .bloblangrc.yaml
log_level: info
max_inline_document_bytes: 150000
max_inline_result_bytes: 100
```

| Key | Default | Current use |
| --- | --- | --- |
| `bloblang_docs_url` | `https://docs.redpanda.com/redpanda-connect/guides/bloblang` | Documentation links |
| `log_level` | `info` | Server logging |
| `max_inline_document_bytes` | `150000` | Execution size limit |
| `max_inline_result_bytes` | `100` | Inline label truncation; does not set preview print width |
| `diagnostics_debounce` | `200ms` | Retained legacy setting; current handler does not debounce validation |
| `inline_result_debounce` | `100ms` | Retained legacy setting; current handler does not debounce evaluation |
| `partial_exec_cache_size` | `1000` | Retained legacy setting; current executor does not maintain this cache |
| `partial_exec_cache_ttl` | `5m` | Retained legacy setting; current executor does not maintain this cache |

Every key has an environment override named `BLOBLANG_LSP_<UPPERCASE_KEY>`, for example `BLOBLANG_LSP_MAX_INLINE_RESULT_BYTES=200`. Startup settings are loaded once; changing them requires restarting the server.

Viper can also discover `.bloblangrc.json`, but its startup reader only recognizes the legacy snake_case settings. Keep editor JSON limited to its schema, and configure legacy settings through environment variables or a separate server config file. The editor configuration reader does not accept legacy server keys.
