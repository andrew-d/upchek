// Package runner provides a mechanism for running a single healthcheck script
// and returning the results.
package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Result represents the result of running a healthcheck script.
type Result struct {
	// Name is the name of the script.
	Name string

	// ExitCode is the exit code of the script.
	ExitCode int

	// Stdout is the standard output of the script.
	Stdout string

	// Stderr is the standard error of the script.
	Stderr string
}

// IsSuccess returns true if the script exited successfully.
func (r *Result) IsSuccess() bool {
	return r.ExitCode == 0
}

func Run(ctx context.Context, scriptPath string) (*Result, error) {
	// First, make sure the script is executable.
	st, err := os.Stat(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat script: %w", err)
	}
	if st.Mode()&0111 == 0 {
		return nil, fmt.Errorf("script is not executable")
	}

	resultName := filepath.Base(scriptPath)

	// Next, wire up the buffers for stdout and stderr.
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// TODO: additional file descriptor for structured metadata.

	// Run the script.
	err = cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &Result{
				Name:     resultName,
				ExitCode: exitErr.ExitCode(),
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
			}, nil
		}
		return nil, fmt.Errorf("failed to run script: %w", err)
	}

	// Script ran successfully.
	return &Result{
		Name:     resultName,
		ExitCode: 0,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}
