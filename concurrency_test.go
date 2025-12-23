package assetmin

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCacheConcurrency(t *testing.T) {
	env := setupTestEnv("cache_concurrency", t)
	env.AssetsHandler.SetBuildOnDisk(true)
	defer env.CleanDirectory()

	// Create a test file
	jsFileName := "script1.js"
	jsFilePath := filepath.Join(env.BaseDir, jsFileName)
	jsContent := []byte("console.log('Hello from JS');")
	require.NoError(t, os.WriteFile(jsFilePath, jsContent, 0644))

	// Initial event to populate the asset
	require.NoError(t, env.AssetsHandler.NewFileEvent(jsFileName, ".js", jsFilePath, "create"))

	var wg sync.WaitGroup
	numReaders := 100

	// Simulate concurrent reads
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := env.AssetsHandler.mainJsHandler.GetMinifiedContent(env.AssetsHandler.min)
			require.NoError(t, err)
		}()
	}

	// Simulate a concurrent write
	wg.Add(1)
	go func() {
		defer wg.Done()
		newContent := []byte("console.log('Updated JS');")
		require.NoError(t, os.WriteFile(jsFilePath, newContent, 0644))
		require.NoError(t, env.AssetsHandler.NewFileEvent(jsFileName, ".js", jsFilePath, "write"))
	}()

	wg.Wait()

	// Final check of the content
	finalContent, err := env.AssetsHandler.mainJsHandler.GetMinifiedContent(env.AssetsHandler.min)
	require.NoError(t, err)
	require.Contains(t, string(finalContent), "Updated JS")
}
