package bloblang

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/teyfix/bloblang-lsp/internal/config"
)

func testExecutorConfig() *config.Config {
	return &config.Config{
		MaxInlineDocumentBytes: 150000,
		MaxInlineResultBytes:   100,
		PartialExecCacheSize:   100,
		PartialExecCacheTTL:    time.Minute,
	}
}

func TestExecutePartial(t *testing.T) {
	executor := NewExecutor(NewEnvironment(), testExecutorConfig())
	result, err := executor.ExecutePartial("file:///map.blobl", map[string]interface{}{"name": "alice"}, "root = this.name", 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, `"alice"`, result.Text)
	assert.False(t, result.Truncated)
}

func TestExecutePartialTruncates(t *testing.T) {
	cfg := testExecutorConfig()
	cfg.MaxInlineResultBytes = 5
	executor := NewExecutor(NewEnvironment(), cfg)
	result, err := executor.ExecutePartial("file:///map.blobl", map[string]interface{}{"name": "alice"}, "root = this.name", 0)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Truncated)
	assert.Equal(t, `"alic...`, result.Text)
	assert.Equal(t, `"alice"`, result.Full)
}

func TestExecutePartialSkipsLargeDocument(t *testing.T) {
	cfg := testExecutorConfig()
	cfg.MaxInlineDocumentBytes = 10
	executor := NewExecutor(NewEnvironment(), cfg)
	result, err := executor.ExecutePartial("file:///map.blobl", map[string]interface{}{"name": "alice"}, "root = this.name", 0)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestExecutePartialInvalidatesCache(t *testing.T) {
	executor := NewExecutor(NewEnvironment(), testExecutorConfig())
	uri := "file:///map.blobl"
	result, err := executor.ExecutePartial(uri, map[string]interface{}{"name": "alice"}, "root = this.name", 0)
	require.NoError(t, err)
	require.Equal(t, `"alice"`, result.Text)

	executor.InvalidateDocument(uri)
	result, err = executor.ExecutePartial(uri, map[string]interface{}{"name": "bob"}, "root = this.name", 0)
	require.NoError(t, err)
	require.Equal(t, `"bob"`, result.Text)
}
