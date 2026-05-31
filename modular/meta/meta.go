// Package meta holds application-level identity constants shared across all
// sub-packages. It must remain dependency-free to avoid import cycles.
package meta

// ServerName is the canonical LSP server identifier used in XDG config
// directory paths, server info announcements, and command namespacing.
const ServerName = "bloblang-lsp"

// ServerVersion is the version string reported in the LSP initialize response.
const ServerVersion = "v0.0.1"
