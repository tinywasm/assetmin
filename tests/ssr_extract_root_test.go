package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
	"testing"
)

func TestExtract_RootCSS_FromLiteral(t *testing.T) {
	parentDir := t.TempDir()

	// Create parent go.mod
	gomod := `module example.com/test
go 1.21
`
	if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write parent go.mod: %v", err)
	}

	moduleDir := createSSRTestModule(t, parentDir, "example.com/test/theme", "theme",
		`type Theme struct{}

func (t *Theme) RenderCSS() interface{ String() string } {
	return StringValue("")
}

func (t *Theme) RootCSS() interface{ String() string } {
	return StringValue(":root{--x:1;}")
}

func (t *Theme) RenderHTML() string { return "" }
func (t *Theme) RenderJS() string { return "" }
func (t *Theme) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }
`)

	assets, err := assetmin.ExtractSSRAssets(moduleDir)
	if err != nil {
		t.Fatal(err)
	}

	if assets.RootCSS != ":root{--x:1;}" {
		t.Errorf("expected RootCSS %q, got %q", ":root{--x:1;}", assets.RootCSS)
	}
}

func TestExtract_RootCSS_Missing(t *testing.T) {
	parentDir := t.TempDir()

	// Create parent go.mod with workspace setup
	gomod := `module example.com/test
go 1.21

require example.com/test/noroot v0.0.0
replace example.com/test/noroot => ./noroot
`
	if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write parent go.mod: %v", err)
	}

	// Create noroot module directory
	moduleDir := filepath.Join(parentDir, "noroot")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("Failed to create noroot dir: %v", err)
	}

	// Write noroot go.mod
	noRootGomod := `module example.com/test/noroot
go 1.21
`
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(noRootGomod), 0644); err != nil {
		t.Fatalf("Failed to write noroot go.mod: %v", err)
	}

	// Write noroot ssr.go
	ssrGo := `//go:build !wasm

package noroot

type Noroot struct{}

func (n *Noroot) RenderCSS() interface{ String() string } {
	return StringValue(".component { color: blue; }")
}

func (n *Noroot) RenderHTML() string { return "" }
func (n *Noroot) RenderJS() string { return "" }
func (n *Noroot) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }

func SSRInstance() *Noroot {
	return &Noroot{}
}
`
	if err := os.WriteFile(filepath.Join(moduleDir, "ssr.go"), []byte(ssrGo), 0644); err != nil {
		t.Fatalf("Failed to write noroot ssr.go: %v", err)
	}

	assets, err := assetmin.ExtractSSRAssets(moduleDir)
	if err != nil {
		t.Fatal(err)
	}

	if assets.RootCSS != "" {
		t.Errorf("expected empty RootCSS, got %q", assets.RootCSS)
	}
}

func TestExtract_BothRootAndRender(t *testing.T) {
	parentDir := t.TempDir()

	// Create parent go.mod
	gomod := `module example.com/test
go 1.21
`
	if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write parent go.mod: %v", err)
	}

	moduleDir := createSSRTestModule(t, parentDir, "example.com/test/combined", "combined",
		`type Combined struct{}

func (c *Combined) RootCSS() interface{ String() string } {
	return StringValue(":root { --primary: blue; }")
}

func (c *Combined) RenderCSS() interface{ String() string } {
	return StringValue(".btn { background: var(--primary); }")
}

func (c *Combined) RenderHTML() string { return "<button></button>" }
func (c *Combined) RenderJS() string { return "" }
func (c *Combined) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }
`)

	assets, err := assetmin.ExtractSSRAssets(moduleDir)
	if err != nil {
		t.Fatal(err)
	}

	if assets.RootCSS != ":root { --primary: blue; }" {
		t.Errorf("expected RootCSS, got %q", assets.RootCSS)
	}
	if assets.CSS != ".btn { background: var(--primary); }" {
		t.Errorf("expected CSS, got %q", assets.CSS)
	}
}

// TestExtract_RootCSS_FromEmbed tests RootCSS from embedded files (now via struct methods)
func TestExtract_RootCSS_FromEmbed(t *testing.T) {
	parentDir := t.TempDir()

	// Create parent go.mod
	gomod := `module example.com/test
go 1.21
`
	if err := os.WriteFile(filepath.Join(parentDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write parent go.mod: %v", err)
	}

	moduleDir := createSSRTestModule(t, parentDir, "example.com/test/embed", "embed",
		`type Embed struct{}

const embeddedCSS = ":root { --bg: #fff; }"

func (e *Embed) RootCSS() interface{ String() string } {
	return StringValue(embeddedCSS)
}

func (e *Embed) RenderCSS() interface{ String() string } {
	return StringValue("")
}

func (e *Embed) RenderHTML() string { return "" }
func (e *Embed) RenderJS() string { return "" }
func (e *Embed) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }
`)

	assets, err := assetmin.ExtractSSRAssets(moduleDir)
	if err != nil {
		t.Fatal(err)
	}

	expected := ":root { --bg: #fff; }"
	if assets.RootCSS != expected {
		t.Errorf("expected RootCSS %q, got %q", expected, assets.RootCSS)
	}
}
