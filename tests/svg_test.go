//go:build !wasm

package assetmin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywasm/svg/sprite"
)

// TestSvgSpriteGeneration verifica que la funcionalidad de generación de sprites SVG
// funciona correctamente, utilizando contentOpen para la apertura del SVG,
// contentMiddle para los iconos (símbolos) y contentClose para el cierre.
func TestSvgSpriteGeneration(t *testing.T) {
	t.Run("uc07_svg_sprite_creation", func(t *testing.T) {
		env := setupTestEnv("uc07_svg_sprite_creation", t)
		env.AssetsHandler.FlushToDisk()
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
		err := am.InjectSpriteIcon("manual-icon", "<path d='M1 2h3'/>", "0 0 16 16")
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

		err := am.InjectSpriteIcon("icon-1", "<path d='1'/>", "0 0 16 16")
		if err != nil {
			t.Fatal(err)
		}

		// Try to inject again with same ID
		err = am.InjectSpriteIcon("icon-1", "<path d='2'/>", "0 0 16 16")
		if err == nil {
			t.Error("Should have failed when injecting duplicate icon ID")
		}

		if !strings.Contains(err.Error(), "already registered") {
			t.Errorf("Error should mention 'already registered', got: %v", err)
		}
	})
}

func TestSvgSpriteRefactoring(t *testing.T) {
	env := setupTestEnv("svg_sprite_refactoring", t)
	am := env.AssetsHandler

	// helper to create a sprite with a single icon
	spriteWithIcon := func(id string) *sprite.Sprite {
		s := sprite.NewSprite()
		s.AddRaw(id, "<path d='M1 2'/>", "0 0 16 16")
		return s
	}

	t.Run("Idempotent / no duplicates", func(t *testing.T) {
		err := am.UpdateSSRModule("mod", "", nil, "", spriteWithIcon("dup"))
		if err != nil {
			t.Fatalf("UpdateSSRModule failed: %v", err)
		}
		err = am.UpdateSSRModule("mod", "", nil, "", spriteWithIcon("dup"))
		if err != nil {
			t.Fatalf("UpdateSSRModule failed second time: %v", err)
		}

		// Check the content via ContainsSVG
		if !am.ContainsSVG("dup") {
			t.Errorf("Expected sprite to contain 'dup' icon")
		}

		// Count occurrences of id="dup"
		if err := am.RegenerateHTMLCache(); err != nil {
			t.Fatalf("RegenerateHTMLCache failed: %v", err)
		}
		html := string(am.GetCachedHTML())
		count := strings.Count(html, "id=\"dup\"")
		if count != 1 {
			t.Errorf("Expected id='dup' to appear exactly once, got %d", count)
		}
	})

	t.Run("Cross-module union", func(t *testing.T) {
		err := am.UpdateSSRModule("a", "", nil, "", spriteWithIcon("ia"))
		if err != nil {
			t.Fatal(err)
		}
		err = am.UpdateSSRModule("b", "", nil, "", spriteWithIcon("ib"))
		if err != nil {
			t.Fatal(err)
		}

		if !am.ContainsSVG("ia") || !am.ContainsSVG("ib") {
			t.Error("Expected both 'ia' and 'ib' icons to be present")
		}

		if err := am.RegenerateHTMLCache(); err != nil {
			t.Fatal(err)
		}
		html := string(am.GetCachedHTML())
		if strings.Count(html, "id=\"ia\"") != 1 || strings.Count(html, "id=\"ib\"") != 1 {
			t.Errorf("Expected each icon to appear exactly once")
		}
	})

	t.Run("Replace on re-extract", func(t *testing.T) {
		err := am.UpdateSSRModule("mod2", "", nil, "", spriteWithIcon("old"))
		if err != nil {
			t.Fatal(err)
		}
		if !am.ContainsSVG("old") {
			t.Error("Expected 'old' icon to be present")
		}

		err = am.UpdateSSRModule("mod2", "", nil, "", spriteWithIcon("new"))
		if err != nil {
			t.Fatal(err)
		}
		if am.ContainsSVG("old") {
			t.Error("Expected 'old' icon to be replaced/removed")
		}
		if !am.ContainsSVG("new") {
			t.Error("Expected 'new' icon to be present")
		}
	})
}
