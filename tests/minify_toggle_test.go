package assetmin_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/assetmin"
)

func TestMinifyToggle(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: t.TempDir(),
	})

	if am.Label() != "Minify: ON" {
		t.Errorf("expected Minify: ON, got %s", am.Label())
	}

	am.Execute()
	if am.Label() != "Minify: OFF" {
		t.Errorf("expected Minify: OFF, got %s", am.Label())
	}

	am.Execute()
	if am.Label() != "Minify: ON" {
		t.Errorf("expected Minify: ON, got %s", am.Label())
	}
}

func TestMinifyToggle_RegeneratesAssets(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "dist")
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
	})

	cssContent := "  .foo  {  color:  red;  }  "
	cssFile := filepath.Join(tmpDir, "style.css")
	os.WriteFile(cssFile, []byte(cssContent), 0644)

	am.NewFileEvent("style.css", ".css", cssFile, "write")

	// Minification ON
	minified, _ := am.GetMinifiedCSS()
	if bytes.Contains(minified, []byte("  ")) {
		t.Error("CSS should be minified")
	}

	// Toggle OFF
	am.Execute()
	unminified, _ := am.GetMinifiedCSS()
	if !bytes.Contains(unminified, []byte("  ")) {
		t.Error("CSS should NOT be minified after toggle")
	}

	// Toggle ON
	am.Execute()
	minified2, _ := am.GetMinifiedCSS()
	if bytes.Contains(minified2, []byte("  ")) {
		t.Error("CSS should be minified again after toggle")
	}
}

func TestMinifyToggle_DiskRewrite(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "dist")
	am := assetmin.NewAssetMin(&assetmin.Config{
		OutputDir: outDir,
	})
	am.SetExternalSSRCompiler(nil, true) // Enable buildOnDisk

	cssContent := "  .foo  {  color:  red;  }  "
	cssFile := filepath.Join(tmpDir, "style.css")
	os.WriteFile(cssFile, []byte(cssContent), 0644)

	am.NewFileEvent("style.css", ".css", cssFile, "write")

	outputPath := filepath.Join(outDir, "style.css")

	// Minification ON
	content, _ := os.ReadFile(outputPath)
	if bytes.Contains(content, []byte("  ")) {
		t.Error("Disk file should be minified")
	}

	// Toggle OFF
	am.Execute()
	content, _ = os.ReadFile(outputPath)
	if !bytes.Contains(content, []byte("  ")) {
		t.Error("Disk file should NOT be minified after toggle")
	}
}
