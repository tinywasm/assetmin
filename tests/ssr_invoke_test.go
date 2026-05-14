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
func SSRInstance() *T { return &T{} }
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
	if !aliases[0].HasInstance || !aliases[0].HasRoot || !aliases[0].HasRender {
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
