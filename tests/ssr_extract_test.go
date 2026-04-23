package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractSSRAssets(t *testing.T) {
	t.Run("ExtractLiteralCSS", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `package test
func RenderCSS() string { return ".cls{color:red;}" }
`
		os.WriteFile(filepath.Join(tmpDir, "ssr.go"), []byte(content), 0644)

		assets, err := assetmin.ExtractSSRAssets(tmpDir)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if assets.CSS != ".cls{color:red;}" {
			t.Errorf("Expected CSS, got %q", assets.CSS)
		}
	})

	t.Run("ExtractRawStringJS", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := "package test\nfunc RenderJS() string { return `console.log(\"hello\");` }\n"
		os.WriteFile(filepath.Join(tmpDir, "ssr.go"), []byte(content), 0644)

		assets, err := assetmin.ExtractSSRAssets(tmpDir)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if assets.JS != `console.log("hello");` {
			t.Errorf("Expected JS, got %q", assets.JS)
		}
	})

	t.Run("ExtractEmbedCSS", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(filepath.Join(tmpDir, "theme.css"), []byte("body{margin:0;}"), 0644)
		content := `package test
import "embed"
//go:embed theme.css
var ThemeCSS string
func RenderCSS() string { return ThemeCSS }
`
		os.WriteFile(filepath.Join(tmpDir, "ssr.go"), []byte(content), 0644)

		assets, err := assetmin.ExtractSSRAssets(tmpDir)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if assets.CSS != "body{margin:0;}" {
			t.Errorf("Expected embedded CSS, got %q", assets.CSS)
		}
	})

	t.Run("ExtractIconSvg", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := `package test
func IconSvg() map[string]string {
	return map[string]string{
		"home": "<svg>home</svg>",
		"user": "<svg>user</svg>",
	}
}
`
		os.WriteFile(filepath.Join(tmpDir, "ssr.go"), []byte(content), 0644)

		assets, err := assetmin.ExtractSSRAssets(tmpDir)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if assets.Icons["home"] != "<svg>home</svg>" {
			t.Errorf("Expected home icon, got %q", assets.Icons["home"])
		}
		if assets.Icons["user"] != "<svg>user</svg>" {
			t.Errorf("Expected user icon, got %q", assets.Icons["user"])
		}
	})
}
