package benthos

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSampleInline(t *testing.T) {
	parser, err := NewBloblang()
	require.NoError(t, err)
	defer parser.Close("test")

	sample, err := ExtractSample(parser, "file:///map.blobl", "#!input {\"key\":\"val\"}\nroot = this.key", t.TempDir())
	require.NoError(t, err)
	require.NotNil(t, sample)
	assert.Equal(t, "inline", sample.Source)
	assert.Equal(t, map[string]interface{}{"key": "val"}, sample.Value)
	assert.Equal(t, 0, sample.Line)
}

func TestExtractSampleFromFile(t *testing.T) {
	parser, err := NewBloblang()
	require.NoError(t, err)
	defer parser.Close("test")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"$bloblang":{"input":{"name":"alice"}}}`), 0o600))

	sample, err := ExtractSample(parser, "file:///map.blobl", "#!sample_from data.json\nroot = this.name", dir)
	require.NoError(t, err)
	require.NotNil(t, sample)
	assert.Equal(t, map[string]interface{}{"name": "alice"}, sample.Value)
	assert.Equal(t, filepath.Join(dir, "data.json"), sample.Source)
	assert.Equal(t, 0, sample.Line)
}

func TestExtractSampleAfterMappingIgnored(t *testing.T) {
	parser, err := NewBloblang()
	require.NoError(t, err)
	defer parser.Close("test")

	sample, err := ExtractSample(parser, "file:///map.blobl", "root = this\n#!input {\"key\":\"val\"}", t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, sample)
}

func TestExtractSampleMalformedJSON(t *testing.T) {
	parser, err := NewBloblang()
	require.NoError(t, err)
	defer parser.Close("test")

	sample, err := ExtractSample(parser, "file:///map.blobl", "#!input {", t.TempDir())
	assert.Error(t, err)
	assert.Nil(t, sample)
}

func TestExtractSampleMissingDirective(t *testing.T) {
	parser, err := NewBloblang()
	require.NoError(t, err)
	defer parser.Close("test")

	sample, err := ExtractSample(parser, "file:///map.blobl", "# normal comment\n\nroot = this", t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, sample)
}
