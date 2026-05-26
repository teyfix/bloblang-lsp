package bloblang

import (
	"fmt"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/teyfix/bloblang-lsp/modular/pipeline"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// SampleEvaluator decouples bloblang Reducers from the sample package.
type SampleEvaluator interface {
	GetSample(uri string) (interface{}, bool)
	ExecuteCumulative(uri string, text string, throughLine int) (*PartialResult, error)
}

// 1. Syntax Reducer
type SyntaxReducer struct{}

func NewSyntaxReducer() *SyntaxReducer {
	return &SyntaxReducer{}
}

func (r *SyntaxReducer) Name() string { return "syntax" }

func (r *SyntaxReducer) Interest() []pipeline.DocumentEvent {
	return []pipeline.DocumentEvent{pipeline.EventDidOpen, pipeline.EventDidChange}
}

func (r *SyntaxReducer) Handle(state *pipeline.DocumentState, change *pipeline.DocumentChange) *pipeline.EventResult {
	return &pipeline.EventResult{
		ListDiagnostic: SyntaxDiagnostics(state.Text, state.Tree),
	}
}

// 2. Environment Reducer
type EnvReducer struct{}

func NewEnvReducer() *EnvReducer {
	return &EnvReducer{}
}

func (r *EnvReducer) Name() string { return "environment" }

func (r *EnvReducer) Interest() []pipeline.DocumentEvent {
	return []pipeline.DocumentEvent{pipeline.EventDidOpen, pipeline.EventDidChange}
}

func (r *EnvReducer) Handle(state *pipeline.DocumentState, change *pipeline.DocumentChange) *pipeline.EventResult {
	return &pipeline.EventResult{
		ListDiagnostic: EnvDiagnostics(state.Text, state.BaseDir),
	}
}

// 3. Import Hint Reducer
type ImportHintReducer struct{}

func NewImportHintReducer() *ImportHintReducer {
	return &ImportHintReducer{}
}

func (r *ImportHintReducer) Name() string { return "import_hint" }

func (r *ImportHintReducer) Interest() []pipeline.DocumentEvent {
	return []pipeline.DocumentEvent{pipeline.EventDidOpen, pipeline.EventDidChange}
}

func (r *ImportHintReducer) Handle(state *pipeline.DocumentState, change *pipeline.DocumentChange) *pipeline.EventResult {
	return &pipeline.EventResult{
		ListDiagnostic: ImportHintDiagnostics(string(state.URI), state.Text, state.Tree, state.BaseDir),
	}
}

// 4. Completion Reducer
type CompletionReducer struct {
	completionItems []protocol.CompletionItem
}

func NewCompletionReducer(completionItems []protocol.CompletionItem) *CompletionReducer {
	return &CompletionReducer{
		completionItems: completionItems,
	}
}

func (r *CompletionReducer) Name() string { return "completion" }

func (r *CompletionReducer) Interest() []pipeline.DocumentEvent {
	return []pipeline.DocumentEvent{pipeline.EventCompletion}
}

func (r *CompletionReducer) Handle(state *pipeline.DocumentState, change *pipeline.DocumentChange) *pipeline.EventResult {
	items := r.completionItems
	if state.Text != "" {
		lines := strings.Split(state.Text, "\n")
		switch getCompletionContext(lines, change.Completion.Position.Line, change.Completion.Position.Character) {
		case "method":
			items = filterCompletions(items, protocol.CompletionItemKindMethod)
		case "function":
			items = filterCompletions(items, protocol.CompletionItemKindFunction)
		case "variable":
			items = []protocol.CompletionItem{}
		}
	}
	return &pipeline.EventResult{
		CompletionList: &protocol.CompletionList{Items: items},
	}
}

// 5. Hover Reducer
type HoverReducer struct {
	functionDocs map[string]protocol.MarkupContent
	methodDocs   map[string]protocol.MarkupContent
	evaluator    SampleEvaluator
}

func NewHoverReducer(fnDocs, methDocs map[string]protocol.MarkupContent, eval SampleEvaluator) *HoverReducer {
	return &HoverReducer{
		functionDocs: fnDocs,
		methodDocs:   methDocs,
		evaluator:    eval,
	}
}

func (r *HoverReducer) Name() string { return "hover" }

func (r *HoverReducer) Interest() []pipeline.DocumentEvent {
	return []pipeline.DocumentEvent{pipeline.EventHover}
}

func (r *HoverReducer) Handle(state *pipeline.DocumentState, change *pipeline.DocumentChange) *pipeline.EventResult {
	if state.Text == "" || state.Tree == nil {
		return &pipeline.EventResult{}
	}

	lineIdx := change.Hover.Position.Line
	col := change.Hover.Position.Character
	root := state.Tree.RootNode()
	point := tree_sitter.Point{
		Row:    uint(lineIdx),
		Column: uint(col),
	}

	node := root.DescendantForPointRange(point, point)
	if node == nil {
		return &pipeline.EventResult{}
	}

	// 1. Root assignment hover (cumulative result display)
	if node.Kind() == "root" && node.Parent() != nil && node.Parent().Kind() == "root_assignment" {
		_, ok := r.evaluator.GetSample(string(state.URI))
		if !ok {
			return &pipeline.EventResult{
				Hover: &protocol.Hover{
					Contents: protocol.MarkupContent{
						Kind:  protocol.Markdown,
						Value: "Provide a sample with `#!sample {\"key\": \"value\"}`",
					},
				},
			}
		}
		result, err := r.evaluator.ExecuteCumulative(string(state.URI), state.Text, lineIdx)
		if err != nil || result == nil {
			return &pipeline.EventResult{}
		}
		return &pipeline.EventResult{
			Hover: &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: fmt.Sprintf("```json\n%s\n```", result.Full),
				},
				Range: &protocol.Range{
					Start: protocol.Position{Line: int(node.StartPosition().Row), Character: int(node.StartPosition().Column)},
					End:   protocol.Position{Line: int(node.EndPosition().Row), Character: int(node.EndPosition().Column)},
				},
			},
		}
	}

	// 2. Functions & Methods hover
	if node.Kind() == "identifier" && node.Parent() != nil {
		parent := node.Parent()
		token := node.Utf8Text([]byte(state.Text))
		var doc protocol.MarkupContent
		var found bool
		if parent.Kind() == "method_call" {
			doc, found = r.methodDocs[token]
		} else if parent.Kind() == "call_expr" {
			doc, found = r.functionDocs[token]
		}
		if found {
			return &pipeline.EventResult{
				Hover: &protocol.Hover{
					Contents: doc,
					Range: &protocol.Range{
						Start: protocol.Position{Line: int(node.StartPosition().Row), Character: int(node.StartPosition().Column)},
						End:   protocol.Position{Line: int(node.EndPosition().Row), Character: int(node.EndPosition().Column)},
					},
				},
			}
		}
	}

	return &pipeline.EventResult{}
}

// Helpers
func getCompletionContext(lines []string, line, char int) string {
	if line < 0 || line >= len(lines) {
		return ""
	}
	l := lines[line]
	if char < 0 || char > len(l) {
		return ""
	}
	prefix := l[:char]
	if strings.HasSuffix(prefix, ".") {
		return "method"
	}
	if strings.HasSuffix(prefix, "@") || strings.HasSuffix(prefix, "$") {
		return "variable"
	}
	return "function"
}

func filterCompletions(items []protocol.CompletionItem, kind protocol.CompletionItemKind) []protocol.CompletionItem {
	var filtered []protocol.CompletionItem
	for _, item := range items {
		if item.Kind != nil && *item.Kind == kind {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
