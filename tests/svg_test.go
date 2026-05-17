package assetmin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSvgSpriteGeneration verifica que la funcionalidad de generación de sprites SVG
// funciona correctamente, utilizando contentOpen para la apertura del SVG,
// contentMiddle para los iconos (símbolos) y contentClose para el cierre.
func TestSvgSpriteGeneration(t *testing.T) {
	t.Run("uc07_svg_sprite_creation", func(t *testing.T) {
		env := setupTestEnv("uc07_svg_sprite_creation", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir() // Ensure public directory exists

		// Create a test directory for svg icons
		iconsDir := filepath.Join(env.BaseDir, "web", "icons")
		if err := os.MkdirAll(iconsDir, 0755); err != nil {
			t.Fatalf("Failed to create icons directory: %v", err)
		}

		// Create some mock icon files
		icons := map[string]string{
			"icon-1.svg": `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/></svg>`,
			"icon-2.svg": `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-5 14H7v-2h7v2zm3-4H7v-2h10v2zm0-4H7V7h10v2z"/></svg>`,
		}

		for name, content := range icons {
			filePath := filepath.Join(iconsDir, name)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to write icon %s: %v", name, err)
			}
			// Trigger processing
			if err := env.AssetsHandler.NewFileEvent(name, ".svg", filePath, "create"); err != nil {
				t.Fatalf("NewFileEvent failed for %s: %v", name, err)
			}
		}

		// Verify the sprite exists in memory
		am := env.AssetsHandler

		// Check if symbols are present
		if !am.ContainsSVG("icon-1") || !am.ContainsSVG("icon-2") {
			t.Errorf("Sprite should contain symbols for all icons")
		}

		if !am.ContainsSVG("<symbol") {
			t.Errorf("Icons should be wrapped in <symbol> tags")
		}
	})

	t.Run("manual_icon_injection", func(t *testing.T) {
		env := setupTestEnv("manual_icon_injection", t)
		am := env.AssetsHandler

		// Manual injection of a symbol
		err := am.InjectSpriteIcon("manual-icon", "<path d='M1 2h3'/>")
		if err != nil {
			t.Fatalf("InjectSpriteIcon failed: %v", err)
		}

		if !am.HasIcon("manual-icon") {
			t.Error("Manual icon should be registered")
		}

		if !am.ContainsSVG(`id="manual-icon"`) {
			t.Error("Sprite should contain manual icon")
		}
	})

	t.Run("duplicate_icons_prevention", func(t *testing.T) {
		env := setupTestEnv("duplicate_icons", t)
		am := env.AssetsHandler

		err := am.InjectSpriteIcon("icon-1", "<path d='1'/>")
		if err != nil {
			t.Fatal(err)
		}

		// Try to inject again with same ID
		err = am.InjectSpriteIcon("icon-1", "<path d='2'/>")
		if err == nil {
			t.Error("Should have failed when injecting duplicate icon ID")
		}

		if !strings.Contains(err.Error(), "already registered") {
			t.Errorf("Error should mention 'already registered', got: %v", err)
		}
	})
}
