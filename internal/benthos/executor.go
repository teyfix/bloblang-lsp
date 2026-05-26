package benthos

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/teyfix/bloblang-lsp/internal/ascii"
	"github.com/teyfix/bloblang-lsp/internal/config"
	pretty "github.com/teyfix/bloblang-lsp/internal/tidwall"
)

type PartialResult struct {
	Text      string
	Truncated bool
	Full      string
}

type Executor struct {
	cache  *expirable.LRU[string, *PartialResult]
	benv   *bloblang.Environment
	config *config.Config
}

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

// ExecuteCumulative executes all root-assignment statements from the beginning of the
// document through throughLine (inclusive) and returns the resulting value.
//
// throughLine == -1 is a special case meaning "before any assignment"; in that case
// the raw sample is returned as-is (marshalled to JSON).
//
// This is used by code lenses (pass throughLine = lineIdx-1 to show the input state
// before the line) and inlay hints (pass throughLine = lineIdx to show the output
// state after the line).
func (e *Executor) ExecuteCumulative(parser *Bloblang, uri string, sample interface{}, docText string, throughLine int) (*PartialResult, error) {
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

func (e *Executor) InvalidateDocument(uri string) {
	prefix := uri + ":"
	for _, key := range e.cache.Keys() {
		if strings.HasPrefix(key, prefix) {
			e.cache.Remove(key)
		}
	}
}

func looksIncomplete(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "unexpected end") ||
		strings.Contains(msg, "unterminated")
}
