package main

import (
	"context"
	"log"

	"github.com/owenrumney/go-lsp/server"
	_ "github.com/redpanda-data/connect/v4/public/components/crypto"
	_ "github.com/redpanda-data/connect/v4/public/components/ffi"
	_ "github.com/redpanda-data/connect/v4/public/components/io"
	_ "github.com/redpanda-data/connect/v4/public/components/msgpack"
	_ "github.com/redpanda-data/connect/v4/public/components/pure"
	_ "github.com/redpanda-data/connect/v4/public/components/pure/extended"
	_ "github.com/redpanda-data/connect/v4/public/components/sql/base"
	_ "github.com/redpanda-data/connect/v4/public/components/text"
	bloblangpkg "github.com/teyfix/bloblang-lsp/internal/bloblang"
	"github.com/teyfix/bloblang-lsp/internal/config"
	"github.com/teyfix/bloblang-lsp/internal/logger"
	"github.com/teyfix/bloblang-lsp/internal/lsp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	logr := logger.NewLogger(cfg)
	benv := bloblangpkg.NewEnvironment()
	items, fnData, methData := bloblangpkg.BuildCompletionCache(benv)
	fnDocs, methDocs := bloblangpkg.BuildAllDocs(fnData, methData, cfg.BloblangDocsURL)
	executor := bloblangpkg.NewExecutor(benv, cfg)
	handler := lsp.NewHandler(cfg, logr, benv, items, fnDocs, methDocs, executor)
	if err := lsp.Run(context.Background(), handler, logr, server.RunStdio()); err != nil {
		log.Fatal(err)
	}
}
