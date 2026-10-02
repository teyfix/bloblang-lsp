package lsp

import (
	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/teyfix/bloblang-lsp/internal/editorconfig"
	"path/filepath"
	"strings"
)

// Read on demand, so edits, creations and deletions take effect without restarting.
func (h *Handler) workspaceConfig(uri protocol.DocumentURI) editorconfig.Config {
	dir := h.baseDirForURI(uri)
	best := ""
	for _, root := range h.workspaceRoots {
		if dir == root || strings.HasPrefix(dir, root+string(filepath.Separator)) {
			if len(root) > len(best) {
				best = root
			}
		}
	}
	if best == "" {
		best = h.workspaceRoot
	}
	if best == "" {
		best = dir
	}
	c, err := editorconfig.Load(filepath.Join(best, ".bloblangrc.json"))
	if err != nil {
		h.logger.Debug("invalid workspace configuration", "error", err)
	}
	return c
}
