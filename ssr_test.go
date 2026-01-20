package assetmin

import (
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
		GetRuntimeInitializerJS: func() (string, error) {
			return "", nil // No init code needed for this test
		},
	}
	am := NewAssetMin(config)
	am.goModHandler.SetRootPath(tmpDir)

	ssrCompileCalled := false
	am.SetOnSSRCompile(func() {
		ssrCompileCalled = true
	})

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

	// Test case: SSR mode - go.mod contains assetmin dependency.
	// Expected: onSSRCompile is called. No internal processing.
	t.Run("ssr mode delegates to external handler", func(t *testing.T) {
		// Create go.mod with assetmin dependency to activate SSR mode
		goModContent := `module test
go 1.23
require ` + PackageName + ` v0.0.1
`
		goModPath := filepath.Join(tmpDir, "go.mod")
		err := os.WriteFile(goModPath, []byte(goModContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Trigger go.mod event to update internal state
		am.NewFileEvent("go.mod", ".mod", goModPath, "write")

		if !am.isSSRMode() {
			t.Fatal("expected SSR mode to be active after go.mod update")
		}

		ssrCompileCalled = false
		am.NewFileEvent("test.js", ".js", jsPath, "write")

		if !ssrCompileCalled {
			t.Errorf("expected onSSRCompile to be called in SSR mode")
		}
	})
}
