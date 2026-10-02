package lsp

import (
	protocol "github.com/owenrumney/go-lsp/lsp"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func positionByte(text string, p protocol.Position) int {
	lines := strings.Split(text, "\n")
	if p.Line < 0 {
		return 0
	}
	if p.Line >= len(lines) {
		return len(text)
	}
	offset := 0
	for i := 0; i < p.Line; i++ {
		offset += len(lines[i]) + 1
	}
	units := 0
	for i, r := range lines[p.Line] {
		if units >= p.Character {
			return offset + i
		}
		units += len(utf16.Encode([]rune{r}))
	}
	return offset + len(lines[p.Line])
}
func bytePosition(text string, offset int) protocol.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	s := text[:offset]
	line := strings.Count(s, "\n")
	start := strings.LastIndex(s, "\n") + 1
	units := 0
	for _, r := range s[start:] {
		if r != utf8.RuneError {
			units += len(utf16.Encode([]rune{r}))
		} else {
			units++
		}
	}
	return protocol.Position{Line: line, Character: units}
}
func byteColumn(text string, p protocol.Position) protocol.Position {
	offset := positionByte(text, p)
	start := strings.LastIndex(text[:offset], "\n") + 1
	return protocol.Position{Line: p.Line, Character: offset - start}
}
func rangeUTF16(text string, r protocol.Range) protocol.Range {
	lines := strings.Split(text, "\n")
	convert := func(p protocol.Position) protocol.Position {
		if p.Line < 0 || p.Line >= len(lines) {
			return p
		}
		col := p.Character
		if col > len(lines[p.Line]) {
			col = len(lines[p.Line])
		}
		return protocol.Position{Line: p.Line, Character: len(utf16.Encode([]rune(lines[p.Line][:col])))}
	}
	return protocol.Range{Start: convert(r.Start), End: convert(r.End)}
}
