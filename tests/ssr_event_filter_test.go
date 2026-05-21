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
	am.EnableSSRMode()
	am.SetSSRCompiler(func() error {
		compiled = true
		return nil
	})

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
		RootDir:   t.TempDir(),
	})
	am.EnableSSRMode()
	am.SetSSRCompiler(func() error {
		compiled = true
		return nil
	})

	// Create a dummy module with go.mod, ssr.go and style.css
	cssPath := filepath.Join(tmpDir, "style.css")
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module tmp\ngo 1.21\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ssrGo := "//go:build !wasm\n\npackage tmp\ntype T struct{}\nfunc (t *T) RenderCSS() *Stylesheet { return New(\"body { color: red; }\") }\ntype Stylesheet string\nfunc (s Stylesheet) String() string { return string(s) }\nfunc New(s string) *Stylesheet { return (*Stylesheet)(&s) }\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "ssr.go"), []byte(ssrGo), 0644); err != nil {
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
