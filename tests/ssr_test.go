package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestSSRModeDelegation validates AssetMin behavior in SSR (Server-Side Rendering) mode.
//
// SSR Mode is activated when the project's go.mod contains a dependency on assetmin.
// In this mode, AssetMin delegates all asset compilation to an external handler
// instead of processing internally.
//
// Expected behavior:
//   - Normal mode (no assetmin dependency): Assets are processed internally via
//     UpdateFileContentInMemory + processAsset. onSSRCompile is NOT called.
//   - SSR mode (assetmin dependency present): NewFileEvent calls onSSRCompile()
//     and returns early. No internal processing occurs.
func TestSSRModeDelegation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ssr_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	ac :=  &assetmin.Config{
		OutputDir: filepath.Join(tmpDir, "dist"),
	}
	am := assetmin.NewAssetMin(ac)

	ssrCompileCalled := false

	// Create a test JS file
	jsPath := filepath.Join(tmpDir, "test.js")
	err = os.WriteFile(jsPath, []byte("var x = 1;"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Test case: Normal mode - no go.mod with assetmin dependency.
	// Expected: Internal processing occurs. onSSRCompile is NOT called.
	t.Run("normal mode processes internally", func(t *testing.T) {
		ssrCompileCalled = false

		err := am.NewFileEvent("test.js", ".js", jsPath, "write")
		if err != nil {
			t.Fatal(err)
		}

		if ssrCompileCalled {
			t.Errorf("expected internal compilation (onSSRCompile should NOT be called)")
		}
	})

	// Test case: SSR mode - manually activated via EnableSSRMode.
	// Expected: onSSRCompile is called for .go files.
	t.Run("ssr mode delegates to external handler", func(t *testing.T) {
		ssrCompileCalled = false
		am.EnableSSRMode()
		am.SetSSRCompiler(func() error {
			ssrCompileCalled = true
			return nil
		})

		if !am.IsSSRMode() {
			t.Fatal("expected SSR mode to be active after EnableSSRMode")
		}

		// Trigger event for a .go file
		goPath := filepath.Join(tmpDir, "main.go")
		_ = os.WriteFile(goPath, []byte("package main"), 0644)
		_ = am.NewFileEvent("main.go", ".go", goPath, "write")

		if !ssrCompileCalled {
			t.Errorf("expected onSSRCompile to be called in SSR mode for .go file")
		}

		// .js file should NOT trigger compilation (it's either hot-reloaded or ignored)
		ssrCompileCalled = false
		am.NewFileEvent("test.js", ".js", jsPath, "write")
		if ssrCompileCalled {
			t.Errorf("expected onSSRCompile NOT to be called for .js file in SSR mode")
		}
	})

	// Test case: SSR mode with FlushToDisk().
	// Expected:
	// 1. Files are written to disk.
	// 2. onSSRCompile is NOT called automatically.
	// 3. Existing files ARE overwritten on FlushToDisk (B1 fix).
	// 4. Files ARE updated on subsequent watcher events.
	t.Run("ssr mode with FlushToDisk() handles overwrite and updates", func(t *testing.T) {
		outputDir := filepath.Join(tmpDir, "ssr_disk_test")
		ac := &assetmin.Config{
			OutputDir: outputDir,
		}
		am := assetmin.NewAssetMin(ac)

		// Create a "pre-existing" file in output dir
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			t.Fatal(err)
		}
		existingFile := filepath.Join(outputDir, "style.css")
		existingContent := "/* preserved */"
		os.WriteFile(existingFile, []byte(existingContent), 0644)

		ssrCompileCalled := false
		am.EnableSSRMode()
		am.SetSSRCompiler(func() error {
			ssrCompileCalled = true
			return nil
		})

		if ssrCompileCalled {
			t.Error("expected onSSRCompile NOT to be called immediately")
		}

		if err := am.FlushToDisk(); err != nil {
			t.Fatalf("FlushToDisk: %v", err)
		}

		// Verify overwrite: style.css should HAVE BEEN overwritten (fix B1)
		content, _ := os.ReadFile(existingFile)
		if string(content) == existingContent {
			t.Errorf("expected existing file to be overwritten")
		}

		// Verify other files WERE written
		jsFile := filepath.Join(outputDir, "script.js")
		if _, err := os.Stat(jsFile); os.IsNotExist(err) {
			t.Error("expected script.js to be created")
		}

		// Verify watcher update: subsequent NewFileEvent SHOULD trigger callback for .go files
		newGoContent := "package main"
		sourceGo := filepath.Join(tmpDir, "source.go")
		os.WriteFile(sourceGo, []byte(newGoContent), 0644)

		ssrCompileCalled = false
		err = am.NewFileEvent("source.go", ".go", sourceGo, "write")
		if err != nil {
			t.Fatal(err)
		}

		if !ssrCompileCalled {
			t.Error("expected onSSRCompile to be called on watcher event for .go file")
		}

		// Verify script.js was NOT updated (because NewFileEvent returns early in SSR mode)
		// It should still have the content from the initial safe build (or empty if it didn't exist)
		content, _ = os.ReadFile(jsFile)
		if bytes.Contains(content, []byte("updated")) {
			t.Errorf("expected script.js NOT to be updated by watcher in SSR mode, but it was")
		}
	})
}
