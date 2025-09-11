package image

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
func BuildAndPush(ctx context.Context, logger *zap.Logger, config *BuildConfig) (*BuildResult, error) {
	logger.Info("Starting container image build",
		zap.String("image_url", config.ImageURL),
		zap.String("dockerfile", config.Dockerfile),
		zap.String("context", config.Context))

	// Prepare build arguments
	buildArgs := []string{"build"}

	// Add dockerfile path
	buildArgs = append(buildArgs, "--file", config.Dockerfile)

	// Add image tag
	buildArgs = append(buildArgs, "--tag", config.ImageURL)

	// Configure TLS verification
	if !config.TLSVerify {
		buildArgs = append(buildArgs, "--tls-verify=false")
	}

	// Add custom build arguments
	for _, arg := range config.BuildArgs {
		if arg != "" {
			buildArgs = append(buildArgs, "--build-arg", arg)
		}
	}

	// Add build args file if specified
	if config.BuildArgsFile != "" {
		buildArgs = append(buildArgs, "--build-arg-file", config.BuildArgsFile)
	}

	// Configure hermetic build
	if config.Hermetic && config.PrefetchInput != "" {
		if err := setupHermeticBuild(config, &buildArgs); err != nil {
			return nil, fmt.Errorf("failed to setup hermetic build: %w", err)
		}
	}

	// Add commit SHA as label
	if config.CommitSHA != "" {
		buildArgs = append(buildArgs, "--label", fmt.Sprintf("io.konflux.commit=%s", config.CommitSHA))
	}

	// Add expiration label if specified
	if config.ImageExpiresAfter != "" {
		expirationTime := time.Now().Add(parseDuration(config.ImageExpiresAfter))
		buildArgs = append(buildArgs, "--label", fmt.Sprintf("quay.expires-after=%s", expirationTime.Format(time.RFC3339)))
	}

	// Add build context as the LAST argument (buildah build expects: buildah build [flags] context)
	buildArgs = append(buildArgs, ".")

	// Execute buildah build using unshare like the official buildah task
	logger.Info("Executing buildah build", zap.Strings("args", buildArgs))

	// Build the buildah command string like the official task does
	buildahCmdArray := []string{"buildah"}
	buildahCmdArray = append(buildahCmdArray, buildArgs...)

	// Use printf to properly quote the command like the official task
	var quotedArgs []string
	for _, arg := range buildahCmdArray {
		quotedArgs = append(quotedArgs, fmt.Sprintf("%q", arg))
	}
	buildahCmd := strings.Join(quotedArgs, " ")

	// Use unshare with the same arguments as the official buildah task (UBI 10 supports these)
	unshareArgs := []string{
		"unshare", "-Uf", "--keep-caps", "-r",
		"--map-users", "1,1,65536",
		"--map-groups", "1,1,65536",
		"-w", config.Context,
		"--mount", "--", "sh", "-c", buildahCmd,
	}

	cmd := exec.CommandContext(ctx, unshareArgs[0], unshareArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = config.Context

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("buildah build failed: %w", err)
	}

	// Push the image
	logger.Info("Pushing image to registry")
	pushArgs := []string{"push"}
	if !config.TLSVerify {
		pushArgs = append(pushArgs, "--tls-verify=false")
	}
	pushArgs = append(pushArgs, config.ImageURL)

	pushCmd := exec.CommandContext(ctx, "buildah", pushArgs...)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr

	if err := pushCmd.Run(); err != nil {
		return nil, fmt.Errorf("buildah push failed: %w", err)
	}

	// Get image digest
	digest, err := getImageDigest(ctx, config.ImageURL, config.TLSVerify)
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

// setupHermeticBuild configures buildah for hermetic builds with prefetched dependencies
func setupHermeticBuild(config *BuildConfig, buildArgs *[]string) error {
	// Add network isolation
	*buildArgs = append(*buildArgs, "--network", "none")

	// Mount prefetched dependencies
	if config.PrefetchPath != "" {
		cachi2OutputPath := filepath.Join(config.PrefetchPath, "output")
		if _, err := os.Stat(cachi2OutputPath); err == nil {
			*buildArgs = append(*buildArgs, "--volume", fmt.Sprintf("%s:/cachi2/output:ro", cachi2OutputPath))
		}

		// Add cachi2 environment file
		cachi2EnvPath := filepath.Join(config.PrefetchPath, "cachi2.env")
		if _, err := os.Stat(cachi2EnvPath); err == nil {
			*buildArgs = append(*buildArgs, "--env-file", cachi2EnvPath)
		}
	}

	return nil
}

// getImageDigest retrieves the digest of a pushed image
func getImageDigest(ctx context.Context, imageURL string, tlsVerify bool) (string, error) {
	args := []string{"inspect", "--format", "{{.Digest}}"}
	if !tlsVerify {
		args = append(args, "--tls-verify=false")
	}
	args = append(args, "docker://"+imageURL)

	cmd := exec.CommandContext(ctx, "skopeo", args...)
	output, err := cmd.Output()
	if err != nil {
		// Get stderr for better error reporting
		if exitError, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("failed to inspect image: %w, stderr: %s", err, string(exitError.Stderr))
		}
		return "", fmt.Errorf("failed to inspect image: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// CheckImageExists checks if an image exists in the registry
func CheckImageExists(ctx context.Context, imageURL string, tlsVerify bool) (bool, error) {
	args := []string{"inspect", "--retry-times", "3", "--no-tags", "--raw"}
	if !tlsVerify {
		args = append(args, "--tls-verify=false")
	}
	args = append(args, fmt.Sprintf("docker://%s", imageURL))

	cmd := exec.CommandContext(ctx, "skopeo", args...)
	err := cmd.Run()
	return err == nil, nil
}

// parseDuration parses duration strings like "1h", "2d", "3w"
func parseDuration(s string) time.Duration {
	if s == "" {
		return 0
	}

	// Simple parser for common duration formats
	if strings.HasSuffix(s, "h") {
		if h, err := time.ParseDuration(s); err == nil {
			return h
		}
	}
	if strings.HasSuffix(s, "d") {
		if days := strings.TrimSuffix(s, "d"); days != "" {
			if d, err := time.ParseDuration(days + "h"); err == nil {
				return d * 24
			}
		}
	}
	if strings.HasSuffix(s, "w") {
		if weeks := strings.TrimSuffix(s, "w"); weeks != "" {
			if w, err := time.ParseDuration(weeks + "h"); err == nil {
				return w * 24 * 7
			}
		}
	}

	// Fallback to standard duration parsing
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	return 0
}
