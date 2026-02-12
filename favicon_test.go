package assetmin

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		if err := os.WriteFile(faviconPath, []byte(faviconContent), 0644); err != nil {
			t.Fatalf("Failed to write favicon file: %v", err)
		}

		// Process the favicon file with create event
		if err := env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"); err != nil {
			t.Fatalf("Error processing favicon event: %v", err)
		}

		// Verify favicon was copied to output folder
		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		if _, err := os.Stat(outputFaviconPath); os.IsNotExist(err) {
			t.Fatalf("Favicon should be copied to output folder, but was not found at %s", outputFaviconPath)
		}

		// Read output content
		content, err := os.ReadFile(outputFaviconPath)
		if err != nil {
			t.Fatalf("Failed to read output favicon: %v", err)
		}

		// Verify content is minified SVG
		svgContent := string(content)
		if !strings.Contains(svgContent, "<svg") {
			t.Errorf("Should contain SVG tag")
		}
		if !strings.Contains(svgContent, "circle") {
			t.Errorf("Should contain circle element")
		}
		if strings.Contains(svgContent, "sprite-icons") {
			t.Errorf("Should not be wrapped as sprite")
		}
		if strings.Contains(svgContent, "<defs>") {
			t.Errorf("Should not contain sprite defs")
		}

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
		if err := os.MkdirAll(iconsDir, 0755); err != nil {
			t.Fatalf("Failed to create icons directory: %v", err)
		}

		faviconPath := filepath.Join(env.BaseDir, "favicon.svg")
		iconPath := filepath.Join(iconsDir, "home.svg")

		if err := os.WriteFile(faviconPath, []byte(faviconContent), 0644); err != nil {
			t.Fatalf("Failed to write favicon file: %v", err)
		}
		if err := os.WriteFile(iconPath, []byte(iconContent), 0644); err != nil {
			t.Fatalf("Failed to write icon file: %v", err)
		}

		// Process both files
		if err := env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"); err != nil {
			t.Fatalf("Error processing favicon event: %v", err)
		}
		if err := env.AssetsHandler.NewFileEvent("home.svg", ".svg", iconPath, "create"); err != nil {
			t.Fatalf("Error processing icon event: %v", err)
		}

		// Verify favicon output
		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		if _, err := os.Stat(outputFaviconPath); os.IsNotExist(err) {
			t.Fatalf("Favicon output should exist at %s", outputFaviconPath)
		}

		faviconOut, err := os.ReadFile(outputFaviconPath)
		if err != nil {
			t.Fatalf("Failed to read output favicon: %v", err)
		}
		faviconStr := string(faviconOut)

		// Favicon should be standalone SVG
		if !strings.Contains(faviconStr, "<svg") {
			t.Errorf("Favicon should be SVG")
		}
		if !strings.Contains(faviconStr, "circle") {
			t.Errorf("Favicon should have its content")
		}
		if strings.Contains(faviconStr, "icon-home") {
			t.Errorf("Favicon should not contain sprite icons")
		}

		// Verify sprite output
		outputSpritePath := filepath.Join(env.PublicDir, "icons.svg")
		if _, err := os.Stat(outputSpritePath); os.IsNotExist(err) {
			t.Fatalf("Sprite output should exist at %s", outputSpritePath)
		}

		spriteOut, err := os.ReadFile(outputSpritePath)
		if err != nil {
			t.Fatalf("Failed to read output sprite: %v", err)
		}
		spriteStr := string(spriteOut)

		// Sprite should contain the icon but not the favicon
		if !strings.Contains(spriteStr, "icon-home") {
			t.Errorf("Sprite should contain icon")
		}
		if !strings.Contains(spriteStr, "sprite-icons") {
			t.Errorf("Sprite should have sprite class")
		}
		if strings.Contains(spriteStr, "circle") {
			t.Errorf("Sprite should not contain favicon content")
		}

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
		if err := os.WriteFile(faviconPath, []byte(initialContent), 0644); err != nil {
			t.Fatalf("Failed to write initial favicon: %v", err)
		}
		if err := env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"); err != nil {
			t.Fatalf("Error processing creation event: %v", err)
		}

		// Update favicon content
		updatedContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<rect x="10" y="10" width="80" height="80" fill="green"/>
</svg>`

		if err := os.WriteFile(faviconPath, []byte(updatedContent), 0644); err != nil {
			t.Fatalf("Failed to update favicon: %v", err)
		}
		if err := env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "write"); err != nil {
			t.Fatalf("Error processing write event: %v", err)
		}

		// Verify updated output
		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		content, err := os.ReadFile(outputFaviconPath)
		if err != nil {
			t.Fatalf("Failed to read output favicon: %v", err)
		}

		svgContent := string(content)
		if !strings.Contains(svgContent, "rect") {
			t.Errorf("Should contain updated rect element")
		}
		if strings.Contains(svgContent, "circle") {
			t.Errorf("Should not contain old circle element")
		}

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
		if err := os.WriteFile(faviconPath, []byte(faviconContent), 0644); err != nil {
			t.Fatalf("Failed to write favicon: %v", err)
		}
		if err := env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"); err != nil {
			t.Fatalf("Error processing creation event: %v", err)
		}

		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		if _, err := os.Stat(outputFaviconPath); os.IsNotExist(err) {
			t.Fatalf("Favicon should exist before delete at %s", outputFaviconPath)
		}

		// Delete favicon source file
		if err := os.Remove(faviconPath); err != nil {
			t.Fatalf("Failed to remove favicon source: %v", err)
		}
		if err := env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "remove"); err != nil {
			t.Fatalf("Error processing removal event: %v", err)
		}

		// Verify output is empty or minimal after delete
		content, err := os.ReadFile(outputFaviconPath)
		if err != nil {
			t.Fatalf("Failed to read output favicon: %v", err)
		}

		// After deletion, the output should be minimal (no content in contentMiddle)
		svgContent := string(content)
		if strings.Contains(svgContent, "circle") {
			t.Errorf("Should not contain deleted circle")
		}

		env.CleanDirectory()
	})
}

func TestFaviconCacheHeaders(t *testing.T) {
	t.Run("favicon_immutable_in_dev_mode", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		setup.config.DevMode = true
		am := NewAssetMin(setup.config)

		// Add favicon content
		faviconPath := setup.createTempFile("favicon.svg", `<svg xmlns="http://www.w3.org/2000/svg"><circle cx="50" cy="50" r="40"/></svg>`)
		if err := am.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"); err != nil {
			t.Fatalf("Error processing favicon event: %v", err)
		}

		mux := http.NewServeMux()
		am.RegisterRoutes(mux)
		server := httptest.NewServer(mux)
		defer server.Close()

		// favicon URL without prefix = "/favicon.svg"
		resp, err := http.Get(server.URL + "/favicon.svg")
		if err != nil {
			t.Fatalf("HTTP GET favicon failed: %v", err)
		}
		defer resp.Body.Close()

		cc := resp.Header.Get("Cache-Control")
		if strings.Contains(cc, "no-store") || strings.Contains(cc, "no-cache") {
			t.Errorf("Favicon should NOT have no-cache in DevMode, got: %q", cc)
		}
		if !strings.Contains(cc, "max-age") && !strings.Contains(cc, "immutable") {
			t.Errorf("Favicon should have immutable/max-age cache, got: %q", cc)
		}
	})
}
