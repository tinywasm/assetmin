package assetmin

import (
	"strings"
	"testing"
)

// Mock components for testing

type mockComponent struct {
	css  string
	js   string
	html string
	role byte // 'r' for public ('*'), 'u' for user, etc.
}

func (m *mockComponent) RenderCSS() string {
	return m.css
}

func (m *mockComponent) RenderJS() string {
	return m.js
}

func (m *mockComponent) RenderHTML() string {
	return m.html
}

func (m *mockComponent) AllowedRoles(action byte) []byte {
	if action == 'r' {
		if m.role == '*' {
			return []byte{'*'}
		}
		return []byte{'u'}
	}
	return nil
}

type mockIconProvider struct {
	icons []map[string]string
}

func (m *mockIconProvider) IconSvg() []map[string]string {
	return m.icons
}

// Tests

func TestRegisterComponents_CSS_JS(t *testing.T) {
	env := setupTestEnv("comp_css_js", t)
	am := env.AssetsHandler

	comp := &mockComponent{
		css: "body { color: red; }",
		js:  "console.log('test');",
	}

	err := am.RegisterComponents(comp)
	if err != nil {
		t.Fatalf("RegisterComponents failed: %v", err)
	}

	// Verify CSS
	if !containsContent(am.mainStyleCssHandler.contentMiddle, "body { color: red; }") {
		t.Error("CSS content not found in mainStyleCssHandler")
	}

	// Verify JS
	if !containsContent(am.mainJsHandler.contentMiddle, "console.log('test');") {
		t.Error("JS content not found in mainJsHandler")
	}
}

func TestRegisterComponents_Icons(t *testing.T) {
	env := setupTestEnv("comp_icons", t)
	am := env.AssetsHandler

	comp := &mockIconProvider{
		icons: []map[string]string{
			{"id": "icon-a", "svg": "<path d='a'/>"},
			{"id": "icon-b", "svg": "<path d='b'/>"},
		},
	}

	err := am.RegisterComponents(comp)
	if err != nil {
		t.Fatalf("RegisterComponents failed: %v", err)
	}

	// Verify icons are registered
	if !am.registeredIconIDs["icon-a"] || !am.registeredIconIDs["icon-b"] {
		t.Error("Icons not registered in map")
	}

	// Verify sprite content
	if !containsContent(am.spriteSvgHandler.contentMiddle, `id="icon-a"`) {
		t.Error("Icon A not found in sprite handler")
	}
}

func TestRegisterComponents_HTML_SSR_Public(t *testing.T) {
	env := setupTestEnv("comp_ssr_public", t)
	am := env.AssetsHandler

	comp := &mockComponent{
		html: "<div>Hello Public</div>",
		role: '*', // Public
	}

	err := am.RegisterComponents(comp)
	if err != nil {
		t.Fatalf("RegisterComponents failed: %v", err)
	}

	if !containsContent(am.indexHtmlHandler.contentMiddle, "<div>Hello Public</div>") {
		t.Error("Public HTML should be injected into indexHtmlHandler")
	}
}

func TestRegisterComponents_HTML_SSR_Private(t *testing.T) {
	env := setupTestEnv("comp_ssr_private", t)
	am := env.AssetsHandler

	comp := &mockComponent{
		html: "<div>Hello Private</div>",
		role: 'u', // User only (not public)
	}

	err := am.RegisterComponents(comp)
	if err != nil {
		t.Fatalf("RegisterComponents failed: %v", err)
	}

	if containsContent(am.indexHtmlHandler.contentMiddle, "<div>Hello Private</div>") {
		t.Error("Private HTML should NOT be injected into indexHtmlHandler")
	}
}

// Helper
func containsContent(files []*contentFile, substr string) bool {
	for _, f := range files {
		if strings.Contains(string(f.content), substr) {
			return true
		}
	}
	return false
}
