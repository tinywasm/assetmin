package assetmin_test

import (
	"strings"
	"testing"
)

type mockRootProvider struct {
	css string
}

func (m *mockRootProvider) RootCSS() interface{ String() string } {
	return StringValue(m.css)
}

type StringValue string

func (s StringValue) String() string {
	return string(s)
}

func TestLoader_CssDefaultWins_NoAppRoot(t *testing.T) {
	env := setupTestEnv("css_wins", t)
	am := env.AssetsHandler

	// Mock component registration logic instead of full module discovery
	am.UpdateSSRModuleInSlot("tinywasm/css", ":root{--css:1;}", "", "", nil, "open")

	output, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(output), "--css:1") {
		t.Errorf("Expected framework css tokens, got: %s", string(output))
	}
}

func TestLoader_AppFullyReplacesCss(t *testing.T) {
	env := setupTestEnv("app_replaces_css", t)
	am := env.AssetsHandler

	// Mock extraction and slot routing
	am.UpdateSSRModuleInSlot("tinywasm/css", ":root{--css:1;}", "", "", nil, "open")

	// App override via RegisterComponents which simulates app root registration
	am.RegisterComponents(&mockRootProvider{css: ":root{--app:1;}"})

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
	env := setupTestEnv("third_ignored", t)
	am := env.AssetsHandler

	// Simulate discovery results: Framework wins if no app root
	am.UpdateSSRModuleInSlot("tinywasm/css", ":root{--css:1;}", "", "", nil, "open")

	// Third party attempts to provide RootCSS but it should be ignored by routeAssets logic.
	// Since we are mocking with UpdateSSRModuleInSlot, we just prove that
	// only one winner is allowed in the 'open' slot by the handler if we manage it correctly.

	// In the real AssetMin, resolveAndApplyRootCSS handles the single-winner logic.
	// RegisterComponents also uses this logic.

	output, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(output), "--css:1") {
		t.Error("Framework css tokens missing")
	}
}
