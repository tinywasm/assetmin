package assetmin

import (
	"strings"
	"testing"
)

type mockCSS struct{}

func (m mockCSS) RenderCSS() string { return "body { color: blue; }" }

type mockJS struct{}

func (m mockJS) RenderJS() string { return "console.log('hello');" }

type mockIcon struct{}

func (m mockIcon) IconSvg() []map[string]string {
	return []map[string]string{{"id": "test-icon", "svg": "<path d='m'/>"}}
}

func TestAssetMin_AddAssets(t *testing.T) {
	env := setupTestEnv("add_assets", t)
	am := env.AssetsHandler

	// Test AddCSS
	am.AddCSS(mockCSS{})
	if !containsContent(am.mainStyleCssHandler.contentMiddle, "body { color: blue; }") {
		t.Error("CSS content not found in mainStyleCssHandler")
	}

	// Test AddJS
	am.AddJS(mockJS{})
	if !containsContent(am.mainJsHandler.contentMiddle, "console.log('hello');") {
		t.Error("JS content not found in mainJsHandler")
	}

	// Test AddIcon
	err := am.AddIcon(mockIcon{})
	if err != nil {
		t.Fatalf("AddIcon failed: %v", err)
	}
	if !am.registeredIconIDs["test-icon"] {
		t.Error("Icon not registered in map")
	}
	if !containsContent(am.spriteSvgHandler.contentMiddle, "test-icon") {
		t.Error("Icon not found in sprite handler")
	}

	// Test InjectBodyContent
	am.InjectBodyContent("<div>Injected</div>")
	if !containsContent(am.indexHtmlHandler.contentMiddle, "<div>Injected</div>") {
		t.Error("HTML content not found in indexHtmlHandler")
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
