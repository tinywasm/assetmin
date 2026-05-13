package benchmark

import (
	"fmt"
	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkExtractSSRAssets_SingleModule measures extraction time for a single component module.
func BenchmarkExtractSSRAssets_SingleModule(b *testing.B) {
	tmpDir := b.TempDir()

	// Setup parent go.mod
	gomod := `module example.com/bench
go 1.24
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(gomod), 0644); err != nil {
		b.Fatalf("Failed to write parent go.mod: %v", err)
	}

	// Create a test module
	moduleDir := createTestModule(b, tmpDir, "example.com/bench/button", "button",
		`type Button struct{}
func (b *Button) RenderCSS() *css.Stylesheet {
	return css.New(css.Raw(".button { padding: 1rem; color: blue; }"))
}
func (b *Button) RenderHTML() string { return "<button></button>" }
func (b *Button) RenderJS() string   { return "console.log('button');" }
func (b *Button) IconSvg() map[string]string { return nil }
`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := assetmin.ExtractSSRAssets(moduleDir)
		if err != nil {
			b.Fatalf("Error extracting assets: %v", err)
		}
	}
}

// BenchmarkExtractSSRAssets_ThreeModules measures extraction time for three component modules.
func BenchmarkExtractSSRAssets_ThreeModules(b *testing.B) {
	tmpDir := b.TempDir()

	// Setup parent go.mod
	gomod := `module example.com/bench
go 1.24
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(gomod), 0644); err != nil {
		b.Fatalf("Failed to write parent go.mod: %v", err)
	}

	// Create three test modules
	buttonDir := createTestModule(b, tmpDir, "example.com/bench/button", "button",
		`type Button struct{}
func (b *Button) RenderCSS() *css.Stylesheet {
	return css.New(css.Raw(".btn { padding: 0.5rem; }"))
}
func (b *Button) RenderHTML() string { return "<button></button>" }
func (b *Button) RenderJS() string   { return "" }
func (b *Button) IconSvg() map[string]string { return nil }
`)

	cardDir := createTestModule(b, tmpDir, "example.com/bench/card", "card",
		`type Card struct{}
func (c *Card) RenderCSS() *css.Stylesheet {
	return css.New(css.Raw(".card { border: 1px solid #ccc; border-radius: 4px; }"))
}
func (c *Card) RenderHTML() string { return "<div class=\"card\"></div>" }
func (c *Card) RenderJS() string   { return "" }
func (c *Card) IconSvg() map[string]string { return nil }
`)

	formDir := createTestModule(b, tmpDir, "example.com/bench/form", "form",
		`type Form struct{}
func (f *Form) RenderCSS() *css.Stylesheet {
	return css.New(css.Raw("input, textarea { padding: 0.25rem; border: 1px solid #ddd; }"))
}
func (f *Form) RenderHTML() string { return "<form></form>" }
func (f *Form) RenderJS() string   { return "" }
func (f *Form) IconSvg() map[string]string { return nil }
`)

	// Warm up cache by extracting all modules once
	assetmin.ExtractSSRAssets(buttonDir)
	assetmin.ExtractSSRAssets(cardDir)
	assetmin.ExtractSSRAssets(formDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Measure cached extraction (most common case in production)
		_, err := assetmin.ExtractSSRAssets(buttonDir)
		if err != nil {
			b.Fatalf("Error extracting button assets: %v", err)
		}
	}
}

// BenchmarkExtractSSRAssets_LargeCSS measures extraction with more substantial CSS code.
func BenchmarkExtractSSRAssets_LargeCSS(b *testing.B) {
	tmpDir := b.TempDir()

	// Setup parent go.mod
	gomod := `module example.com/bench
go 1.24
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(gomod), 0644); err != nil {
		b.Fatalf("Failed to write parent go.mod: %v", err)
	}

	// Create a module with more substantial CSS
	cssRaw := `.component {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 2rem;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  border-radius: 8px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
  transition: all 0.3s ease;
}

.component:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 12px rgba(0, 0, 0, 0.15);
}

.component--variant-dark {
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
  color: #fff;
}

.component__header {
  font-size: 1.5rem;
  font-weight: 600;
  margin-bottom: 1rem;
}

.component__content {
  flex: 1;
  overflow-y: auto;
}

.component__footer {
  display: flex;
  gap: 0.5rem;
  padding-top: 1rem;
  border-top: 1px solid rgba(0, 0, 0, 0.1);
}
`

	moduleDir := createTestModule(b, tmpDir, "example.com/bench/design", "design",
		fmt.Sprintf(`type Design struct{}
func (d *Design) RenderCSS() *css.Stylesheet {
	return css.New(css.Raw(%q))
}
func (d *Design) RenderHTML() string { return "<div class=\"component\"></div>" }
func (d *Design) RenderJS() string   { return "" }
func (d *Design) IconSvg() map[string]string { return nil }
`, cssRaw))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := assetmin.ExtractSSRAssets(moduleDir)
		if err != nil {
			b.Fatalf("Error extracting assets: %v", err)
		}
	}
}

// BenchmarkIncrementalChange mide el wall-time real del dev loop:
// edita un .go entre iteraciones, forzando invalidación de hash y re-compile.
func BenchmarkIncrementalChange(b *testing.B) {
    tmpDir := b.TempDir()

    // Setup parent go.mod
    gomod := `module example.com/bench
go 1.24
`
    if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(gomod), 0644); err != nil {
        b.Fatalf("Failed to write parent go.mod: %v", err)
    }

    // Create a test module
    modulePath := "example.com/bench/button"
    pkgName := "button"
    moduleDir := filepath.Join(tmpDir, pkgName)
    if err := os.MkdirAll(moduleDir, 0755); err != nil {
        b.Fatalf("Failed to create module directory: %v", err)
    }

    // Write go.mod
    modContent := fmt.Sprintf("module %s\n\ngo 1.24\n\nrequire github.com/tinywasm/css v0.0.4\n", modulePath)
    if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(modContent), 0644); err != nil {
        b.Fatalf("Failed to write go.mod: %v", err)
    }

    // Initial ssr.go
    writeSsr := func(val int) {
        content := fmt.Sprintf(`//go:build !wasm
package button
import "github.com/tinywasm/css"
type Button struct{}
func (b *Button) RenderCSS() *css.Stylesheet {
	return css.New(css.Raw(".btn{color:rgb(%d,0,0);}"))
}
func SSRInstance() *Button { return &Button{} }
`, val)
        if err := os.WriteFile(filepath.Join(moduleDir, "ssr.go"), []byte(content), 0644); err != nil {
            b.Fatalf("Failed to write ssr.go: %v", err)
        }
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // 1. Modificar ssr.go para invalidar hash
        writeSsr(i % 256)

        // 2. Medir ExtractSSRAssets end-to-end (forzará recompilación porque el hash cambió)
        _, err := assetmin.ExtractSSRAssets(moduleDir)
        if err != nil {
            b.Fatalf("Error extracting assets: %v", err)
        }
    }
}

// createTestModule is a helper to create a properly structured test module.
func createTestModule(b *testing.B, parentDir, modulePath, pkgName, body string) string {
	moduleDir := filepath.Join(parentDir, pkgName)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		b.Fatalf("Failed to create module directory: %v", err)
	}

	// Write go.mod
	gomod := fmt.Sprintf("module %s\n\ngo 1.24\n\nrequire github.com/tinywasm/css v0.0.4\n", modulePath)
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(gomod), 0644); err != nil {
		b.Fatalf("Failed to write go.mod: %v", err)
	}

	// Write ssr.go
	structName := "Stub"
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
		b.Fatalf("Failed to write ssr.go: %v", err)
	}

	return moduleDir
}
