package testutil

import (
	"encoding/json"
	"strings"
	"testing"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/stretchr/testify/require"
)

func AssertDiagnosticAt(t *testing.T, diags []protocol.Diagnostic, line int, substr string) {
	t.Helper()
	for _, diag := range diags {
		if diag.Range.Start.Line == line && strings.Contains(diag.Message, substr) {
			return
		}
	}
	t.Fatalf("diagnostic at line %d containing %q not found in %#v", line, substr, diags)
}

func AssertCompletionContains(t *testing.T, items []protocol.CompletionItem, label string) {
	t.Helper()
	for _, item := range items {
		if item.Label == label {
			return
		}
	}
	t.Fatalf("completion %q not found", label)
}

func AssertInlayHintAt(t *testing.T, hints []protocol.InlayHint, line int, contains string) {
	t.Helper()
	for _, hint := range hints {
		if hint.Position.Line != line {
			continue
		}
		var label string
		require.NoError(t, json.Unmarshal(hint.Label, &label))
		if strings.Contains(label, contains) {
			return
		}
	}
	t.Fatalf("inlay hint at line %d containing %q not found", line, contains)
}
