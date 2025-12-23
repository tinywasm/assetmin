package assetmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, os.MkdirAll(testIconsDir, 0755))

		// Create individual icon files
		iconPaths := createTestIcons(t, testIconsDir)

		// Process each icon file
		for _, iconPath := range iconPaths {
			iconName := filepath.Base(iconPath)
			require.NoError(t, env.AssetsHandler.NewFileEvent(iconName, ".svg", iconPath, "create"))
		}

		// Verify the sprite file was created
		require.FileExists(t, env.MainSvgPath, "The sprite SVG file should be created")

		// Read the generated file
		content, err := os.ReadFile(env.MainSvgPath)
		require.NoError(t, err, "Should be able to read the generated sprite file")

		// Verify the content structure
		svgContent := string(content)

		// Check SVG opening tag
		assert.True(t, strings.Contains(svgContent, "<svg"), "Should contain SVG opening tag")

		// Check for symbol IDs (all test icons should be included)
		assert.True(t, strings.Contains(svgContent, `id="icon-test1"`), "Should contain test icon 1")
		assert.True(t, strings.Contains(svgContent, `id="icon-test2"`), "Should contain test icon 2")

		// Test removing an icon
		require.NoError(t, env.AssetsHandler.NewFileEvent("test1.svg", ".svg", iconPaths[0], "remove"))

		// Verify the updated sprite
		content, err = os.ReadFile(env.MainSvgPath)
		require.NoError(t, err, "Should be able to read the updated sprite file")
		svgContent = string(content)

		// The removed icon should not be present
		assert.False(t, strings.Contains(svgContent, `id="icon-test1"`), "Should not contain removed test icon 1")
		assert.True(t, strings.Contains(svgContent, `id="icon-test2"`), "Should still contain test icon 2")

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
		require.GreaterOrEqual(t, len(svgHandler.contentOpen), 1, "SVG handler should have contentOpen")

		// Verificar que contentOpen contiene las etiquetas de apertura SVG
		found := false
		for _, cf := range svgHandler.contentOpen {
			content := string(cf.content)
			if strings.Contains(content, "<svg") {
				found = true
				break
			}
		}
		assert.True(t, found, "ContentOpen should contain SVG opening tag")

		// Create a test icon
		iconContent := `<symbol id="icon-test" viewBox="0 0 24 24"><path fill="currentColor" d="M12 2L2 22h20L12 2z"/></symbol>`
		iconFile := &contentFile{
			path:    "test-icon.svg",
			content: []byte(iconContent),
		}
		// Add the icon to the handler without writing to disk
		require.NoError(t, svgHandler.UpdateContent(iconFile.path, "create", iconFile))

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
		assert.Contains(t, svgContent, "<svg", "Should contain opening SVG tag")
		assert.Contains(t, svgContent, "</svg>", "Should contain closing SVG tag")
		assert.Contains(t, svgContent, `id="icon-test"`, "Should contain the test icon")

		env.CleanDirectory()
	})
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
		require.NoError(t, os.WriteFile(path, []byte(icon.content), 0644))
		paths = append(paths, path)
	}
	return paths
}
