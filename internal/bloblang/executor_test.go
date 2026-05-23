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

func TestExecuteCumulativeSingleLine(t *testing.T) {
	executor := NewExecutor(NewEnvironment(), testExecutorConfig())
	doc := "#!sample {\"name\":\"alice\",\"age\":30}\nroot.name = this.name"
	sample := map[string]interface{}{"name": "alice", "age": 30}

	// through line 1 (the only root assignment) → {"name":"alice"}
	result, err := executor.ExecuteCumulative("file:///map.blobl", sample, doc, 1)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, `{"name":"alice"}`, result.Text)
}

func TestExecuteCumulativeMultiAssignment(t *testing.T) {
	executor := NewExecutor(NewEnvironment(), testExecutorConfig())
	doc := "#!sample {}\nroot.name = this.name\nroot.age = this.age"
	sample := map[string]interface{}{"name": "alice", "age": 30}

	// through line 1 (first assignment) → only name field
	result1, err := executor.ExecuteCumulative("file:///map.blobl", sample, doc, 1)
	require.NoError(t, err)
	require.NotNil(t, result1)
	assert.Equal(t, `{"name":"alice"}`, result1.Text)

	// through line 2 (both assignments) → both fields
	result2, err := executor.ExecuteCumulative("file:///map.blobl", sample, doc, 2)
	require.NoError(t, err)
	require.NotNil(t, result2)
	assert.Equal(t, `{"age":30,"name":"alice"}`, result2.Text)
}

func TestExecuteCumulativeBeforeFirstLine(t *testing.T) {
	executor := NewExecutor(NewEnvironment(), testExecutorConfig())
	doc := "#!sample {}\nroot.name = this.name\nroot.age = this.age"
	sample := map[string]interface{}{"name": "alice", "age": 30}

	// throughLine = -1 means "before any assignment" → raw sample
	result, err := executor.ExecuteCumulative("file:///map.blobl", sample, doc, -1)
	require.NoError(t, err)
	require.NotNil(t, result)
	// raw sample marshalled
	assert.Equal(t, `{"age":30,"name":"alice"}`, result.Text)
}

func TestExecuteCumulativeInvalidatesCache(t *testing.T) {
	executor := NewExecutor(NewEnvironment(), testExecutorConfig())
	uri := "file:///map.blobl"
	doc := "#!sample {}\nroot.name = this.name"
	sample := map[string]interface{}{"name": "alice"}

	result, err := executor.ExecuteCumulative(uri, sample, doc, 1)
	require.NoError(t, err)
	require.Equal(t, `{"name":"alice"}`, result.Text)

	executor.InvalidateDocument(uri)
	result, err = executor.ExecuteCumulative(uri, map[string]interface{}{"name": "bob"}, doc, 1)
	require.NoError(t, err)
	require.Equal(t, `{"name":"bob"}`, result.Text)
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
