package lsp

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/owenrumney/go-lsp/document"
	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/teyfix/bloblang-lsp/internal/benthos"
	"github.com/teyfix/bloblang-lsp/internal/config"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Handler struct {
	config          *config.Config
	logger          *slog.Logger
	benv            *bloblang.Environment
	parser          *benthos.Bloblang
	completionItems []protocol.CompletionItem
	functionDocs    map[string]protocol.MarkupContent
	methodDocs      map[string]protocol.MarkupContent

	documents *document.Store
	client    *server.Client
	executor  *benthos.Executor

	mu sync.Mutex

	workspaceRoot  string
	importBaseDirs map[protocol.DocumentURI]string
	importBasesMu  sync.RWMutex

	samples   map[protocol.DocumentURI]*benthos.Sample
	samplesMu sync.RWMutex

	sampleDiagnostics   map[protocol.DocumentURI][]protocol.Diagnostic
	sampleDiagnosticsMu sync.RWMutex

	lastDiagnostics   map[protocol.DocumentURI][]protocol.Diagnostic
	lastDiagnosticsMu sync.RWMutex
	validationHook    func(protocol.DocumentURI, []protocol.Diagnostic)

	execDiagnostics   map[protocol.DocumentURI][]protocol.Diagnostic
	execDiagnosticsMu sync.RWMutex

	latestVersion map[protocol.DocumentURI]uint64
	refreshInlays bool
	refreshLenses bool
}

func NewHandler(cfg *config.Config, logger *slog.Logger) (*Handler, error) {
	benv := benthos.NewEnvironment()
	parser, err := benthos.NewBloblang()

	if err != nil {
		return nil, err
	}

	items, fnData, methData := benthos.BuildCompletionCache(benv)
	fnDocs, methDocs := benthos.BuildAllDocs(fnData, methData, cfg.BloblangDocsURL)
	executor := benthos.NewExecutor(benv, cfg)

	return &Handler{
		config:            cfg,
		logger:            logger,
		benv:              benv,
		parser:            parser,
		completionItems:   items,
		functionDocs:      fnDocs,
		methodDocs:        methDocs,
		documents:         document.NewStore(),
		executor:          executor,
		importBaseDirs:    make(map[protocol.DocumentURI]string),
		samples:           make(map[protocol.DocumentURI]*benthos.Sample),
		sampleDiagnostics: make(map[protocol.DocumentURI][]protocol.Diagnostic),
		lastDiagnostics:   make(map[protocol.DocumentURI][]protocol.Diagnostic),
		execDiagnostics:   make(map[protocol.DocumentURI][]protocol.Diagnostic),
		latestVersion:     make(map[protocol.DocumentURI]uint64),
	}, nil
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
	if w := params.Capabilities.Workspace; w != nil {
		if w.InlayHint != nil && w.InlayHint.RefreshSupport != nil {
			h.refreshInlays = *w.InlayHint.RefreshSupport
		}
		if w.CodeLens != nil && w.CodeLens.RefreshSupport != nil {
			h.refreshLenses = *w.CodeLens.RefreshSupport
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
			HoverProvider:              new(true),
			DefinitionProvider:         new(true),
			ReferencesProvider:         new(true),
			DocumentFormattingProvider: new(true),
			InlayHintProvider:          &protocol.InlayHintOptions{},
			CodeLensProvider:           &protocol.CodeLensOptions{},
			ExecuteCommandProvider:     &protocol.ExecuteCommandOptions{Commands: []string{"bloblang-lsp.showResult", "bloblang-lsp.openFile"}},
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
	if yamlDocument(uri) {
		h.syncRegions(uri, doc.Text())
	} else {
		h.updateSample(uri, doc.Text())
	}
	h.executor.InvalidateDocument(string(uri))
	h.clearExecDiagnostics(uri)

	h.mu.Lock()
	h.latestVersion[uri]++
	ver := h.latestVersion[uri]
	h.mu.Unlock()

	go func(v uint64) { h.validateDocument(context.Background(), uri, v); h.scheduleRefresh(uri) }(ver)
	return nil
}

func (h *Handler) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	doc, err := h.documents.Change(params)
	if err != nil {
		return err
	}
	uri := params.TextDocument.URI
	if yamlDocument(uri) {
		h.syncRegions(uri, doc.Text())
	} else {
		h.updateSample(uri, doc.Text())
	}
	h.executor.InvalidateDocument(string(uri))
	h.clearExecDiagnostics(uri)

	h.mu.Lock()
	h.latestVersion[uri]++
	ver := h.latestVersion[uri]
	h.mu.Unlock()

	go func(v uint64) { h.validateDocument(context.Background(), uri, v); h.scheduleRefresh(uri) }(ver)
	return nil
}

func (h *Handler) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	uri := params.TextDocument.URI
	h.clearRegions(uri)
	h.documents.Close(params)
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

	h.mu.Lock()
	h.latestVersion[uri]++
	delete(h.latestVersion, uri)
	h.mu.Unlock()

	go h.publishDiagnostics(context.Background(), uri, nil)
	return nil
}

func (h *Handler) Completion(ctx context.Context, params *protocol.CompletionParams) (*protocol.CompletionList, error) {
	if yamlDocument(params.TextDocument.URI) && !strings.Contains(string(params.TextDocument.URI), "#bloblang-") {
		r, pos, ok := h.regionAt(params.TextDocument.URI, params.Position)
		if !ok {
			return &protocol.CompletionList{Items: []protocol.CompletionItem{}}, nil
		}
		cp := *params
		cp.TextDocument.URI = r.uri
		cp.Position = pos
		return h.Completion(ctx, &cp)
	}
	items := h.completionItems
	if text, ok := h.documents.Text(params.TextDocument.URI); ok {
		lines := strings.Split(text, "\n")
		pos := byteColumn(text, params.Position)
		switch getCompletionContext(lines, pos.Line, pos.Character) {
		case "method":
			items = filterCompletions(items, protocol.CompletionItemKindMethod)
			items = h.methodCompletion(params.TextDocument.URI, text, params.Position, items)
		case "function":
			items = filterCompletions(items, protocol.CompletionItemKindFunction)
		case "variable":
			items = h.variableCompletions(params.TextDocument.URI, text, params.Position)
		}
	}
	return &protocol.CompletionList{Items: items}, nil
}

func (h *Handler) Hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	uri := params.TextDocument.URI
	if yamlDocument(uri) && !strings.Contains(string(uri), "#bloblang-") {
		r, pos, ok := h.regionAt(uri, params.Position)
		if !ok {
			return nil, nil
		}
		cp := *params
		cp.TextDocument.URI = r.uri
		cp.Position = pos
		v, err := h.Hover(ctx, &cp)
		if v != nil && v.Range != nil {
			host, _ := h.documents.Text(uri)
			*v.Range = r.hostRange(host, *v.Range)
		}
		return v, err
	}
	text, ok := h.documents.Text(uri)
	if !ok {
		return nil, nil
	}
	position := byteColumn(text, params.Position)
	lineIdx := position.Line
	col := position.Character

	tree, parseErr := h.parser.Parse(string(uri), text)
	if parseErr != nil || tree == nil {
		return nil, nil
	}

	defer tree.Close()
	root := tree.RootNode()
	point := tree_sitter.Point{
		Row:    uint(lineIdx),
		Column: uint(col),
	}

	node := root.DescendantForPointRange(point, point)
	if node == nil {
		return nil, nil
	}

	if sample := h.getSample(uri); sample != nil {
		statement := node
		for statement != nil && !benthosStatement(statement.Kind()) {
			statement = statement.Parent()
		}
		if statement != nil && statement.Parent() != nil && statement.Kind() != "map_declaration" {
			var result *benthos.PartialResult
			var err error
			if node.Kind() == "root" && node.Parent() != nil && node.Parent().Kind() == "root_assignment" {
				if statement.Parent().Kind() == "source" {
					result, err = h.executor.ExecuteThrough(h.parser, string(uri), sample, text, statement.StartByte())
				} else {
					result, err = h.executor.EvaluateExpression(h.parser, string(uri), sample, text, statement, node)
				}
			} else {
				expression := hoverExpression(node)
				if expression != nil {
					result, err = h.executor.EvaluateExpression(h.parser, string(uri), sample, text, statement, expression)
				}
			}
			if err == nil && result != nil {
				v := valueHover(result, node)
				if v.Range != nil {
					*v.Range = rangeUTF16(text, *v.Range)
				}
				return v, nil
			}
		}
	}

	// 2. Functions & Methods hover
	if node.Kind() == "identifier" && node.Parent() != nil {
		parent := node.Parent()
		token := node.Utf8Text([]byte(text))
		var doc protocol.MarkupContent
		var found bool
		if parent.Kind() == "method_call" {
			doc, found = h.methodDocs[token]
		} else if parent.Kind() == "call_expr" {
			doc, found = h.functionDocs[token]
		}
		if found {
			return &protocol.Hover{
				Contents: protocol.NewHoverContents(doc.Kind, doc.Value),
				Range: &protocol.Range{
					Start: bytePosition(text, int(node.StartByte())),
					End:   bytePosition(text, int(node.EndByte())),
				},
			}, nil
		}
	}

	return nil, nil
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
	sample, err := benthos.ExtractSample(h.parser, string(uri), text, h.baseDirForURI(uri))
	h.samplesMu.Lock()
	h.samples[uri] = sample
	h.samplesMu.Unlock()
	h.sampleDiagnosticsMu.Lock()
	defer h.sampleDiagnosticsMu.Unlock()
	if err == nil {
		delete(h.sampleDiagnostics, uri)
		if sample == nil {
			severity := protocol.SeverityInformation
			stem := "mapping"
			if p, e := uriToPath(uri); e == nil {
				stem = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
			}
			h.sampleDiagnostics[uri] = []protocol.Diagnostic{{Range: protocol.Range{}, Severity: &severity, Source: "bloblang sample", Message: stem + ".sample.json not found; add one to get hints"}}
		}
		return
	}
	severity := protocol.SeverityError
	source := "bloblang"
	line := sampleDirectiveLine(text)
	if se, ok := err.(*benthos.SampleError); ok {
		line = se.Line
	}
	h.sampleDiagnostics[uri] = []protocol.Diagnostic{{
		Range:    protocol.Range{Start: protocol.Position{Line: line, Character: 0}, End: protocol.Position{Line: line, Character: 0}},
		Severity: &severity,
		Source:   source,
		Message:  err.Error(),
	}}
}

func (h *Handler) getSample(uri protocol.DocumentURI) *benthos.Sample {
	h.samplesMu.RLock()
	defer h.samplesMu.RUnlock()
	return h.samples[uri]
}

func (h *Handler) sampleDiagnosticsFor(uri protocol.DocumentURI) []protocol.Diagnostic {
	h.sampleDiagnosticsMu.RLock()
	defer h.sampleDiagnosticsMu.RUnlock()
	return append([]protocol.Diagnostic(nil), h.sampleDiagnostics[uri]...)
}

// clearExecDiagnostics removes any previously stored execution diagnostics for uri.
// Called on DidOpen/DidChange so stale errors don't persist after the document is edited.
func (h *Handler) clearExecDiagnostics(uri protocol.DocumentURI) {
	h.execDiagnosticsMu.Lock()
	delete(h.execDiagnostics, uri)
	h.execDiagnosticsMu.Unlock()
}

// publishExecDiagnostics stores the given execution-time diagnostics for uri and
// re-publishes the merged set (last known parse/import diagnostics + exec diagnostics).
// Passing an empty or nil slice clears any previously stored exec diagnostics.
func (h *Handler) publishExecDiagnostics(ctx context.Context, uri protocol.DocumentURI, diags []protocol.Diagnostic) {
	h.execDiagnosticsMu.Lock()
	if len(diags) == 0 {
		delete(h.execDiagnostics, uri)
	} else {
		h.execDiagnostics[uri] = append([]protocol.Diagnostic(nil), diags...)
	}
	h.execDiagnosticsMu.Unlock()

	// Merge with the last known parse/import diagnostics and republish.
	h.lastDiagnosticsMu.RLock()
	base := append([]protocol.Diagnostic(nil), h.lastDiagnostics[uri]...)
	h.lastDiagnosticsMu.RUnlock()

	h.execDiagnosticsMu.RLock()
	exec := append([]protocol.Diagnostic(nil), h.execDiagnostics[uri]...)
	h.execDiagnosticsMu.RUnlock()

	merged := append(base, exec...)
	if h.client == nil || strings.Contains(string(uri), "#bloblang-") {
		return
	}
	go func() {
		if err := h.client.PublishDiagnostics(context.Background(), &protocol.PublishDiagnosticsParams{URI: uri, Diagnostics: merged}); err != nil {
			h.logger.Debug("publish diagnostics failed", "uri", uri, "err", err)
		}
	}()
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

func rawJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
