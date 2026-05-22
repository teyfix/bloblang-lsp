package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	protocol "github.com/owenrumney/go-lsp/lsp"
)

func (h *Handler) InlayHint(_ context.Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	uri := params.TextDocument.URI
	sample := h.getSample(uri)
	if sample == nil {
		return []protocol.InlayHint{}, nil
	}
	text, ok := h.documents.Text(uri)
	if !ok {
		return []protocol.InlayHint{}, nil
	}
	lines := strings.Split(text, "\n")
	hints := make([]protocol.InlayHint, 0)
	for lineIdx, line := range lines {
		if !rootAssignRe.MatchString(line) {
			continue
		}
		result, err := h.executor.ExecutePartial(string(uri), sample.Value, text, lineIdx)
		if err != nil || result == nil {
			continue
		}
		label, _ := json.Marshal(" = " + result.Text)
		hints = append(hints, protocol.InlayHint{
			Position: protocol.Position{Line: lineIdx, Character: len(line)},
			Label:    label,
			Tooltip: &protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: fmt.Sprintf("```json\n%s\n```", result.Full),
			},
		})
	}
	return hints, nil
}

func (h *Handler) scheduleRefresh(uri protocol.DocumentURI) {
	h.inlayLensMu.Lock()
	if cancel, ok := h.inlayCancel[uri]; ok {
		cancel()
	}
	if cancel, ok := h.lensCancel[uri]; ok {
		cancel()
	}
	inlayCtx, inlayCancel := context.WithCancel(context.Background())
	lensCtx, lensCancel := context.WithCancel(context.Background())
	h.inlayCancel[uri] = inlayCancel
	h.lensCancel[uri] = lensCancel
	h.inlayLensMu.Unlock()

	go h.refreshAfter(inlayCtx, true)
	go h.refreshAfter(lensCtx, false)
}

func (h *Handler) refreshAfter(ctx context.Context, inlay bool) {
	select {
	case <-time.After(h.config.InlineResultDebounce):
		if h.client == nil || ctx.Err() != nil {
			return
		}
		if inlay {
			_ = h.client.InlayHintRefresh(ctx)
		} else {
			_ = h.client.CodeLensRefresh(ctx)
		}
	case <-ctx.Done():
	}
}

func (h *Handler) cancelRefresh(uri protocol.DocumentURI) {
	h.inlayLensMu.Lock()
	defer h.inlayLensMu.Unlock()
	if cancel, ok := h.inlayCancel[uri]; ok {
		cancel()
		delete(h.inlayCancel, uri)
	}
	if cancel, ok := h.lensCancel[uri]; ok {
		cancel()
		delete(h.lensCancel, uri)
	}
}
