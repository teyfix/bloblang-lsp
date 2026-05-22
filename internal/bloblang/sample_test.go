package bloblang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractSampleInline(t *testing.T) {
	sample, err := ExtractSample("#!sample {\"key\":\"val\"}\nroot = this.key", t.TempDir())
	require.NoError(t, err)
	require.NotNil(t, sample)
	assert.Equal(t, "inline", sample.Source)
	assert.Equal(t, map[string]interface{}{"key": "val"}, sample.Value)
}

func TestExtractSampleFromFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.json"), []byte(`{"name":"alice"}`), 0o600))

	sample, err := ExtractSample("#!sample_from data.json\nroot = this.name", dir)
	require.NoError(t, err)
	require.NotNil(t, sample)
	assert.Equal(t, map[string]interface{}{"name": "alice"}, sample.Value)
	assert.Equal(t, filepath.Join(dir, "data.json"), sample.Source)
}

func TestExtractSampleAfterMappingIgnored(t *testing.T) {
	sample, err := ExtractSample("root = this\n#!sample {\"key\":\"val\"}", t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, sample)
}

func TestExtractSampleMalformedJSON(t *testing.T) {
	sample, err := ExtractSample("#!sample {", t.TempDir())
	assert.Error(t, err)
	assert.Nil(t, sample)
}

func TestExtractSampleMissingDirective(t *testing.T) {
	sample, err := ExtractSample("# normal comment\n\nroot = this", t.TempDir())
	require.NoError(t, err)
	assert.Nil(t, sample)
}
