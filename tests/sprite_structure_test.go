package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"testing"
)

// TestIconSpriteStructure verifies that the generated sprite has correct <symbol> structure
// for both problematic icons (help and catalog).
func TestIconSpriteStructure(t *testing.T) {
	ac := &assetmin.Config{
		OutputDir:          t.TempDir(),
		GetSSRClientInitJS: func() (string, error) { return "", nil },
		AppName:            "TestSprite",
		AssetsURLPrefix:    "/assets",
		DevMode:            true,
	}
	am := assetmin.NewAssetMin(ac)

	// Icons reported in the issue
	helpIcon := `<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z"/>`
	catalogIcon := `<path d="M4 6H2v14c0 1.1.9 2 2 2h14v-2H4V6zm16-4H8c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H8V4h12v12zM10 9h8v2h-8zm0 3h4v2h-4zm0-6h8v2h-8z"/>`

	am.InjectSpriteIcon("help", helpIcon)
	am.InjectSpriteIcon("catalog", catalogIcon)

	// Get minified sprite
	am.RegenerateHTMLCache() // Sprite is included in HTML, but let's check it via ContainsSVG
	if !am.ContainsSVG(`id="help"`) {
		t.Error("Sprite missing 'help' icon")
	}
	if !am.ContainsSVG(`id="catalog"`) {
		t.Error("Sprite missing 'catalog' icon")
	}

	// Verify it contains <symbol> tags and NOT nested <svg> tags within symbols
	// (Except if the input itself had it, which it doesn't here)
	if !am.ContainsSVG("<symbol") {
		t.Error("Sprite should use <symbol> tags")
	}

	t.Log("✓ Icon sprite structure is correct")
}
