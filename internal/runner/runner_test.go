package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRunScriptExecution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		scriptContent string
		wantExitCode  int
		wantStdout    string
		wantStderr    string
	}{
		{
			name:          "success",
			scriptContent: "#!/bin/sh\necho 'success'\nexit 0\n",
			wantExitCode:  0,
			wantStdout:    "success\n",
			wantStderr:    "",
		},
		{
			name:          "failure",
			scriptContent: "#!/bin/sh\necho 'error message' >&2\nexit 1\n",
			wantExitCode:  1,
			wantStdout:    "",
			wantStderr:    "error message\n",
		},
		{
			name:          "mixed_output",
			scriptContent: "#!/bin/sh\necho 'standard output'\necho 'error output' >&2\nexit 0\n",
			wantExitCode:  0,
			wantStdout:    "standard output\n",
			wantStderr:    "error output\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tempDir := t.TempDir()

			// Create the script
			scriptPath := filepath.Join(tempDir, tt.name+".sh")
			err := os.WriteFile(scriptPath, []byte(tt.scriptContent), 0755)
			if err != nil {
				t.Fatalf("failed to write test script: %v", err)
			}

			// Run the script
			result, err := Run(context.Background(), scriptPath)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}

			// Verify the result
			want := &Result{
				Name:     tt.name + ".sh",
				ExitCode: tt.wantExitCode,
				Stdout:   tt.wantStdout,
				Stderr:   tt.wantStderr,
			}

			if !cmp.Equal(result, want) {
				t.Errorf("Run() result mismatch (-got +want):\n%s", cmp.Diff(result, want))
			}
		})
	}
}

func TestRunScriptNotFound(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "nonexistent.sh")

	_, err := Run(context.Background(), scriptPath)
	if err == nil {
		t.Error("Run() error = nil, want error for nonexistent script")
	}
}

func TestRunScriptNotExecutable(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a non-executable script
	scriptPath := filepath.Join(tempDir, "non_executable.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho 'hello'\n"), 0644) // Note: 0644 is not executable
	if err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	_, err = Run(context.Background(), scriptPath)
	if err == nil {
		t.Error("Run() error = nil, want error for non-executable script")
	}
}

func TestRunWithCanceledContext(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()

	// Create a script that sleeps
	scriptPath := filepath.Join(tempDir, "sleeping.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nsleep 10\n"), 0755)
	if err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = Run(ctx, scriptPath)
	if err == nil {
		t.Error("Run() error = nil, want error for canceled context")
	}
}
