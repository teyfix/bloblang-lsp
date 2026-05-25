package main

import (
	"context"
	"log"

	"github.com/owenrumney/go-lsp/server"
	"github.com/teyfix/bloblang-lsp/internal/bloblang"
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
	benv := bloblang.NewEnvironment()
	items, fnData, methData := bloblang.BuildCompletionCache(benv)
	fnDocs, methDocs := bloblang.BuildAllDocs(fnData, methData, cfg.BloblangDocsURL)
	executor := bloblang.NewExecutor(benv, cfg)
	handler := lsp.NewHandler(cfg, logr, benv, items, fnDocs, methDocs, executor)
	if err := lsp.Run(context.Background(), handler, logr, server.RunStdio()); err != nil {
		log.Fatal(err)
	}
}
