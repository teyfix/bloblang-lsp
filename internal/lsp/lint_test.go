package lsp

import (
	"context"
	"encoding/json"
	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lintCodes(h *Handler, uri protocol.DocumentURI, s string) map[string]int {
	out := map[string]int{}
	for _, d := range h.lintDiagnostics(uri, s) {
		var id string
		_ = json.Unmarshal(d.Code, &id)
		out[id]++
	}
	return out
}
func TestInitialLintRules(t *testing.T) {
	h, uri := featureHandler(t)
	cases := []struct{ rule, yes, no string }{
		{"correctness/environment/require-fallback", `root.artifact_dir = env("ARTIFACT_DIR")`, `root.artifact_dir = env("ARTIFACT_DIR").or(throw("required"))`},
		{"correctness/variables/no-unused-let", "let unused = 1\nroot = this", "let used = 1\nroot = $used"},
		{"style/assignments/prefer-grouped", "root.foo = this.foo\nroot.bar = this.bar\nroot.baz = 3", "root.foo = root.bar\nroot.bar = this.bar\nroot.baz = 3"},
		{"style/objects/prefer-with", `root = {foo:this.foo, bar:this.bar}`, `root = {renamed:this.foo}`},
		{"style/objects/prefer-without", "root = this\nroot.foo = deleted()", "root.foo = 1\nroot.bar = deleted()"},
		{"style/objects/combine-without", `root = this.without("a").without("b")`, `root = this.without("a", "b")`},
		{"style/arrays/prefer-any", `root = this.items.filter(this.ok).length() > 0`, `root = this.items.filter(this.ok).length() > 1`},
	}
	for _, c := range cases {
		require.Positive(t, lintCodes(h, uri, c.yes)[c.rule], c.rule)
		require.Zero(t, lintCodes(h, uri, c.no)[c.rule], c.rule)
	}
	require.Positive(t, lintCodes(h, uri, `root = env("X").catch("fallback")`)[cases[0].rule])
	require.Zero(t, lintCodes(h, uri, `root = env("X").not_null().catch(throw("required"))`)[cases[0].rule])
	require.Zero(t, lintCodes(h, uri, `root = env("X") | "fallback"`)[cases[0].rule])
	require.Zero(t, lintCodes(h, uri, "# bloblang-lint-disable-next-line "+cases[0].rule+" -- deliberate\nroot = env(\"X\")")[cases[0].rule])
	require.Zero(t, lintCodes(h, uri, "root = this\nroot.foo = {new:2}\nroot.bar = 2\nroot.baz = 3")["style/assignments/prefer-grouped"])
}
func TestLintConfigAndSafeQuickFix(t *testing.T) {
	h, uri := featureHandler(t)
	text := `root = this.without("a").without("b")`
	openFeature(t, h, uri, text)
	p := &protocol.CodeActionParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}, Range: protocol.Range{End: protocol.Position{Line: 99}}}
	actions, e := h.CodeAction(context.Background(), p)
	require.NoError(t, e)
	require.Len(t, actions, 1)
	require.Equal(t, `this.without("a", "b")`, actions[0].Edit.Changes[uri][0].NewText)
	require.NoError(t, os.WriteFile(filepath.Join(h.workspaceRoot, ".bloblangrc.json"), []byte(`{"lint":{"rules":{"style/objects/combine-without":"off"}}}`), 0600))
	require.Empty(t, h.lintDiagnostics(uri, text))
	openFeature(t, h, uri, strings.Replace(text, `"b"`, `throw("effect")`, 1))
	actions, e = h.CodeAction(context.Background(), p)
	require.NoError(t, e)
	require.Empty(t, actions)
}
