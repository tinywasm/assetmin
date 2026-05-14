package assetmin_test

import (
	"fmt"
	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to create properly structured test modules
func createSSRTestModule(t *testing.T, parentDir, modulePath, pkgName, body string) string {
	moduleDir := filepath.Join(parentDir, pkgName)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("Failed to create module directory: %v", err)
	}

	// Write go.mod for the submodule
	gomod := fmt.Sprintf("module %s\n\ngo 1.24\n\nrequire github.com/tinywasm/css v0.0.4\n", modulePath)
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Update parent go.mod to include replace directive
	parentGomod := filepath.Join(parentDir, "go.mod")
	content, err := os.ReadFile(parentGomod)
	if err != nil {
		t.Fatalf("Failed to read parent go.mod: %v", err)
	}

	gomodContent := string(content)
	if !strings.Contains(gomodContent, modulePath) {
		// Add replace directive
		replaceDir, _ := filepath.Abs(moduleDir)
		replaceDir = filepath.ToSlash(replaceDir)
		gomodContent += fmt.Sprintf("\nreplace %s => %s\n", modulePath, replaceDir)
		if err := os.WriteFile(parentGomod, []byte(gomodContent), 0644); err != nil {
			t.Fatalf("Failed to update parent go.mod: %v", err)
		}
	}

	// Write ssr.go
	structName := "Test"
	if len(pkgName) > 0 {
		structName = string(pkgName[0]-32) + pkgName[1:]
	}
	ssrGo := fmt.Sprintf(`//go:build !wasm

package %s

import "github.com/tinywasm/css"

%s

func SSRInstance() *%s {
	return &%s{}
}
`, pkgName, body, structName, structName)

	if err := os.WriteFile(filepath.Join(moduleDir, "ssr.go"), []byte(ssrGo), 0644); err != nil {
		t.Fatalf("Failed to write ssr.go: %v", err)
	}

	return moduleDir
}

func TestExtractSSRAssets(t *testing.T) {
	t.Run("ExtractLiteralCSS", func(t *testing.T) {
		parentDir := t.TempDir()

		// Create parent go.mod
		gomod := "module example.com/test\ngo 1.24\n"
		if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
			t.Fatalf("Failed to write parent go.mod: %v", err)
		}

		moduleDir := createSSRTestModule(t, parentDir, "example.com/test/css", "css",
			`type Css struct{}
func (c *Css) RenderCSS() *css.Stylesheet {
	return css.New(css.Raw(".cls{color:red;}"))
}
func (c *Css) RenderHTML() string { return "" }
func (c *Css) RenderJS() string { return "" }
func (c *Css) IconSvg() map[string]string { return nil }
`)

		assets, err := assetmin.ExtractSSRAssets(moduleDir)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if assets.CSS != ".cls{color:red;}" {
			t.Errorf("Expected CSS, got %q", assets.CSS)
		}
	})

	t.Run("ExtractRawStringJS", func(t *testing.T) {
		parentDir := t.TempDir()

		// Create parent go.mod
		gomod := "module example.com/test\ngo 1.24\n"
		if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
			t.Fatalf("Failed to write parent go.mod: %v", err)
		}

		moduleDir := createSSRTestModule(t, parentDir, "example.com/test/js", "js",
			`type Js struct{}
func (j *Js) RenderCSS() *css.Stylesheet {
	return nil
}
func (j *Js) RenderHTML() string { return "" }
func (j *Js) RenderJS() string { return "console.log(\"hello\");" }
func (j *Js) IconSvg() map[string]string { return nil }
`)

		assets, err := assetmin.ExtractSSRAssets(moduleDir)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if assets.JS != `console.log("hello");` {
			t.Errorf("Expected JS, got %q", assets.JS)
		}
	})

	t.Run("ExtractIconSvg", func(t *testing.T) {
		parentDir := t.TempDir()

		// Create parent go.mod
		gomod := "module example.com/test\ngo 1.24\n"
		if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
			t.Fatalf("Failed to write parent go.mod: %v", err)
		}

		moduleDir := createSSRTestModule(t, parentDir, "example.com/test/icons", "icons",
			`type Icons struct{}
func (i *Icons) RenderCSS() *css.Stylesheet {
	return nil
}
func (i *Icons) RenderHTML() string { return "" }
func (i *Icons) RenderJS() string { return "" }
func (i *Icons) IconSvg() map[string]string {
	return map[string]string{
		"home": "<svg>home</svg>",
		"user": "<svg>user</svg>",
	}
}
`)

		assets, err := assetmin.ExtractSSRAssets(moduleDir)
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

	// Receiver methods are the real-world pattern (e.g. func (c *Component) IconSvg()).
	// Compile-and-invoke handles them by calling SSRInstance().
	t.Run("ExtractIconSvg_ReceiverMethod", func(t *testing.T) {
		parentDir := t.TempDir()

		// Create parent go.mod
		gomod := "module example.com/test\ngo 1.24\n"
		if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
			t.Fatalf("Failed to write parent go.mod: %v", err)
		}

		moduleDir := createSSRTestModule(t, parentDir, "example.com/test/search", "search",
			`type Search struct{}
func (s *Search) RenderCSS() *css.Stylesheet {
	return nil
}
func (s *Search) RenderHTML() string { return "" }
func (s *Search) RenderJS() string { return "" }
func (s *Search) IconSvg() map[string]string {
	return map[string]string{
		"search-arrow-down": "<path fill=\"currentColor\" d=\"M1.5 4.5l6.5 7 6.5-7H1.5z\"/>",
	}
}
`)

		assets, err := assetmin.ExtractSSRAssets(moduleDir)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if assets.Icons["search-arrow-down"] == "" {
			t.Errorf("Expected search-arrow-down icon from receiver method, got empty")
		}
	})

	t.Run("ExtractCSS_ReceiverMethod", func(t *testing.T) {
		parentDir := t.TempDir()

		// Create parent go.mod
		gomod := "module example.com/test\ngo 1.24\n"
		if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
			t.Fatalf("Failed to write parent go.mod: %v", err)
		}

		moduleDir := createSSRTestModule(t, parentDir, "example.com/test/ss", "ss",
			`type Ss struct{}
func (s *Ss) RenderCSS() *css.Stylesheet {
	return css.New(css.Raw(".ss{color:red;}"))
}
func (s *Ss) RenderHTML() string { return "" }
func (s *Ss) RenderJS() string { return "" }
func (s *Ss) IconSvg() map[string]string { return nil }
`)

		assets, err := assetmin.ExtractSSRAssets(moduleDir)
		if err != nil {
			t.Fatalf("Error: %v", err)
		}
		if assets.CSS != ".ss{color:red;}" {
			t.Errorf("Expected CSS from receiver method, got %q", assets.CSS)
		}
	})
}
