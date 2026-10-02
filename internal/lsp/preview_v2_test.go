package lsp

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspacePreviewReloadAndDeclarationHover(t *testing.T) {
	h, uri := featureHandler(t)
	lang, body := h.preview(uri, `{"name":"Ada","items":[1,2]}`)
	require.Equal(t, "yaml", lang)
	require.Contains(t, body, "name: Ada")
	require.NoError(t, os.WriteFile(filepath.Join(h.workspaceRoot, ".bloblangrc.json"), []byte(`{"formatter":{"printWidth":30},"preview":{"format":"json"}}`), 0600))
	lang, body = h.preview(uri, `{"a":1}`)
	require.Equal(t, "json", lang)
	require.Equal(t, `{"a": 1}`, body)
	text := "#!input {\"name\":\"Ada\"}\nlet person = this.name\nroot = $person\n"
	openFeature(t, h, uri, text)
	v := hoverAt(t, h, uri, text, "person =")
	require.NotNil(t, v)
	require.Contains(t, v.Contents.Value(), "Ada")
	v = hoverAt(t, h, uri, text, "$person")
	require.NotNil(t, v)
	require.Contains(t, v.Contents.Value(), "Ada")
	require.NoError(t, os.WriteFile(filepath.Join(h.workspaceRoot, ".bloblangrc.json"), []byte(`{"preview":{"format":"bogus"}}`), 0600))
	lang, _ = h.preview(uri, `{"a":1}`)
	require.Equal(t, "yaml", lang)
}
func TestDeletedAndExpressionOnlyDocumentation(t *testing.T) {
	h, uri := featureHandler(t)
	text := "root.foo = deleted()"
	openFeature(t, h, uri, text)
	v := hoverAt(t, h, uri, text, "deleted")
	require.NotNil(t, v)
	require.Contains(t, v.Contents.Value(), "deleted")
	uri = fileURI(filepath.Join(h.workspaceRoot, "config.yaml"))
	text = "check: |\n  !errored() && this.state != \"processing-ready\"\n"
	openFeature(t, h, uri, text)
	v = hoverAt(t, h, uri, text, "errored")
	require.NotNil(t, v)
	require.Contains(t, v.Contents.Value(), "errored")
}
func TestInputDirectiveCompletedClearsErrors(t *testing.T) {
	h, uri := featureHandler(t)
	for _, text := range []string{"#!input {\"foo\":\nroot = this", "#!input {\"foo\":\"bar\"}\nroot = this"} {
		openFeature(t, h, uri, text)
		ds := h.diagnosticsText(uri, text)
		if strings.Contains(text, "bar") {
			for _, d := range ds {
				require.NotContains(t, d.Message, "unexpected")
				require.NotContains(t, d.Message, "end")
			}
			require.NotNil(t, h.getSample(uri))
		} else {
			require.NotEmpty(t, ds)
		}
	}
}

func TestDirectiveWithoutMappingIsValidEditorState(t *testing.T) {
	h, uri := featureHandler(t)
	text := `#!input {"foo":"bar"}`
	openFeature(t, h, uri, text)
	require.Empty(t, h.diagnosticsText(uri, text))
	require.NotNil(t, h.getSample(uri))
}
