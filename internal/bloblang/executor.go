package bloblang

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/teyfix/bloblang-lsp/internal/config"
	pretty "github.com/teyfix/bloblang-lsp/internal/tidwall"
)

// rootAssignLineRe matches any line that starts a root assignment (simple, dot, or bracket path).
// This mirrors the lsp.rootAssignRe pattern but lives here to avoid an import cycle.
var rootAssignLineRe = regexp.MustCompile(`^\s*root[\s.\[].*=`)

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

func (e *Executor) ExecutePartial(uri string, sample interface{}, docText string, rootLine int) (*PartialResult, error) {
	if len(docText) > e.config.MaxInlineDocumentBytes {
		return nil, nil
	}
	key := uri + ":" + strconv.Itoa(rootLine)
	if result, ok := e.cache.Get(key); ok {
		return result, nil
	}

	lines := strings.Split(docText, "\n")
	if rootLine < 0 || rootLine >= len(lines) {
		return nil, fmt.Errorf("root line out of range")
	}

	end := statementEnd(lines, rootLine)
	var parsed *bloblang.Executor
	var err error
	for attempts := 0; attempts < 4; attempts++ {
		truncated := strings.Join(lines[rootLine:end], "\n")
		parsed, err = e.benv.Parse(truncated)
		if err == nil {
			break
		}
		if end >= len(lines) || !looksIncomplete(err) {
			return nil, err
		}
		end++
	}
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
		text = text[:e.config.MaxInlineResultBytes] + "..."
		truncated = true
	}

	result := &PartialResult{Text: text, Truncated: truncated, Full: full}
	e.cache.Add(key, result)
	return result, nil
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
func (e *Executor) ExecuteCumulative(uri string, sample interface{}, docText string, throughLine int) (*PartialResult, error) {
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
			text = text[:e.config.MaxInlineResultBytes] + "..."
			truncated = true
		}
		result := &PartialResult{Text: text, Truncated: truncated, Full: full}
		e.cache.Add(key, result)
		return result, nil
	}

	lines := strings.Split(docText, "\n")
	if throughLine >= len(lines) {
		return nil, fmt.Errorf("through line out of range")
	}

	// Collect all root-assignment statement blocks from line 0 through throughLine.
	var snippetLines []string
	i := 0
	for i <= throughLine {
		line := lines[i]
		if rootAssignLineRe.MatchString(line) {
			// Include this line and any continuation lines (non-empty, non-root, non-let).
			end := statementEnd(lines, i)
			// Clamp to throughLine so we don't include lines beyond the requested boundary.
			if end > throughLine+1 {
				end = throughLine + 1
			}
			snippetLines = append(snippetLines, lines[i:end]...)
			i = end
		} else {
			i++
		}
	}

	if len(snippetLines) == 0 {
		// No root assignments up to throughLine — return raw sample.
		return e.ExecuteCumulative(uri, sample, docText, -1)
	}

	snippet := strings.Join(snippetLines, "\n")
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
		text = text[:e.config.MaxInlineResultBytes] + "..."
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

func statementEnd(lines []string, rootLine int) int {
	end := rootLine + 1
	for end < len(lines) {
		line := lines[end]
		if strings.TrimSpace(line) == "" {
			break
		}
		if strings.HasPrefix(line, "root") || strings.HasPrefix(line, "let") {
			break
		}
		end++
	}
	return end
}

func looksIncomplete(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "unexpected end") ||
		strings.Contains(msg, "unterminated")
}
