package assetmin_test

import (
	"strings"
	"testing"

	"github.com/tinywasm/assetmin"
	"github.com/tinywasm/css"
)

type rootProvider struct{}
func (p *rootProvider) RootCSS() *css.Stylesheet { return css.NewStylesheet(css.Raw(":root{--a:1;}")) }

type rootAndCssProvider struct{}
func (p *rootAndCssProvider) RootCSS() *css.Stylesheet { return css.NewStylesheet(css.Raw(":root{--b:2;}")) }
func (p *rootAndCssProvider) RenderCSS() *css.Stylesheet { return css.NewStylesheet(css.Raw(".comp{color:red;}")) }

func TestRegister_RootCssProvider_NonEmpty(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{})
	p := &rootProvider{}
	if err := am.RegisterComponents(p); err != nil {
		t.Fatal(err)
	}

	cssStr, _ := am.GetMinifiedCSS()
	expected := "--a:1" // Minifier might strip semicolon
	if !strings.Contains(string(cssStr), expected) {
		t.Errorf("Expected RootCSS %q, got %q", expected, string(cssStr))
	}
}

type rootProviderA struct{}
func (p *rootProviderA) RootCSS() *css.Stylesheet { return css.NewStylesheet(css.Raw(":root{--a:1;}")) }

type rootProviderB struct{}
func (p *rootProviderB) RootCSS() *css.Stylesheet { return css.NewStylesheet(css.Raw(":root{--b:1;}")) }

func TestRegister_RootCssOverrides(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{})

	am.RegisterComponents(&rootProviderA{})
	cssStr, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(cssStr), "--a:1") {
		t.Fatal("A not found")
	}

	am.RegisterComponents(&rootProviderB{})
	cssStr, _ = am.GetMinifiedCSS()
	if !strings.Contains(string(cssStr), "--b:1") {
		t.Error("B not found")
	}
}

func TestRegister_RootAndCssProvider(t *testing.T) {
	am := assetmin.NewAssetMin(&assetmin.Config{})
	p := &rootAndCssProvider{}
	am.RegisterComponents(p)

	cssStr, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(cssStr), "--b:2") {
		t.Error("RootCSS missing")
	}
	if !strings.Contains(string(cssStr), ".comp{color:red}") {
		t.Error("RenderCSS missing")
	}
}
