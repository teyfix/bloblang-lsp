package lsp_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/teyfix/bloblang-lsp/internal/testutil"
)

func TestIntegrationMethodCompletion(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/method.blobl"
	h.OpenDocument(uri, "root = this.")
	items := h.GetCompletions(uri, protocol.Position{Line: 0, Character: len("root = this.")})
	require.NotEmpty(t, items)
	for _, item := range items {
		require.NotNil(t, item.Kind)
		assert.Equal(t, protocol.CompletionItemKindMethod, *item.Kind)
	}
	testutil.AssertCompletionContains(t, items, "string")
}

func TestIntegrationFunctionCompletion(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/function.blobl"
	h.OpenDocument(uri, "root = ")
	items := h.GetCompletions(uri, protocol.Position{Line: 0, Character: len("root = ")})
	require.NotEmpty(t, items)
	for _, item := range items {
		require.NotNil(t, item.Kind)
		assert.Equal(t, protocol.CompletionItemKindFunction, *item.Kind)
	}
	testutil.AssertCompletionContains(t, items, "json")
}

func TestIntegrationFunctionHover(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/hover.blobl"
	h.OpenDocument(uri, "root = json(\"name\")")
	hover := h.GetHover(uri, protocol.Position{Line: 0, Character: len("root = js")})
	require.NotNil(t, hover)
	assert.Contains(t, hover.Contents.Value, "# [json]")
}

func TestIntegrationDiagnostics(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/diag.blobl"
	h.OpenDocument(uri, "root = (")
	diags := h.WaitForDiagnostics(uri, time.Second)
	require.NotEmpty(t, diags)
	assert.Equal(t, protocol.SeverityError, *diags[0].Severity)

	h.Server.ClearDiagnostics()
	h.ChangeDocument(uri, "root = \"ok\"")
	diags = h.WaitForDiagnostics(uri, time.Second)
	assert.Empty(t, diags)
}

func TestIntegrationDebounceCorrectness(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/debounce.blobl"
	h.OpenDocument(uri, "root = \"ok\"")
	_ = h.WaitForDiagnostics(uri, time.Second)
	h.Server.ClearDiagnostics()

	for i := 0; i < 10; i++ {
		h.ChangeDocument(uri, "root = \"ok\"")
	}
	time.Sleep(100 * time.Millisecond)
	published := h.Server.AllDiagnostics()
	require.Len(t, published, 1)
	assert.Equal(t, protocol.DocumentURI(uri), published[0].URI)
}

func TestIntegrationCloseClearsDiagnostics(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/close.blobl"
	h.OpenDocument(uri, "root = (")
	diags := h.WaitForDiagnostics(uri, time.Second)
	require.NotEmpty(t, diags)
	h.Server.ClearDiagnostics()
	h.CloseDocument(uri)
	diags = h.WaitForDiagnostics(uri, time.Second)
	assert.Empty(t, diags)
}

func TestIntegrationImportHints(t *testing.T) {
	h := testutil.NewHarness(t)
	dir := filepath.ToSlash(t.TempDir())
	uri := "file:///" + strings.TrimPrefix(dir, "/") + "/map.blobl"
	h.OpenDocument(uri, "import \"foo.blobl\"\nroot = \"ok\"")
	diags := h.WaitForDiagnostics(uri, time.Second)
	testutil.AssertDiagnosticAt(t, diags, 0, filepath.Join(filepath.FromSlash(dir), "foo.blobl"))
}

func TestIntegrationUntitledImportHints(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "untitled:///draft"
	h.OpenDocument(uri, "import \"foo.blobl\"\nroot = \"ok\"")
	diags := h.WaitForDiagnostics(uri, time.Second)
	require.NotEmpty(t, diags)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	testutil.AssertDiagnosticAt(t, diags, 0, filepath.Join(home, "foo.blobl"))
	testutil.AssertDiagnosticAt(t, diags, 0, "Import base directory")
}

func TestIntegrationInlayHintsAndCodeLens(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/inlay.blobl"
	h.OpenDocument(uri, "#!sample {\"name\":\"alice\"}\nroot = this.name")

	// Inlay hint shows the OUTPUT after the line: root = this.name → "alice"
	hints := h.GetInlayHints(uri)
	testutil.AssertInlayHintAt(t, hints, 1, `= "alice"`)

	// Code lens shows the INPUT before the line: no prior assignments → raw sample {"name":"alice"}
	lenses := h.GetCodeLenses(uri)
	require.Len(t, lenses, 1)
	require.NotNil(t, lenses[0].Command)
	assert.Contains(t, lenses[0].Command.Title, "alice")
}

func TestIntegrationNoSampleInlayHintsEmpty(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/no-sample.blobl"
	h.OpenDocument(uri, "root = this.name")
	assert.Empty(t, h.GetInlayHints(uri))
}

func TestIntegrationTruncatedCodeLensCommand(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/truncated.blobl"
	h.OpenDocument(uri, "#!sample {\"name\":\"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz\"}\nroot = this.name")
	lenses := h.GetCodeLenses(uri)
	require.Len(t, lenses, 1)
	require.NotNil(t, lenses[0].Command)
	assert.Equal(t, "bloblang-lsp.showResult", lenses[0].Command.Command)
	require.Len(t, lenses[0].Command.Arguments, 3)
	var full string
	require.NoError(t, json.Unmarshal(lenses[0].Command.Arguments[2], &full))
	assert.Contains(t, full, "abcdefghijklmnopqrstuvwxyz")
}

func TestIntegrationRootHover(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/root-hover.blobl"
	h.OpenDocument(uri, "#!sample {\"name\":\"alice\"}\nroot = this.name")
	hover := h.GetHover(uri, protocol.Position{Line: 1, Character: 1})
	require.NotNil(t, hover)
	assert.Contains(t, hover.Contents.Value, "```json")
	assert.Contains(t, hover.Contents.Value, `"alice"`)
}

func TestIntegrationRootHoverNoSample(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/root-hover-no-sample.blobl"
	h.OpenDocument(uri, "root = this.name")
	hover := h.GetHover(uri, protocol.Position{Line: 0, Character: 1})
	require.NotNil(t, hover)
	assert.Contains(t, hover.Contents.Value, "#!sample")
}

func TestIntegrationRootHoverNonAssignment(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/root-hover-non-assignment.blobl"
	h.OpenDocument(uri, "let root = \"value\"")
	hover := h.GetHover(uri, protocol.Position{Line: 0, Character: 5})
	assert.Nil(t, hover)
}

func TestIntegrationSubPathInlayHintsAndCodeLens(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/subpath.blobl"
	// sub-path assignment: root.name = this.name
	h.OpenDocument(uri, "#!sample {\"name\":\"alice\"}\nroot.name = this.name")

	// inlay hint should appear on line 1 showing the output {"name":"alice"}
	hints := h.GetInlayHints(uri)
	testutil.AssertInlayHintAt(t, hints, 1, `"alice"`)

	// code lens should appear on line 1
	lenses := h.GetCodeLenses(uri)
	require.Len(t, lenses, 1)
	require.NotNil(t, lenses[0].Command)
}

func TestIntegrationCumulativeLensesAndHints(t *testing.T) {
	h := testutil.NewHarness(t)
	uri := "file:///tmp/cumulative.blobl"
	doc := "#!sample {\"name\":\"alice\",\"age\":30}\nroot.name = this.name\nroot.age = this.age"
	h.OpenDocument(uri, doc)

	// inlay hints: line 1 shows output after first assignment → {"name":"alice"}
	//              line 2 shows output after both assignments → {"age":30,"name":"alice"}
	hints := h.GetInlayHints(uri)
	testutil.AssertInlayHintAt(t, hints, 1, `"alice"`)
	testutil.AssertInlayHintAt(t, hints, 2, `"age"`)

	// code lenses: line 1 shows input before first assignment → raw sample
	//              line 2 shows input before second assignment → {"name":"alice"}
	lenses := h.GetCodeLenses(uri)
	require.Len(t, lenses, 2)

	// lens on line 1 (before any assignment) → raw sample contains "alice" and "age":30
	assert.Equal(t, 1, lenses[0].Range.Start.Line)
	assert.Contains(t, lenses[0].Command.Title, "alice")

	// lens on line 2 (after first assignment) → {"name":"alice"} only
	assert.Equal(t, 2, lenses[1].Range.Start.Line)
	assert.Contains(t, lenses[1].Command.Title, "alice")
	assert.NotContains(t, lenses[1].Command.Title, "age")
}
