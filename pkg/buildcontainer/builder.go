package buildcontainer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/konflux-ci/monolithic-builder/pkg/exec"
	"github.com/konflux-ci/monolithic-builder/pkg/git"
	"github.com/konflux-ci/monolithic-builder/pkg/image"
	"github.com/konflux-ci/monolithic-builder/pkg/prefetch"
	"go.uber.org/zap"
)

// Builder implements the monolithic build-container functionality
type Builder struct {
	logger *zap.Logger
	config *Config
	runner exec.CommandRunner
}

// NewBuilder creates a new Builder instance
func NewBuilder(logger *zap.Logger, config *Config, runner exec.CommandRunner) *Builder {
	return &Builder{
		logger: logger,
		config: config,
		runner: runner,
	}
}

// Execute runs the complete monolithic build process
func (b *Builder) Execute(ctx context.Context) error {
	b.logger.Info("Starting monolithic build-container task",
		zap.String("image_url", b.config.ImageURL),
		zap.String("git_url", b.config.GitURL),
		zap.String("revision", b.config.GitRevision))

	// Step 1: Initialize - check if we need to build
	shouldBuild, err := b.initializeAndCheckBuild(ctx)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Write build result for potential pipeline consumption
	if err := b.writeResult("build", fmt.Sprintf("%t", shouldBuild)); err != nil {
		return fmt.Errorf("failed to write build result: %w", err)
	}

	// Step 2: Always clone repository to get git info (required for pipeline results)
	b.logger.Info("Cloning repository")
	gitResult, err := b.cloneRepository(ctx)
	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Write git results (always required for Konflux pipeline traceability)
	if err := b.writeResult("commit", gitResult.CommitSHA); err != nil {
		return fmt.Errorf("failed to write commit result: %w", err)
	}
	if err := b.writeResult("url", gitResult.URL); err != nil {
		return fmt.Errorf("failed to write url result: %w", err)
	}

	// Always write image results (required for downstream tasks like build-image-index)
	if err := b.writeResult("IMAGE_URL", b.config.ImageURL); err != nil {
		return fmt.Errorf("failed to write IMAGE_URL result: %w", err)
	}

	if !shouldBuild {
		b.logger.Info("Skipping build - image already exists and rebuild not requested")

		// Get digest of existing image for downstream tasks
		digest, err := b.getExistingImageDigest(ctx)
		if err != nil {
			b.logger.Warn("Failed to get existing image digest, using empty value", zap.Error(err))
			digest = ""
		}

		if err := b.writeResult("IMAGE_DIGEST", digest); err != nil {
			return fmt.Errorf("failed to write IMAGE_DIGEST result: %w", err)
		}

		b.logger.Info("Skipped build completed - wrote IMAGE_URL and IMAGE_DIGEST results",
			zap.String("image_url", b.config.ImageURL),
			zap.String("image_digest", digest))
		return nil
	}

	// Step 3: Prefetch dependencies (if configured)
	if b.config.PrefetchInput != "" {
		b.logger.Info("Prefetching dependencies")
		if err := b.prefetchDependencies(ctx); err != nil {
			return fmt.Errorf("dependency prefetch failed: %w", err)
		}
	}

	// Step 4: Build container image
	b.logger.Info("Building container image")
	buildResult, err := b.buildContainerImage(ctx, gitResult.CommitSHA)
	if err != nil {
		return fmt.Errorf("container build failed: %w", err)
	}

	// Write build results (IMAGE_URL already written above)
	if err := b.writeResult("IMAGE_DIGEST", buildResult.ImageDigest); err != nil {
		return fmt.Errorf("failed to write IMAGE_DIGEST result: %w", err)
	}

	b.logger.Info("Monolithic build-container task completed successfully",
		zap.String("image_url", buildResult.ImageURL),
		zap.String("image_digest", buildResult.ImageDigest))

	return nil
}

// initializeAndCheckBuild implements the init task functionality
func (b *Builder) initializeAndCheckBuild(ctx context.Context) (bool, error) {
	b.logger.Info("Checking if image build is required",
		zap.String("image_url", b.config.ImageURL),
		zap.Bool("rebuild", b.config.Rebuild),
		zap.Bool("skip_checks", b.config.SkipChecks))

	// Always build if rebuild is requested or checks are skipped
	if b.config.Rebuild || b.config.SkipChecks {
		return true, nil
	}

	// Check if image already exists
	exists, err := image.CheckImageExists(ctx, b.config.ImageURL, b.config.TLSVerify, b.runner)
	if err != nil {
		b.logger.Warn("Failed to check image existence, proceeding with build", zap.Error(err))
		return true, nil
	}

	return !exists, nil
}

// cloneRepository implements the git-clone task functionality
func (b *Builder) cloneRepository(ctx context.Context) (*git.CloneResult, error) {
	cloneConfig := &git.CloneConfig{
		URL:         b.config.GitURL,
		Revision:    b.config.GitRevision,
		Refspec:     b.config.GitRefspec,
		Depth:       b.config.GitDepth,
		Submodules:  b.config.GitSubmodules,
		Destination: filepath.Join(b.config.WorkspacePath, "source"),
		AuthPath:    b.config.GitAuthPath,
	}

	return git.Clone(ctx, b.logger, cloneConfig)
}

// prefetchDependencies implements the prefetch-dependencies task functionality
func (b *Builder) prefetchDependencies(ctx context.Context) error {
	prefetchConfig := &prefetch.Config{
		Input:              b.config.PrefetchInput,
		SourcePath:         filepath.Join(b.config.WorkspacePath, "source"),
		OutputPath:         filepath.Join(b.config.WorkspacePath, "cachi2", "output"),
		DevPackageManagers: b.config.DevPackageManagers,
		LogLevel:           b.config.Cachi2LogLevel,
		ConfigFileContent:  b.config.Cachi2ConfigFileContent,
		GitAuthPath:        b.config.GitAuthPath,
		NetrcPath:          b.config.NetrcPath,
	}

	return prefetch.FetchDependencies(ctx, b.logger, prefetchConfig)
}

// buildContainerImage implements the buildah task functionality
func (b *Builder) buildContainerImage(ctx context.Context, commitSHA string) (*image.BuildResult, error) {
	buildConfig := &image.BuildConfig{
		ImageURL:          b.config.ImageURL,
		Dockerfile:        b.config.Dockerfile,
		Context:           filepath.Join(b.config.WorkspacePath, "source"),
		Hermetic:          b.config.Hermetic,
		PrefetchInput:     b.config.PrefetchInput,
		PrefetchPath:      filepath.Join(b.config.WorkspacePath, "cachi2"),
		ImageExpiresAfter: b.config.ImageExpiresAfter,
		CommitSHA:         commitSHA,
		BuildArgs:         b.config.BuildArgs,
		BuildArgsFile:     b.config.BuildArgsFile,
		TLSVerify:         b.config.TLSVerify,
	}

	return image.BuildAndPush(ctx, b.logger, buildConfig, b.runner)
}

// writeResult writes a result to the Tekton results directory
func (b *Builder) writeResult(name, value string) error {
	resultPath := filepath.Join(b.config.ResultsPath, name)
	return os.WriteFile(resultPath, []byte(value), 0644)
}

// getExistingImageDigest retrieves the digest of an existing image from the registry
func (b *Builder) getExistingImageDigest(ctx context.Context) (string, error) {
	return image.GetImageDigest(ctx, b.config.ImageURL, b.config.TLSVerify, b.runner)
}
