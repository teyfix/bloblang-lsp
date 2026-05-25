package benthos

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Sample struct {
	Value  interface{}
	Source string
}

func ExtractSample(text string, baseDir string) (*Sample, error) {
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			return nil, nil
		}
		switch {
		case strings.HasPrefix(line, "#!sample "):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "#!sample "))
			var value interface{}
			if err := json.Unmarshal([]byte(raw), &value); err != nil {
				return nil, err
			}
			return &Sample{Value: value, Source: "inline"}, nil
		case strings.HasPrefix(line, "#!sample_from "):
			rel := strings.TrimSpace(strings.TrimPrefix(line, "#!sample_from "))
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
			return &Sample{Value: value, Source: abs}, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}
