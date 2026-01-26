package assetmin

import (
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

	config := &Config{
		OutputDir: filepath.Join(tmpDir, "dist"),
		GetSSRClientInitJS: func() (string, error) {
			return "init();", nil // No init code needed for this test
		},
	}
	am := NewAssetMin(config)

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

	// Test case: SSR mode - manually activated via SetExternalSSRCompiler.
	// Expected: onSSRCompile is called. No internal processing.
	t.Run("ssr mode delegates to external handler", func(t *testing.T) {
		ssrCompileCalled = false
		am.SetExternalSSRCompiler(func() error {
			ssrCompileCalled = true
			return nil
		}, false)

		if !am.isSSRMode() {
			t.Fatal("expected SSR mode to be active after SetExternalSSRCompiler")
		}

		am.NewFileEvent("test.js", ".js", jsPath, "write")

		if !ssrCompileCalled {
			t.Errorf("expected onSSRCompile to be called in SSR mode")
		}
	})

	// Test case: SSR mode with buildOnDisk=true.
	// Expected:
	// 1. Files are written to disk.
	// 2. onSSRCompile is called.
	// 3. Existing files are NOT overwritten on initialization (safe write).
	// 4. Files ARE updated on subsequent watcher events.
	t.Run("ssr mode with buildOnDisk=true handles safe write and updates", func(t *testing.T) {
		outputDir := filepath.Join(tmpDir, "ssr_disk_test")
		config := &Config{
			OutputDir: outputDir,
			GetSSRClientInitJS: func() (string, error) {
				return "console.log('init');", nil
			},
		}
		am := NewAssetMin(config)

		// Create a "pre-existing" file in output dir
		err := os.MkdirAll(outputDir, 0755)
		if err != nil {
			t.Fatal(err)
		}
		existingFile := filepath.Join(outputDir, "style.css")
		existingContent := "/* preserved */"
		os.WriteFile(existingFile, []byte(existingContent), 0644)

		ssrCompileCalled := false
		am.SetExternalSSRCompiler(func() error {
			ssrCompileCalled = true
			return nil
		}, true)

		if !ssrCompileCalled {
			t.Error("expected onSSRCompile to be called immediately")
		}

		// Verify safe write: style.css should NOT have been overwritten
		content, _ := os.ReadFile(existingFile)
		if string(content) != existingContent {
			t.Errorf("expected existing file to be preserved, got %s", string(content))
		}

		// Verify other files WERE written (since they didn't exist)
		jsFile := filepath.Join(outputDir, "script.js")
		if _, err := os.Stat(jsFile); os.IsNotExist(err) {
			t.Error("expected script.js to be created")
		}

		// Verify watcher update: subsequent NewFileEvent SHOULD trigger callback but NOT overwrite in SSR mode
		newJSContent := "var updated = true;"
		sourceJS := filepath.Join(tmpDir, "source.js")
		os.WriteFile(sourceJS, []byte(newJSContent), 0644)

		ssrCompileCalled = false
		err = am.NewFileEvent("source.js", ".js", sourceJS, "write")
		if err != nil {
			t.Fatal(err)
		}

		if !ssrCompileCalled {
			t.Error("expected onSSRCompile to be called on watcher event")
		}

		// Verify script.js was NOT updated (because NewFileEvent returns early in SSR mode)
		// It should still have the content from the initial safe build (or empty if it didn't exist)
		content, _ = os.ReadFile(jsFile)
		if bytes.Contains(content, []byte("updated")) {
			t.Errorf("expected script.js NOT to be updated by watcher in SSR mode, but it was")
		}
	})
}
