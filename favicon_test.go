package assetmin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFaviconProcessing verifies that favicon.svg is processed independently
// from sprite icons and copied to the output folder
func TestFaviconProcessing(t *testing.T) {
	t.Run("favicon_svg_processed_to_output", func(t *testing.T) {
		env := setupTestEnv("favicon_svg_processed", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir()

		// Create favicon.svg in theme folder
		faviconContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<circle cx="50" cy="50" r="40" fill="#007acc"/>
	<text x="50" y="65" font-size="50" text-anchor="middle" fill="white">F</text>
</svg>`

		faviconPath := filepath.Join(env.BaseDir, "favicon.svg")
		require.NoError(t, os.WriteFile(faviconPath, []byte(faviconContent), 0644))

		// Process the favicon file with create event
		require.NoError(t, env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"))

		// Verify favicon was copied to output folder
		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		require.FileExists(t, outputFaviconPath, "Favicon should be copied to output folder")

		// Read output content
		content, err := os.ReadFile(outputFaviconPath)
		require.NoError(t, err)

		// Verify content is minified SVG
		svgContent := string(content)
		assert.Contains(t, svgContent, "<svg", "Should contain SVG tag")
		assert.Contains(t, svgContent, "circle", "Should contain circle element")
		assert.NotContains(t, svgContent, "sprite-icons", "Should not be wrapped as sprite")
		assert.NotContains(t, svgContent, "<defs>", "Should not contain sprite defs")

		env.CleanDirectory()
	})

	t.Run("favicon_separate_from_sprite_icons", func(t *testing.T) {
		env := setupTestEnv("favicon_separate_sprite", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir()

		// Create both favicon and sprite icon
		faviconContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<circle cx="50" cy="50" r="40" fill="red"/>
</svg>`

		iconContent := `<symbol id="icon-home" viewBox="0 0 24 24">
	<path fill="currentColor" d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"/>
</symbol>`

		// Create test icon directory
		iconsDir := filepath.Join(env.BaseDir, "icons")
		require.NoError(t, os.MkdirAll(iconsDir, 0755))

		faviconPath := filepath.Join(env.BaseDir, "favicon.svg")
		iconPath := filepath.Join(iconsDir, "home.svg")

		require.NoError(t, os.WriteFile(faviconPath, []byte(faviconContent), 0644))
		require.NoError(t, os.WriteFile(iconPath, []byte(iconContent), 0644))

		// Process both files
		require.NoError(t, env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"))
		require.NoError(t, env.AssetsHandler.NewFileEvent("home.svg", ".svg", iconPath, "create"))

		// Verify favicon output
		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		require.FileExists(t, outputFaviconPath)

		faviconOut, err := os.ReadFile(outputFaviconPath)
		require.NoError(t, err)
		faviconStr := string(faviconOut)

		// Favicon should be standalone SVG
		assert.Contains(t, faviconStr, "<svg", "Favicon should be SVG")
		assert.Contains(t, faviconStr, "circle", "Favicon should have its content")
		assert.NotContains(t, faviconStr, "icon-home", "Favicon should not contain sprite icons")

		// Verify sprite output
		outputSpritePath := filepath.Join(env.PublicDir, "sprite.svg")
		require.FileExists(t, outputSpritePath)

		spriteOut, err := os.ReadFile(outputSpritePath)
		require.NoError(t, err)
		spriteStr := string(spriteOut)

		// Sprite should contain the icon but not the favicon
		assert.Contains(t, spriteStr, "icon-home", "Sprite should contain icon")
		assert.Contains(t, spriteStr, "sprite-icons", "Sprite should have sprite class")
		assert.NotContains(t, spriteStr, "circle", "Sprite should not contain favicon content")

		env.CleanDirectory()
	})

	t.Run("favicon_update_event", func(t *testing.T) {
		env := setupTestEnv("favicon_update", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir()

		// Create initial favicon
		initialContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<circle cx="50" cy="50" r="40" fill="blue"/>
</svg>`

		faviconPath := filepath.Join(env.BaseDir, "favicon.svg")
		require.NoError(t, os.WriteFile(faviconPath, []byte(initialContent), 0644))
		require.NoError(t, env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"))

		// Update favicon content
		updatedContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<rect x="10" y="10" width="80" height="80" fill="green"/>
</svg>`

		require.NoError(t, os.WriteFile(faviconPath, []byte(updatedContent), 0644))
		require.NoError(t, env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "write"))

		// Verify updated output
		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		content, err := os.ReadFile(outputFaviconPath)
		require.NoError(t, err)

		svgContent := string(content)
		assert.Contains(t, svgContent, "rect", "Should contain updated rect element")
		assert.NotContains(t, svgContent, "circle", "Should not contain old circle element")

		env.CleanDirectory()
	})

	t.Run("favicon_delete_event", func(t *testing.T) {
		env := setupTestEnv("favicon_delete", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir()

		// Create favicon
		faviconContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<circle cx="50" cy="50" r="40" fill="orange"/>
</svg>`

		faviconPath := filepath.Join(env.BaseDir, "favicon.svg")
		require.NoError(t, os.WriteFile(faviconPath, []byte(faviconContent), 0644))
		require.NoError(t, env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"))

		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		require.FileExists(t, outputFaviconPath, "Favicon should exist before delete")

		// Delete favicon source file
		require.NoError(t, os.Remove(faviconPath))
		require.NoError(t, env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "remove"))

		// Verify output is empty or minimal after delete
		content, err := os.ReadFile(outputFaviconPath)
		require.NoError(t, err)

		// After deletion, the output should be minimal (no content in contentMiddle)
		svgContent := string(content)
		assert.NotContains(t, svgContent, "circle", "Should not contain deleted circle")

		env.CleanDirectory()
	})
}
