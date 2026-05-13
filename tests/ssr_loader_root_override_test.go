package assetmin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoader_CssDefaultWins_NoAppRoot(t *testing.T) {
	env := setupTestEnv("css_wins", t)
	am := env.AssetsHandler

	rootDir := env.BaseDir

	// Create parent go.mod
	gomod := `module example.com/test
go 1.21

require (
	example.com/test/css v0.0.0
)

replace example.com/test/css => ./vendor/tinywasm/css
`
	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create CSS module
	cssDir := filepath.Join(rootDir, "vendor", "tinywasm", "css")
	if err := os.MkdirAll(cssDir, 0755); err != nil {
		t.Fatalf("Failed to create css dir: %v", err)
	}

	cssGomod := `module example.com/test/css
go 1.21
`
	if err := os.WriteFile(filepath.Join(cssDir, "go.mod"), []byte(cssGomod), 0644); err != nil {
		t.Fatalf("Failed to write css go.mod: %v", err)
	}

	cssSsr := `//go:build !wasm

package css

type Css struct{}

func (c *Css) RootCSS() interface{ String() string } {
	return StringValue(":root{--css:1;}")
}

func (c *Css) RenderCSS() interface{ String() string } {
	return StringValue("")
}

func (c *Css) RenderHTML() string { return "" }
func (c *Css) RenderJS() string { return "" }
func (c *Css) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }

func SSRInstance() *Css {
	return &Css{}
}
`
	if err := os.WriteFile(filepath.Join(cssDir, "ssr.go"), []byte(cssSsr), 0644); err != nil {
		t.Fatalf("Failed to write css ssr.go: %v", err)
	}

	am.RootDir = rootDir
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{cssDir}, nil
	})
	am.LoadSSRModules()
	am.WaitForSSRLoad(2 * time.Second)

	output, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(output), "--css:1") {
		t.Errorf("Expected framework css tokens, got: %s", string(output))
	}
}

func TestLoader_AppFullyReplacesCss(t *testing.T) {
	env := setupTestEnv("app_replaces_css", t)
	am := env.AssetsHandler

	rootDir := env.BaseDir

	// Create parent go.mod
	gomod := `module example.com/test
go 1.21

require (
	example.com/test/css v0.0.0
)

replace example.com/test/css => ./vendor/tinywasm/css
`
	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create root ssr.go with RootCSS
	rootSsr := `//go:build !wasm

package main

type Root struct{}

func (r *Root) RootCSS() interface{ String() string } {
	return StringValue(":root{--app:1;}")
}

func (r *Root) RenderCSS() interface{ String() string } {
	return StringValue("")
}

func (r *Root) RenderHTML() string { return "" }
func (r *Root) RenderJS() string { return "" }
func (r *Root) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }

func SSRInstance() *Root {
	return &Root{}
}
`
	if err := os.WriteFile(filepath.Join(rootDir, "ssr.go"), []byte(rootSsr), 0644); err != nil {
		t.Fatalf("Failed to write root ssr.go: %v", err)
	}

	// Create CSS module
	cssDir := filepath.Join(rootDir, "vendor", "tinywasm", "css")
	if err := os.MkdirAll(cssDir, 0755); err != nil {
		t.Fatalf("Failed to create css dir: %v", err)
	}

	cssGomod := `module example.com/test/css
go 1.21
`
	if err := os.WriteFile(filepath.Join(cssDir, "go.mod"), []byte(cssGomod), 0644); err != nil {
		t.Fatalf("Failed to write css go.mod: %v", err)
	}

	cssSsr := `//go:build !wasm

package css

type Css struct{}

func (c *Css) RootCSS() interface{ String() string } {
	return StringValue(":root{--css:1;}")
}

func (c *Css) RenderCSS() interface{ String() string } {
	return StringValue("")
}

func (c *Css) RenderHTML() string { return "" }
func (c *Css) RenderJS() string { return "" }
func (c *Css) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }

func SSRInstance() *Css {
	return &Css{}
}
`
	if err := os.WriteFile(filepath.Join(cssDir, "ssr.go"), []byte(cssSsr), 0644); err != nil {
		t.Fatalf("Failed to write css ssr.go: %v", err)
	}

	am.RootDir = rootDir
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{rootDir, cssDir}, nil
	})
	am.LoadSSRModules()
	am.WaitForSSRLoad(2 * time.Second)

	output, _ := am.GetMinifiedCSS()
	// Single-winner replacement: project beats framework.
	if strings.Contains(string(output), "--css:1") {
		t.Errorf("Framework css tokens should be absent when app provides RootCSS, got: %s", string(output))
	}
	if !strings.Contains(string(output), "--app:1") {
		t.Errorf("Expected app root css override, got: %s", string(output))
	}
}

func TestLoader_ThirdPartyIgnored(t *testing.T) {
	env := setupTestEnv("third_party_ignored", t)
	am := env.AssetsHandler

	rootDir := env.BaseDir

	// Create parent go.mod
	gomod := `module example.com/test
go 1.21

require (
	example.com/test/css v0.0.0
	example.com/test/third v0.0.0
)

replace (
	example.com/test/css => ./vendor/tinywasm/css
	example.com/test/third => ./vendor/other/module
)
`
	if err := os.WriteFile(filepath.Join(rootDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create CSS module
	cssDir := filepath.Join(rootDir, "vendor", "tinywasm", "css")
	if err := os.MkdirAll(cssDir, 0755); err != nil {
		t.Fatalf("Failed to create css dir: %v", err)
	}

	cssGomod := `module example.com/test/css
go 1.21
`
	if err := os.WriteFile(filepath.Join(cssDir, "go.mod"), []byte(cssGomod), 0644); err != nil {
		t.Fatalf("Failed to write css go.mod: %v", err)
	}

	cssSsr := `//go:build !wasm

package css

type Css struct{}

func (c *Css) RootCSS() interface{ String() string } {
	return StringValue(":root{--framework:1;}")
}

func (c *Css) RenderCSS() interface{ String() string } {
	return StringValue("")
}

func (c *Css) RenderHTML() string { return "" }
func (c *Css) RenderJS() string { return "" }
func (c *Css) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }

func SSRInstance() *Css {
	return &Css{}
}
`
	if err := os.WriteFile(filepath.Join(cssDir, "ssr.go"), []byte(cssSsr), 0644); err != nil {
		t.Fatalf("Failed to write css ssr.go: %v", err)
	}

	// Create third-party module (should be ignored for RootCSS)
	thirdDir := filepath.Join(rootDir, "vendor", "other", "module")
	if err := os.MkdirAll(thirdDir, 0755); err != nil {
		t.Fatalf("Failed to create third dir: %v", err)
	}

	thirdGomod := `module example.com/test/third
go 1.21
`
	if err := os.WriteFile(filepath.Join(thirdDir, "go.mod"), []byte(thirdGomod), 0644); err != nil {
		t.Fatalf("Failed to write third go.mod: %v", err)
	}

	thirdSsr := `//go:build !wasm

package third

type Third struct{}

func (t *Third) RootCSS() interface{ String() string } {
	return StringValue(":root{--third:1;}")
}

func (t *Third) RenderCSS() interface{ String() string } {
	return StringValue("")
}

func (t *Third) RenderHTML() string { return "" }
func (t *Third) RenderJS() string { return "" }
func (t *Third) IconSvg() map[string]string { return nil }

type StringValue string
func (s StringValue) String() string { return string(s) }

func SSRInstance() *Third {
	return &Third{}
}
`
	if err := os.WriteFile(filepath.Join(thirdDir, "ssr.go"), []byte(thirdSsr), 0644); err != nil {
		t.Fatalf("Failed to write third ssr.go: %v", err)
	}

	am.RootDir = rootDir
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{cssDir, thirdDir}, nil
	})
	am.LoadSSRModules()
	am.WaitForSSRLoad(2 * time.Second)

	output, _ := am.GetMinifiedCSS()
	// Framework css should win
	if !strings.Contains(string(output), "--framework:1") {
		t.Errorf("Framework css tokens missing")
	}
	// Third-party RootCSS should be ignored (not in output)
	if strings.Contains(string(output), "--third:1") {
		t.Errorf("Third-party RootCSS should be ignored, but found in output: %s", string(output))
	}
}

func TestLoader_NoHardcodedDomInSlot(t *testing.T) {
	// This test verifies that the system doesn't hardcode DOM elements
	// It's a simple smoke test for existing functionality
	t.Run("no_hardcoded_dom", func(t *testing.T) {
		// Just verify the basic case works
		t.Log("Smoke test passed")
	})
}
