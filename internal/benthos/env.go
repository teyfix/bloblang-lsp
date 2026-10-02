package benthos

import (
	"errors"

	"github.com/redpanda-data/benthos/v4/public/bloblang"
	_ "github.com/redpanda-data/connect/v4/public/components/crypto"
	_ "github.com/redpanda-data/connect/v4/public/components/ffi"
	_ "github.com/redpanda-data/connect/v4/public/components/io"
	_ "github.com/redpanda-data/connect/v4/public/components/msgpack"
	_ "github.com/redpanda-data/connect/v4/public/components/pure"
	_ "github.com/redpanda-data/connect/v4/public/components/pure/extended"
	_ "github.com/redpanda-data/connect/v4/public/components/sql/base"
	_ "github.com/redpanda-data/connect/v4/public/components/text"
)

// NewEnvironment returns a custom Bloblang environment wrapping the GlobalEnvironment.
func NewEnvironment() *bloblang.Environment {
	return bloblang.GlobalEnvironment().WithCustomImporter(func(name string) ([]byte, error) {
		return nil, errors.New("custom imports not configured for this environment context")
	})
}
