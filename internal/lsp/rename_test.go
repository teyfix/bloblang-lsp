package lsp

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/servertest"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func renameAt(t *testing.T, h *Handler, uri protocol.DocumentURI, text, needle, name string) (*protocol.WorkspaceEdit, error) {
	t.Helper()
	offset := strings.Index(text, needle)
	require.GreaterOrEqual(t, offset, 0)
	p := protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}, Position: bytePosition(text, offset)}
	prepared, err := h.PrepareRename(context.Background(), &protocol.PrepareRenameParams{TextDocumentPositionParams: p})
	require.NoError(t, err)
	require.NotNil(t, prepared, needle)
	require.NotContains(t, prepared.Placeholder, "$")
	return h.Rename(context.Background(), &protocol.RenameParams{TextDocumentPositionParams: p, NewName: name})
}
func applyRenameEdits(text string, edits []protocol.TextEdit) string {
	edits = append([]protocol.TextEdit(nil), edits...)
	sort.Slice(edits, func(i, j int) bool {
		return positionByte(text, edits[i].Range.Start) > positionByte(text, edits[j].Range.Start)
	})
	for _, e := range edits {
		text = text[:positionByte(text, e.Range.Start)] + e.NewText + text[positionByte(text, e.Range.End):]
	}
	return text
}
func TestRenameBindingsAndScopes(t *testing.T) {
	h, uri := featureHandler(t)
	text := `let movie = this
map embedded {
 let movie = this.title
 root = $movie
}
root = [$movie, this.movie, "movie", this.apply("embedded")]
`
	openFeature(t, h, uri, text)
	edit, err := renameAt(t, h, uri, text, "movie = this\n", "film")
	require.NoError(t, err)
	require.Len(t, edit.Changes[uri], 2)
	result := applyRenameEdits(text, edit.Changes[uri])
	require.Contains(t, result, "let movie = this.title\n root = $movie")
	require.Contains(t, result, `root = [$film, this.movie, "movie", this.apply("embedded")]`)
	_, err = h.benv.Parse(result)
	require.NoError(t, err)

	text = `root = this.map_each(c -> [c.name, c.items.map_each(c -> c.name), c.title])`
	openFeature(t, h, uri, text)
	edit, err = renameAt(t, h, uri, text, "c -> [", "item")
	require.NoError(t, err)
	result = applyRenameEdits(text, edit.Changes[uri])
	require.Equal(t, `root = this.map_each(item -> [item.name, item.items.map_each(c -> c.name), item.title])`, result) // receiver belongs to outer scope
}
func TestRenameLambdaReceiverScope(t *testing.T) {
	h, uri := featureHandler(t)
	text := `root = this.map_each(c -> c.items.map_each(inner -> [c.name, inner.name]))`
	openFeature(t, h, uri, text)
	edit, err := renameAt(t, h, uri, text, "c ->", "item")
	require.NoError(t, err)
	result := applyRenameEdits(text, edit.Changes[uri])
	require.Equal(t, `root = this.map_each(item -> item.items.map_each(inner -> [item.name, inner.name]))`, result)
	_, err = h.benv.Parse(result)
	require.NoError(t, err)
	_, err = renameAt(t, h, uri, text, "c ->", "inner")
	require.ErrorContains(t, err, "scope")
}
func TestRenameRejectsCollisionsAndInvalidNames(t *testing.T) {
	h, uri := featureHandler(t)
	text := "let a = 1\nlet b = 2\nroot = $a + $b\n"
	openFeature(t, h, uri, text)
	for _, name := range []string{"b", "$bad", "foo.bar", "two words", "1bad", ""} {
		_, err := renameAt(t, h, uri, text, "a =", name)
		require.Error(t, err, name)
	}
	for _, needle := range []string{"root", "1"} {
		prepared, err := h.PrepareRename(context.Background(), &protocol.PrepareRenameParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri}, Position: bytePosition(text, strings.Index(text, needle))}})
		require.NoError(t, err)
		require.Nil(t, prepared)
	}
}
func TestRenameYAMLStylesAndUnicode(t *testing.T) {
	h, _ := featureHandler(t)
	uri := fileURI(filepath.Join(h.workspaceRoot, "config.yaml"))
	for _, text := range []string{
		"mapping: |\n  let movie = \"🎬\"\n  root = $movie\nother: keep\n",
		"mapping: 'let movie = \"🎬\"\n\n  root = $movie'\nother: keep\n",
		"mapping: \"let movie = \\\"🎬\\\"\\nroot = $movie\"\nother: keep\n",
		"mapping: >-\n  let movie = \"🎬\"\n\n  root = $movie\nother: keep\n",
	} {
		openFeature(t, h, uri, text)
		edit, err := renameAt(t, h, uri, text, "$movie", "film")
		require.NoError(t, err)
		require.Len(t, edit.Changes, 1)
		require.Len(t, edit.Changes[uri], 2)
		result := applyRenameEdits(text, edit.Changes[uri])
		var host yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte(result), &host), result)
		require.Contains(t, result, "other: keep")
		regions := extractRegions(uri, result)
		require.Len(t, regions, 1)
		require.Contains(t, regions[0].text, "let film")
		require.Contains(t, regions[0].text, "$film")
		_, err = h.benv.Parse(regions[0].text)
		require.NoError(t, err)
	}
}
func TestRenameMapAcrossImportsAndYAML(t *testing.T) {
	h, uri := featureHandler(t)
	text := "map transform { root = this }\nroot = this.apply(\"transform\")\n"
	path, _ := uriToPath(uri)
	require.NoError(t, os.WriteFile(path, []byte(text), 0600))
	openFeature(t, h, uri, text)
	other := fileURI(filepath.Join(h.workspaceRoot, "consumer.blobl"))
	otherText := "import \"mapping.blobl\"\nroot = this.apply(\"transform\")\n"
	otherPath, _ := uriToPath(other)
	require.NoError(t, os.WriteFile(otherPath, []byte(otherText), 0600))
	yamlURI := fileURI(filepath.Join(h.workspaceRoot, "pipeline.yaml"))
	yamlText := "mapping: |\n  import \"mapping.blobl\"\n  root = this.apply(\"transform\")\nother: keep\n"
	yamlPath, _ := uriToPath(yamlURI)
	require.NoError(t, os.WriteFile(yamlPath, []byte(yamlText), 0600))
	unrelatedPath := filepath.Join(h.workspaceRoot, "unrelated.blobl")
	require.NoError(t, os.WriteFile(unrelatedPath, []byte("map transform { root = this }\nroot = this.apply(\"transform\")"), 0600))
	edit, err := renameAt(t, h, uri, text, "transform {", "convert")
	require.NoError(t, err)
	require.Len(t, edit.Changes, 3)
	require.Len(t, edit.Changes[uri], 2)
	require.Equal(t, strings.ReplaceAll(otherText, "transform", "convert"), applyRenameEdits(otherText, edit.Changes[other]))
	require.Equal(t, strings.ReplaceAll(yamlText, "transform", "convert"), applyRenameEdits(yamlText, edit.Changes[yamlURI]))
	// The live buffer is authoritative even for imported definitions.
	openFeature(t, h, uri, strings.ReplaceAll(text, "transform", "unsaved"))
	otherText = strings.ReplaceAll(otherText, "transform", "unsaved")
	openFeature(t, h, other, otherText)
	edit, err = renameAt(t, h, other, otherText, "unsaved", "renamed")
	require.NoError(t, err)
	require.Len(t, edit.Changes[uri], 2)
	require.Len(t, edit.Changes[other], 1)
}
func TestRenameMapCollisionInImporter(t *testing.T) {
	h, uri := featureHandler(t)
	text := "map transform { root = this }"
	openFeature(t, h, uri, text)
	other := fileURI(filepath.Join(h.workspaceRoot, "consumer.blobl"))
	openFeature(t, h, other, "import \"mapping.blobl\"\nmap convert { root = this }\nroot = this.apply(\"transform\")")
	_, err := renameAt(t, h, uri, text, "transform", "convert")
	require.ErrorContains(t, err, "already exists")
}
func TestRenameOnWire(t *testing.T) {
	h, uri := featureHandler(t)
	client := servertest.New(t, h)
	require.NoError(t, client.DidOpen(uri, "bloblang", "let movie = this\nroot = $movie"))
	prepared, err := client.PrepareRename(uri, 0, 5)
	require.NoError(t, err)
	require.NotNil(t, prepared)
	require.Equal(t, "movie", prepared.Placeholder)
	edit, err := client.Rename(uri, 1, 10, "film")
	require.NoError(t, err)
	require.Len(t, edit.Changes[uri], 2)
}

func TestRenameRepeatedLetAssignments(t *testing.T) {
	h, uri := featureHandler(t)
	text := "let total = 0\nif this.ok {\n let total = $total + 1\n} else {\n let total = $total + 2\n}\nroot = $total\n"
	openFeature(t, h, uri, text)
	edit, err := renameAt(t, h, uri, text, "total = 0", "count")
	require.NoError(t, err)
	result := applyRenameEdits(text, edit.Changes[uri])
	require.Equal(t, strings.ReplaceAll(text, "total", "count"), result)
	_, err = h.benv.Parse(result)
	require.NoError(t, err)
}

func TestRenameYAMLRegionsRemainIndependent(t *testing.T) {
	h, _ := featureHandler(t)
	uri := fileURI(filepath.Join(h.workspaceRoot, "pipeline.yaml"))
	text := "pipeline:\n  processors:\n    - mapping: |\n        map local { root = this }\n        root = this.apply(\"local\")\n    - mapping: |\n        map local { root = this }\n        root = this.apply(\"local\")\n"
	openFeature(t, h, uri, text)
	edit, err := renameAt(t, h, uri, text, "local {", "renamed")
	require.NoError(t, err)
	require.Len(t, edit.Changes[uri], 2)
	result := applyRenameEdits(text, edit.Changes[uri])
	regions := extractRegions(uri, result)
	require.Len(t, regions, 2)
	require.Contains(t, regions[0].text, "map renamed")
	require.Contains(t, regions[0].text, `.apply("renamed")`)
	require.Contains(t, regions[1].text, "map local")
}

func TestRenameEscapedYAMLIdentifier(t *testing.T) {
	h, _ := featureHandler(t)
	uri := fileURI(filepath.Join(h.workspaceRoot, "config.yaml"))
	text := "mapping: \"let movi\\u0065 = 1\\nroot = $movi\\u0065\"\nother: keep\n"
	openFeature(t, h, uri, text)
	edit, err := renameAt(t, h, uri, text, "movi", "film")
	require.NoError(t, err)
	result := applyRenameEdits(text, edit.Changes[uri])
	require.Equal(t, "mapping: \"let film = 1\\nroot = $film\"\nother: keep\n", result)
}
