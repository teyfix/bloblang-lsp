package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"strings"

	protocol "github.com/owenrumney/go-lsp/lsp"
	"github.com/owenrumney/go-lsp/server"
	"github.com/teyfix/bloblang-lsp/modular/config"
	"github.com/teyfix/bloblang-lsp/modular/lsphandler"
	"github.com/teyfix/bloblang-lsp/modular/meta"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// We MUST write logs to os.Stderr, as os.Stdout is reserved for the LSP JSON-RPC transport stream.
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))

	handler, err := lsphandler.NewHandler(cfg, logger)
	if err != nil {
		log.Fatal(err)
	}

	s := server.NewServer(handler,
		server.WithLogger(logger),
		server.WithCapabilityOptions(server.CapabilityOptions{
			Completion: &protocol.CompletionOptions{
				TriggerCharacters: []string{".", "@", "$"},
			},
			ExecuteCommand: &protocol.ExecuteCommandOptions{
				Commands: []string{
					string(meta.CommandOpenFile),
					string(meta.CommandShowResult),
				},
			},
		}),
	)

	if err := s.Run(context.Background(), server.RunStdio()); err != nil {
		log.Fatal(err)
	}
}
