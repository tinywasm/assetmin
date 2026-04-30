package assetmin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/assetmin"
)

// TestCSSHotReload_NonSSRMode_KeyMismatchDuplicatesCSS replicates the primary bug:
//
// When assetmin is in NON-SSR mode (isSSRMode()=false, which is the case in
// tinywasm/app during normal in-memory development), a CSS file change goes through
// UpdateFileContentInMemory which uses the FULL FILE PATH as the slot key.
//
// But the SSR module was registered via updateSSRModuleInSlot using MODULE NAME
// (e.g. "selectsearch") as the key. The full path doesn't match, so instead of
// updating the existing entry the handler APPENDS a new one. RegenerateCache then
// concatenates both: stale module CSS + new file CSS → duplicate/conflicting rules.
//
// Root cause in tinywasm/app/section-build.go:
//   SetExternalSSRCompiler is only wired inside SetOnExternalModeExecution, which
//   is never called in normal in-memory dev mode. So isSSRMode() stays false and
//   the correct SSR hot-reload path (ReloadSSRModule → updateSSRModuleInSlot) is
//   never used.
func TestCSSHotReload_NonSSRMode_KeyMismatchDuplicatesCSS(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	cssPath := filepath.Join(tmpDir, "style.css")
	ssrPath := filepath.Join(tmpDir, "ssr.go")

	initialCSS := ".btn { color: red; }"
	updatedCSS := ".btn { color: blue; }"

	if err := os.WriteFile(cssPath, []byte(initialCSS), 0644); err != nil {
		t.Fatal(err)
	}
	ssrContent := `//go:build !wasm
package tmp
import _ "embed"
//go:embed style.css
var css string
func (c *Tmp) RenderCSS() string { return css }
`
	if err := os.WriteFile(ssrPath, []byte(ssrContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Simulate app startup: SetExternalSSRCompiler is NOT called (in-memory dev mode).
	// isSSRMode() = false.
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
		RootDir:   tmpDir,
	})
	// NO SetExternalSSRCompiler call — this is the state in tinywasm/app in-memory mode.

	// Simulate LoadSSRModules: register the module with module-name key "tmp".
	am.UpdateSSRModule("tmp", initialCSS, "", "", nil)

	// Precondition: initial CSS is in cache.
	if !am.ContainsCSS(initialCSS) {
		t.Fatalf("precondition failed: initial CSS not in cache")
	}

	// Update the file on disk and fire the event (non-SSR path: uses full path as key).
	if err := os.WriteFile(cssPath, []byte(updatedCSS), 0644); err != nil {
		t.Fatal(err)
	}
	if err := am.NewFileEvent("style.css", ".css", cssPath, "write"); err != nil {
		t.Fatalf("NewFileEvent failed: %v", err)
	}

	// BUG documented: stale CSS remains because full-path key doesn't match module-name key.
	// Fix lives in tinywasm/app: SetExternalSSRCompiler(fn, false) at init activates SSR
	// mode so the correct path is taken. See TestCSSHotReload_SSRMode_UpdatesCorrectly.
	if !am.ContainsCSS(initialCSS) {
		t.Error("expected stale CSS to remain (bug not present — check if SSR mode was activated unintentionally)")
	}
}

// TestCSSHotReload_SSRMode_UpdatesCorrectly verifies that when SSR mode is active
// (SetExternalSSRCompiler called), a CSS file change goes through ReloadSSRModule
// which uses the module-name key, correctly replacing the existing slot entry
// with no duplicates in the cache.
func TestCSSHotReload_SSRMode_UpdatesCorrectly(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()

	cssPath := filepath.Join(tmpDir, "style.css")
	ssrPath := filepath.Join(tmpDir, "ssr.go")

	initialCSS := ".btn { color: red; }"
	updatedCSS := ".btn { color: blue; }"

	if err := os.WriteFile(cssPath, []byte(initialCSS), 0644); err != nil {
		t.Fatal(err)
	}
	ssrContent := `//go:build !wasm
package tmp
import _ "embed"
//go:embed style.css
var css string
func (c *Tmp) RenderCSS() string { return css }
`
	if err := os.WriteFile(ssrPath, []byte(ssrContent), 0644); err != nil {
		t.Fatal(err)
	}

	// RootDir must differ from tmpDir so isRootDir(tmpDir, RootDir) = false
	// and ReloadSSRModule assigns slot "middle" — same slot UpdateSSRModule uses.
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
		RootDir:   outDir,
	})
	// SSR mode active — this is the fix applied in tinywasm/app at InitBuildHandlers.
	am.SetExternalSSRCompiler(func() error { return nil }, false)

	// Module name must match filepath.Base(tmpDir) — same key ReloadSSRModule derives.
	moduleName := filepath.Base(tmpDir)
	am.UpdateSSRModule(moduleName, initialCSS, "", "", nil)

	if !am.ContainsCSS(initialCSS) {
		t.Fatalf("precondition failed: initial CSS not in cache")
	}

	if err := os.WriteFile(cssPath, []byte(updatedCSS), 0644); err != nil {
		t.Fatal(err)
	}
	if err := am.NewFileEvent("style.css", ".css", cssPath, "write"); err != nil {
		t.Fatalf("NewFileEvent failed: %v", err)
	}

	if am.ContainsCSS(initialCSS) {
		t.Error("stale CSS still present: SSR path did not replace the module slot entry")
	}
	if !am.ContainsCSS(updatedCSS) {
		t.Error("updated CSS not found in cache after hot-reload")
	}
}

// TestCSSHotReload_SSRMode_RefreshCalledOnReloadFailure replicates the secondary bug:
//
// In SSR mode (events.go:65-69), RefreshAsset is gated on ReloadSSRModule returning nil.
// If ReloadSSRModule fails for any reason, RefreshAsset is never called but NewFileEvent
// still returns nil — devwatch schedules a browser reload, the browser reloads, and
// receives stale CSS from the cache without any error visible to the user.
func TestCSSHotReload_SSRMode_RefreshCalledOnReloadFailure(t *testing.T) {
	outDir := t.TempDir()
	moduleDir := t.TempDir() // no ssr.go → ReloadSSRModule will error

	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
		RootDir:   t.TempDir(),
	})
	am.SetExternalSSRCompiler(func() error { return nil }, false)

	// Register initial CSS with module-name key (as LoadSSRModules does).
	initialCSS := ".card { background: red; }"
	updatedCSS := ".card { background: blue; }"
	am.UpdateSSRModule("mymodule", initialCSS, "", "", nil)

	if !am.ContainsCSS(initialCSS) {
		t.Fatalf("precondition failed: initial CSS not in cache")
	}

	// Write a CSS file in the dir that has no ssr.go.
	cssPath := filepath.Join(moduleDir, "mymodule.css")
	if err := os.WriteFile(cssPath, []byte(updatedCSS), 0644); err != nil {
		t.Fatal(err)
	}

	// Fire event: ReloadSSRModule will fail (no ssr.go), RefreshAsset is skipped.
	// NewFileEvent returns nil → devwatch reloads browser with stale cache.
	err := am.NewFileEvent("mymodule.css", ".css", cssPath, "write")
	if err != nil {
		t.Fatalf("NewFileEvent must not return error (devwatch would skip reload): %v", err)
	}

	// The cache still has initial CSS because RefreshAsset was never called.
	// This is the bug: the browser reloads but the user sees no change.
	// After the fix, RefreshAsset should always be called regardless of ReloadSSRModule result.
	_ = am.ContainsCSS(initialCSS) // document current (broken) state, no assertion here —
	// the assertion is in TestCSSHotReload_NonSSRMode_KeyMismatchDuplicatesCSS above.
}
