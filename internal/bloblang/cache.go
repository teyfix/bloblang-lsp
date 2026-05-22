package bloblang

import (
	"fmt"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
)

const includeDeprecated = false
const defaultDocsURL = "https://docs.redpanda.com/redpanda-connect/guides/bloblang"

// BuildCompletionCache generates the completion cache for all functions and methods.
func BuildCompletionCache(env *bloblang.Environment) (
	[]protocol.CompletionItem,
	map[string]bloblang.TemplateFunctionData,
	map[string]bloblang.TemplateMethodData,
) {
	fnDocs := make(map[string]bloblang.TemplateFunctionData)
	methDocs := make(map[string]bloblang.TemplateMethodData)

	var items []protocol.CompletionItem

	kindFunc := protocol.CompletionItemKindFunction
	kindMethod := protocol.CompletionItemKindMethod

	// Functions
	env.WalkFunctions(func(name string, view *bloblang.FunctionView) {
		data := view.TemplateData()

		switch data.Status {
		case "hidden":
			return // never surface hidden items
		case "deprecated":
			if !includeDeprecated {
				return
			}
		}

		fnDocs[name] = data

		snippet, signature := generatePositionalSnippet(data.Params)
		items = append(items, buildFunctionItem(data, name, snippet, signature, kindFunc, false))

		// When optional params exist, also offer a named-args variant so users
		// can selectively supply middle optional arguments by name.
		if hasOptionalParams(data.Params) {
			namedSnippet := generateNamedArgsSnippet(data.Params)
			items = append(items, buildFunctionItem(data, name, namedSnippet, signature, kindFunc, true))
		}
	})

	// Methods
	env.WalkMethods(func(name string, view *bloblang.MethodView) {
		data := view.TemplateData()

		switch data.Status {
		case "hidden":
			return
		case "deprecated":
			if !includeDeprecated {
				return
			}
		}

		methDocs[name] = data

		snippet, signature := generatePositionalSnippet(data.Params)
		items = append(items, buildMethodItem(data, name, snippet, signature, kindMethod, false))

		if hasOptionalParams(data.Params) {
			namedSnippet := generateNamedArgsSnippet(data.Params)
			items = append(items, buildMethodItem(data, name, namedSnippet, signature, kindMethod, true))
		}
	})

	return items, fnDocs, methDocs
}

// ─── Status helpers ───────────────────────────────────────────────────────────

func statusPrefix(status string) string {
	switch status {
	case "beta":
		return "[β] "
	case "experimental":
		return "[⚗] "
	case "deprecated":
		return "[⚠] "
	default:
		return ""
	}
}

func statusSortText(status, name string) string {
	switch status {
	case "stable", "":
		return "0_" + name
	case "beta", "experimental":
		return "1_" + name
	default:
		return "2_" + name
	}
}

// ─── Param introspection ──────────────────────────────────────────────────────

func isOptionalParam(p bloblang.TemplateParamData) bool {
	return p.IsOptional || p.DefaultMarshalled != ""
}

func hasOptionalParams(params bloblang.TemplateParamsData) bool {
	for _, p := range params.Definitions {
		if isOptionalParam(p) {
			return true
		}
	}
	return false
}

func inferSnippetValue(p bloblang.TemplateParamData) string {
	if p.DefaultMarshalled != "" {
		return p.DefaultMarshalled
	}
	switch strings.ToLower(p.ValueType) {
	case "string":
		return ""
	case "integer", "number", "float":
		return "0"
	case "bool":
		return "true"
	case "array":
		return "[]"
	case "object":
		return "{}"
	case "query expression":
		return p.Name
	default:
		return "null"
	}
}

// ─── Snippet generation ───────────────────────────────────────────────────────

func generatePositionalSnippet(params bloblang.TemplateParamsData) (snippet, signature string) {
	var snippetParts []string
	var sigParts []string

	tabIndex := 1
	for _, p := range params.Definitions {
		optional := isOptionalParam(p)
		optMark := ""
		if optional {
			optMark = "?"
		}
		sigParts = append(sigParts, fmt.Sprintf("%s%s: %s", p.Name, optMark, p.ValueType))

		if optional {
			continue
		}

		pType := strings.ToLower(p.ValueType)
		snippetValue := inferSnippetValue(p)

		switch pType {
		case "query expression":
			snippetParts = append(snippetParts, fmt.Sprintf("%s -> ${%d:%s}", p.Name, tabIndex, p.Name))
		case "string":
			snippetParts = append(snippetParts, fmt.Sprintf("\"${%d:%s}\"", tabIndex, snippetValue))
		default:
			snippetParts = append(snippetParts, fmt.Sprintf("${%d:%s}", tabIndex, snippetValue))
		}
		tabIndex++
	}

	if len(snippetParts) == 0 && params.Variadic {
		snippetParts = append(snippetParts, "\"${1:value}\"")
		sigParts = append(sigParts, "...params")
	}

	return strings.Join(snippetParts, ", "), strings.Join(sigParts, ", ")
}

func generateNamedArgsSnippet(params bloblang.TemplateParamsData) string {
	var parts []string
	for i, p := range params.Definitions {
		tabIndex := i + 1
		pType := strings.ToLower(p.ValueType)
		snippetValue := inferSnippetValue(p)

		var part string
		switch pType {
		case "string":
			part = fmt.Sprintf("%s: \"${%d:%s}\"", p.Name, tabIndex, snippetValue)
		case "query expression":
			if p.DefaultMarshalled != "" {
				part = fmt.Sprintf("%s: ${%d:%s}", p.Name, tabIndex, snippetValue)
			} else {
				part = fmt.Sprintf("%s: %s -> ${%d:%s}", p.Name, p.Name, tabIndex, p.Name)
			}
		default:
			part = fmt.Sprintf("%s: ${%d:%s}", p.Name, tabIndex, snippetValue)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

// ─── Completion item builders ─────────────────────────────────────────────────

func buildFunctionItem(
	data bloblang.TemplateFunctionData,
	name, snippet, signature string,
	kind protocol.CompletionItemKind,
	namedArgs bool,
) protocol.CompletionItem {
	prefix := statusPrefix(data.Status)
	label := prefix + name
	if namedArgs {
		label += " (named args)"
	}

	insertText := name + "()"
	if snippet != "" {
		insertText = fmt.Sprintf("%s(%s)", name, snippet)
	}

	detailText := fmt.Sprintf("%s(%s)", name, signature)
	if data.Category != "" {
		detailText += fmt.Sprintf(" — Function (%s)", data.Category)
	}

	sortText := statusSortText(data.Status, name)
	if namedArgs {
		sortText += "_z"
	}

	doc := BuildDocumentation(data.Name, "functions", data.Description, data.Examples, nil, data.Version, data.Status, defaultDocsURL)

	snippetFormat := protocol.InsertTextFormat(2) // 2 represents Snippet in LSP specification
	item := protocol.CompletionItem{
		Label:            label,
		Kind:             &kind,
		Detail:           detailText,
		Documentation:    &doc,
		InsertText:       insertText,
		InsertTextFormat: &snippetFormat,
		SortText:         sortText,
	}

	if data.Status == "deprecated" {
		item.Tags = []protocol.CompletionItemTag{protocol.CompletionItemTag(1)} // 1 is standard Deprecated tag value
	}

	return item
}

func buildMethodItem(
	data bloblang.TemplateMethodData,
	name, snippet, signature string,
	kind protocol.CompletionItemKind,
	namedArgs bool,
) protocol.CompletionItem {
	prefix := statusPrefix(data.Status)
	label := prefix + name
	if namedArgs {
		label += " (named args)"
	}

	insertText := name + "()"
	if snippet != "" {
		insertText = fmt.Sprintf("%s(%s)", name, snippet)
	}

	primaryCat := ""
	if len(data.Categories) > 0 {
		primaryCat = data.Categories[0].Category
	}
	detailText := fmt.Sprintf(".%s(%s) — Method", name, signature)
	if primaryCat != "" {
		detailText += fmt.Sprintf(" · %s", primaryCat)
	}

	sortText := statusSortText(data.Status, name)
	if namedArgs {
		sortText += "_z"
	}

	doc := BuildDocumentation(data.Name, "methods", data.Description, data.Examples, data.Categories, data.Version, data.Status, defaultDocsURL)

	snippetFormat := protocol.InsertTextFormat(2)
	item := protocol.CompletionItem{
		Label:            label,
		Kind:             &kind,
		Detail:           detailText,
		Documentation:    &doc,
		InsertText:       insertText,
		InsertTextFormat: &snippetFormat,
		SortText:         sortText,
	}

	if data.Status == "deprecated" {
		item.Tags = []protocol.CompletionItemTag{protocol.CompletionItemTag(1)}
	}

	return item
}
