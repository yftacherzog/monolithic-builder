package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.uber.org/zap"
)

// CloneConfig holds configuration for git clone operation
type CloneConfig struct {
	URL         string
	Revision    string
	Refspec     string
	Depth       int
	Submodules  bool
	Destination string
	AuthPath    string
}

// CloneResult holds the results of a git clone operation
type CloneResult struct {
	CommitSHA string
	URL       string
}

// Clone performs git clone operation similar to the git-clone task
func Clone(ctx context.Context, logger *zap.Logger, config *CloneConfig) (*CloneResult, error) {
	logger.Info("Starting git clone",
		zap.String("url", config.URL),
		zap.String("revision", config.Revision),
		zap.String("destination", config.Destination))

	// Ensure destination directory exists
	if err := os.MkdirAll(config.Destination, 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Set up authentication if available
	var auth transport.AuthMethod
	if config.AuthPath != "" {
		var err error
		auth, err = loadAuthFromPath(config.AuthPath)
		if err != nil {
			logger.Warn("Failed to load git authentication", zap.Error(err))
		}
	}

	// Configure clone options
	cloneOptions := &git.CloneOptions{
		URL:      config.URL,
		Progress: os.Stdout,
		Auth:     auth,
	}

	// Set depth for shallow clone
	if config.Depth > 0 {
		cloneOptions.Depth = config.Depth
	}

	// Add custom refspec if specified
	if config.Refspec != "" {
		cloneOptions.ReferenceName = plumbing.ReferenceName(config.Refspec)
	}

	// Perform the clone
	repo, err := git.PlainCloneContext(ctx, config.Destination, false, cloneOptions)
	if err != nil {
		return nil, fmt.Errorf("git clone failed: %w", err)
	}

	// Checkout specific revision if specified
	var commitSHA string
	if config.Revision != "" {
		commitSHA, err = checkoutRevision(repo, config.Revision)
		if err != nil {
			return nil, fmt.Errorf("failed to checkout revision %s: %w", config.Revision, err)
		}
	} else {
		// Get current HEAD commit
		head, err := repo.Head()
		if err != nil {
			return nil, fmt.Errorf("failed to get HEAD: %w", err)
		}
		commitSHA = head.Hash().String()
	}

	// Handle submodules if requested
	if config.Submodules {
		if err := updateSubmodules(repo, auth); err != nil {
			logger.Warn("Failed to update submodules", zap.Error(err))
		}
	}

	logger.Info("Git clone completed successfully",
		zap.String("commit_sha", commitSHA),
		zap.String("url", config.URL))

	return &CloneResult{
		CommitSHA: commitSHA,
		URL:       config.URL,
	}, nil
}

// checkoutRevision checks out a specific revision (branch, tag, or commit)
func checkoutRevision(repo *git.Repository, revision string) (string, error) {
	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	// Try to parse as a commit hash first
	if len(revision) >= 7 && len(revision) <= 40 {
		hash := plumbing.NewHash(revision)
		if err := w.Checkout(&git.CheckoutOptions{Hash: hash}); err == nil {
			return hash.String(), nil
		}
	}

	// Try as a branch reference
	branchRef := plumbing.NewBranchReferenceName(revision)
	if err := w.Checkout(&git.CheckoutOptions{Branch: branchRef}); err == nil {
		head, err := repo.Head()
		if err != nil {
			return "", err
		}
		return head.Hash().String(), nil
	}

	// Try as a tag reference
	tagRef := plumbing.NewTagReferenceName(revision)
	if err := w.Checkout(&git.CheckoutOptions{Branch: tagRef}); err == nil {
		head, err := repo.Head()
		if err != nil {
			return "", err
		}
		return head.Hash().String(), nil
	}

	return "", fmt.Errorf("failed to checkout revision: %s", revision)
}

// updateSubmodules initializes and updates git submodules
func updateSubmodules(repo *git.Repository, auth transport.AuthMethod) error {
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	submodules, err := w.Submodules()
	if err != nil {
		return err
	}

	for _, submodule := range submodules {
		if err := submodule.Update(&git.SubmoduleUpdateOptions{
			Init: true,
			Auth: auth,
		}); err != nil {
			return fmt.Errorf("failed to update submodule %s: %w", submodule.Config().Name, err)
		}
	}

	return nil
}

// loadAuthFromPath loads git authentication from a file path
func loadAuthFromPath(authPath string) (transport.AuthMethod, error) {
	// Try to read username/password from auth path
	usernameFile := filepath.Join(authPath, "username")
	passwordFile := filepath.Join(authPath, "password")

	username, err := os.ReadFile(usernameFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read username: %w", err)
	}

	password, err := os.ReadFile(passwordFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}

	return &http.BasicAuth{
		Username: strings.TrimSpace(string(username)),
		Password: strings.TrimSpace(string(password)),
	}, nil
}
