package lsp

import (
	"context"
	protocol "github.com/owenrumney/go-lsp/lsp"
	"strings"
)

func (h *Handler) DidChangeWatchedFiles(ctx context.Context, _ *protocol.DidChangeWatchedFilesParams) error {
	for _, uri := range h.documents.URIs() {
		if strings.Contains(string(uri), "#bloblang-") {
			continue
		}
		text, _ := h.documents.Text(uri)
		if yamlDocument(uri) {
			h.syncRegions(uri, text)
		} else {
			h.updateSample(uri, text)
		}
		h.executor.InvalidateDocument(string(uri))
		h.clearExecDiagnostics(uri)
		h.mu.Lock()
		h.latestVersion[uri]++
		version := h.latestVersion[uri]
		h.mu.Unlock()
		go h.validateDocument(context.Background(), uri, version)
		h.scheduleRefresh(uri)
	}
	return nil
}
