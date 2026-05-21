//go:build !wasm

package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"path/filepath"
	"testing"
	"os"
)

func TestSSRIntegration(t *testing.T) {
	// Skip if we are in an environment without Go
	if _, err := os.Stat("/usr/local/go/bin/go"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/bin/go"); os.IsNotExist(err) {
			t.Skip("Go compiler not found")
		}
	}

	wd, _ := os.Getwd()
	workspaceDir := filepath.Join(wd, "..", "testdata", "integration_workspace")
	buttonDir := filepath.Join(workspaceDir, "button")

	// Create a dummy go.mod in button dir to make it a module for ExtractSSRAssets
	// In the real workspace it might be a subpackage, but ExtractSSRAssets currently
	// expects a go.mod in the moduleDir or above.

	assets, err := assetmin.ExtractSSRAssets(buttonDir)
	if err != nil {
		t.Fatalf("failed to extract assets from integration workspace: %v", err)
	}

	if assets.CSS != ".btn{color:blue;}" {
		t.Errorf("expected .btn{color:blue;}, got %q", assets.CSS)
	}
}
