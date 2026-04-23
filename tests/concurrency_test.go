package assetmin_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrency(t *testing.T) {
	t.Run("Concurrent JS and CSS processing", func(t *testing.T) {
		env := setupTestEnv("concurrency", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir()

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			env.TestConcurrentFileProcessing(".js", 50)
		}()

		go func() {
			defer wg.Done()
			env.TestConcurrentFileProcessing(".css", 50)
		}()

		wg.Wait()
	})

	t.Run("Concurrent cache access and regeneration", func(t *testing.T) {
		env := setupTestEnv("concurrency-cache", t)
		am := env.AssetsHandler

		// Add initial content
		filePath := filepath.Join(env.BaseDir, "initial.js")
		os.WriteFile(filePath, []byte("console.log('initial');"), 0644)
		am.NewFileEvent("initial.js", ".js", filePath, "create")

		var wg sync.WaitGroup
		numGoroutines := 100
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				if id%2 == 0 {
					// Reader
					if _, err := am.GetMinifiedJS(); err != nil {
						t.Errorf("Error getting minified JS: %v", err)
					}
				} else {
					// Writer (triggers cache invalidation)
					fileName := fmt.Sprintf("file%d.js", id)
					fPath := filepath.Join(env.BaseDir, fileName)
					os.WriteFile(fPath, []byte("console.log('update');"), 0644)
					am.NewFileEvent(fileName, ".js", fPath, "create")
				}
			}(i)
		}

		wg.Wait()

		finalContent, err := am.GetMinifiedJS()
		if err != nil {
			t.Fatalf("Error getting final minified JS: %v", err)
		}
		if len(finalContent) == 0 {
			t.Error("Final minified content is empty")
		}
	})
}

func TestSSRReloadConcurrency(t *testing.T) {
	env := setupTestEnv("ssr_concurrency", t)
	am := env.AssetsHandler

	moduleDir := filepath.Join(env.BaseDir, "mymodule")
	os.MkdirAll(moduleDir, 0755)
	ssrPath := filepath.Join(moduleDir, "ssr.go")

	var wg sync.WaitGroup
	iterations := 100
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func(id int) {
			defer wg.Done()
			content := fmt.Sprintf("package mypkg\nfunc RenderCSS() string { return \".class%d{}\" }", id)
			os.WriteFile(ssrPath, []byte(content), 0644)
			am.ReloadSSRModule(moduleDir)
			am.GetMinifiedCSS()
		}(i)
	}

	wg.Wait()
	t.Log("✓ SSR reload concurrency test completed")
}
