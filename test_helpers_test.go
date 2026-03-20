package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withTempWorkingDir(t *testing.T) {
	t.Helper()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error: %v", err)
	}

	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "project")
	dataDir := filepath.Join(tempDir, "user-home", defaultDataDirName)
	if err := os.MkdirAll(projectDir, 0700); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	t.Setenv(dataDirEnvName, dataDir)
	if err := os.Chdir(projectDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
}

func executeCommand(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	cmd := newRootCommand()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func containsLine(output, line string) bool {
	for _, candidate := range strings.Split(output, "\n") {
		if candidate == line {
			return true
		}
	}
	return false
}
