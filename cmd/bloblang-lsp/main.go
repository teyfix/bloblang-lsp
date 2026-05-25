package main

import (
	"context"
	"log"

	"github.com/owenrumney/go-lsp/server"
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
	handler, err := lsp.NewHandler(cfg, logr)

	if err != nil {
		log.Fatal(err)
	}

	if err := lsp.NewServer(handler, logr).Run(context.Background(), server.RunStdio()); err != nil {
		log.Fatal(err)
	}
}
