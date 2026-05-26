package benthos

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBloblangParserCaching(t *testing.T) {
	b, err := NewBloblang()
	require.NoError(t, err)
	defer b.Close("explore")

	uri := "file:///test.blobl"
	doc1 := `root = this`

	// Parse first time
	tree1, err := b.Parse(uri, doc1)
	require.NoError(t, err)
	require.NotNil(t, tree1)

	// Parse second time with exact same text should return identical tree instance
	tree2, err := b.Parse(uri, doc1)
	require.NoError(t, err)
	assert.True(t, tree1 == tree2, "Should return the same cached tree instance")

	// Parse third time with changed text
	doc2 := `root = this.name`
	tree3, err := b.Parse(uri, doc2)
	require.NoError(t, err)
	require.NotNil(t, tree3)
	assert.False(t, tree1 == tree3, "Should return a new tree instance after changes")

	// Invalidate
	b.InvalidateDocument(uri)
	_, ok := b.trees[uri]
	assert.False(t, ok, "Should remove document from cache")
}
