package fileuri

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "with spaces", "100%20 sample.blobl")
	uri := FromPath(path)
	require.Contains(t, uri, "100%2520%20sample.blobl")
	got, err := Path(uri + "#bloblang-1")
	require.NoError(t, err)
	require.Equal(t, path, got)
}

func TestWindowsDriveURI(t *testing.T) {
	uri := FromPath("C:/Users/Ada/mapping.blobl")
	require.Equal(t, "file:///C:/Users/Ada/mapping.blobl", uri)
	if runtime.GOOS == "windows" {
		path, err := Path(uri)
		require.NoError(t, err)
		require.Equal(t, `C:\Users\Ada\mapping.blobl`, path)
	}
}
