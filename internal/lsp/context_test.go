package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCompletionContext(t *testing.T) {
	lines := []string{
		"root = this.foo",
		"let x = string(",
	}

	// Completing after '.' should detect method context.
	assert.Equal(t, "method", getCompletionContext(lines, 0, 12))

	// Completing inside/after 'foo' should trace back to '.' and detect method context.
	assert.Equal(t, "method", getCompletionContext(lines, 0, 15))

	// Completing after '(' should detect function context.
	assert.Equal(t, "function", getCompletionContext(lines, 1, 15))
}

func TestFindTokenAtPosition(t *testing.T) {
	lines := []string{
		"root = string(this.name)",
	}

	token, start, end, isMethod := findTokenAtPosition(lines, 0, 10) // cursor in 'string'
	assert.Equal(t, "string", token)
	assert.Equal(t, 7, start)
	assert.Equal(t, 13, end)
	assert.False(t, isMethod)

	token, start, end, isMethod = findTokenAtPosition(lines, 0, 21) // cursor in 'name'
	assert.Equal(t, "name", token)
	assert.Equal(t, 19, start)
	assert.Equal(t, 23, end)
	assert.True(t, isMethod)
}
