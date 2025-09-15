package exec

import (
	"context"
	"fmt"
	"strings"
)

// CommandError represents a command execution error for testing
type CommandError struct {
	ExitCode int
	Message  string
}

func (e *CommandError) Error() string {
	return e.Message
}

// MockCommandRunner implements CommandRunner for testing
type MockCommandRunner struct {
	// Commands stores all executed commands for verification
	Commands [][]string

	// Outputs maps command signatures to their outputs
	Outputs map[string][]byte

	// Errors maps command signatures to their errors
	Errors map[string]error

	// DefaultOutput is returned when no specific output is configured
	DefaultOutput []byte

	// DefaultError is returned when no specific error is configured
	DefaultError error
}

// NewMockCommandRunner creates a new mock command runner
func NewMockCommandRunner() *MockCommandRunner {
	return &MockCommandRunner{
		Commands: make([][]string, 0),
		Outputs:  make(map[string][]byte),
		Errors:   make(map[string]error),
	}
}

// Run executes a command and streams output to stdout/stderr (mocked)
func (m *MockCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	// Record the command
	cmd := append([]string{name}, args...)
	m.Commands = append(m.Commands, cmd)

	// Generate command signature for lookup
	signature := m.commandSignature(name, args...)

	// Return configured error if any
	if err, exists := m.Errors[signature]; exists {
		return err
	}

	return m.DefaultError
}

// RunWithOutput executes a command and returns output (mocked)
func (m *MockCommandRunner) RunWithOutput(ctx context.Context, name string, args ...string) ([]byte, error) {
	// Record the command
	cmd := append([]string{name}, args...)
	m.Commands = append(m.Commands, cmd)

	// Generate command signature for lookup
	signature := m.commandSignature(name, args...)

	// Return configured error if any
	if err, exists := m.Errors[signature]; exists {
		return nil, err
	}

	// Return configured output if any
	if output, exists := m.Outputs[signature]; exists {
		return output, nil
	}

	// Return default output and error
	return m.DefaultOutput, m.DefaultError
}

// SetOutput configures the output for a specific command
func (m *MockCommandRunner) SetOutput(name string, output []byte, args ...string) {
	signature := m.commandSignature(name, args...)
	m.Outputs[signature] = output
}

// SetError configures the error for a specific command
func (m *MockCommandRunner) SetError(name string, err error, args ...string) {
	signature := m.commandSignature(name, args...)
	m.Errors[signature] = err
}

// GetExecutedCommands returns all executed commands
func (m *MockCommandRunner) GetExecutedCommands() [][]string {
	return m.Commands
}

// GetLastCommand returns the last executed command
func (m *MockCommandRunner) GetLastCommand() []string {
	if len(m.Commands) == 0 {
		return nil
	}
	return m.Commands[len(m.Commands)-1]
}

// Reset clears all recorded commands and configurations
func (m *MockCommandRunner) Reset() {
	m.Commands = make([][]string, 0)
	m.Outputs = make(map[string][]byte)
	m.Errors = make(map[string]error)
	m.DefaultOutput = nil
	m.DefaultError = nil
}

// commandSignature creates a unique signature for a command
func (m *MockCommandRunner) commandSignature(name string, args ...string) string {
	cmd := append([]string{name}, args...)
	return strings.Join(cmd, " ")
}

// AssertCommandExecuted checks if a specific command was executed
func (m *MockCommandRunner) AssertCommandExecuted(name string, args ...string) bool {
	expected := append([]string{name}, args...)
	for _, cmd := range m.Commands {
		if len(cmd) == len(expected) {
			match := true
			for i, arg := range expected {
				if cmd[i] != arg {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}
	return false
}

// AssertCommandCount checks if the expected number of commands were executed
func (m *MockCommandRunner) AssertCommandCount(expected int) bool {
	return len(m.Commands) == expected
}

// String returns a string representation of all executed commands
func (m *MockCommandRunner) String() string {
	var result []string
	for i, cmd := range m.Commands {
		result = append(result, fmt.Sprintf("%d: %s", i+1, strings.Join(cmd, " ")))
	}
	return strings.Join(result, "\n")
}
