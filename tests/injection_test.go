package assetmin_test

//
import (
	"testing"
)

func TestAssetMin_AddAssets(t *testing.T) {
	env := setupTestEnv("add_assets", t)
	am := env.AssetsHandler

	// Test AddCSS
	am.InjectCSS("mockCSS", "body { color: blue; }")
	if !am.ContainsCSS("body { color: blue; }") {
		t.Error("CSS content not found in mainStyleCssHandler")
	}

	// Test AddJS
	am.InjectJS("mockJS", "console.log('hello');")
	if !am.ContainsJS("console.log('hello');") {
		t.Error("JS content not found in mainJsHandler")
	}

	// Test AddIcon
	err := am.InjectSpriteIcon("test-icon", "<path d='m'/>")
	if err != nil {
		t.Fatalf("AddIcon failed: %v", err)
	}
	if !am.HasIcon("test-icon") {
		t.Error("Icon not registered in map")
	}
	if !am.ContainsSVG("test-icon") {
		t.Error("Icon not found in sprite handler")
	}

	// Test InjectBodyContent
	am.InjectHTML("<div>Injected</div>")
	if !am.ContainsHTML("<div>Injected</div>") {
		t.Error("HTML content not found in indexHtmlHandler")
	}
}
