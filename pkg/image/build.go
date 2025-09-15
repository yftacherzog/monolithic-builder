package image

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/konflux-ci/monolithic-builder/pkg/exec"
	"go.uber.org/zap"
)

// BuildConfig holds configuration for container image build
type BuildConfig struct {
	ImageURL          string
	Dockerfile        string
	Context           string
	Hermetic          bool
	PrefetchInput     string
	PrefetchPath      string
	ImageExpiresAfter string
	CommitSHA         string
	BuildArgs         []string
	BuildArgsFile     string
	TLSVerify         bool
}

// BuildResult holds the results of a container image build
type BuildResult struct {
	ImageURL    string
	ImageDigest string
}

// BuildAndPush builds and pushes a container image using buildah
func BuildAndPush(ctx context.Context, logger *zap.Logger, config *BuildConfig, runner exec.CommandRunner) (*BuildResult, error) {
	logger.Info("Starting container image build",
		zap.String("image_url", config.ImageURL),
		zap.String("dockerfile", config.Dockerfile),
		zap.String("context", config.Context))

	// Build the buildah build command
	buildArgs := BuildahBuildCommand(config)
	logger.Info("Executing buildah build", zap.Strings("args", buildArgs))

	// Execute buildah build using unshare wrapper for rootless execution
	unshareCmd := UnshareCommand(buildArgs, config.Context)
	if err := runner.Run(ctx, unshareCmd[0], unshareCmd[1:]...); err != nil {
		return nil, fmt.Errorf("buildah build failed: %w", err)
	}

	// Push the image
	logger.Info("Pushing image to registry")
	pushArgs := BuildahPushCommand(config)
	if err := runner.Run(ctx, "buildah", pushArgs...); err != nil {
		return nil, fmt.Errorf("buildah push failed: %w", err)
	}

	// Get image digest
	digest, err := getImageDigest(ctx, config.ImageURL, config.TLSVerify, runner)
	if err != nil {
		logger.Warn("Failed to get image digest", zap.Error(err))
		digest = ""
	}

	logger.Info("Container image build completed successfully",
		zap.String("image_url", config.ImageURL),
		zap.String("image_digest", digest))

	return &BuildResult{
		ImageURL:    config.ImageURL,
		ImageDigest: digest,
	}, nil
}

// getImageDigest retrieves the digest of a pushed image
func getImageDigest(ctx context.Context, imageURL string, tlsVerify bool, runner exec.CommandRunner) (string, error) {
	args := SkopeoInspectCommand(imageURL, tlsVerify)

	output, err := runner.RunWithOutput(ctx, "skopeo", args...)
	if err != nil {
		return "", fmt.Errorf("skopeo inspect failed: %w", err)
	}

	// Parse JSON output to extract digest
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("failed to parse skopeo output: %w", err)
	}

	digest, ok := result["Digest"].(string)
	if !ok {
		return "", fmt.Errorf("digest not found in skopeo output")
	}

	return digest, nil
}

// CheckImageExists checks if an image exists in the registry
func CheckImageExists(ctx context.Context, imageURL string, tlsVerify bool, runner exec.CommandRunner) (bool, error) {
	args := SkopeoExistsCommand(imageURL, tlsVerify)

	err := runner.Run(ctx, "skopeo", args...)
	return err == nil, nil
}
