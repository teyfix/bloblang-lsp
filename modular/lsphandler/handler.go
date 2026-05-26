package lsphandler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
	"github.com/teyfix/bloblang-lsp/modular/bloblang"
	"github.com/teyfix/bloblang-lsp/modular/config"
	"github.com/teyfix/bloblang-lsp/modular/pipeline"
	"github.com/teyfix/bloblang-lsp/modular/pretty"
	"github.com/teyfix/bloblang-lsp/modular/sample"
)

// Handler bridges the JSON-RPC interface to the reactive pipeline.Registry.
type Handler struct {
	config   *config.Config
	logger   *slog.Logger
	registry *pipeline.Registry
	client   *server.Client
}

// NewHandler instantiates a Handler.
func NewHandler(cfg *config.Config, logger *slog.Logger) (*Handler, error) {
	h := &Handler{
		config: cfg,
		logger: logger,
	}

	benv := bloblang.NewEnvironment()
	cache := bloblang.BuildCompletionCache(benv)
	fnDocs, methDocs := bloblang.BuildAllDocs(cache.FnDocs, cache.MethDocs, cfg.BloblangDocsURL)
	sm := sample.NewManager(cfg)

	h.registry = pipeline.NewRegistry(cfg, func(uri protocol.DocumentURI, diags []protocol.Diagnostic) {
		if h.client != nil {
			ctx := context.Background()
			_ = h.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
				URI:         uri,
				Diagnostics: diags,
			})
			go func() {
				refreshCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				_ = h.client.InlayHintRefresh(refreshCtx)
				_ = h.client.CodeLensRefresh(refreshCtx)
			}()
		}
	}, func() pipeline.Parser {
		p, _ := bloblang.NewParser()
		return p
	})

	h.registry.Register(bloblang.NewSyntaxReducer())
	h.registry.Register(bloblang.NewEnvReducer())
	h.registry.Register(bloblang.NewImportHintReducer())
	h.registry.Register(sample.NewSampleReducer(sm))
	h.registry.Register(bloblang.NewCompletionReducer(cache.Items))
	h.registry.Register(bloblang.NewHoverReducer(fnDocs, methDocs, sm))
	h.registry.Register(sample.NewInlayHintsReducer(sm))
	h.registry.Register(sample.NewCodeLensesReducer(sm))

	return h, nil
}

// SetClient receives the server client interface.
func (h *Handler) SetClient(client *server.Client) {
	h.client = client
}

// Initialize performs LSP capabilities handshake and sets workspace context.
func (h *Handler) Initialize(_ context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	var workspaceRoot string
	if len(params.WorkspaceFolders) > 0 {
		if path, err := uriToPath(params.WorkspaceFolders[0].URI); err == nil {
			workspaceRoot = path
		}
	} else if params.RootURI != nil {
		if path, err := uriToPath(*params.RootURI); err == nil {
			workspaceRoot = path
		}
	}

	if workspaceRoot != "" {
		h.registry.SetWorkspaceRoot(workspaceRoot)
	}

	openClose := true
	return &protocol.InitializeResult{
		Capabilities: protocol.ServerCapabilities{
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: &openClose,
				Change:    protocol.SyncFull,
			},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{".", "@", "$"},
			},
			HoverProvider:     new(true),
			InlayHintProvider: &protocol.InlayHintOptions{},
			CodeLensProvider:  &protocol.CodeLensOptions{},
			ExecuteCommandProvider: &protocol.ExecuteCommandOptions{
				Commands: []string{
					string(pipeline.CommandShowResult),
					string(pipeline.CommandOpenFile),
				},
			},
		},
		ServerInfo: &protocol.ServerInfo{
			Name:    string(pipeline.ServerName),
			Version: string(pipeline.ServerVersion),
		},
	}, nil
}

// Shutdown shuts down the server.
func (h *Handler) Shutdown(_ context.Context) error {
	return nil
}

// SetTrace sets trace configuration.
func (h *Handler) SetTrace(_ context.Context, _ *protocol.SetTraceParams) error {
	return nil
}

// DidOpen forwards document opening notification to registry.
func (h *Handler) DidOpen(_ context.Context, params *protocol.DidOpenTextDocumentParams) error {
	actor := h.registry.GetOrCreateActor(params.TextDocument.URI)
	actor.Inbox() <- &pipeline.DocumentChange{
		Kind:    pipeline.EventDidOpen,
		DidOpen: params,
	}
	return nil
}

// DidChange forwards document modification notification to registry.
func (h *Handler) DidChange(_ context.Context, params *protocol.DidChangeTextDocumentParams) error {
	actor := h.registry.GetOrCreateActor(params.TextDocument.URI)
	actor.Inbox() <- &pipeline.DocumentChange{
		Kind:     pipeline.EventDidChange,
		DidChange: params,
	}
	return nil
}

// DidClose forwards document closure notification to registry.
func (h *Handler) DidClose(_ context.Context, params *protocol.DidCloseTextDocumentParams) error {
	actor := h.registry.GetOrCreateActor(params.TextDocument.URI)
	actor.Inbox() <- &pipeline.DocumentChange{
		Kind:     pipeline.EventDidClose,
		DidClose: params,
	}
	h.registry.RemoveActor(params.TextDocument.URI)
	return nil
}

// Completion handles autocomplete requests synchronously.
func (h *Handler) Completion(_ context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	actor := h.registry.GetOrCreateActor(params.TextDocument.URI)
	replyChan := make(chan *pipeline.EventResult)
	actor.Inbox() <- &pipeline.DocumentChange{
		Kind:       pipeline.EventCompletion,
		Completion: params,
		ReplyTo:    replyChan,
	}
	res := <-replyChan
	if res.CompletionList == nil {
		return &protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil
	}
	return res.CompletionList, nil
}

// Hover handles hover context cards synchronously.
func (h *Handler) Hover(_ context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	actor := h.registry.GetOrCreateActor(params.TextDocument.URI)
	replyChan := make(chan *pipeline.EventResult)
	actor.Inbox() <- &pipeline.DocumentChange{
		Kind:    pipeline.EventHover,
		Hover:   params,
		ReplyTo: replyChan,
	}
	res := <-replyChan
	return res.Hover, nil
}

// InlayHint handles inlay result display generation synchronously.
func (h *Handler) InlayHint(_ context.Context, params *protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	actor := h.registry.GetOrCreateActor(params.TextDocument.URI)
	replyChan := make(chan *pipeline.EventResult)
	actor.Inbox() <- &pipeline.DocumentChange{
		Kind:      pipeline.EventInlayHint,
		InlayHint: params,
		ReplyTo:   replyChan,
	}
	res := <-replyChan
	if res.ListInlayHint == nil {
		return []protocol.InlayHint{}, nil
	}
	return res.ListInlayHint, nil
}

// CodeLens handles document macro triggers synchronously.
func (h *Handler) CodeLens(_ context.Context, params *protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	actor := h.registry.GetOrCreateActor(params.TextDocument.URI)
	replyChan := make(chan *pipeline.EventResult)
	actor.Inbox() <- &pipeline.DocumentChange{
		Kind:     pipeline.EventCodeLens,
		CodeLens: params,
		ReplyTo:  replyChan,
	}
	res := <-replyChan
	if res.ListCodeLens == nil {
		return []protocol.CodeLens{}, nil
	}
	return res.ListCodeLens, nil
}

// ExecuteCommand runs client command procedures.
func (h *Handler) ExecuteCommand(ctx context.Context, params *protocol.ExecuteCommandParams) (any, error) {
	if h.client == nil {
		return nil, fmt.Errorf("LSP client interface not initialized")
	}

	if params.Command == string(pipeline.CommandOpenFile) && len(params.Arguments) == 1 {
		var uriStr string
		if err := json.Unmarshal(params.Arguments[0], &uriStr); err != nil {
			return nil, err
		}
		return h.client.ShowDocument(ctx, &protocol.ShowDocumentParams{
			URI:       protocol.URI(uriStr),
			TakeFocus: new(true),
		})
	}

	if params.Command == string(pipeline.CommandShowResult) && len(params.Arguments) == 1 {
		formatted := pretty.PrettyOptions(params.Arguments[0], &pretty.Options{
			Width:    h.config.MaxInlineResultBytes,
			Prefix:   "",
			Indent:   "  ",
			SortKeys: false,
		})

		tmpFile, err := os.CreateTemp("", "bloblang-sample-*.json")
		if err != nil {
			return nil, err
		}

		if _, err := tmpFile.Write(formatted); err != nil {
			tmpFile.Close()
			return nil, err
		}
		tmpFile.Close()

		tmpPath := filepath.ToSlash(tmpFile.Name())
		if !strings.HasPrefix(tmpPath, "/") {
			tmpPath = "/" + tmpPath
		}

		uri := "file://" + tmpPath

		return h.client.ShowDocument(ctx, &protocol.ShowDocumentParams{
			URI:       protocol.URI(uri),
			TakeFocus: new(true),
		})
	}

	return nil, fmt.Errorf("unknown command: %s", params.Command)
}

// Helper Functions

func uriToPath(uri protocol.DocumentURI) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil {
		return "", err
	}
	if u.Scheme != "file" {
		return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}
	p, err := url.PathUnescape(u.Path)
	if err != nil {
		return "", err
	}
	if os.PathSeparator == '\\' && len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	return filepath.FromSlash(p), nil
}
