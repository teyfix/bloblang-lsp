package benthos

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Sample struct {
	Value  interface{}
	Source string
	Line   int
}

func ExtractSample(parser *Bloblang, uri string, text string, baseDir string) (*Sample, error) {
	tree, err := parser.Parse(uri, text)
	if err != nil {
		return nil, err
	}
	root := tree.RootNode()
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child.Kind() != "comment" {
			// A non-comment statement node is encountered. Any sample directives must be placed before statements.
			return nil, nil
		}

		commentText := child.Utf8Text([]byte(text))
		lineText := strings.TrimSpace(commentText)

		switch {
		case strings.HasPrefix(lineText, "#!sample "):
			raw := strings.TrimSpace(strings.TrimPrefix(lineText, "#!sample "))
			var value interface{}
			if err := json.Unmarshal([]byte(raw), &value); err != nil {
				return nil, err
			}
			return &Sample{
				Value:  value,
				Source: "inline",
				Line:   int(child.StartPosition().Row),
			}, nil
		case strings.HasPrefix(lineText, "#!sample_from "):
			rel := strings.TrimSpace(strings.TrimPrefix(lineText, "#!sample_from "))
			if rel == "" {
				return nil, fmt.Errorf("missing sample file path")
			}
			source := filepath.Join(baseDir, rel)
			data, err := os.ReadFile(source)
			if err != nil {
				return nil, err
			}
			var value interface{}
			if err := json.Unmarshal(data, &value); err != nil {
				return nil, err
			}
			abs, err := filepath.Abs(source)
			if err != nil {
				abs = source
			}
			return &Sample{
				Value:  value,
				Source: abs,
				Line:   int(child.StartPosition().Row),
			}, nil
		}
	}
	return nil, nil
}
