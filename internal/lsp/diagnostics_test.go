package lsp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertErrorToDiagnostics(t *testing.T) {
	// Standard parse error with line X char Y.
	err := errors.New("failed to parse: line 2 char 5: unexpected token")
	diags := convertErrorToDiagnostics("dummy", err)

	assert.Len(t, diags, 1)
	assert.Equal(t, 1, diags[0].Range.Start.Line)
	assert.Equal(t, 4, diags[0].Range.Start.Character)
	assert.Equal(t, "unexpected token", diags[0].Message)

	// Fallback when line / char not found.
	err2 := errors.New("something went wrong entirely")
	diags2 := convertErrorToDiagnostics("dummy", err2)

	assert.Len(t, diags2, 1)
	assert.Equal(t, 0, diags2[0].Range.Start.Line)
	assert.Equal(t, 0, diags2[0].Range.Start.Character)
	assert.Equal(t, "something went wrong entirely", diags2[0].Message)
}
