package assetmin_test

import (
	"fmt"
	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
	"testing"
)

// writeStubModule writes a minimal stub Go module for SSR extraction testing.
func writeStubModule(t *testing.T, parentDir, modulePath, pkgName, body string) string {
	moduleDir := filepath.Join(parentDir, pkgName)
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("Failed to create module directory: %v", err)
	}

	// Write go.mod
	gomod := fmt.Sprintf("module %s\n\ngo 1.21\n", modulePath)
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Write ssr.go with package and SSRInstance()
	structName := "Stub"
	if len(pkgName) > 0 {
		structName = string(pkgName[0]-32) + pkgName[1:]
	}
	ssrGo := fmt.Sprintf(`//go:build !wasm

package %s

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

// TestSSRInvokeBasic demonstrates the compile-and-invoke mechanism for SSR asset extraction.
func TestSSRInvokeBasic(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a parent go.mod to establish a project root
	gomod := `module example.com/test
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write parent go.mod: %v", err)
	}

	// Create a stub button module using the helper
	buttonDir := writeStubModule(t, tmpDir, "example.com/test/button", "button",
		`type Button struct{}

func (b *Button) RenderCSS() interface{ String() string } {
	return StringValue(".button { color: blue; }")
}

func (b *Button) RenderHTML() string { return "<button></button>" }
func (b *Button) RenderJS() string   { return "" }
func (b *Button) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }
`)

	// Extract assets
	assets, err := assetmin.ExtractSSRAssets(buttonDir)
	if err != nil {
		t.Fatalf("Error extracting assets: %v", err)
	}

	// Verify assets were extracted
	if assets == nil {
		t.Fatal("Expected assets, got nil")
	}

	if assets.CSS != ".button { color: blue; }" {
		t.Errorf("Expected CSS '.button { color: blue; }', got %q", assets.CSS)
	}

	if assets.HTML != "<button></button>" {
		t.Errorf("Expected HTML '<button></button>', got %q", assets.HTML)
	}
}

// TestSSRInvokeMultipleModules demonstrates extracting assets from multiple modules in one invocation.
func TestSSRInvokeMultipleModules(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a parent go.mod
	gomod := `module example.com/testapp
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write parent go.mod: %v", err)
	}

	// Create button module
	buttonDir := writeStubModule(t, tmpDir, "example.com/testapp/button", "button",
		`type Button struct{}

func (b *Button) RenderCSS() interface{ String() string } {
	return StringValue(".btn { padding: 1rem; }")
}

func (b *Button) RenderHTML() string { return "<button></button>" }
func (b *Button) RenderJS() string   { return "console.log('button');" }
func (b *Button) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }
`)

	// Create card module
	cardDir := writeStubModule(t, tmpDir, "example.com/testapp/card", "card",
		`type Card struct{}

func (c *Card) RenderCSS() interface{ String() string } {
	return StringValue(".card { border: 1px solid #ccc; }")
}

func (c *Card) RenderHTML() string { return "<div class=\"card\"></div>" }
func (c *Card) RenderJS() string   { return "console.log('card');" }
func (c *Card) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }
`)

	// Extract assets for button
	buttonAssets, err := assetmin.ExtractSSRAssets(buttonDir)
	if err != nil {
		t.Fatalf("Error extracting button assets: %v", err)
	}

	if buttonAssets.CSS != ".btn { padding: 1rem; }" {
		t.Errorf("Expected button CSS, got %q", buttonAssets.CSS)
	}

	// Extract assets for card
	cardAssets, err := assetmin.ExtractSSRAssets(cardDir)
	if err != nil {
		t.Fatalf("Error extracting card assets: %v", err)
	}

	if cardAssets.CSS != ".card { border: 1px solid #ccc; }" {
		t.Errorf("Expected card CSS, got %q", cardAssets.CSS)
	}
}
