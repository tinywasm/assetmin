package assetmin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tinywasm/assetmin"
)

func TestExtract_RootCSS_FromEmbed(t *testing.T) {
	dir := t.TempDir()
	ssrPath := filepath.Join(dir, "ssr.go")
	cssPath := filepath.Join(dir, "theme.css")

	cssContent := ":root { --bg: #fff; }"
	os.WriteFile(cssPath, []byte(cssContent), 0644)

	ssrCode := `package test
import "embed"
//go:embed theme.css
var rootCSS string
func RootCSS() string { return rootCSS }
`
	os.WriteFile(ssrPath, []byte(ssrCode), 0644)

	assets, err := assetmin.ExtractSSRAssets(dir)
	if err != nil {
		t.Fatal(err)
	}

	if assets.RootCSS != cssContent {
		t.Errorf("expected RootCSS %q, got %q", cssContent, assets.RootCSS)
	}
}

func TestExtract_RootCSS_FromLiteral(t *testing.T) {
	dir := t.TempDir()
	ssrPath := filepath.Join(dir, "ssr.go")

	ssrCode := `package test
func RootCSS() string { return ":root{--x:1;}" }
`
	os.WriteFile(ssrPath, []byte(ssrCode), 0644)

	assets, err := assetmin.ExtractSSRAssets(dir)
	if err != nil {
		t.Fatal(err)
	}

	expected := ":root{--x:1;}"
	if assets.RootCSS != expected {
		t.Errorf("expected RootCSS %q, got %q", expected, assets.RootCSS)
	}
}

func TestExtract_RootCSS_FromConcat(t *testing.T) {
	dir := t.TempDir()
	ssrPath := filepath.Join(dir, "ssr.go")

	ssrCode := `package test
func RootCSS() string { return ":root{" + "}" }
`
	os.WriteFile(ssrPath, []byte(ssrCode), 0644)

	assets, err := assetmin.ExtractSSRAssets(dir)
	if err != nil {
		t.Fatal(err)
	}

	expected := ":root{}"
	if assets.RootCSS != expected {
		t.Errorf("expected RootCSS %q, got %q", expected, assets.RootCSS)
	}
}

func TestExtract_RootCSS_Missing(t *testing.T) {
	dir := t.TempDir()
	ssrPath := filepath.Join(dir, "ssr.go")

	ssrCode := `package test
func RenderCSS() string { return "body{color:red;}" }
`
	os.WriteFile(ssrPath, []byte(ssrCode), 0644)

	assets, err := assetmin.ExtractSSRAssets(dir)
	if err != nil {
		t.Fatal(err)
	}

	if assets.RootCSS != "" {
		t.Errorf("expected empty RootCSS, got %q", assets.RootCSS)
	}
}

func TestExtract_BothRootAndRender(t *testing.T) {
	dir := t.TempDir()
	ssrPath := filepath.Join(dir, "ssr.go")

	ssrCode := `package test
func RootCSS() string { return ":root{--a:1;}" }
func RenderCSS() string { return ".comp{color:blue;}" }
`
	os.WriteFile(ssrPath, []byte(ssrCode), 0644)

	assets, err := assetmin.ExtractSSRAssets(dir)
	if err != nil {
		t.Fatal(err)
	}

	if assets.RootCSS != ":root{--a:1;}" {
		t.Errorf("unexpected RootCSS: %q", assets.RootCSS)
	}
	if assets.CSS != ".comp{color:blue;}" {
		t.Errorf("unexpected CSS: %q", assets.CSS)
	}
}

func TestExtract_RootCSS_UnparseableExpr(t *testing.T) {
	dir := t.TempDir()
	ssrPath := filepath.Join(dir, "ssr.go")

	ssrCode := `package test
func RootCSS() string { return computeIt() }
`
	os.WriteFile(ssrPath, []byte(ssrCode), 0644)

	assets, err := assetmin.ExtractSSRAssets(dir)
	if err != nil {
		t.Fatal(err)
	}

	if assets.RootCSS != "" {
		t.Errorf("expected empty RootCSS for unparseable expr, got %q", assets.RootCSS)
	}
}
