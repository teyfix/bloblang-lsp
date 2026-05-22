package bloblang

import (
	"testing"

	"github.com/redpanda-data/benthos/v4/public/bloblang"
	"github.com/stretchr/testify/assert"
)

func TestBuildDocumentation(t *testing.T) {
	ex := []bloblang.TemplateExampleData{
		{
			Summary: "test summary",
			Mapping: "root = this",
			Results: [][2]string{
				{"input", "output"},
			},
		},
	}
	doc := BuildDocumentation("test_func", "functions", "this is a test description", ex, nil, "v1.0.0", "beta", "https://docs.redpanda.com/redpanda-connect/guides/bloblang")

	assert.Contains(t, doc.Value, "# [test_func]")
	assert.Contains(t, doc.Value, "**[β Beta]**")
	assert.Contains(t, doc.Value, "this is a test description")
	assert.Contains(t, doc.Value, "test summary")
	assert.Contains(t, doc.Value, "Mapping")
	assert.Contains(t, doc.Value, "Input")
	assert.Contains(t, doc.Value, "Output")
	assert.Contains(t, doc.Value, "v1.0.0")
}

func TestBuildAllDocs(t *testing.T) {
	fnDocs := map[string]bloblang.TemplateFunctionData{
		"test_func": {
			Name:        "test_func",
			Description: "test desc",
			Status:      "beta",
		},
	}
	methDocs := map[string]bloblang.TemplateMethodData{
		"test_meth": {
			Name:        "test_meth",
			Description: "meth desc",
			Status:      "stable",
		},
	}

	fnMarkup, methMarkup := BuildAllDocs(fnDocs, methDocs, "https://docs.redpanda.com/redpanda-connect/guides/bloblang")
	assert.Contains(t, fnMarkup["test_func"].Value, "# [test_func]")
	assert.Contains(t, methMarkup["test_meth"].Value, "# [test_meth]")
}

func TestBuildAllDocsKnownEntries(t *testing.T) {
	env := NewEnvironment()
	_, fnDocs, methDocs := BuildCompletionCache(env)
	fnMarkup, methMarkup := BuildAllDocs(fnDocs, methDocs, "https://docs.redpanda.com/redpanda-connect/guides/bloblang")

	assert.Contains(t, fnMarkup, "json")
	assert.NotEmpty(t, fnMarkup["json"].Value)
	assert.Contains(t, methMarkup, "string")
	assert.NotEmpty(t, methMarkup["string"].Value)
}
