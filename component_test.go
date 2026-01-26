package assetmin

import (
	"strings"
	"testing"
)

func TestAssetMin_AddAssets(t *testing.T) {
	env := setupTestEnv("add_assets", t)
	am := env.AssetsHandler

	// Test AddCSS
	am.AddCSS("test-site", "body { color: blue; }")
	if !containsContent(am.mainStyleCssHandler.contentMiddle, "body { color: blue; }") {
		t.Error("CSS content not found in mainStyleCssHandler")
	}

	// Test AddJS
	am.AddJS("test-site", "console.log('hello');")
	if !containsContent(am.mainJsHandler.contentMiddle, "console.log('hello');") {
		t.Error("JS content not found in mainJsHandler")
	}

	// Test AddIcon
	err := am.AddIcon("test-icon", "<path d='m'/>")
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
