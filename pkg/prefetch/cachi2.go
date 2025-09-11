package prefetch

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"go.uber.org/zap"
)

// Config holds configuration for dependency prefetching
type Config struct {
	Input              string
	SourcePath         string
	OutputPath         string
	DevPackageManagers bool
	LogLevel           string
	ConfigFileContent  string
	GitAuthPath        string
	NetrcPath          string
}

// FetchDependencies uses Cachi2 to prefetch build dependencies
func FetchDependencies(ctx context.Context, logger *zap.Logger, config *Config) error {
	logger.Info("Starting dependency prefetch with Cachi2",
		zap.String("input", config.Input),
		zap.String("source_path", config.SourcePath),
		zap.String("output_path", config.OutputPath))

	if config.Input == "" {
		logger.Info("No prefetch input provided, skipping dependency prefetch")
		return nil
	}

	// Ensure output directory exists
	if err := os.MkdirAll(config.OutputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Setup authentication if available
	if err := setupAuthentication(config); err != nil {
		logger.Warn("Failed to setup authentication", zap.Error(err))
	}

	// Write config file if provided
	if config.ConfigFileContent != "" {
		configPath := filepath.Join(config.OutputPath, "cachi2.yaml")
		if err := os.WriteFile(configPath, []byte(config.ConfigFileContent), 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	}

	// Build cachi2 fetch-deps command
	args := []string{"fetch-deps"}
	args = append(args, fmt.Sprintf("--source=%s", config.SourcePath))
	args = append(args, fmt.Sprintf("--output=%s", config.OutputPath))

	// Add dev package managers flag if enabled
	if config.DevPackageManagers {
		args = append(args, "--dev-package-managers")
	}

	// Set log level
	if config.LogLevel != "" {
		args = append(args, fmt.Sprintf("--log-level=%s", config.LogLevel))
	}

	// Add input specification
	args = append(args, config.Input)

	// Execute cachi2 fetch-deps
	logger.Info("Executing cachi2 fetch-deps", zap.Strings("args", args))
	cmd := exec.CommandContext(ctx, "cachi2", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cachi2 fetch-deps failed: %w", err)
	}

	// Generate environment file
	if err := generateEnvironmentFile(ctx, logger, config.OutputPath); err != nil {
		return fmt.Errorf("failed to generate environment file: %w", err)
	}

	// Inject files
	if err := injectFiles(ctx, logger, config.OutputPath); err != nil {
		return fmt.Errorf("failed to inject files: %w", err)
	}

	logger.Info("Dependency prefetch completed successfully")
	return nil
}

// generateEnvironmentFile creates the cachi2 environment file
func generateEnvironmentFile(ctx context.Context, logger *zap.Logger, outputPath string) error {
	args := []string{"generate-env", outputPath}
	args = append(args, "--format", "env")
	args = append(args, "--for-output-dir", "/cachi2/output")
	args = append(args, "--output", filepath.Join(filepath.Dir(outputPath), "cachi2.env"))

	logger.Info("Generating cachi2 environment file", zap.Strings("args", args))
	cmd := exec.CommandContext(ctx, "cachi2", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// injectFiles injects prefetched files into the build context
func injectFiles(ctx context.Context, logger *zap.Logger, outputPath string) error {
	args := []string{"inject-files", outputPath}
	args = append(args, "--for-output-dir", "/cachi2/output")

	logger.Info("Injecting cachi2 files", zap.Strings("args", args))
	cmd := exec.CommandContext(ctx, "cachi2", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// setupAuthentication configures authentication for cachi2
func setupAuthentication(config *Config) error {
	// Setup git authentication
	if config.GitAuthPath != "" {
		// Copy git auth to home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		gitConfigDir := filepath.Join(homeDir, ".git")
		if err := os.MkdirAll(gitConfigDir, 0700); err != nil {
			return fmt.Errorf("failed to create git config directory: %w", err)
		}

		// Copy authentication files
		authFiles := []string{"username", "password", ".gitconfig"}
		for _, file := range authFiles {
			srcPath := filepath.Join(config.GitAuthPath, file)
			dstPath := filepath.Join(gitConfigDir, file)

			if _, err := os.Stat(srcPath); err == nil {
				if err := copyFile(srcPath, dstPath); err != nil {
					return fmt.Errorf("failed to copy auth file %s: %w", file, err)
				}
			}
		}
	}

	// Setup netrc authentication
	if config.NetrcPath != "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		srcPath := filepath.Join(config.NetrcPath, ".netrc")
		dstPath := filepath.Join(homeDir, ".netrc")

		if _, err := os.Stat(srcPath); err == nil {
			if err := copyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy netrc file: %w", err)
			}
			// Set proper permissions for .netrc
			if err := os.Chmod(dstPath, 0600); err != nil {
				return fmt.Errorf("failed to set netrc permissions: %w", err)
			}
		}
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
