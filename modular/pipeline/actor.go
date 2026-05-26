package pipeline

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/teyfix/bloblang-lsp/modular/config"
)

// Registry manages a collection of active, single-threaded FileActors.
type Registry struct {
	mu            sync.RWMutex
	actors        map[protocol.DocumentURI]*FileActor
	reducers      []Reducer
	publishFn     func(protocol.DocumentURI, []protocol.Diagnostic)
	config        *config.Config
	workspaceRoot string
	parserFactory func() Parser
}

// NewRegistry creates a new Registry.
func NewRegistry(cfg *config.Config, publishFn func(protocol.DocumentURI, []protocol.Diagnostic), parserFactory func() Parser) *Registry {
	return &Registry{
		actors:        make(map[protocol.DocumentURI]*FileActor),
		publishFn:     publishFn,
		config:        cfg,
		parserFactory: parserFactory,
	}
}

// Register registers a new Reducer plug-in.
func (r *Registry) Register(reducer Reducer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reducers = append(r.reducers, reducer)
}

// SetWorkspaceRoot registers the workspace folder context path for resolving relative imports.
func (r *Registry) SetWorkspaceRoot(root string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workspaceRoot = root
}

// GetOrCreateActor starts and returns a FileActor for the given URI if it is not already open.
func (r *Registry) GetOrCreateActor(uri protocol.DocumentURI) *FileActor {
	r.mu.Lock()
	defer r.mu.Unlock()

	if actor, ok := r.actors[uri]; ok {
		return actor
	}

	actor := NewFileActor(uri, r, r.parserFactory())
	r.actors[uri] = actor
	go actor.loop()
	return actor
}

// RemoveActor signals the FileActor for the given URI to shut down and deletes it.
func (r *Registry) RemoveActor(uri protocol.DocumentURI) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if actor, ok := r.actors[uri]; ok {
		actor.Stop()
		delete(r.actors, uri)
	}
}

// FileActor coordinates single-threaded sequential tasks for a single document.
type FileActor struct {
	uri       protocol.DocumentURI
	inbox     chan *DocumentChange
	registry  *Registry
	parser    Parser
	config    *config.Config
	publishFn func(protocol.DocumentURI, []protocol.Diagnostic)

	// State
	state   DocumentState
	version uint64
	results map[string]*EventResult

	// Internals
	partialsInbox chan partialResultMsg
	done          chan struct{}
}

type partialResultMsg struct {
	version uint64
	name    string
	res     *EventResult
}

// NewFileActor creates a new FileActor.
func NewFileActor(uri protocol.DocumentURI, r *Registry, parser Parser) *FileActor {
	baseDir := r.baseDirForURI(uri)

	return &FileActor{
		uri:           uri,
		inbox:         make(chan *DocumentChange, 50),
		registry:      r,
		parser:        parser,
		config:        r.config,
		publishFn:     r.publishFn,
		partialsInbox: make(chan partialResultMsg, 50),
		done:          make(chan struct{}),
		results:       make(map[string]*EventResult),
		state: DocumentState{
			URI:     uri,
			BaseDir: baseDir,
		},
	}
}

// Stop terminates the actor loop.
func (a *FileActor) Stop() {
	close(a.done)
}

// Inbox returns the event inbox channel.
func (a *FileActor) Inbox() chan *DocumentChange {
	return a.inbox
}

// loop runs the single-threaded sequential event loop.
func (a *FileActor) loop() {
	defer a.parser.Close()

	var debounceTimer *time.Timer
	var debounceChan <-chan time.Time

	for {
		select {
		case <-a.done:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case change := <-a.inbox:
			switch change.Kind {
			case EventDidClose:
				stateSnapshot := DocumentState{
					URI:     a.state.URI,
					Text:    a.state.Text,
					BaseDir: a.state.BaseDir,
					Tree:    a.state.Tree,
				}
				changeSnapshot := *change
				for _, reducer := range a.registry.reducers {
					if IsInterested(reducer, EventDidClose) {
						reducer.Handle(&stateSnapshot, &changeSnapshot)
					}
				}
				a.publishFn(a.uri, nil)
				return

			case EventDidOpen, EventDidChange:
				// Reset debounce
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				// Update text state
				if change.Kind == EventDidOpen {
					a.state.Text = change.DidOpen.TextDocument.Text
				} else {
					if len(change.DidChange.ContentChanges) > 0 {
						a.state.Text = change.DidChange.ContentChanges[0].Text
					}
				}

				// Parse document
				tree, err := a.parser.Parse(string(a.uri), a.state.Text)
				if err == nil {
					a.state.Tree = tree
				}

				// Clear dynamic results from reducers that are not interested in edit events
				for _, r := range a.registry.reducers {
					if !IsInterested(r, EventDidOpen) && !IsInterested(r, EventDidChange) {
						delete(a.results, r.Name())
					}
				}

				// Increment version
				a.version++

				// Trigger concurrent Reducers
				stateSnapshot := DocumentState{
					URI:     a.state.URI,
					Text:    a.state.Text,
					BaseDir: a.state.BaseDir,
					Tree:    a.state.Tree,
				}
				changeSnapshot := *change
				currentVersion := a.version

				for _, reducer := range a.registry.reducers {
					if IsInterested(reducer, change.Kind) {
						go func(r Reducer) {
							res := r.Handle(&stateSnapshot, &changeSnapshot)
							a.partialsInbox <- partialResultMsg{
								version: currentVersion,
								name:    r.Name(),
								res:     res,
							}
						}(reducer)
					}
				}

				// Schedule a debounce timer to publish diagnostics
				debounceTimer = time.NewTimer(a.config.DiagnosticsDebounce)
				debounceChan = debounceTimer.C

			case EventCompletion, EventHover, EventInlayHint, EventCodeLens:
				stateSnapshot := DocumentState{
					URI:     a.state.URI,
					Text:    a.state.Text,
					BaseDir: a.state.BaseDir,
					Tree:    a.state.Tree,
				}
				changeSnapshot := *change
				merged := &EventResult{}
				for _, reducer := range a.registry.reducers {
					if IsInterested(reducer, change.Kind) {
						res := reducer.Handle(&stateSnapshot, &changeSnapshot)
						if res != nil {
							// Cache result under name to support cumulative diagnostics
							a.results[reducer.Name()] = res

							// Merge into response
							if len(res.ListInlayHint) > 0 {
								merged.ListInlayHint = append(merged.ListInlayHint, res.ListInlayHint...)
							}
							if len(res.ListCodeLens) > 0 {
								merged.ListCodeLens = append(merged.ListCodeLens, res.ListCodeLens...)
							}
							if res.Hover != nil {
								merged.Hover = res.Hover
							}
							if res.CompletionList != nil {
								merged.CompletionList = res.CompletionList
							}
						}
					}
				}
				if change.Kind == EventInlayHint || change.Kind == EventCodeLens {
					a.publish()
				}
				change.ReplyTo <- merged
				close(change.ReplyTo)
			}

		case msg := <-a.partialsInbox:
			if msg.version != a.version {
				// Discard stale worker results
				continue
			}
			a.results[msg.name] = msg.res

		case <-debounceChan:
			a.publish()
			debounceChan = nil
		}
	}
}

// publish merges and emits the current diagnostics to the client.
func (a *FileActor) publish() {
	var merged []protocol.Diagnostic
	for _, res := range a.results {
		if res != nil && len(res.ListDiagnostic) > 0 {
			merged = append(merged, res.ListDiagnostic...)
		}
	}
	if merged == nil {
		merged = []protocol.Diagnostic{}
	}
	a.publishFn(a.uri, merged)
}

func (r *Registry) baseDirForURI(uri protocol.DocumentURI) string {
	r.mu.RLock()
	root := r.workspaceRoot
	r.mu.RUnlock()

	if path, err := uriToPath(uri); err == nil {
		return filepath.Dir(path)
	}
	if root != "" {
		return root
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return os.TempDir()
}

// Global Static Reducers

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
