package repository

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func RunContextEngine(projectPath string) (string, error) {
	// Find the binary
	binaryPath := findContextEngineBinary()
	if binaryPath == "" {
		return "", nil // Graceful fallback — no binary found
	}

	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	fmt.Printf("🔍 Scanning codebase: %s\n", absProjectPath)

	// Run the context engine
	cmd := exec.Command(binaryPath, absProjectPath)
	cmd.Dir = absProjectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("context engine failed: %w", err)
	}

	// The context engine writes to code_context.txt in its working directory
	contextFile := filepath.Join(absProjectPath, "code_context.txt")
	if _, err := os.Stat(contextFile); os.IsNotExist(err) {
		return "", fmt.Errorf("context engine did not generate code_context.txt")
	}

	return contextFile, nil
}

// findContextEngineBinary looks for the context-engine binary
func findContextEngineBinary() string {
	// 1. Check relative to current working directory
	cwd, _ := os.Getwd()
	localBinary := filepath.Join(cwd, "context_engine", "context-engine")
	if _, err := os.Stat(localBinary); err == nil {
		return localBinary
	}

	// 2. Check system PATH
	if path, err := exec.LookPath("context-engine"); err == nil {
		return path
	}

	return ""
}
