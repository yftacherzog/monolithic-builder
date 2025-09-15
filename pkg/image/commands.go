package image

import (
	"fmt"
	"strings"
	"time"
)

// BuildahBuildCommand builds the buildah build command arguments
func BuildahBuildCommand(config *BuildConfig) []string {
	args := []string{"build"}

	// Add dockerfile path
	args = append(args, "--file", config.Dockerfile)

	// Add image tag
	args = append(args, "--tag", config.ImageURL)

	// Configure TLS verification
	if !config.TLSVerify {
		args = append(args, "--tls-verify=false")
	}

	// Add custom build arguments
	for _, arg := range config.BuildArgs {
		if arg != "" {
			args = append(args, "--build-arg", arg)
		}
	}

	// Add build args file if specified
	if config.BuildArgsFile != "" {
		args = append(args, "--build-arg-file", config.BuildArgsFile)
	}

	// Configure hermetic build
	if config.Hermetic && config.PrefetchInput != "" {
		// Add hermetic build configuration
		if config.PrefetchPath != "" {
			args = append(args, "--volume", fmt.Sprintf("%s:/tmp/cachi2:Z", config.PrefetchPath))
		}
		args = append(args, "--network=none")
	}

	// Add commit SHA as label
	if config.CommitSHA != "" {
		args = append(args, "--label", fmt.Sprintf("io.konflux.commit=%s", config.CommitSHA))
	}

	// Add expiration label if specified
	if config.ImageExpiresAfter != "" {
		expirationTime := time.Now().Add(parseDuration(config.ImageExpiresAfter))
		args = append(args, "--label", fmt.Sprintf("quay.expires-after=%s", expirationTime.Format(time.RFC3339)))
	}

	// Add build context as the LAST argument (buildah build expects: buildah build [flags] context)
	args = append(args, ".")

	return args
}

// UnshareCommand wraps a buildah command with unshare for rootless execution
func UnshareCommand(buildahArgs []string, context string) []string {
	// Build the buildah command string like the official task does
	buildahCmdArray := []string{"buildah"}
	buildahCmdArray = append(buildahCmdArray, buildahArgs...)

	// Use printf to properly quote the command like the official task
	var quotedArgs []string
	for _, arg := range buildahCmdArray {
		quotedArgs = append(quotedArgs, fmt.Sprintf("%q", arg))
	}
	buildahCmd := strings.Join(quotedArgs, " ")

	// Use unshare with the same arguments as the official buildah task (UBI 10 supports these)
	return []string{
		"unshare", "-Uf", "--keep-caps", "-r",
		"--map-users", "1,1,65536",
		"--map-groups", "1,1,65536",
		"-w", context,
		"--mount", "--", "sh", "-c", buildahCmd,
	}
}

// BuildahPushCommand builds the buildah push command arguments
func BuildahPushCommand(config *BuildConfig) []string {
	args := []string{"push"}

	if !config.TLSVerify {
		args = append(args, "--tls-verify=false")
	}

	args = append(args, config.ImageURL)
	return args
}

// SkopeoInspectCommand builds the skopeo inspect command arguments
func SkopeoInspectCommand(imageURL string, tlsVerify bool) []string {
	args := []string{"inspect"}

	if !tlsVerify {
		args = append(args, "--tls-verify=false")
	}

	args = append(args, "docker://"+imageURL)
	return args
}

// SkopeoExistsCommand builds the skopeo inspect command for checking image existence
func SkopeoExistsCommand(imageURL string, tlsVerify bool) []string {
	args := []string{"inspect", "--raw"}

	if !tlsVerify {
		args = append(args, "--tls-verify=false")
	}

	args = append(args, fmt.Sprintf("docker://%s", imageURL))
	return args
}

// parseDuration parses duration strings like "1h", "2d", "3w"
func parseDuration(duration string) time.Duration {
	if duration == "" {
		return 0
	}

	// Simple parsing for common formats
	if strings.HasSuffix(duration, "h") {
		if hours, err := time.ParseDuration(duration); err == nil {
			return hours
		}
	}
	if strings.HasSuffix(duration, "d") {
		if len(duration) > 1 {
			if days := duration[:len(duration)-1]; days != "" {
				if d, err := time.ParseDuration(days + "h"); err == nil {
					return d * 24
				}
			}
		}
	}
	if strings.HasSuffix(duration, "w") {
		if len(duration) > 1 {
			if weeks := duration[:len(duration)-1]; weeks != "" {
				if w, err := time.ParseDuration(weeks + "h"); err == nil {
					return w * 24 * 7
				}
			}
		}
	}

	// Fallback to standard time.ParseDuration
	if d, err := time.ParseDuration(duration); err == nil {
		return d
	}

	return 0
}
