//go:build !wasm

package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
	"testing"
	"strings"
)

func TestModulesToAliases(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a module with some features
	modDir := filepath.Join(tmpDir, "my-mod")
	os.MkdirAll(modDir, 0755)
	ssrGo := `package mymod
func (t *T) RootCSS() *Stylesheet { return nil }
func (t *T) RenderCSS() *Stylesheet { return nil }
`
	os.WriteFile(filepath.Join(modDir, "ssr.go"), []byte(ssrGo), 0644)

	modules := []assetmin.Module{
		{Path: "example.com/my-mod", Dir: modDir},
		{Path: "001-mod", Dir: ""}, // numeric start
	}

	aliases := assetmin.ModulesToAliases(modules)

	if len(aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d", len(aliases))
	}

	// Check first module
	if aliases[0].Alias != "my_mod" {
		t.Errorf("expected alias my_mod, got %s", aliases[0].Alias)
	}
	if aliases[0].ReceiverType != "T" {
		t.Errorf("expected ReceiverType T, got %s", aliases[0].ReceiverType)
	}
	if !aliases[0].HasRoot || !aliases[0].HasRender {
		t.Errorf("expected all features to be detected for my-mod")
	}

	// Check numeric alias
	if aliases[1].Alias != "_001_mod" {
		t.Errorf("expected alias _001_mod, got %s", aliases[1].Alias)
	}
}

func TestGenerateExtractorMain(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a module with some features to ensure it is imported
	modDir := filepath.Join(tmpDir, "mod1")
	os.MkdirAll(modDir, 0755)
	os.WriteFile(filepath.Join(modDir, "ssr.go"), []byte("package mod1\nfunc RenderCSS() {}"), 0644)

	outputFile := filepath.Join(tmpDir, "main.go")

	modules := []assetmin.Module{
		{Path: "example.com/mod1", Dir: modDir},
	}

	err := assetmin.GenerateExtractorMain(outputFile, modules)
	if err != nil {
		t.Fatalf("failed to generate main.go: %v", err)
	}

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatal(err)
	}

	sContent := string(content)
	if !strings.Contains(sContent, "package main") {
		t.Error("missing package main")
	}
	if !strings.Contains(sContent, "mod1 \"example.com/mod1\"") {
		t.Error("missing import")
	}
}

func TestExtract_NoSSRInstanceFunction(t *testing.T) {
	tmpDir := t.TempDir()
	modDir := filepath.Join(tmpDir, "mod")
	os.MkdirAll(modDir, 0755)

	ssrGo := `package mod
type M struct{}
func (m *M) RenderCSS() stylesheet { return "CSS" }
type stylesheet string
func (s stylesheet) String() string { return string(s) }
`
	os.WriteFile(filepath.Join(modDir, "ssr.go"), []byte(ssrGo), 0644)

	modules := []assetmin.Module{
		{Path: "example.com/mod", Dir: modDir},
	}

	aliases := assetmin.ModulesToAliases(modules)
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}
	if aliases[0].ReceiverType != "M" {
		t.Errorf("expected ReceiverType M, got %s", aliases[0].ReceiverType)
	}
	if !aliases[0].HasRender {
		t.Errorf("expected HasRender to be true")
	}
}

func TestExtract_CssOnly(t *testing.T) {
	tmpDir := t.TempDir()
	modDir := filepath.Join(tmpDir, "mod")
	os.MkdirAll(modDir, 0755)

	os.WriteFile(filepath.Join(modDir, "css.go"), []byte(`package mod
type C struct{}
func (c *C) RenderCSS() stylesheet { return "a{}" }
type stylesheet string
func (s stylesheet) String() string { return string(s) }
`), 0644)

	modules := []assetmin.Module{{Path: "example.com/mod", Dir: modDir}}
	aliases := assetmin.ModulesToAliases(modules)
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}
	if !aliases[0].HasRender {
		t.Error("expected HasRender true for css.go-only module")
	}
	if aliases[0].ReceiverType != "C" {
		t.Errorf("expected ReceiverType C, got %s", aliases[0].ReceiverType)
	}
}

func TestExtract_CssPlusSvg(t *testing.T) {
	tmpDir := t.TempDir()
	modDir := filepath.Join(tmpDir, "mod")
	os.MkdirAll(modDir, 0755)

	os.WriteFile(filepath.Join(modDir, "css.go"), []byte(`package mod
type C struct{}
func (c *C) RenderCSS() stylesheet { return "b{}" }
type stylesheet string
func (s stylesheet) String() string { return string(s) }
`), 0644)
	os.WriteFile(filepath.Join(modDir, "svg.go"), []byte(`package mod
func (c *C) IconSvg() map[string]string { return map[string]string{"x": "<svg/>"} }
`), 0644)

	modules := []assetmin.Module{{Path: "example.com/mod", Dir: modDir}}
	aliases := assetmin.ModulesToAliases(modules)
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}
	if !aliases[0].HasRender {
		t.Error("expected HasRender true")
	}
	if !aliases[0].HasIcons {
		t.Error("expected HasIcons true from svg.go")
	}
	if aliases[0].ReceiverType != "C" {
		t.Errorf("expected shared ReceiverType C, got %s", aliases[0].ReceiverType)
	}
}

func TestExtract_AllFour(t *testing.T) {
	tmpDir := t.TempDir()
	modDir := filepath.Join(tmpDir, "mod")
	os.MkdirAll(modDir, 0755)

	os.WriteFile(filepath.Join(modDir, "css.go"), []byte(`package mod
type C struct{}
func (c *C) RootCSS() stylesheet { return ":root{}" }
func (c *C) RenderCSS() stylesheet { return "c{}" }
type stylesheet string
func (s stylesheet) String() string { return string(s) }
`), 0644)
	os.WriteFile(filepath.Join(modDir, "js.go"), []byte(`package mod
func (c *C) RenderJS() []string { return nil }
`), 0644)
	os.WriteFile(filepath.Join(modDir, "html.go"), []byte(`package mod
func (c *C) RenderHTML() string { return "<div/>" }
`), 0644)
	os.WriteFile(filepath.Join(modDir, "svg.go"), []byte(`package mod
func (c *C) IconSvg() map[string]string { return nil }
`), 0644)

	modules := []assetmin.Module{{Path: "example.com/mod", Dir: modDir}}
	aliases := assetmin.ModulesToAliases(modules)
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}
	a := aliases[0]
	if !a.HasRoot || !a.HasRender || !a.HasHTML || !a.HasIcons {
		t.Errorf("expected all features detected: root=%v render=%v html=%v icons=%v",
			a.HasRoot, a.HasRender, a.HasHTML, a.HasIcons)
	}
	if a.ReceiverType != "C" {
		t.Errorf("expected ReceiverType C, got %s", a.ReceiverType)
	}
}

func TestExtract_NoSSRFiles(t *testing.T) {
	tmpDir := t.TempDir()
	modDir := filepath.Join(tmpDir, "mod")
	os.MkdirAll(modDir, 0755)
	os.WriteFile(filepath.Join(modDir, "main.go"), []byte("package mod\n"), 0644)

	modules := []assetmin.Module{{Path: "example.com/mod", Dir: modDir}}
	aliases := assetmin.ModulesToAliases(modules)
	// Module is always aliased; when no SSR source files exist no features are detected.
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}
	a := aliases[0]
	if a.HasRoot || a.HasRender || a.HasHTML || a.HasJS || a.HasIcons {
		t.Errorf("expected no features for module with no SSR files, got %+v", a)
	}
}

func TestExtract_PackageLevelFuncs(t *testing.T) {
	tmpDir := t.TempDir()
	modDir := filepath.Join(tmpDir, "mod")
	os.MkdirAll(modDir, 0755)

	os.WriteFile(filepath.Join(modDir, "css.go"), []byte(`package mod
func RenderCSS() stylesheet { return "pkg{}" }
type stylesheet string
func (s stylesheet) String() string { return string(s) }
`), 0644)

	modules := []assetmin.Module{{Path: "example.com/mod", Dir: modDir}}
	aliases := assetmin.ModulesToAliases(modules)
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d", len(aliases))
	}
	if aliases[0].ReceiverType != "" {
		t.Errorf("expected empty ReceiverType for package-level func, got %q", aliases[0].ReceiverType)
	}
	if !aliases[0].HasRender {
		t.Error("expected HasRender true for package-level RenderCSS")
	}
}
