package main

import (
	"context"
	"os"

	"github.com/konflux-ci/monolithic-builder/pkg/buildcontainer"
	"github.com/konflux-ci/monolithic-builder/pkg/exec"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	ctx := context.Background()

	config, err := buildcontainer.LoadConfigFromEnv()
	if err != nil {
		logger.Error("Failed to load configuration", zap.Error(err))
		os.Exit(1)
	}

	runner := exec.NewRealCommandRunner()
	builder := buildcontainer.NewBuilder(logger, config, runner)
	if err := builder.Execute(ctx); err != nil {
		logger.Error("Command execution failed", zap.Error(err))
		os.Exit(1)
	}
}
