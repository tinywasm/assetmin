package assetmin

import (
	"strings"
	"testing"
)

// TestIconSpriteStructure verifies that the generated sprite has correct <symbol> structure
// for both problematic icons (help and catalog).
func TestIconSpriteStructure(t *testing.T) {
	ac := &Config{
		OutputDir:          t.TempDir(),
		GetSSRClientInitJS: func() (string, error) { return "", nil },
		AppName:            "TestSprite",
		AssetsURLPrefix:    "/assets",
		DevMode:            true,
	}
	am := NewAssetMin(ac)

	// The EXACT icon definitions from the codebase
	iconHelp := `<path fill="currentColor" fill-rule="evenodd" d="M16 8A8 8 0 1 1 0 8a8 8 0 0 1 16 0zM5.496 6.033h.825c.138 0 .248-.113.266-.25.09-.656.54-1.134 1.342-1.134.688 0 1.314.343 1.314 1.168 0 .635-.374.927-.965 1.371-.673.489-1.206 1.06-1.168 1.987l.003.217a.25.25 0 0 0 .25.246h.811a.25.25 0 0 0 .25-.25v-.105c0-.718.273-.927 1.01-1.486.609-.463 1.244-.977 1.244-2.056 0-1.511-1.276-2.241-2.726-2.241-1.385 0-2.439.728-2.536 2.193a.25.25 0 0 0 .249.257zM8 12.75c-.5 0-.917-.417-.917-.917 0-.5.417-.917.917-.917.5 0 .917.417.917.917 0 .5-.417.917-.917.917z"/>`

	iconCatalog := `<path fill="currentColor" d="M11.5 0.5l-3.5 3 4.5 3 3.5-3z"></path>
<path fill="currentColor" d="M8 3.5l-3.5-3-4.5 3 3.5 3z"></path>
<path fill="currentColor" d="M12.5 6.5l3.5 3-4.5 2.5-3.5-3z"></path>
<path fill="currentColor" d="M8 9l-4.5-2.5-3.5 3 4.5 2.5z"></path>
<path fill="currentColor" d="M11.377 13.212l-3.377-2.895-3.377 2.895-2.123-1.179v1.467l5.5 2.5 5.5-2.5v-1.467z"></path>`

	// Inject icons
	if err := am.InjectSpriteIcon("icon-help", iconHelp); err != nil {
		t.Fatalf("Failed to inject icon-help: %v", err)
	}
	if err := am.InjectSpriteIcon("catalog-module", iconCatalog); err != nil {
		t.Fatalf("Failed to inject catalog-module: %v", err)
	}

	// Get the generated sprite content
	spriteContent, err := am.spriteSvgHandler.GetMinifiedContent(am.min)
	if err != nil {
		t.Fatalf("Failed to get sprite content: %v", err)
	}

	spriteStr := string(spriteContent)
	t.Logf("Generated sprite content:\n%s\n", spriteStr)

	// CRITICAL CHECKS:

	// 1. Check that icon-help symbol has correct viewBox
	if !strings.Contains(spriteStr, `id="icon-help"`) {
		t.Error("Missing icon-help symbol in sprite")
	}
	if !strings.Contains(spriteStr, `viewBox="0 0 16 16"`) {
		t.Error("Missing or incorrect viewBox in sprite symbols")
	}

	// 2. Check that catalog-module symbol exists
	if !strings.Contains(spriteStr, `id="catalog-module"`) {
		t.Error("Missing catalog-module symbol in sprite")
	}

	// 3. Check that there are NO nested <svg> tags (which would cause rendering issues)
	// Count <svg occurrences - there should be exactly 1 (the root sprite container)
	svgCount := strings.Count(spriteStr, "<svg")
	if svgCount != 1 {
		t.Errorf("Expected exactly 1 <svg> tag (root sprite), found %d. Possible nested SVGs causing rendering issues.", svgCount)
	}

	// 4. Check that all paths are present
	if !strings.Contains(spriteStr, "M16 8A8") {
		t.Error("icon-help path data is missing or corrupted")
	}
	if !strings.Contains(spriteStr, "M11.377 13.212") {
		t.Error("catalog-module last path data is missing or corrupted (truncation issue)")
	}

	// 5. Verify symbol count
	symbolCount := strings.Count(spriteStr, "<symbol")
	if symbolCount != 2 {
		t.Errorf("Expected 2 <symbol> elements, found %d", symbolCount)
	}

	t.Log("All sprite structure checks passed!")
}
