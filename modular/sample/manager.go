package sample

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/teyfix/bloblang-lsp/modular/bloblang"
	"github.com/teyfix/bloblang-lsp/modular/config"
	"github.com/teyfix/bloblang-lsp/modular/meta"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Sample represents the parsed sample input state.
type Sample struct {
	Value  interface{}
	Source string
	Line   int
}

type documentState struct {
	rawDirective string
	sample       *Sample
	sampleErr    error

	// Cache parameters for external files
	resolvedPath string
	lastModTime  time.Time
}

// Manager orchestrates thread-safe sample extraction, file caching, and LSP evaluation overlays.
type Manager struct {
	mu       sync.RWMutex
	states   map[string]*documentState
	config   *config.Config
	parser   *bloblang.Parser
	executor *bloblang.Executor
}

// NewManager constructs a Manager.
func NewManager(cfg *config.Config) *Manager {
	parser, _ := bloblang.NewParser()
	executor := bloblang.NewExecutor(bloblang.NewEnvironment(), cfg)

	return &Manager{
		states:   make(map[string]*documentState),
		config:   cfg,
		parser:   parser,
		executor: executor,
	}
}

// Update evaluates a document's sample directive and refreshes file reads dynamically.
func (m *Manager) Update(uri string, text string, tree *tree_sitter.Tree, baseDir string) []protocol.Diagnostic {
	m.mu.Lock()
	defer m.mu.Unlock()

	line, dirText, isSampleFrom := findSampleDirective(text, tree)
	if dirText == "" {
		m.states[uri] = nil
		return nil
	}

	state := m.states[uri]
	if state == nil {
		state = &documentState{}
		m.states[uri] = state
	}

	// Dynamic file-read caching validation
	if state.rawDirective == dirText {
		if !isSampleFrom {
			if state.sampleErr != nil {
				return m.diagnosticsForError(state.sampleErr, line)
			}
			return nil
		}

		if info, err := os.Stat(state.resolvedPath); err == nil {
			if info.ModTime().Equal(state.lastModTime) {
				if state.sampleErr != nil {
					return m.diagnosticsForError(state.sampleErr, line)
				}
				return nil
			}
		}
	}

	state.rawDirective = dirText
	state.sampleErr = nil

	if !isSampleFrom {
		var val interface{}
		if err := json.Unmarshal([]byte(dirText), &val); err != nil {
			state.sampleErr = err
			state.sample = nil
			return m.diagnosticsForError(err, line)
		}
		state.sample = &Sample{
			Value:  val,
			Source: "inline",
			Line:   line,
		}
		return nil
	}

	resolved := filepath.Join(baseDir, dirText)
	state.resolvedPath = resolved

	info, err := os.Stat(resolved)
	if err != nil {
		state.sampleErr = err
		state.sample = nil
		return m.diagnosticsForError(err, line)
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		state.sampleErr = err
		state.sample = nil
		return m.diagnosticsForError(err, line)
	}

	var val interface{}
	if err := json.Unmarshal(data, &val); err != nil {
		state.sampleErr = err
		state.sample = nil
		return m.diagnosticsForError(err, line)
	}

	abs, err := filepath.Abs(resolved)
	if err != nil {
		abs = resolved
	}

	state.lastModTime = info.ModTime()
	state.sample = &Sample{
		Value:  val,
		Source: abs,
		Line:   line,
	}
	return nil
}

// Remove cleans up resources for a closed file URI.
func (m *Manager) Remove(uri string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, uri)
	m.executor.InvalidateDocument(uri)
	m.parser.InvalidateDocument(uri)
}

// Close frees the underlying parser.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parser.Close()
}

// GetSample retrieves the current loaded sample.
func (m *Manager) GetSample(uri string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state := m.states[uri]
	if state == nil || state.sample == nil {
		return nil, false
	}
	return state.sample, true
}

// ExecuteCumulative evaluates mappings cumulatively for a specific URI, locking thread-safely.
func (m *Manager) ExecuteCumulative(uri string, text string, throughLine int) (*bloblang.PartialResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.states[uri]
	if state == nil || state.sample == nil {
		return nil, fmt.Errorf("no sample loaded")
	}
	return m.executor.ExecuteCumulative(m.parser, uri, state.sample.Value, text, throughLine)
}

func (m *Manager) diagnosticsForError(err error, line int) []protocol.Diagnostic {
	severity := protocol.SeverityError
	return []protocol.Diagnostic{{
		Range:    protocol.Range{Start: protocol.Position{Line: line, Character: 0}, End: protocol.Position{Line: line, Character: 0}},
		Severity: &severity,
		Source:   string(meta.ServerName),
		Message:  err.Error(),
	}}
}

func findSampleDirective(text string, tree *tree_sitter.Tree) (int, string, bool) {
	if text == "" || tree == nil {
		return 0, "", false
	}
	root := tree.RootNode()
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(uint(i))
		if child.Kind() != "comment" {
			break
		}

		commentText := child.Utf8Text([]byte(text))
		lineText := strings.TrimSpace(commentText)

		if strings.HasPrefix(lineText, "#!sample ") {
			return int(child.StartPosition().Row), strings.TrimSpace(strings.TrimPrefix(lineText, "#!sample ")), false
		}
		if strings.HasPrefix(lineText, "#!sample_from ") {
			return int(child.StartPosition().Row), strings.TrimSpace(strings.TrimPrefix(lineText, "#!sample_from ")), true
		}
	}
	return 0, "", false
}
