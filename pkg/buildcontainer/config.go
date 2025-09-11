package buildcontainer

import (
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
		BuildArgs:     getEnvArray("BUILD_ARGS"),
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
		return strings.Split(value, ",")
	}
	return []string{}
}
