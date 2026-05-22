package bloblang

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
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
		full = string(pretty.PrettyOptions(encoded, &pretty.Options{
			Width:    e.config.MaxInlineResultBytes,
			Prefix:   "",
			Indent:   "  ",
			SortKeys: false,
		}))
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
