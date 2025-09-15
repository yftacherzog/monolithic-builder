package main

import (
	"os"

	"github.com/konflux-ci/monolithic-builder/pkg/buildcontainer"
	"github.com/konflux-ci/monolithic-builder/pkg/exec"
	"github.com/konflux-ci/monolithic-builder/pkg/imageindex"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	rootCmd := &cobra.Command{
		Use:   "monolithic-builder",
		Short: "Monolithic builder for Konflux pipelines",
		Long:  "A unified builder that consolidates multiple Tekton pipeline tasks into efficient Go-based implementations.",
	}

	// Add subcommands
	rootCmd.AddCommand(buildContainerCmd(logger))
	rootCmd.AddCommand(buildImageIndexCmd(logger))

	// Support environment variable routing for Tekton
	if cmd := os.Getenv("MONOLITHIC_COMMAND"); cmd != "" {
		// Prepend the command to args for Cobra to process
		os.Args = append([]string{os.Args[0], cmd}, os.Args[1:]...)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildContainerCmd(logger *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build-container [build-args...]",
		Short: "Build container image using buildah",
		Long: `Build a container image using buildah with the provided build arguments.
Build arguments should be in the format KEY=value and will be passed to buildah build as --build-arg flags.`,
		Args: cobra.ArbitraryArgs, // Accept any number of positional arguments
		RunE: func(cmd *cobra.Command, args []string) error {
			// args contains the build arguments: ["KEY1=value1", "KEY2=value2", ...]
			config, err := buildcontainer.LoadConfig(args)
			if err != nil {
				logger.Error("Failed to load build-container configuration", zap.Error(err))
				return err
			}

			// Create command runner
			runner := exec.NewRealCommandRunner()
			builder := buildcontainer.NewBuilder(logger, config, runner)
			if err := builder.Execute(cmd.Context()); err != nil {
				logger.Error("Build-container execution failed", zap.Error(err))
				return err
			}

			return nil
		},
	}
}

func buildImageIndexCmd(logger *zap.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "build-image-index",
		Short: "Build multi-platform image index",
		Long:  `Build a multi-platform image index from the provided container images.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := imageindex.LoadConfigFromEnv()
			if err != nil {
				logger.Error("Failed to load build-image-index configuration", zap.Error(err))
				return err
			}

			builder := imageindex.NewBuilder(logger, config)
			if err := builder.Execute(cmd.Context()); err != nil {
				logger.Error("Build-image-index execution failed", zap.Error(err))
				return err
			}

			return nil
		},
	}
}
