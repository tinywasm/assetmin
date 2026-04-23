package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFaviconProcessing(t *testing.T) {
	t.Run("favicon_svg_processed_to_output", func(t *testing.T) {
		env := setupTestEnv("favicon_svg_processed", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir()

		faviconContent := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="#007acc"/></svg>`
		faviconPath := filepath.Join(env.BaseDir, "favicon.svg")
		os.WriteFile(faviconPath, []byte(faviconContent), 0644)

		if err := env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create"); err != nil {
			t.Fatalf("Error: %v", err)
		}

		outputFaviconPath := filepath.Join(env.PublicDir, "favicon.svg")
		if _, err := os.Stat(outputFaviconPath); os.IsNotExist(err) {
			t.Fatalf("Favicon NOT found at %s", outputFaviconPath)
		}
	})

	t.Run("favicon_separate_from_sprite_icons", func(t *testing.T) {
		env := setupTestEnv("favicon_separate", t)
		env.AssetsHandler.SetBuildOnDisk(true)
		env.CreatePublicDir()

		faviconPath := filepath.Join(env.BaseDir, "favicon.svg")
		os.WriteFile(faviconPath, []byte(`<svg id="fav"><circle/></svg>`), 0644)

		iconPath := filepath.Join(env.BaseDir, "home.svg")
		os.WriteFile(iconPath, []byte(`<svg id="home-svg"><path/></svg>`), 0644)

		env.AssetsHandler.NewFileEvent("favicon.svg", ".svg", faviconPath, "create")
		env.AssetsHandler.NewFileEvent("home.svg", ".svg", iconPath, "create")

		// Favicon check
		favOut, _ := os.ReadFile(filepath.Join(env.PublicDir, "favicon.svg"))
		if !strings.Contains(string(favOut), "fav") {
			t.Error("Favicon output missing its content")
		}

		// Sprite check
		if !env.AssetsHandler.ContainsSVG("home") {
			t.Error("Sprite bundle missing 'home' icon")
		}
	})
}

func TestFaviconCacheHeaders(t *testing.T) {
	setup := newTestSetup(t)
	setup.ac.DevMode = true
	am := assetmin.NewAssetMin(setup.ac)

	faviconPath := setup.createTempFile("favicon.svg", `<svg xmlns="http://www.w3.org/2000/svg"><circle cx="50" cy="50" r="40"/></svg>`)
	am.NewFileEvent("favicon.svg", ".svg", faviconPath, "create")

	mux := http.NewServeMux()
	am.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, _ := http.Get(server.URL + "/favicon.svg")
	cc := resp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "max-age") {
		t.Errorf("Expected max-age in Cache-Control, got: %q", cc)
	}
}
