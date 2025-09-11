package main

import (
	"context"
	"os"

	"github.com/konflux-ci/monolithic-builder/pkg/imageindex"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	ctx := context.Background()

	config, err := imageindex.LoadConfigFromEnv()
	if err != nil {
		logger.Error("Failed to load configuration", zap.Error(err))
		os.Exit(1)
	}

	builder := imageindex.NewBuilder(logger, config)
	if err := builder.Execute(ctx); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
}
