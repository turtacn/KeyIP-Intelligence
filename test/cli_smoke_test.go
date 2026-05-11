//go:build integration

// Package clitest provides CLI smoke tests for the KeyIP-Intelligence binary.
package clitest

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildCLI compiles the keyip binary and returns its path.
func buildCLI(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "keyip")

	cmd := exec.Command("go", "build", "-o", binary, "./cmd/keyip")
	cmd.Dir = projectRoot(t)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build CLI: %v\n%s", err, out)
	}

	return binary
}

// projectRoot returns the absolute path to the project root by locating go.mod.
func projectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the test directory to find go.mod.
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("project root (go.mod) not found")
		}
		dir = parent
	}
	return ""
}

// runKeyip executes the CLI with given args and returns (exitCode, output).
func runKeyip(t *testing.T, binary string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), string(out)
		}
		t.Fatalf("failed to run %s %v: %v", binary, args, err)
	}
	return 0, string(out)
}

// TestCLISmoke compiles the CLI and runs basic smoke tests on all subcommands.
func TestCLISmoke(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("Go compiler not found, skipping CLI smoke test")
	}

	binary := buildCLI(t)

	t.Run("help", func(t *testing.T) {
		code, out := runKeyip(t, binary, "--help")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
		}
		if out == "" {
			t.Fatal("expected non-empty output for --help")
		}
	})

	t.Run("version", func(t *testing.T) {
		code, out := runKeyip(t, binary, "version")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
		}
	})

	t.Run("completion bash", func(t *testing.T) {
		code, out := runKeyip(t, binary, "completion", "bash")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
		}
		if out == "" {
			t.Fatal("expected non-empty output for bash completion")
		}
	})

	t.Run("config validate --help", func(t *testing.T) {
		code, out := runKeyip(t, binary, "config", "validate", "--help")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
		}
	})

	t.Run("search --help", func(t *testing.T) {
		code, out := runKeyip(t, binary, "search", "--help")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
		}
	})

	t.Run("assess --help", func(t *testing.T) {
		code, out := runKeyip(t, binary, "assess", "--help")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
		}
	})

	t.Run("lifecycle --help", func(t *testing.T) {
		code, out := runKeyip(t, binary, "lifecycle", "--help")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
		}
	})

	t.Run("report --help", func(t *testing.T) {
		code, out := runKeyip(t, binary, "report", "--help")
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d\noutput: %s", code, out)
		}
	})
}
