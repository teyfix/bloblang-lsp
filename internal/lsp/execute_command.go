package lsp

import (
	"context"
	"encoding/json"

	protocol "github.com/owenrumney/go-lsp/lsp"
)

func (h *Handler) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (any, error) {
	if params.Command != "bloblang-lsp.showResult" || len(params.Arguments) < 3 || h.client == nil {
		return nil, nil
	}
	var full string
	if err := json.Unmarshal(params.Arguments[2], &full); err != nil {
		return nil, err
	}
	return nil, h.client.ShowMessage(ctx, &protocol.ShowMessageParams{
		Type:    protocol.MessageTypeInfo,
		Message: full,
	})
}
