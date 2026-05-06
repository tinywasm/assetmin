package assetmin_test

import (
	"strings"
	"testing"

	"github.com/tinywasm/assetmin"
)

type rootProvider struct{}
func (p *rootProvider) RootCSS() string { return ":root{--a:1;}" }

type rootAndCssProvider struct{}
func (p *rootAndCssProvider) RootCSS() string { return ":root{--b:2;}" }
func (p *rootAndCssProvider) RenderCSS() string { return ".comp{color:red;}" }

func TestRegister_RootCssProvider_NonEmpty(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{})
	p := &rootProvider{}
	if err := am.RegisterComponents(p); err != nil {
		t.Fatal(err)
	}

	css, _ := am.GetMinifiedCSS()
	expected := "--a:1" // Minifier might strip semicolon
	if !strings.Contains(string(css), expected) {
		t.Errorf("Expected RootCSS %q, got %q", expected, string(css))
	}
}

type rootProviderA struct{}
func (p *rootProviderA) RootCSS() string { return ":root{--a:1;}" }

type rootProviderB struct{}
func (p *rootProviderB) RootCSS() string { return ":root{--b:1;}" }

func TestRegister_RootCssOverrides(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{})

	am.RegisterComponents(&rootProviderA{})
	css, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(css), "--a:1") {
		t.Fatal("A not found")
	}

	am.RegisterComponents(&rootProviderB{})
	css, _ = am.GetMinifiedCSS()
	if !strings.Contains(string(css), "--b:1") {
		t.Error("B not found")
	}
}

func TestRegister_RootAndCssProvider(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{})
	p := &rootAndCssProvider{}
	am.RegisterComponents(p)

	css, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(css), "--b:2") {
		t.Error("RootCSS missing")
	}
	if !strings.Contains(string(css), ".comp{color:red}") {
		t.Error("RenderCSS missing")
	}
}
