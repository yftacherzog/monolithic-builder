package imageindex

import (
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration parameters for the monolithic build-image-index task
type Config struct {
	// Image configuration
	ImageURL          string
	CommitSHA         string
	ImageExpiresAfter string
	AlwaysBuildIndex  bool
	Images            []string

	// Workspace paths
	ResultsPath string

	// Registry configuration
	TLSVerify bool
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() (*Config, error) {
	config := &Config{
		ImageURL:          getEnv("IMAGE", ""),
		CommitSHA:         getEnv("COMMIT_SHA", ""),
		ImageExpiresAfter: getEnv("IMAGE_EXPIRES_AFTER", ""),
		AlwaysBuildIndex:  getEnvBool("ALWAYS_BUILD_INDEX", false),
		Images:            getEnvArray("IMAGES"),
		ResultsPath:       getEnv("RESULTS_PATH", "/tekton/results"),
		TLSVerify:         getEnvBool("TLSVERIFY", true),
	}

	return config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvArray(key string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return []string{}
}
