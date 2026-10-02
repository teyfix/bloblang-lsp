// config-schema writes the schema from the workspace configuration registry.
package main

import (
	"encoding/json"
	"github.com/teyfix/bloblang-lsp/internal/editorconfig"
	"os"
)

func main() {
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	if err := e.Encode(editorconfig.Schema()); err != nil {
		panic(err)
	}
}
