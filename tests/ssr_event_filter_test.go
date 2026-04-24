package assetmin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/assetmin"
)

func TestSSRMode_GoTriggersCompile(t *testing.T) {
	compiled := false
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
	})
	am.SetExternalSSRCompiler(func() error {
		compiled = true
		return nil
	}, false)

	err := am.NewFileEvent("ssr.go", ".go", "/path/ssr.go", "write")
	if err != nil {
		t.Fatalf("NewFileEvent failed: %v", err)
	}

	if !compiled {
		t.Error("expected onSSRCompile to be called for .go file")
	}
}

func TestSSRMode_EmbeddedAssetHotReload(t *testing.T) {
	compiled := false
	tmpDir := t.TempDir()
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
		RootDir:   tmpDir,
	})
	am.SetExternalSSRCompiler(func() error {
		compiled = true
		return nil
	}, false)

	// Create a dummy module with ssr.go and style.css
	ssrPath := filepath.Join(tmpDir, "ssr.go")
	cssPath := filepath.Join(tmpDir, "style.css")

	ssrContent := `package tmp
func RenderCSS() string { return "body { color: red; }" }
`
	if err := os.WriteFile(ssrPath, []byte(ssrContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cssPath, []byte("body { color: blue; }"), 0644); err != nil {
		t.Fatal(err)
	}

	// Trigger event for .css
	err := am.NewFileEvent("style.css", ".css", cssPath, "write")
	if err != nil {
		t.Fatalf("NewFileEvent failed: %v", err)
	}

	if compiled {
		t.Error("expected onSSRCompile NOT to be called for .css file")
	}

	// Verify CSS was updated (ReloadSSRModule should have been called)
	if !am.ContainsCSS("body { color: red; }") {
		t.Error("CSS cache was not updated from ssr.go")
	}
}

func TestSSRMode_LooseAssetIgnored(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
	})
	am.SetExternalSSRCompiler(func() error { return nil }, false)

	// No ssr.go in this dir
	cssPath := filepath.Join(t.TempDir(), "style.css")
	if err := os.WriteFile(cssPath, []byte("body { color: blue; }"), 0644); err != nil {
		t.Fatal(err)
	}

	err := am.NewFileEvent("style.css", ".css", cssPath, "write")
	if err != nil {
		t.Fatalf("NewFileEvent failed: %v", err)
	}

	if am.ContainsCSS("body { color: blue; }") {
		t.Error("Loose asset should have been ignored in SSR mode")
	}
}

func TestNonSSRMode_ProcessesAllEvents(t *testing.T) {
	outDir := t.TempDir()
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
	})
	// NOT calling SetExternalSSRCompiler

	cssPath := filepath.Join(t.TempDir(), "style.css")
	if err := os.WriteFile(cssPath, []byte("body { color: blue; }"), 0644); err != nil {
		t.Fatal(err)
	}

	err := am.NewFileEvent("style.css", ".css", cssPath, "write")
	if err != nil {
		t.Fatalf("NewFileEvent failed: %v", err)
	}

	if !am.ContainsCSS("body { color: blue; }") {
		t.Error("Asset should have been processed normally in non-SSR mode")
	}
}
