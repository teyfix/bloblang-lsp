package lsp

import (
	"context"
	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestRegionSamplePrecedenceAndWatching(t *testing.T) {
	h, _ := featureHandler(t)
	uri := fileURI(filepath.Join(h.workspaceRoot, "config.yaml"))
	for name, value := range map[string]string{"config.sample.json": "shared", "config.sample-001.yaml": "numbered", "selected.yml": "explicit"} {
		require.NoError(t, os.WriteFile(filepath.Join(h.workspaceRoot, name), []byte("{\"$bloblang\":{\"input\":\""+value+"\"}}"), 0600))
	}
	text := "mapping: root = this\nprocessors:\n  - mapping: from \"external.blobl\"\n  - mapping: root = this\n"
	openFeature(t, h, uri, text)
	rs := h.regions(uri)
	require.Len(t, rs, 2)
	require.Equal(t, "numbered", h.getSample(rs[0].uri).Value)
	require.Equal(t, "shared", h.getSample(rs[1].uri).Value)
	text = "# bloblang-sample: selected.yml\nmapping: root = this\n"
	openFeature(t, h, uri, text)
	rs = h.regions(uri)
	require.Equal(t, "explicit", h.getSample(rs[0].uri).Value)
	require.NoError(t, os.Remove(filepath.Join(h.workspaceRoot, "selected.yml")))
	require.NoError(t, h.DidChangeWatchedFiles(context.Background(), &protocol.DidChangeWatchedFilesParams{}))
	require.Nil(t, h.getSample(rs[0].uri))
	ds := h.sampleDiagnosticsFor(rs[0].uri)
	require.NotEmpty(t, ds)
	require.Equal(t, protocol.SeverityError, *ds[0].Severity)
}
