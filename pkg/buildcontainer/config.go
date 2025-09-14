package buildcontainer

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration parameters for the monolithic build-container task
type Config struct {
	// Git configuration
	GitURL        string
	GitRevision   string
	GitRefspec    string
	GitDepth      int
	GitSubmodules bool

	// Image configuration
	ImageURL          string
	Dockerfile        string
	Context           string
	Rebuild           bool
	SkipChecks        bool
	Hermetic          bool
	TLSVerify         bool
	ImageExpiresAfter string

	// Prefetch configuration
	PrefetchInput           string
	DevPackageManagers      bool
	Cachi2LogLevel          string
	Cachi2ConfigFileContent string

	// Build configuration
	BuildArgs     []string
	BuildArgsFile string
	CommitSHA     string

	// Workspace paths
	WorkspacePath string
	ResultsPath   string

	// Authentication
	GitAuthPath string
	NetrcPath   string
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() (*Config, error) {
	return LoadConfig(nil)
}

// LoadConfig loads configuration from environment variables and optional build args
func LoadConfig(buildArgs []string) (*Config, error) {
	config := &Config{
		// Git defaults
		GitURL:        getEnv("GIT_URL", ""),
		GitRevision:   getEnv("GIT_REVISION", ""),
		GitRefspec:    getEnv("GIT_REFSPEC", ""),
		GitDepth:      getEnvInt("GIT_DEPTH", 1),
		GitSubmodules: getEnvBool("GIT_SUBMODULES", true),

		// Image defaults
		ImageURL:          getEnv("IMAGE_URL", ""),
		Dockerfile:        getEnv("DOCKERFILE", "./Dockerfile"),
		Context:           getEnv("CONTEXT", "."),
		Rebuild:           getEnvBool("REBUILD", false),
		SkipChecks:        getEnvBool("SKIP_CHECKS", false),
		Hermetic:          getEnvBool("HERMETIC", false),
		TLSVerify:         getEnvBool("TLSVERIFY", true),
		ImageExpiresAfter: getEnv("IMAGE_EXPIRES_AFTER", ""),

		// Prefetch defaults
		PrefetchInput:           getEnv("PREFETCH_INPUT", ""),
		DevPackageManagers:      getEnvBool("DEV_PACKAGE_MANAGERS", false),
		Cachi2LogLevel:          getEnv("LOG_LEVEL", "info"),
		Cachi2ConfigFileContent: getEnv("CONFIG_FILE_CONTENT", ""),

		// Build defaults
		BuildArgs:     buildArgs,
		BuildArgsFile: getEnv("BUILD_ARGS_FILE", ""),
		CommitSHA:     getEnv("COMMIT_SHA", ""),

		// Workspace paths
		WorkspacePath: getEnv("WORKSPACE_PATH", "/workspace"),
		ResultsPath:   getEnv("RESULTS_PATH", "/tekton/results"),

		// Authentication
		GitAuthPath: getEnv("GIT_AUTH_PATH", ""),
		NetrcPath:   getEnv("NETRC_PATH", ""),
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

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvArray(key string) []string {
	if value := os.Getenv(key); value != "" {
		// Handle both space-separated (from Tekton array expansion) and comma-separated values
		if strings.Contains(value, ",") {
			return strings.Split(value, ",")
		}
		// Split on spaces and filter out empty strings
		parts := strings.Fields(value)
		return parts
	}
	return []string{}
}

func getEnvArrayJSON(key string) []string {
	if value := os.Getenv(key); value != "" {
		// Handle JSON array format from Tekton array parameters
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			// Parse as JSON array
			var arr []string
			if err := json.Unmarshal([]byte(value), &arr); err == nil {
				return arr
			}
		}
		// Fallback to space-separated parsing
		return getEnvArray(key)
	}
	return []string{}
}
