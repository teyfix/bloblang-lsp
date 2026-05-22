package bloblang

import (
	"errors"

	"github.com/redpanda-data/benthos/v4/public/bloblang"
)

// NewEnvironment returns a custom Bloblang environment wrapping the GlobalEnvironment.
func NewEnvironment() *bloblang.Environment {
	return bloblang.GlobalEnvironment().WithCustomImporter(func(name string) ([]byte, error) {
		return nil, errors.New("custom imports not configured for this environment context")
	})
}
