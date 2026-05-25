package benthos

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/redpanda-data/benthos/v4/public/bloblang"
)

var kebabDelimiter = regexp.MustCompile(`[^a-z0-9]+`)

func renderCodeBlock(value string) string {
	value = strings.TrimSpace(value)

	var parsed any

	// Try JSON parse first
	if err := json.Unmarshal([]byte(value), &parsed); err == nil {
		pretty, err := json.MarshalIndent(parsed, "", "  ")
		if err == nil {
			single := regexp.MustCompile(`\n\s*`).ReplaceAllString(string(pretty), " ")
			return fmt.Sprintf("```json\n%s\n```\n", single)
		}
	}

	// Fallback
	return fmt.Sprintf("```txt\n%s\n```\n", value)
}

func toKebabCase(s string) string {
	return kebabDelimiter.ReplaceAllString(strings.ToLower(s), "-")
}

// BuildDocumentation assembles a Markdown MarkupContent value from all available metadata.
func BuildDocumentation(
	name string,
	kind string,
	description string,
	examples []bloblang.TemplateExampleData,
	categories []bloblang.TemplateMethodCategoryData,
	ver string,
	status string,
	docsURL string,
) protocol.MarkupContent {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# [%s](%s/%s/#%s) – %s\n\n", name, docsURL, kind, name, kind))

	// Status badge — shown at the top so it's immediately visible on hover.
	switch status {
	case "beta":
		sb.WriteString("> **[β Beta]** This component is in beta and its behaviour may change.\n\n")
	case "experimental":
		sb.WriteString("> **[⚗ Experimental]** This component is experimental and may be removed or changed.\n\n")
	case "deprecated":
		sb.WriteString("> **[⚠ Deprecated]** Avoid using this in new mappings.\n\n")
	}

	if description != "" {
		sb.WriteString(description)
		sb.WriteString("\n\n")
	}

	// Examples section — capped at 3 to keep hover docs readable.
	const maxExamples = 3
	if len(examples) > 0 {
		sb.WriteString("## Examples\n\n")

		shown := examples
		truncated := 0
		if len(examples) > maxExamples {
			shown = examples[:maxExamples]
			truncated = len(examples) - maxExamples
		}

		for i, ex := range shown {
			if ex.Summary != "" {
				sb.WriteString(strings.ReplaceAll(fmt.Sprintf("%s\n\n", ex.Summary), "#####", "###"))
			}

			sb.WriteString("### Mapping\n\n")
			sb.WriteString("```bloblang\n")
			sb.WriteString(strings.TrimSpace(ex.Mapping))
			sb.WriteString("\n```\n\n")

			// Render the input→output table only when the example is tested
			// (SkipTesting examples may produce different results in practice).
			if len(ex.Results) > 0 && !ex.SkipTesting {
				for j, r := range ex.Results {
					suffix := ""

					if len(ex.Results) > 1 {
						suffix = fmt.Sprintf(" – #%d", j+1)
					}

					sb.WriteString(
						fmt.Sprintf("#### Input%s\n\n", suffix),
					)
					sb.WriteString(renderCodeBlock(r[0]))
					sb.WriteString("\n")

					sb.WriteString(
						fmt.Sprintf("#### Output%s\n\n", suffix),
					)
					sb.WriteString(renderCodeBlock(r[1]))
					sb.WriteString("\n")
				}
				sb.WriteString("\n")
			}

			if ex.SkipTesting {
				sb.WriteString("> ⚠ This example is not tested in CI — results may vary.\n\n")
			}

			// Separator between examples
			if i < len(shown)-1 {
				sb.WriteString("---\n\n")
			}
		}

		if truncated > 0 {
			sb.WriteString(fmt.Sprintf("_…and %d more examples_\n\n", truncated))
		}
	}

	// Method categories — only present for MethodView items.
	if len(categories) > 0 {
		sb.WriteString("## Categories\n")

		for _, c := range categories {
			sb.WriteString(
				fmt.Sprintf("### [%s](%s/methods/#%s)\n\n", c.Category, docsURL, toKebabCase(c.Category)),
			)

			if c.Description != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", c.Description))
			}
		}
	}

	// Version footer.
	if ver != "" {
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("_Introduced in Benthos **%s**_\n", ver))
	}

	return protocol.MarkupContent{
		Kind:  protocol.MarkupKind("markdown"),
		Value: strings.TrimSpace(sb.String()),
	}
}

// BuildAllDocs pre-compiles markup contents for hover support.
func BuildAllDocs(
	fnDocs map[string]bloblang.TemplateFunctionData,
	methDocs map[string]bloblang.TemplateMethodData,
	docsURL string,
) (map[string]protocol.MarkupContent, map[string]protocol.MarkupContent) {
	fnMarkup := make(map[string]protocol.MarkupContent)
	methMarkup := make(map[string]protocol.MarkupContent)

	for name, data := range fnDocs {
		fnMarkup[name] = BuildDocumentation(data.Name, "functions", data.Description, data.Examples, nil, data.Version, data.Status, docsURL)
	}

	for name, data := range methDocs {
		methMarkup[name] = BuildDocumentation(data.Name, "methods", data.Description, data.Examples, data.Categories, data.Version, data.Status, docsURL)
	}

	return fnMarkup, methMarkup
}
