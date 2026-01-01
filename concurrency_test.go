package assetmin

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCacheConcurrency(t *testing.T) {
	env := setupTestEnv("cache_concurrency", t)
	env.AssetsHandler.SetBuildOnDisk(true)
	defer env.CleanDirectory()

	// Create a test file
	jsFileName := "script1.js"
	jsFilePath := filepath.Join(env.BaseDir, jsFileName)
	jsContent := []byte("console.log('Hello from JS');")
	if err := os.WriteFile(jsFilePath, jsContent, 0644); err != nil {
		t.Fatalf("Failed to write JS file: %v", err)
	}

	// Initial event to populate the asset
	if err := env.AssetsHandler.NewFileEvent(jsFileName, ".js", jsFilePath, "create"); err != nil {
		t.Fatalf("Error processing creation event: %v", err)
	}

	var wg sync.WaitGroup
	numReaders := 100

	// Simulate concurrent reads
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := env.AssetsHandler.mainJsHandler.GetMinifiedContent(env.AssetsHandler.min); err != nil {
				t.Errorf("Error getting minified content: %v", err)
			}
		}()
	}

	// Simulate a concurrent write
	wg.Add(1)
	go func() {
		defer wg.Done()
		newContent := []byte("console.log('Updated JS');")
		if err := os.WriteFile(jsFilePath, newContent, 0644); err != nil {
			t.Errorf("Failed to update JS file: %v", err)
		}
		if err := env.AssetsHandler.NewFileEvent(jsFileName, ".js", jsFilePath, "write"); err != nil {
			t.Errorf("Error processing write event: %v", err)
		}
	}()

	wg.Wait()

	// Final check of the content
	finalContent, err := env.AssetsHandler.mainJsHandler.GetMinifiedContent(env.AssetsHandler.min)
	if err != nil {
		t.Fatalf("Error getting final content: %v", err)
	}
	if !strings.Contains(string(finalContent), "Updated JS") {
		t.Errorf("Final content mismatch: expected \"Updated JS\" to be in content")
	}
}
