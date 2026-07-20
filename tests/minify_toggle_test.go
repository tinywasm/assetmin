//go:build !wasm

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

	if am.Label() != assetmin.MinifyLabel {
		t.Errorf("expected Label() = %s, got %s", assetmin.MinifyLabel, am.Label())
	}

	if am.Value() != assetmin.MinifyOptionOn {
		t.Errorf("expected Value() = %s, got %s", assetmin.MinifyOptionOn, am.Value())
	}

	am.Change(assetmin.MinifyOptionOff)
	if am.Value() != assetmin.MinifyOptionOff {
		t.Errorf("expected Value() = %s, got %s", assetmin.MinifyOptionOff, am.Value())
	}

	am.Change(assetmin.MinifyOptionOn)
	if am.Value() != assetmin.MinifyOptionOn {
		t.Errorf("expected Value() = %s, got %s", assetmin.MinifyOptionOn, am.Value())
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
	am.Change(assetmin.MinifyOptionOff)
	unminified, _ := am.GetMinifiedCSS()
	if !bytes.Contains(unminified, []byte("  ")) {
		t.Error("CSS should NOT be minified after toggle")
	}

	// Toggle ON
	am.Change(assetmin.MinifyOptionOn)
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
	am.FlushToDisk() // Enable diskMirrored

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
	am.Change(assetmin.MinifyOptionOff)
	content, _ = os.ReadFile(outputPath)
	if !bytes.Contains(content, []byte("  ")) {
		t.Error("Disk file should NOT be minified after toggle")
	}
}
