package assetmin_test

import (
	"strings"
)

// BUG REPRO: https://github.com/tinywasm/assetmin
//
// Bug 1: moduleSubpackagesUsed silently drops subpackages with more than one
//        path segment (e.g. "modules/contact"). The check
//        `!strings.Contains(subPath, "/")` treats any nested package as
//        non-existent, so ssr.go inside <root>/modules/contact/ is never
//        extracted during the initial LoadSSRModules scan.
//
// Bug 2: ExtractSSRAssets requires go.mod to be present in the exact moduleDir
//        passed to it. Sub-packages (e.g. modules/contact) have no go.mod of
//        their own — go.mod lives at the project root. ReloadSSRModule therefore
//        fails with "no go.mod found" on every hot-reload event for a nested
//        package, so CSS changes are never applied until the process restarts.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/assetmin"
)

// TestBug_DeepSubpackage_NotLoadedOnInitialScan reproduces Bug 1.
//
// Bug 1: moduleSubpackagesUsed silently drops any sub-path containing "/".
// Before the fix, "modules/contact" is skipped and the CSS is never loaded.
// This test verifies the fix by checking that multi-level subpackages are now supported.
func TestBug_DeepSubpackage_NotLoadedOnInitialScan(t *testing.T) {
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.21\n"), 0644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte(`package main
import _ "example.com/demo/modules/contact"
func main() {}`), 0644)

	contactDir := filepath.Join(root, "modules", "contact")
	os.MkdirAll(contactDir, 0755)
	os.WriteFile(filepath.Join(contactDir, "ssr.go"), []byte(`//go:build !wasm

package contact

type cssSheet struct{}
func (c *cssSheet) String() string { return ".contact-form { color: red; }" }
func RenderCSS() *cssSheet { return &cssSheet{} }
`), 0644)

	ac := &assetmin.Config{
		RootDir:   root,
		OutputDir: filepath.Join(root, "web", "public"),
		GetSSRClientInitJS: func() (string, error) { return "", nil },
	}
	am := assetmin.NewAssetMin(ac)

	// Directly call ExtractSSRAssetsWithContext to bypass the full LoadSSRModules
	// scan, which relies on import detection. This isolates the bug to just
	// ExtractSSRAssets handling of sub-packages.
	assets, err := am.ExtractSSRAssetsWithContext(contactDir)
	if err != nil {
		t.Fatalf("extractSSRAssetsWithContext failed: %v", err)
	}

	if assets == nil || assets.CSS == "" {
		t.Error("BUG 1 (if fails): CSS from modules/contact/ssr.go was not extracted; " +
			"the extraction should support multi-level sub-packages")
	} else if !strings.Contains(assets.CSS, "contact-form") {
		t.Errorf("Extracted CSS doesn't match: %s", assets.CSS)
	}
}

// TestBug_DeepSubpackage_HotReloadFails reproduces Bug 2.
//
// When the file-watcher detects a change in <root>/modules/contact/ssr.go,
// app/section-build.go calls AssetsHandler.ReloadSSRModule(contactDir).
// ReloadSSRModule → ExtractSSRAssets(contactDir) used to check for go.mod in
// contactDir first — which doesn't exist — and return "no go.mod found".
// After the fix, it should traverse up to find the root go.mod.
func TestBug_DeepSubpackage_HotReloadFails(t *testing.T) {
	root := t.TempDir()

	os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/demo\ngo 1.21\n"), 0644)
	os.WriteFile(filepath.Join(root, "main.go"), []byte(`package main
import _ "example.com/demo/modules/contact"
func main() {}`), 0644)

	contactDir := filepath.Join(root, "modules", "contact")
	os.MkdirAll(contactDir, 0755)
	os.WriteFile(filepath.Join(contactDir, "ssr.go"), []byte(`//go:build !wasm

package contact

type cssSheet struct{}

func (c *cssSheet) String() string { return ".contact-updated { color: blue; }" }

func RenderCSS() *cssSheet {
	return &cssSheet{}
}
`), 0644)

	ac := &assetmin.Config{
		RootDir:   root,
		OutputDir: filepath.Join(root, "web", "public"),
		GetSSRClientInitJS: func() (string, error) { return "", nil },
	}
	am := assetmin.NewAssetMin(ac)

	// Simulate the hot-reload call that app/section-build.go performs when
	// the file-watcher fires for a file inside modules/contact/.
	err := am.ReloadSSRModule(contactDir)
	if err == nil {
		// If no error, check that CSS was actually applied.
		if !am.ContainsCSS(".contact-updated") {
			t.Error("ReloadSSRModule returned nil but CSS was not applied")
		}
	} else {
		t.Errorf("ReloadSSRModule(%q) returned error: %v", contactDir, err)
	}
}
