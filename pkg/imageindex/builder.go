package imageindex

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// Builder implements the monolithic build-image-index functionality
type Builder struct {
	logger *zap.Logger
	config *Config
}

// NewBuilder creates a new Builder instance
func NewBuilder(logger *zap.Logger, config *Config) *Builder {
	return &Builder{
		logger: logger,
		config: config,
	}
}

// Execute runs the complete monolithic build-image-index process
func (b *Builder) Execute(ctx context.Context) error {
	b.logger.Info("Starting monolithic build-image-index task",
		zap.String("image_url", b.config.ImageURL),
		zap.Strings("images", b.config.Images),
		zap.Bool("always_build_index", b.config.AlwaysBuildIndex))

	// Determine if we should build an index
	shouldBuildIndex := b.shouldBuildIndex()

	var resultImageURL, resultImageDigest string

	if shouldBuildIndex && len(b.config.Images) > 1 {
		// Build multi-architecture index
		b.logger.Info("Building multi-architecture image index")
		indexResult, err := b.buildImageIndex(ctx)
		if err != nil {
			return fmt.Errorf("failed to build image index: %w", err)
		}
		resultImageURL = indexResult.ImageURL
		resultImageDigest = indexResult.ImageDigest
	} else if len(b.config.Images) == 1 {
		// Single image - extract URL and digest
		b.logger.Info("Single image provided, extracting details")
		imageRef := b.config.Images[0]
		parts := strings.Split(imageRef, "@")
		if len(parts) == 2 {
			resultImageURL = parts[0]
			resultImageDigest = parts[1]
		} else {
			resultImageURL = imageRef
			// Try to get digest
			digest, err := b.getImageDigest(ctx, imageRef)
			if err != nil {
				b.logger.Warn("Failed to get image digest", zap.Error(err))
				resultImageDigest = ""
			} else {
				resultImageDigest = digest
			}
		}
	} else {
		return fmt.Errorf("no images provided for index creation")
	}

	// Add expiration label if specified
	if b.config.ImageExpiresAfter != "" {
		if err := b.addExpirationLabel(ctx, resultImageURL); err != nil {
			b.logger.Warn("Failed to add expiration label", zap.Error(err))
		}
	}

	// Write results
	if err := b.writeResult("IMAGE_URL", resultImageURL); err != nil {
		return fmt.Errorf("failed to write IMAGE_URL result: %w", err)
	}
	if err := b.writeResult("IMAGE_DIGEST", resultImageDigest); err != nil {
		return fmt.Errorf("failed to write IMAGE_DIGEST result: %w", err)
	}

	b.logger.Info("Monolithic build-image-index task completed successfully",
		zap.String("image_url", resultImageURL),
		zap.String("image_digest", resultImageDigest))

	return nil
}

// shouldBuildIndex determines whether to build an image index
func (b *Builder) shouldBuildIndex() bool {
	// Always build if explicitly requested
	if b.config.AlwaysBuildIndex {
		return true
	}

	// Build index if we have multiple images
	return len(b.config.Images) > 1
}

// ImageIndexResult holds the results of building an image index
type ImageIndexResult struct {
	ImageURL    string
	ImageDigest string
}

// buildImageIndex creates a multi-architecture image index
func (b *Builder) buildImageIndex(ctx context.Context) (*ImageIndexResult, error) {
	// Create a manifest list using buildah
	manifestName := b.config.ImageURL + "-index"

	// Create manifest
	b.logger.Info("Creating image manifest", zap.String("manifest", manifestName))
	createArgs := []string{"manifest", "create", manifestName}

	cmd := exec.CommandContext(ctx, "buildah", createArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create manifest: %w", err)
	}

	// Add images to manifest
	for _, imageRef := range b.config.Images {
		b.logger.Info("Adding image to manifest", zap.String("image", imageRef))
		addArgs := []string{"manifest", "add", manifestName, imageRef}

		addCmd := exec.CommandContext(ctx, "buildah", addArgs...)
		addCmd.Stdout = os.Stdout
		addCmd.Stderr = os.Stderr

		if err := addCmd.Run(); err != nil {
			return nil, fmt.Errorf("failed to add image %s to manifest: %w", imageRef, err)
		}
	}

	// Push manifest to registry
	b.logger.Info("Pushing image index to registry")
	pushArgs := []string{"manifest", "push", "--all", manifestName, fmt.Sprintf("docker://%s", b.config.ImageURL)}

	if !b.config.TLSVerify {
		pushArgs = append(pushArgs, "--tls-verify=false")
	}

	pushCmd := exec.CommandContext(ctx, "buildah", pushArgs...)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr

	if err := pushCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to push manifest: %w", err)
	}

	// Get the digest of the pushed index
	digest, err := b.getImageDigest(ctx, b.config.ImageURL)
	if err != nil {
		b.logger.Warn("Failed to get index digest", zap.Error(err))
		digest = ""
	}

	// Clean up local manifest
	rmArgs := []string{"manifest", "rm", manifestName}
	rmCmd := exec.CommandContext(ctx, "buildah", rmArgs...)
	_ = rmCmd.Run() // Ignore errors for cleanup

	return &ImageIndexResult{
		ImageURL:    b.config.ImageURL,
		ImageDigest: digest,
	}, nil
}

// getImageDigest retrieves the digest of an image
func (b *Builder) getImageDigest(ctx context.Context, imageURL string) (string, error) {
	args := []string{"inspect", "--format", "{{.Digest}}"}
	if !b.config.TLSVerify {
		args = append(args, "--tls-verify=false")
	}
	args = append(args, fmt.Sprintf("docker://%s", imageURL))

	cmd := exec.CommandContext(ctx, "skopeo", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to inspect image: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// addExpirationLabel adds expiration label to the image
// NOTE: The original build-image-index task declares IMAGE_EXPIRES_AFTER parameter
// but does not actually implement the functionality. We match this behavior.
func (b *Builder) addExpirationLabel(ctx context.Context, imageURL string) error {
	if b.config.ImageExpiresAfter == "" {
		return nil
	}

	// Log that we received the parameter (matching original task behavior)
	b.logger.Info("IMAGE_EXPIRES_AFTER parameter received",
		zap.String("image", imageURL),
		zap.String("expires_after", b.config.ImageExpiresAfter))

	// TODO: Implement expiration label functionality
	// The original build-image-index task declares this parameter but doesn't implement it
	b.logger.Warn("IMAGE_EXPIRES_AFTER functionality not yet implemented (matches original task)")

	return nil
}

// writeResult writes a result to the Tekton results directory
func (b *Builder) writeResult(name, value string) error {
	resultPath := filepath.Join(b.config.ResultsPath, name)
	return os.WriteFile(resultPath, []byte(value), 0644)
}
