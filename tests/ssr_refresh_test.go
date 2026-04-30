package assetmin_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestReloadSSRModule_OnlyRefreshesChangedAssets(t *testing.T) {
	env := setupTestEnv("ssr_refresh_selective", t, nil)
	am := env.AssetsHandler

	// Prepare updated assets: ONLY CSS changed
	tmpDir := t.TempDir()
	ssrFile := filepath.Join(tmpDir, "ssr.go")
	ssrContent := `package tmp
func RenderCSS() string { return ".new-css {}" }
`
	if err := os.WriteFile(ssrFile, []byte(ssrContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Register initial assets using the SAME module name as tmpDir's base
	moduleName := filepath.Base(tmpDir)
	am.UpdateSSRModule(moduleName, ".old-css {}", "console.log('old js')", "<div>old html</div>", nil)

	// Ensure they are in cache
	if !am.ContainsCSS(".old-css") {
		t.Fatal("CSS not in cache")
	}

	// We need to use a directory that AssetMin knows about or can extract from.
	// ReloadSSRModule will call ExtractSSRAssets(tmpDir)
	if err := am.ReloadSSRModule(tmpDir); err != nil {
		t.Fatalf("ReloadSSRModule failed: %v", err)
	}

	// Verify CSS was updated
	if !am.ContainsCSS(".new-css") {
		t.Error("CSS was not updated")
	}
	if am.ContainsCSS(".old-css") {
		t.Error("Old CSS still in cache")
	}
}

func TestReloadSSRModule_ConcurrentCallsNoDeadlock(t *testing.T) {
	env := setupTestEnv("ssr_deadlock", t, nil)
	am := env.AssetsHandler

	tmpDir := t.TempDir()
	ssrFile := filepath.Join(tmpDir, "ssr.go")
	ssrContent := `package tmp
func RenderCSS() string { return ".new-css {}" }
`
	if err := os.WriteFile(ssrFile, []byte(ssrContent), 0644); err != nil {
		t.Fatal(err)
	}

	const iterations = 50
	var wg sync.WaitGroup
	wg.Add(iterations)

	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			_ = am.ReloadSSRModule(tmpDir)
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Deadlock detected or timeout")
	}
}

func TestRefreshWasmAssets_RefreshesJSAndHTMLOnly(t *testing.T) {
	var refreshCount int
	mockInit := func() (string, error) {
		refreshCount++
		return fmt.Sprintf("// runtime v%d", refreshCount), nil
	}

	env := setupTestEnv("wasm_refresh", t, mockInit)
	am := env.AssetsHandler

	// Initial check: GetInitCodeJS might call mockInit
	initCode1, _ := am.GetInitCodeJS()

	// RefreshJSAssets() should refresh .js and .html, triggering another call to mockInit
	am.RefreshJSAssets()

	// Verify regeneration
	initCode2, _ := am.GetInitCodeJS()
	if initCode1 == initCode2 {
		t.Errorf("Expected init code to change after refresh, but both were: %s", initCode1)
	}
}
