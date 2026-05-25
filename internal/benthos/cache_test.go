package benthos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildCompletionCache(t *testing.T) {
	env := NewEnvironment()
	items, fnDocs, methDocs := BuildCompletionCache(env)

	assert.NotEmpty(t, items)
	assert.NotEmpty(t, fnDocs)
	assert.NotEmpty(t, methDocs)

	seen := make(map[string]bool)
	for _, it := range items {
		assert.NotEmpty(t, it.Label)
		kind := 0
		if it.Kind != nil {
			kind = int(*it.Kind)
		}
		key := it.Label + ":" + string(rune(kind))
		assert.False(t, seen[key], "duplicate completion item: %s", it.Label)
		seen[key] = true
	}
	assert.True(t, len(seen) > 0)
}
