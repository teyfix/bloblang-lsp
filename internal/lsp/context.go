package lsp

import (
	protocol "github.com/owenrumney/go-lsp/lsp"
)

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_'
}

// findTokenAtPosition extracts the identifier under (or adjacent to) the cursor.
func findTokenAtPosition(lines []string, lineIdx, col int) (token string, tokenStart, tokenEnd int, isMethod bool) {
	if lineIdx < 0 || lineIdx >= len(lines) {
		return "", 0, 0, false
	}

	line := lines[lineIdx]
	if col > len(line) {
		col = len(line)
	}

	// If the cursor is not directly on an identifier char, nudge it onto one.
	if col < len(line) && !isIdentChar(line[col]) {
		if col > 0 && isIdentChar(line[col-1]) {
			col--
		} else {
			for col < len(line) && !isIdentChar(line[col]) {
				col++
			}
			if col >= len(line) {
				return "", 0, 0, false
			}
		}
	}

	if col >= len(line) || !isIdentChar(line[col]) {
		return "", 0, 0, false
	}

	// Walk backward to token start.
	start := col
	for start > 0 && isIdentChar(line[start-1]) {
		start--
	}

	// Walk forward to token end.
	end := col
	for end < len(line) && isIdentChar(line[end]) {
		end++
	}

	token = line[start:end]
	if token == "" {
		return "", 0, 0, false
	}

	isMethod = isMethodContext(lines, lineIdx, start)
	return token, start, end, isMethod
}

// isMethodContext walks backward from the token start skipping whitespace and line breaks.
func isMethodContext(lines []string, lineIdx, startCol int) bool {
	col := startCol - 1
	for lineIdx >= 0 {
		line := lines[lineIdx]
		for col >= 0 {
			c := line[col]
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
				col--
				continue
			}
			return c == '.'
		}
		lineIdx--
		if lineIdx >= 0 {
			col = len(lines[lineIdx]) - 1
		}
	}
	return false
}

// isFunctionCallContext checks whether the token is followed by '(' on the same line (skipping whitespace).
func isFunctionCallContext(line string, endCol int) bool {
	for i := endCol; i < len(line); i++ {
		c := line[i]
		if c == ' ' || c == '\t' || c == '\r' {
			continue
		}
		return c == '('
	}
	return false
}

// getCompletionContext inspects the text before the cursor to decide whether the user is completing a method, variable, or general function.
func getCompletionContext(lines []string, lineIdx, col int) string {
	if lineIdx < 0 || lineIdx >= len(lines) {
		return ""
	}

	// If the cursor sits inside or just after an identifier, walk back to the character preceding that identifier.
	c := col - 1
	if c >= 0 {
		line := lines[lineIdx]
		for c >= 0 && isIdentChar(line[c]) {
			c--
		}
	}

	// Walk backward across whitespace and line breaks.
	l := lineIdx
	for l >= 0 {
		line := lines[l]
		for c >= 0 {
			ch := line[c]
			if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' {
				c--
				continue
			}
			switch ch {
			case '.':
				return "method"
			case '$', '@':
				return "variable"
			case '=', '(', ',', '{', '[', ';', '|', '&', '+', '-', '*', '/', '%', '!', '>', '<', '~':
				return "function"
			}
			return ""
		}
		l--
		if l >= 0 {
			c = len(lines[l]) - 1
		}
	}
	return "function"
}

// filterCompletions returns only the items whose Kind matches the requested kind.
func filterCompletions(items []protocol.CompletionItem, kind protocol.CompletionItemKind) []protocol.CompletionItem {
	var out []protocol.CompletionItem
	for _, it := range items {
		if it.Kind != nil && *it.Kind == kind {
			out = append(out, it)
		}
	}
	return out
}
