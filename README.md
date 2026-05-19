# Bloblang Language Server

A [Language Server Protocol (LSP)](https://microsoft.github.io/language-server-protocol/) implementation for [Bloblang](https://docs.redpanda.com/redpanda-connect/guides/bloblang/about/), the powerful mapping language used in [Redpanda Connect](https://docs.redpanda.com/redpanda-connect/).

## Features

- **Intelligent Code Completion** — Auto-complete Bloblang functions and methods with contextual awareness
  - Function completions triggered after operators like `=`, `(`, `{`, `[`, `|`, `;`
  - Method completions triggered after `.` (e.g., `this.foo.bar.`)
  - Named-argument variants for functions/methods with optional parameters
  - Visual status indicators: `[β]` beta, `[⚗]` experimental, `[⚠]` deprecated

- **Rich Hover Documentation** — Detailed Markdown documentation on hover
  - Function/method signatures with parameter types
  - Status badges and deprecation warnings
  - Inline examples with input/output tables
  - Links to official [Redpanda documentation](https://docs.redpanda.com/redpanda-connect/guides/bloblang/)

- **Real-time Diagnostics** — Live error detection and validation
  - Parse error reporting with line/character positioning
  - Custom import resolution for `import "./maps.blobl"` statements
  - Diagnostics published via LSP `textDocument/publishDiagnostics`

- **Snippet Support** — Tab-stop placeholders for rapid coding
  - Positional arguments for required parameters
  - Named arguments for optional parameters
  - Smart defaults (e.g., `0` for numbers, `[]` for arrays)

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [Releases](https://github.com/teyfix/bloblang-lsp/releases) page:

| Platform | Architectures |
|----------|--------------|
| Linux    | amd64, arm64 |
| macOS    | amd64, arm64 |
| Windows  | amd64, arm64 |

### From Source

Requires [Go](https://go.dev/) 1.26+ and [Task](https://taskfile.dev/):

```bash
git clone https://github.com/teyfix/bloblang-lsp.git
cd bloblang-lsp
task build
```

The binary will be available at `target/language-server-<os>-<arch>`.

## Editor Configuration

### Neovim (nvim-lspconfig)

```lua
require'lspconfig'.bloblang.setup{
  cmd = { '/path/to/language-server-linux-amd64' },
  filetypes = { 'bloblang', 'blobl' },
  root_dir = require'lspconfig'.util.root_pattern('.git', 'Taskfile.yml'),
}
```

### Emacs (lsp-mode)

```elisp
(lsp-register-client
 (make-lsp-client :new-connection (lsp-stdio-connection "/path/to/language-server-linux-amd64")
                  :major-modes '(bloblang-mode)
                  :server-id 'bloblang-lsp))
```

### Sublime Text (LSP)

```json
{
  "clients": {
    "bloblang-lsp": {
      "enabled": true,
      "command": ["/path/to/language-server-linux-amd64"],
      "selector": "source.bloblang"
    }
  }
}
```

## Supported LSP Capabilities

| Feature | Method | Status |
|---------|--------|--------|
| Text Document Sync | `textDocument/didOpen` | ✅ Full document sync |
| | `textDocument/didChange` | ✅ |
| | `textDocument/didClose` | ✅ |
| Completion | `textDocument/completion` | ✅ Functions, methods, variables |
| Hover | `textDocument/hover` | ✅ Function/method docs |
| Diagnostics | `textDocument/publishDiagnostics` | ✅ Parse errors |

## Architecture

The server is built on:

- **[glsp](https://github.com/tliron/glsp)** — Go LSP framework
- **[Redpanda Connect](https://github.com/redpanda-data/connect)** — Bloblang parser and runtime
- **[Benthos](https://github.com/redpanda-data/benthos)** — Core Bloblang environment

## Development

```bash
# Build for current platform
task build:target OS=linux ARCH=amd64

# Build all platforms
task build

# Run tests
go test ./...

# Release (requires GitHub token)
git tag v1.0.0
git push origin v1.0.0
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

## License

[MIT](LICENSE)

## Acknowledgments

- [Redpanda Data](https://redpanda.com/) for the excellent Bloblang language and documentation
- The [Language Server Protocol](https://microsoft.github.io/language-server-protocol/) team at Microsoft
