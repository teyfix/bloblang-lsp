package pipeline

import (
	protocol "github.com/owenrumney/go-lsp/lsp"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/teyfix/bloblang-lsp/modular/meta"
)

// DocumentEvent represents all LSP lifecycle and request events.
type DocumentEvent string

const (
	EventDidOpen    DocumentEvent = "DidOpen"
	EventDidChange  DocumentEvent = "DidChange"
	EventDidClose   DocumentEvent = "DidClose"
	EventCompletion DocumentEvent = "Completion"
	EventHover      DocumentEvent = "Hover"
	EventInlayHint  DocumentEvent = "InlayHint"
	EventCodeLens   DocumentEvent = "CodeLens"
)

// ServerInfo contains metadata regarding the LSP server.
type ServerInfo string

const (
	ServerName    ServerInfo = ServerInfo(meta.ServerName)
	ServerVersion ServerInfo = ServerInfo(meta.ServerVersion)
)

// CommandName represents valid client commands executed by the LSP server.
type CommandName string

const (
	CommandShowResult CommandName = CommandName(meta.CommandShowResult)
	CommandOpenFile   CommandName = CommandName(meta.CommandOpenFile)
)

// DiagnosticSource identifies which subsystem generated a diagnostic.
type DiagnosticSource string

const (
	SourceSyntax   DiagnosticSource = "bloblang (syntax)"
	SourceBloblang DiagnosticSource = "bloblang"
)

// DocumentChange carries payloads for incoming LSP events to be processed by
// actors and reducers.
type DocumentChange struct {
	Kind       DocumentEvent
	Hover      *protocol.HoverParams
	Completion *protocol.CompletionParams
	DidOpen    *protocol.DidOpenTextDocumentParams
	DidChange  *protocol.DidChangeTextDocumentParams
	DidClose   *protocol.DidCloseTextDocumentParams
	CodeLens   *protocol.CodeLensParams
	InlayHint  *protocol.InlayHintParams

	// ReplyTo handles synchronous non-reducible request/response routing.
	// The dispatcher allocates the channel; the actor closes it after writing.
	ReplyTo chan *EventResult
}

// DocumentState holds the current sequential in-memory state of a document
// as maintained by its owning FileActor.
type DocumentState struct {
	URI     protocol.DocumentURI
	Text    string
	BaseDir string
	Tree    *tree_sitter.Tree
}

// EventResult holds reducible outputs (diagnostics/lenses/hints) and
// non-reducible outcomes (hover/completion). Version is used by the collector
// to discard stale results from superseded edits.
type EventResult struct {
	Version uint64

	// Reducible — merged progressively as workers complete.
	ListCodeLens   []protocol.CodeLens
	ListDiagnostic []protocol.Diagnostic
	ListInlayHint  []protocol.InlayHint

	// Non-reducible — returned directly to the requester via ReplyTo.
	Hover          *protocol.Hover
	CompletionList *protocol.CompletionList
}

// Reducer is a pluggable interface that reactively reduces document state changes.
type Reducer interface {
	// Name is a human-readable identifier for logging/debugging.
	Name() string
	// Interest declares which event kinds this reducer handles.
	Interest() []DocumentEvent
	// Handle computes a partial result from the current state and the incoming change.
	Handle(state *DocumentState, change *DocumentChange) *EventResult
}

// IsInterested reports whether a reducer cares about the given event kind.
func IsInterested(r Reducer, k DocumentEvent) bool {
	for _, i := range r.Interest() {
		if i == k {
			return true
		}
	}
	return false
}

// Parser is a pluggable interface for AST syntax tree parsing.
type Parser interface {
	Parse(uri string, text string) (*tree_sitter.Tree, error)
	Close()
}
