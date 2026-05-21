package assetmin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/assetmin"
)

func TestCSSHotReload_NonSSRMode_KeyMismatchDuplicatesCSS(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := t.TempDir()
	cssPath := filepath.Join(tmpDir, "style.css")
	initialCSS := ".btn { color: red; }"
	updatedCSS := ".btn { color: blue; }"

	if err := os.WriteFile(cssPath, []byte(initialCSS), 0644); err != nil {
		t.Fatal(err)
	}

	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
		RootDir:   tmpDir,
	})

	am.UpdateSSRModule("tmp", initialCSS, nil, "", nil)

	if !am.ContainsCSS(initialCSS) {
		t.Fatalf("precondition failed")
	}

	if err := os.WriteFile(cssPath, []byte(updatedCSS), 0644); err != nil {
		t.Fatal(err)
	}
	if err := am.NewFileEvent("style.css", ".css", cssPath, "write"); err != nil {
		t.Fatalf("NewFileEvent failed: %v", err)
	}

	if !am.ContainsCSS(initialCSS) {
		t.Error("expected stale CSS to remain")
	}
}

func TestCSSHotReload_SSRMode_UpdatesCorrectly(t *testing.T) {
	outDir := t.TempDir()

	initialCSS := ".btn { color: red; }"
	updatedCSS := ".btn { color: blue; }"

	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
		RootDir:   outDir,
	})
	am.EnableSSRMode()
	am.SetSSRCompiler(func() error { return nil })

	moduleName := "tmp"
	am.UpdateSSRModule(moduleName, initialCSS, nil, "", nil)

	if !am.ContainsCSS(initialCSS) {
		t.Fatalf("precondition failed")
	}

	am.UpdateSSRModule(moduleName, updatedCSS, nil, "", nil)

	if am.ContainsCSS(initialCSS) {
		t.Error("stale CSS still present")
	}
	if !am.ContainsCSS(updatedCSS) {
		t.Error("updated CSS not found")
	}
}

func TestCSSHotReload_SSRMode_RefreshCalledOnReloadFailure(t *testing.T) {
	outDir := t.TempDir()
	moduleDir := t.TempDir()

	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
		RootDir:   t.TempDir(),
	})
	am.EnableSSRMode()
	am.SetSSRCompiler(func() error { return nil })

	initialCSS := ".card { background: red; }"
	updatedCSS := ".card { background: blue; }"
	am.UpdateSSRModule("mymodule", initialCSS, nil, "", nil)

	cssPath := filepath.Join(moduleDir, "mymodule.css")
	if err := os.WriteFile(cssPath, []byte(updatedCSS), 0644); err != nil {
		t.Fatal(err)
	}
}
