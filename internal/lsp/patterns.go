package lsp

import "regexp"

var (
	rootAssignRe = regexp.MustCompile(`^\s*root\s.*=`)
	importRe     = regexp.MustCompile(`^\s*import\s+"([^"]+)"`)
)
