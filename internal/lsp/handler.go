package lsp

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/owenrumney/go-lsp/document"
	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
	redbloblang "github.com/redpanda-data/benthos/v4/public/bloblang"
	bloblangpkg "github.com/teyfix/bloblang-lsp/internal/bloblang"
	"github.com/teyfix/bloblang-lsp/internal/config"
)

type Handler struct {
	config          *config.Config
	logger          *slog.Logger
	benv            *redbloblang.Environment
	completionItems []protocol.CompletionItem
	functionDocs    map[string]protocol.MarkupContent
	methodDocs      map[string]protocol.MarkupContent

	documents *document.Store
	client    *server.Client
	executor  *bloblangpkg.Executor

	mu          sync.Mutex
	cancelFuncs map[protocol.DocumentURI]context.CancelFunc

	workspaceRoot  string
	importBaseDirs map[protocol.DocumentURI]string
	importBasesMu  sync.RWMutex

	samples   map[protocol.DocumentURI]*bloblangpkg.Sample
	samplesMu sync.RWMutex

	sampleDiagnostics   map[protocol.DocumentURI][]protocol.Diagnostic
	sampleDiagnosticsMu sync.RWMutex

	inlayCancel       map[protocol.DocumentURI]context.CancelFunc
	lensCancel        map[protocol.DocumentURI]context.CancelFunc
	inlayLensMu       sync.Mutex
	lastDiagnostics   map[protocol.DocumentURI][]protocol.Diagnostic
	lastDiagnosticsMu sync.RWMutex
	validationHook    func(protocol.DocumentURI, []protocol.Diagnostic)
}

func NewHandler(cfg *config.Config, logger *slog.Logger, benv *redbloblang.Environment, items []protocol.CompletionItem, functionDocs map[string]protocol.MarkupContent, methodDocs map[string]protocol.MarkupContent, executor *bloblangpkg.Executor) *Handler {
	if executor == nil {
		executor = bloblangpkg.NewExecutor(benv, cfg)
	}
	return &Handler{
		config:            cfg,
		logger:            logger,
		benv:              benv,
		completionItems:   items,
		functionDocs:      functionDocs,
		methodDocs:        methodDocs,
		documents:         document.NewStore(),
		executor:          executor,
		cancelFuncs:       make(map[protocol.DocumentURI]context.CancelFunc),
		importBaseDirs:    make(map[protocol.DocumentURI]string),
		samples:           make(map[protocol.DocumentURI]*bloblangpkg.Sample),
		sampleDiagnostics: make(map[protocol.DocumentURI][]protocol.Diagnostic),
		inlayCancel:       make(map[protocol.DocumentURI]context.CancelFunc),
		lensCancel:        make(map[protocol.DocumentURI]context.CancelFunc),
		lastDiagnostics:   make(map[protocol.DocumentURI][]protocol.Diagnostic),
	}
}

func (h *Handler) SetClient(client *server.Client) {
	h.client = client
}

func (h *Handler) Initialize(_ context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	if len(params.WorkspaceFolders) > 0 {
		if path, err := uriToPath(params.WorkspaceFolders[0].URI); err == nil {
			h.workspaceRoot = path
		}
	} else if params.RootURI != nil {
		if path, err := uriToPath(*params.RootURI); err == nil {
			h.workspaceRoot = path
		}
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
			HoverProvider:          ptrTo(true),
			InlayHintProvider:      &protocol.InlayHintOptions{},
			CodeLensProvider:       &protocol.CodeLensOptions{},
			ExecuteCommandProvider: &protocol.ExecuteCommandOptions{Commands: []string{"bloblang-lsp.showResult"}},
		},
		ServerInfo: &protocol.ServerInfo{Name: "bloblang-lsp", Version: "v0.0.1"},
	}, nil
}

func (h *Handler) Shutdown(_ context.Context) error {
	return nil
}

func (h *Handler) SetTrace(_ context.Context, _ *protocol.SetTraceParams) error {
	return nil
}

func (h *Handler) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	doc, err := h.documents.Open(params)
	if err != nil {
		return err
	}
	uri := params.TextDocument.URI
	baseDir := h.baseDirForURI(uri)
	h.importBasesMu.Lock()
	h.importBaseDirs[uri] = baseDir
	h.importBasesMu.Unlock()
	h.updateSample(uri, doc.Text())
	h.executor.InvalidateDocument(string(uri))
	h.scheduleValidation(uri)
	h.scheduleRefresh(uri)
	if h.config.DiagnosticsDebounce == 0 {
		h.validateDocument(ctx, uri)
	}
	return nil
}

func (h *Handler) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	doc, err := h.documents.Change(params)
	if err != nil {
		return err
	}
	uri := params.TextDocument.URI
	h.updateSample(uri, doc.Text())
	h.executor.InvalidateDocument(string(uri))
	h.scheduleValidation(uri)
	h.scheduleRefresh(uri)
	if h.config.DiagnosticsDebounce == 0 {
		h.validateDocument(ctx, uri)
	}
	return nil
}

func (h *Handler) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	uri := params.TextDocument.URI
	h.documents.Close(params)
	h.cancelValidation(uri)
	h.cancelRefresh(uri)
	h.importBasesMu.Lock()
	delete(h.importBaseDirs, uri)
	h.importBasesMu.Unlock()
	h.samplesMu.Lock()
	delete(h.samples, uri)
	h.samplesMu.Unlock()
	h.sampleDiagnosticsMu.Lock()
	delete(h.sampleDiagnostics, uri)
	h.sampleDiagnosticsMu.Unlock()
	h.executor.InvalidateDocument(string(uri))
	h.publishDiagnostics(ctx, uri, nil)
	return nil
}

func (h *Handler) Completion(_ context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	items := h.completionItems
	if text, ok := h.documents.Text(params.TextDocument.URI); ok {
		lines := strings.Split(text, "\n")
		switch getCompletionContext(lines, params.Position.Line, params.Position.Character) {
		case "method":
			items = filterCompletions(items, protocol.CompletionItemKindMethod)
		case "function":
			items = filterCompletions(items, protocol.CompletionItemKindFunction)
		case "variable":
			items = []protocol.CompletionItem{}
		}
	}
	return &protocol.CompletionList{Items: items}, nil
}

func (h *Handler) Hover(_ context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	uri := params.TextDocument.URI
	text, ok := h.documents.Text(uri)
	if !ok {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	lineIdx := params.Position.Line
	if lineIdx < 0 || lineIdx >= len(lines) {
		return nil, nil
	}
	line := lines[lineIdx]
	col := params.Position.Character
	if col > len(line) {
		col = len(line)
	}

	token, tokenStart, tokenEnd, isMethod := findTokenAtPosition(lines, lineIdx, col)
	if token == "" {
		return nil, nil
	}

	if token == "root" {
		if !rootAssignRe.MatchString(line) {
			return nil, nil
		}
		sample := h.getSample(uri)
		if sample == nil {
			return &protocol.Hover{Contents: protocol.MarkupContent{
				Kind:  protocol.Markdown,
				Value: "Provide a sample with `#!sample {\"key\": \"value\"}`",
			}}, nil
		}
		result, err := h.executor.ExecutePartial(string(uri), sample.Value, text, lineIdx)
		if err != nil || result == nil {
			return nil, nil
		}
		return &protocol.Hover{Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: "```json\n" + result.Full + "\n```",
		}}, nil
	}

	if !isFunctionCallContext(line, tokenEnd) {
		return nil, nil
	}
	var doc protocol.MarkupContent
	var found bool
	if isMethod {
		doc, found = h.methodDocs[token]
	} else {
		doc, found = h.functionDocs[token]
	}
	if !found {
		return nil, nil
	}
	return &protocol.Hover{
		Contents: doc,
		Range: &protocol.Range{
			Start: protocol.Position{Line: lineIdx, Character: tokenStart},
			End:   protocol.Position{Line: lineIdx, Character: tokenEnd},
		},
	}, nil
}

func (h *Handler) scheduleValidation(uri protocol.DocumentURI) {
	h.mu.Lock()
	if cancel, ok := h.cancelFuncs[uri]; ok {
		cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.cancelFuncs[uri] = cancel
	h.mu.Unlock()

	go func() {
		select {
		case <-time.After(h.config.DiagnosticsDebounce):
			h.validateDocument(ctx, uri)
		case <-ctx.Done():
		}
	}()
}

func (h *Handler) cancelValidation(uri protocol.DocumentURI) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cancel, ok := h.cancelFuncs[uri]; ok {
		cancel()
		delete(h.cancelFuncs, uri)
	}
}

func (h *Handler) baseDirForURI(uri protocol.DocumentURI) string {
	h.importBasesMu.RLock()
	if base := h.importBaseDirs[uri]; base != "" {
		h.importBasesMu.RUnlock()
		return base
	}
	h.importBasesMu.RUnlock()
	if path, err := uriToPath(uri); err == nil {
		return filepath.Dir(path)
	}
	if h.workspaceRoot != "" {
		return h.workspaceRoot
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return os.TempDir()
}

func (h *Handler) updateSample(uri protocol.DocumentURI, text string) {
	sample, err := bloblangpkg.ExtractSample(text, h.baseDirForURI(uri))
	h.samplesMu.Lock()
	h.samples[uri] = sample
	h.samplesMu.Unlock()
	h.sampleDiagnosticsMu.Lock()
	defer h.sampleDiagnosticsMu.Unlock()
	if err == nil {
		delete(h.sampleDiagnostics, uri)
		return
	}
	severity := protocol.SeverityError
	source := "bloblang"
	line := sampleDirectiveLine(text)
	h.sampleDiagnostics[uri] = []protocol.Diagnostic{{
		Range:    protocol.Range{Start: protocol.Position{Line: line, Character: 0}, End: protocol.Position{Line: line, Character: 0}},
		Severity: &severity,
		Source:   source,
		Message:  err.Error(),
	}}
}

func (h *Handler) getSample(uri protocol.DocumentURI) *bloblangpkg.Sample {
	h.samplesMu.RLock()
	defer h.samplesMu.RUnlock()
	return h.samples[uri]
}

func (h *Handler) sampleDiagnosticsFor(uri protocol.DocumentURI) []protocol.Diagnostic {
	h.sampleDiagnosticsMu.RLock()
	defer h.sampleDiagnosticsMu.RUnlock()
	return append([]protocol.Diagnostic(nil), h.sampleDiagnostics[uri]...)
}

func sampleDirectiveLine(text string) int {
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#!sample ") || strings.HasPrefix(line, "#!sample_from ") {
			return i
		}
		if line != "" && !strings.HasPrefix(line, "#") {
			break
		}
	}
	return 0
}

func decodedWorkspacePath(uri protocol.DocumentURI) string {
	u, err := url.Parse(string(uri))
	if err != nil {
		return ""
	}
	p, err := url.PathUnescape(u.Path)
	if err != nil {
		return ""
	}
	return filepath.FromSlash(p)
}

func ptrTo[T any](v T) *T {
	return &v
}

func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
