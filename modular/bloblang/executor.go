package bloblang

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/teyfix/bloblang-lsp/modular/ascii"
	"github.com/teyfix/bloblang-lsp/modular/config"
	"github.com/teyfix/bloblang-lsp/modular/pretty"
)

// PartialResult is the output of a single cumulative execution pass.
type PartialResult struct {
	// Text is the (possibly truncated) inline display value.
	Text string
	// Truncated is true when the full value exceeded MaxInlineResultBytes.
	Truncated bool
	// Full is the pretty-printed full value (used in hover cards).
	Full string
}

// Executor runs cumulative Bloblang mappings for inline result display.
// Each FileActor owns one Executor and calls it only from its event loop,
// so no synchronisation is required.
type Executor struct {
	cache  *expirable.LRU[string, *PartialResult]
	benv   *bloblang.Environment
	config *config.Config
}

// NewExecutor creates an Executor backed by an LRU cache sized from cfg.
func NewExecutor(benv *bloblang.Environment, cfg *config.Config) *Executor {
	size := cfg.PartialExecCacheSize
	if size <= 0 {
		size = 1
	}
	return &Executor{
		cache:  expirable.NewLRU[string, *PartialResult](size, nil, cfg.PartialExecCacheTTL),
		benv:   benv,
		config: cfg,
	}
}

func isStatementKind(kind string) bool {
	switch kind {
	case "root_assignment", "let_assignment", "meta_assignment", "map_declaration", "import_statement":
		return true
	}
	return false
}

// ExecuteCumulative executes all root-assignment statements from the beginning
// of the document through throughLine (inclusive) and returns the result.
//
// throughLine == -1 means "before any assignment"; in that case the raw sample
// is returned as-is (marshalled to JSON).
func (e *Executor) ExecuteCumulative(parser *Parser, uri string, sample interface{}, docText string, throughLine int) (*PartialResult, error) {
	if len(docText) > e.config.MaxInlineDocumentBytes {
		return nil, nil
	}

	key := uri + ":cumulative:" + strconv.Itoa(throughLine)
	if result, ok := e.cache.Get(key); ok {
		return result, nil
	}

	// Special case: before any assignment — return the raw sample.
	if throughLine < 0 {
		encoded, err := json.Marshal(sample)
		if err != nil {
			return nil, err
		}
		full := string(encoded)
		text := full
		truncated := false
		if e.config.MaxInlineResultBytes >= 0 && len(text) > e.config.MaxInlineResultBytes {
			full = strings.TrimRight(string(pretty.PrettyOptions(encoded, &pretty.Options{
				Width:    e.config.MaxInlineResultBytes,
				Prefix:   "",
				Indent:   "  ",
				SortKeys: false,
			})), "\n")
			text = text[:e.config.MaxInlineResultBytes] + ascii.Ellipsis
			truncated = true
		}
		result := &PartialResult{Text: text, Truncated: truncated, Full: full}
		e.cache.Add(key, result)
		return result, nil
	}

	tree, err := parser.Parse(uri, docText)
	if err != nil {
		return nil, err
	}

	// Collect all statement nodes whose start row is <= throughLine.
	root := tree.RootNode()
	var snippetParts []string
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if !isStatementKind(child.Kind()) {
			continue
		}
		if int(child.StartPosition().Row) <= throughLine {
			snippetParts = append(snippetParts, child.Utf8Text([]byte(docText)))
		}
	}

	if len(snippetParts) == 0 {
		// No root assignments up to throughLine — return raw sample.
		return e.ExecuteCumulative(parser, uri, sample, docText, -1)
	}

	snippet := strings.Join(snippetParts, "\n")
	parsed, err := e.benv.Parse(snippet)
	if err != nil {
		return nil, err
	}

	value, err := parsed.Query(sample)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	full := string(encoded)
	text := full
	truncated := false
	if e.config.MaxInlineResultBytes >= 0 && len(text) > e.config.MaxInlineResultBytes {
		full = strings.TrimRight(string(pretty.PrettyOptions(encoded, &pretty.Options{
			Width:    e.config.MaxInlineResultBytes,
			Prefix:   "",
			Indent:   "  ",
			SortKeys: false,
		})), "\n")
		text = text[:e.config.MaxInlineResultBytes] + ascii.Ellipsis
		truncated = true
	}

	result := &PartialResult{Text: text, Truncated: truncated, Full: full}
	e.cache.Add(key, result)
	return result, nil
}

// InvalidateDocument evicts all cached results for the given URI.
func (e *Executor) InvalidateDocument(uri string) {
	prefix := uri + ":"
	for _, key := range e.cache.Keys() {
		if strings.HasPrefix(key, prefix) {
			e.cache.Remove(key)
		}
	}
}

// LooksIncomplete reports whether a parse error looks like an incomplete
// (still-being-typed) expression rather than a real mistake.
func LooksIncomplete(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "unexpected end") ||
		strings.Contains(msg, "unterminated")
}
