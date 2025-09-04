package main

import (
	"log/slog"
	"os"
)

// logger is the package-wide structured logger.
// -----------------------------------------------------------------------------

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))