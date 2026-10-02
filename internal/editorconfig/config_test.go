package editorconfig

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidationAndDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".bloblangrc.json")
	c, e := Load(p)
	require.NoError(t, e)
	require.Equal(t, 80, c.Formatter.PrintWidth)
	require.Equal(t, "hint", c.Setting("style/objects/prefer-with").Severity)
	for _, raw := range []string{`{"lint":{"rules":{"unknown":"warn"}}}`, `{"formatter":{"printWidth":1}}`, `{"preview":{"format":"xml"}}`, `{"lint":{"rules":{"style/objects/prefer-with":{"severity":"warn","minAssignments":3}}}}`, `{"formatter":{"typo":80}}`} {
		require.NoError(t, os.WriteFile(p, []byte(raw), 0600))
		c, e = Load(p)
		require.Error(t, e, raw)
		require.Equal(t, Default(), c)
	}
	require.NoError(t, os.WriteFile(p, []byte(`{"lint":{"rules":{"style/assignments/prefer-grouped":{"severity":"hint","minAssignments":5}}}}`), 0600))
	c, e = Load(p)
	require.NoError(t, e)
	require.Equal(t, 5, c.Setting("style/assignments/prefer-grouped").MinAssignments)
}
