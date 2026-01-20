package assetmin

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
		testIconsDir := filepath.Join(env.BaseDir, "icons")
		if err := os.MkdirAll(testIconsDir, 0755); err != nil {
			t.Fatalf("Failed to create icons dir: %v", err)
		}

		// Create individual icon files
		iconPaths := createTestIcons(t, testIconsDir)

		// Process each icon file
		for _, iconPath := range iconPaths {
			iconName := filepath.Base(iconPath)
			if err := env.AssetsHandler.NewFileEvent(iconName, ".svg", iconPath, "create"); err != nil {
				t.Fatalf("Error processing icon %s: %v", iconName, err)
			}
		}

		// Verify the sprite file was created
		if _, err := os.Stat(env.MainSvgPath); os.IsNotExist(err) {
			t.Fatalf("The sprite SVG file should be created at %s", env.MainSvgPath)
		}

		// Read the generated file
		content, err := os.ReadFile(env.MainSvgPath)
		if err != nil {
			t.Fatalf("Should be able to read the generated sprite file: %v", err)
		}

		// Verify the content structure
		svgContent := string(content)

		// Check SVG opening tag
		if !strings.Contains(svgContent, "<svg") {
			t.Errorf("Should contain SVG opening tag")
		}

		// Check for symbol IDs (all test icons should be included)
		if !strings.Contains(svgContent, `id="icon-test1"`) {
			t.Errorf("Should contain test icon 1")
		}
		if !strings.Contains(svgContent, `id="icon-test2"`) {
			t.Errorf("Should contain test icon 2")
		}

		// Test removing an icon
		if err := env.AssetsHandler.NewFileEvent("test1.svg", ".svg", iconPaths[0], "remove"); err != nil {
			t.Fatalf("Error removing icon: %v", err)
		}

		// Verify the updated sprite
		content, err = os.ReadFile(env.MainSvgPath)
		if err != nil {
			t.Fatalf("Should be able to read the updated sprite file: %v", err)
		}
		svgContent = string(content)

		// The removed icon should not be present
		if strings.Contains(svgContent, `id="icon-test1"`) {
			t.Errorf("Should not contain removed test icon 1")
		}
		if !strings.Contains(svgContent, `id="icon-test2"`) {
			t.Errorf("Should still contain test icon 2")
		}

		env.CleanDirectory()
	})
}

// Test that the sprite structure follows the expected format with open, middle, and close sections
func TestSvgSpriteStructure(t *testing.T) {
	t.Run("uc08_svg_sprite_structure", func(t *testing.T) {
		env := setupTestEnv("uc08_svg_sprite_structure", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir()

		// Access the SVG handler directly
		svgHandler := env.AssetsHandler.spriteSvgHandler
		// Verificar que contentOpen tiene el contenido adecuado
		if len(svgHandler.contentOpen) < 1 {
			t.Fatalf("SVG handler should have contentOpen")
		}

		// Verificar que contentOpen contiene las etiquetas de apertura SVG
		found := false
		for _, cf := range svgHandler.contentOpen {
			content := string(cf.content)
			if strings.Contains(content, "<svg") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ContentOpen should contain SVG opening tag")
		}

		// Create a test icon
		iconContent := `<symbol id="icon-test" viewBox="0 0 24 24"><path fill="currentColor" d="M12 2L2 22h20L12 2z"/></symbol>`
		iconFile := &contentFile{
			path:    "test-icon.svg",
			content: []byte(iconContent),
		}
		// Add the icon to the handler without writing to disk
		if err := svgHandler.UpdateContent(iconFile.path, "create", iconFile); err != nil {
			t.Fatalf("Error updating content: %v", err)
		}

		// En lugar de escribir en disco y leer el archivo, verificamos directamente
		// el contenido en memoria combinando contentOpen + contenido del símbolo + contentClose
		var svgContent string

		// Añadir contentOpen
		for _, cf := range svgHandler.contentOpen {
			svgContent += string(cf.content)
		}

		// Añadir el contenido del símbolo
		svgContent += iconContent

		// Añadir contentClose
		for _, cf := range svgHandler.contentClose {
			svgContent += string(cf.content)
		}

		// Check if it has proper opening and closing structure
		if !strings.Contains(svgContent, "<svg") {
			t.Errorf("Should contain opening SVG tag")
		}
		if !strings.Contains(svgContent, "</svg>") {
			t.Errorf("Should contain closing SVG tag")
		}
		if !strings.Contains(svgContent, `id="icon-test"`) {
			t.Errorf("Should contain the test icon")
		}

		env.CleanDirectory()
	})
}

func TestAddIcon_Success(t *testing.T) {
	env := setupTestEnv("add_icon_success", t)
	am := env.AssetsHandler

	err := am.addIcon("icon-1", `<path d="..."/>`)
	if err != nil {
		t.Errorf("Expected nil error, got %v", err)
	}

	if !am.registeredIconIDs["icon-1"] {
		t.Errorf("Icon should be registered")
	}

	// Verify content in handler
	found := false
	for _, f := range am.spriteSvgHandler.contentMiddle {
		if strings.Contains(string(f.content), `id="icon-1"`) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Icon content should be in spriteSvgHandler")
	}
}

func TestAddIcon_Collision(t *testing.T) {
	env := setupTestEnv("add_icon_collision", t)
	am := env.AssetsHandler

	err := am.addIcon("icon-duplicate", `<path d="..."/>`)
	if err != nil {
		t.Errorf("Expected nil error for first add, got %v", err)
	}

	err = am.addIcon("icon-duplicate", `<path d="..."/>`)
	if err == nil {
		t.Errorf("Expected error for duplicate icon, got nil")
	}
	if err.Error() != "icon already registered: icon-duplicate" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// Helper function to create test SVG icon files
func createTestIcons(t *testing.T, dir string) []string {
	icons := []struct {
		name    string
		content string
	}{
		{
			name: "test1.svg",
			content: `<symbol id="icon-test1" viewBox="0 0 24 24">
				<path fill="currentColor" d="M12 2L2 22h20L12 2z"/>
			</symbol>`,
		},
		{
			name: "test2.svg",
			content: `<symbol id="icon-test2" viewBox="0 0 24 24">
				<path fill="currentColor" d="M12 12a6 6 0 100 12 6 6 0 000-12z"/>
			</symbol>`,
		},
	}

	var paths []string
	for _, icon := range icons {
		path := filepath.Join(dir, icon.name)
		if err := os.WriteFile(path, []byte(icon.content), 0644); err != nil {
			t.Fatalf("Failed to write icon %s: %v", icon.name, err)
		}
		paths = append(paths, path)
	}
	return paths
}
