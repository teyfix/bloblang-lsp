package lsp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRootAssignRe(t *testing.T) {
	tests := []struct {
		line        string
		shouldMatch bool
	}{
		// simple root assignment — must match
		{"root = this", true},
		{"root = \"hello\"", true},
		{"  root = this", true},
		{"\troot = this", true},
		// dot sub-path — must match
		{"root.field = this", true},
		{"root.field = \"hello\"", true},
		{"root.nested.field = this", true},
		{"  root.nested.field = this", true},
		// bracket sub-path — must match
		{"root[\"key\"] = this", true},
		{"root[\"key\"] = \"value\"", true},
		{"root[0] = this", true},
		{"\troot[\"arr\"][0] = this", true},
		// non-root lines — must NOT match
		{"notroot = this", false},
		{"let root = \"value\"", false},
		{"// root = this", false},
		{"root", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := rootAssignRe.MatchString(tt.line)
			if tt.shouldMatch {
				assert.True(t, got, "expected %q to match rootAssignRe", tt.line)
			} else {
				assert.False(t, got, "expected %q NOT to match rootAssignRe", tt.line)
			}
		})
	}
}
