package lsp

import (
	"bufio"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

// Opt in with a newline-delimited fixture list; private workspace files are not
// required by the normal test suite and are never modified.
func TestFormatterCorpus(t *testing.T) {
	list := os.Getenv("BLOBLANG_CORPUS_LIST")
	if list == "" {
		t.Skip("set BLOBLANG_CORPUS_LIST to a newline-delimited mapping fixture list")
	}
	f, err := os.Open(list)
	require.NoError(t, err)
	defer f.Close()
	h, _ := featureHandler(t)
	scanner := bufio.NewScanner(f)
	count, changed, canonical, refused := 0, 0, 0, 0
	for scanner.Scan() {
		path := scanner.Text()
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err, path)
		source := string(data)
		uri := fileURI(path)
		formatted, valid := h.formatText(uri, source, 2)
		count++
		if !valid {
			refused++
			t.Errorf("formatter refused %s", path)
			continue
		}
		if source == formatted {
			canonical++
		} else {
			changed++
		}
		twice, valid := h.formatText(uri, formatted, 2)
		require.True(t, valid, path)
		require.Equal(t, formatted, twice, path)
	}
	require.NoError(t, scanner.Err())
	t.Logf("corpus=%d changed=%d canonical=%d refused=%d", count, changed, canonical, refused)
	require.Greater(t, count, 0)
}
