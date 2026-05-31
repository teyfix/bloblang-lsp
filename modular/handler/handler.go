package lsphandler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/owenrumney/go-lsp/document"
	"github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
	"github.com/teyfix/bloblang-lsp/modular/ast"
	"github.com/teyfix/bloblang-lsp/modular/config"
	meta "github.com/teyfix/bloblang-lsp/modular/meta"
	"github.com/teyfix/bloblang-lsp/modular/sample"
)

// Handler bridges the JSON-RPC interface to the reactive pipeline.Registry.
type Handler struct {
	ast    *ast.Bloblang
	store  *document.Store
	config *config.Config
	logger *slog.Logger
	client *server.Client
}

// NewHandler instantiates a Handler.
func NewHandler(cfg *config.Config, logger *slog.Logger) (*Handler, error) {
	h := &Handler{
		config: cfg,
		logger: logger,
	}

	return h, nil
}

// SetClient receives the server client interface.
func (h *Handler) SetClient(client *server.Client) {
	h.client = client
}

// Initialize performs LSP capabilities handshake and sets workspace context.
func (h *Handler) Initialize(_ context.Context, params *lsp.InitializeParams) (*lsp.InitializeResult, error) {
	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptions{
				OpenClose: new(true),
				Change:    lsp.SyncFull,
			},
			CompletionProvider: &lsp.CompletionOptions{
				TriggerCharacters: []string{".", "@", "$"},
			},
			HoverProvider:     new(true),
			InlayHintProvider: &lsp.InlayHintOptions{},
			CodeLensProvider:  &lsp.CodeLensOptions{},
			ExecuteCommandProvider: &lsp.ExecuteCommandOptions{
				Commands: []string{
					string(meta.CommandShowResult),
					string(meta.CommandOpenFile),
				},
			},
		},
		ServerInfo: &lsp.ServerInfo{
			Name:    string(meta.ServerName),
			Version: string(meta.ServerVersion),
		},
	}, nil
}

// Shutdown shuts down the server.
func (h *Handler) Shutdown(_ context.Context) error {
	return nil
}

// SetTrace sets trace configuration.
func (h *Handler) SetTrace(_ context.Context, _ *lsp.SetTraceParams) error {
	return nil
}

// DidOpen forwards document opening notification to registry.
func (h *Handler) DidOpen(_ context.Context, params *lsp.DidOpenTextDocumentParams) error {
	h.store.Open(params)
	doc, getErr := h.store.Get(params.TextDocument.URI)

	if getErr {
		return fmt.Errorf("could not get document")
	}

	text := doc.Text()
	tree, err := h.ast.Parse(text)

	if err != nil {
		return err
	}

	go sample.Sample(text, tree)

	return nil
}

// DidChange forwards document modification notification to registry.
func (h *Handler) DidChange(_ context.Context, params *lsp.DidChangeTextDocumentParams) error {
	return nil
}

// DidClose forwards document closure notification to registry.
func (h *Handler) DidClose(_ context.Context, params *lsp.DidCloseTextDocumentParams) error {
	return nil
}

// Completion handles autocomplete requests synchronously.
func (h *Handler) Completion(_ context.Context, params *lsp.CompletionParams) (*lsp.CompletionList, error) {
	return &lsp.CompletionList{}, nil
}

// Hover handles hover context cards synchronously.
func (h *Handler) Hover(_ context.Context, params *lsp.HoverParams) (*lsp.Hover, error) {
	return &lsp.Hover{}, nil
}

// InlayHint handles inlay result display generation synchronously.
func (h *Handler) InlayHint(_ context.Context, params *lsp.InlayHintParams) ([]lsp.InlayHint, error) {
	return []lsp.InlayHint{}, nil
}

// CodeLens handles document macro triggers synchronously.
func (h *Handler) CodeLens(_ context.Context, params *lsp.CodeLensParams) ([]lsp.CodeLens, error) {
	return []lsp.CodeLens{}, nil
}

// ExecuteCommand runs client command procedures.
func (h *Handler) ExecuteCommand(ctx context.Context, params *lsp.ExecuteCommandParams) (error, error) {
	return nil, fmt.Errorf("unknown command: %s", params.Command)
}
