//go:build !wasm

package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"github.com/tinywasm/router/mock"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterRoutes(t *testing.T) {
	t.Run("registers asset routes", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)

		// Add some content to trigger cache generation
		if err := am.NewFileEvent("test.js", ".js", setup.createTempFile("test.js", "var a=1;"), "create"); err != nil {
			t.Fatalf("Error processing JS creation: %v", err)
		}
		if err := am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{}"), "create"); err != nil {
			t.Fatalf("Error processing CSS creation: %v", err)
		}

		r := newTestRouter(am)
		routes := r.Routes()

		// Verify routes are registered
		if len(routes) == 0 {
			t.Fatal("Expected routes to be registered")
		}

		routePaths := make(map[string]bool)
		for _, route := range routes {
			routePaths[route.Path] = true
		}

		// Check key routes exist
		if !routePaths["/"] {
			t.Error("index route (/) not registered")
		}
		if !routePaths["/style.css"] {
			t.Error("CSS route not registered")
		}
		if !routePaths["/script.js"] {
			t.Error("JS route not registered")
		}
	})

	t.Run("registers assets with prefix", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		setup.ac.AssetsURLPrefix = "/static/"
		am := assetmin.NewAssetMin(setup.ac)

		if err := am.NewFileEvent("test.js", ".js", setup.createTempFile("test.js", "var b=2;"), "create"); err != nil {
			t.Fatalf("Error processing JS creation: %v", err)
		}

		r := newTestRouter(am)
		routes := r.Routes()

		routePaths := make(map[string]bool)
		for _, route := range routes {
			routePaths[route.Path] = true
		}

		// Index should still be at root
		if !routePaths["/"] {
			t.Error("index route (/) not registered")
		}

		// Assets should be under /static/
		if !routePaths["/static/script.js"] {
			t.Error("/static/script.js route not registered")
		}
	})

	t.Run("sprite is NOT exposed as route", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)

		r := newTestRouter(am)
		routes := r.Routes()

		routePaths := make(map[string]bool)
		for _, route := range routes {
			routePaths[route.Path] = true
		}

		// Sprite should not have its own route
		if routePaths["/icons.svg"] {
			t.Error("sprite (/icons.svg) should not be exposed as route")
		}
	})
}

func TestSpriteInjectedInHTML(t *testing.T) {
	setup := newTestSetup(t)
	defer setup.cleanup()

	am := assetmin.NewAssetMin(setup.ac)

	// Inject an icon
	err := am.InjectSpriteIcon("test-icon", "<path d='M0 0h1'/>")
	if err != nil {
		t.Fatalf("InjectSpriteIcon failed: %v", err)
	}

	r := newTestRouter(am)

	// Invoke the index handler with mock context
	ctx := &mock.Context{
		InPath:   "/",
		InMethod: "GET",
	}
	r.Invoke("GET", "/", ctx)

	htmlContent := string(ctx.ResponseBody())
	if !strings.Contains(htmlContent, "test-icon") {
		t.Error("index.html should contain injected icon ID")
	}
	if !strings.Contains(htmlContent, "<svg") {
		t.Error("index.html should contain <svg> tag for sprite")
	}
}

func TestWorks(t *testing.T) {
	t.Run("default does not write to disk", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)

		err := am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{color:red}"), "create")
		if err != nil {
			t.Fatalf("Error processing CSS creation: %v", err)
		}

		// Check file does NOT exist
		_, err = os.Stat(filepath.Join(setup.outputDir, "style.css"))
		if !os.IsNotExist(err) {
			t.Errorf("File style.css should NOT exist when FlushToDisk has not been called")
		}
	})

	t.Run("FlushToDisk enables disk mirroring", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)
		if err := am.FlushToDisk(); err != nil {
			t.Fatalf("FlushToDisk: %v", err)
		}

		err := am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{color:red}"), "create")
		if err != nil {
			t.Fatalf("Error processing CSS creation: %v", err)
		}

		// Check file EXISTS
		content, err := os.ReadFile(filepath.Join(setup.outputDir, "style.css"))
		if err != nil {
			t.Fatalf("Failed to read output CSS: %v", err)
		}
		if string(content) != "body{color:red}" {
			t.Errorf("File style.css: expected body{color:red}, got %q", string(content))
		}
	})

	t.Run("HTML link and script tags respect URL prefix", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		setup.ac.AssetsURLPrefix = "/assets"
		am := assetmin.NewAssetMin(setup.ac)

		r := newTestRouter(am)

		// Invoke the index handler
		ctx := &mock.Context{
			InPath:   "/",
			InMethod: "GET",
		}
		r.Invoke("GET", "/", ctx)

		htmlContent := string(ctx.ResponseBody())
		if !strings.Contains(htmlContent, `href="/assets/style.css"`) {
			t.Errorf("HTML should contain href=\"/assets/style.css\"")
		}
		if !strings.Contains(htmlContent, `src="/assets/script.js"`) {
			t.Errorf("HTML should contain src=\"/assets/script.js\"")
		}
	})

	t.Run("InjectBodyContent is served in index.html", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)

		am.InjectHTML("<div id='custom'>Injected</div>")

		r := newTestRouter(am)

		// Invoke the index handler
		ctx := &mock.Context{
			InPath:   "/",
			InMethod: "GET",
		}
		r.Invoke("GET", "/", ctx)

		htmlContent := string(ctx.ResponseBody())
		if !strings.Contains(htmlContent, "<div id='custom'>Injected</div>") {
			t.Error("index.html should contain injected content")
		}
	})
}
