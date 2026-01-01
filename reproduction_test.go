package assetmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitialRegistration(t *testing.T) {
	env := setupTestEnv("initial_registration", t)
	defer env.CleanDirectory()

	// Create test files
	file1Path := filepath.Join(env.BaseDir, "script1.js")
	file2Path := filepath.Join(env.BaseDir, "script2.js")
	if err := os.WriteFile(file1Path, []byte("console.log('File 1');"), 0644); err != nil {
		t.Fatalf("Failed to write file 1: %v", err)
	}
	if err := os.WriteFile(file2Path, []byte("console.log('File 2');"), 0644); err != nil {
		t.Fatalf("Failed to write file 2: %v", err)
	}

	// Process in false
	env.AssetsHandler.SetBuildOnDisk(false)
	if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "create"); err != nil {
		t.Fatalf("Error processing file 1 create: %v", err)
	}
	if err := env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "create"); err != nil {
		t.Fatalf("Error processing file 2 create: %v", err)
	}

	// Verify no file is written
	if _, err := os.Stat(env.MainJsPath); !os.IsNotExist(err) {
		t.Errorf("File should not be written in false, err: %v", err)
	}

	// Switch to true and trigger a write
	env.AssetsHandler.SetBuildOnDisk(true)
	if err := env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "write"); err != nil {
		t.Fatalf("Error processing file 1 write: %v", err)
	}

	// Verify file is written with all content
	if _, err := os.Stat(env.MainJsPath); os.IsNotExist(err) {
		t.Fatalf("File should be written in true at %s", env.MainJsPath)
	}
	content, err := os.ReadFile(env.MainJsPath)
	if err != nil {
		t.Fatalf("Failed to read main.js: %v", err)
	}
	if !strings.Contains(string(content), "File 1") {
		t.Errorf("Output should contain 'File 1'")
	}
	if !strings.Contains(string(content), "File 2") {
		t.Errorf("Output should contain 'File 2'")
	}
}
