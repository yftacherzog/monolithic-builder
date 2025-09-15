package exec

import (
	"context"
	"os"
	"os/exec"
)

// CommandRunner interface abstracts command execution for testability
type CommandRunner interface {
	// Run executes a command and streams output to stdout/stderr
	Run(ctx context.Context, name string, args ...string) error

	// RunWithOutput executes a command and returns output
	RunWithOutput(ctx context.Context, name string, args ...string) ([]byte, error)
}

// RealCommandRunner implements CommandRunner using os/exec
type RealCommandRunner struct{}

// NewRealCommandRunner creates a new real command runner
func NewRealCommandRunner() *RealCommandRunner {
	return &RealCommandRunner{}
}

// Run executes a command and streams output to stdout/stderr
func (r *RealCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// RunWithOutput executes a command and returns output
func (r *RealCommandRunner) RunWithOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}
