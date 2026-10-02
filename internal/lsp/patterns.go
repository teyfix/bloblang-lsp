package lsp

import "regexp"

var (
	rootAssignRe = regexp.MustCompile(`^\s*root[\s.\[].*=`)
)
