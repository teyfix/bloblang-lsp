package benthos

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestSampleDirectiveContract(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "message.yaml"), []byte("$bloblang:\n  input:\n    a: 1\n  meta:\n    k: before\n"), 0600))
	s, err := ExtractSample(nil, "file:///mapping.blobl", "#!sample_from message.yaml\n#!meta |\n#| k: after\n#| list:\n#|   - one\nroot = this", dir)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a": 1}, s.Value)
	require.Equal(t, "after", s.Meta["k"])
	require.Len(t, s.Dependencies, 1)
	_, err = ExtractSample(nil, "file:///mapping.blobl", "#!sample {\"input\":null}\n#!sample {\"input\":1}", dir)
	require.ErrorContains(t, err, "Duplicate")
	s, err = ExtractSample(nil, "file:///mapping.blobl", "#!sample {input: null, meta: {k: value}}\nroot=this", dir)
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Nil(t, s.Value)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "raw.json"), []byte(`{"input":1}`), 0600))
	_, err = ExtractSample(nil, "file:///mapping.blobl", "#!input_from raw.json", dir)
	require.ErrorContains(t, err, "$bloblang")
}
func TestOutputScalarsDeletionAndOverlay(t *testing.T) {
	p := newTestParser(t)
	e := NewExecutor(NewEnvironment(), testExecutorConfig())
	s := &Sample{Value: map[string]any{"a": 1}, Meta: map[string]any{"k": "v"}}
	for snippet, want := range map[string]string{`root = "123"`: `"123"`, `root = null`: `null`, `root = meta("k")`: `"v"`} {
		r, err := e.ExecuteThrough(p, "file:///mapping.blobl", s, snippet, uint(len(snippet)))
		require.NoError(t, err)
		require.Equal(t, want, r.Full)
	}
	snippet := `root = deleted()`
	r, err := e.ExecuteThrough(p, "file:///mapping.blobl", s, snippet, uint(len(snippet)))
	require.NoError(t, err)
	require.True(t, r.Deleted)
	s.Root = map[string]any{"ref": "a"}
	s.HasRoot = true
	snippet = `root.value = this.get(root.ref)`
	r, err = e.ExecuteThrough(p, "file:///mapping.blobl", s, snippet, uint(len(snippet)))
	require.NoError(t, err)
	require.Equal(t, `{"ref":"a","value":1}`, r.Full)
}
func TestEntireInlineSampleOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mapping.sample.json"), []byte("broken"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mapping.sample.yaml"), []byte("broken"), 0600))
	sample, err := ExtractSample(nil, "file:///mapping.blobl", "#!sample {input: {a: 1}}\nroot=this", dir)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"a": 1}, sample.Value)
	_, err = ExtractSample(nil, "file:///mapping.blobl", "#!input\nroot=this", t.TempDir())
	require.ErrorContains(t, err, "Missing value")
	sample, err = ExtractSample(nil, "file:///mapping.blobl", "#!input null\nroot=this", t.TempDir())
	require.NoError(t, err)
	require.NotNil(t, sample)
	require.Nil(t, sample.Value)
}
