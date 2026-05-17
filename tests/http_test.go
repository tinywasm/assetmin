package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterRoutes(t *testing.T) {
	t.Run("serves assets from root", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)
		mux := http.NewServeMux()
		am.RegisterRoutes(mux)
		server := httptest.NewServer(mux)
		defer server.Close()

		// Add some content to trigger cache generation
		if err := am.NewFileEvent("test.js", ".js", setup.createTempFile("test.js", "var a=1;"), "create"); err != nil {
			t.Fatalf("Error processing JS creation: %v", err)
		}
		if err := am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{}"), "create"); err != nil {
			t.Fatalf("Error processing CSS creation: %v", err)
		}

		// Test index
		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("HTTP GET / failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /: expected status OK, got %v", resp.StatusCode)
		}
		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/html" {
			t.Errorf("GET /: expected content-type text/html, got %v", contentType)
		}

		// Test JS
		resp, err = http.Get(server.URL + "/script.js")
		if err != nil {
			t.Fatalf("HTTP GET /script.js failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /script.js: expected status OK, got %v", resp.StatusCode)
		}
		contentType = resp.Header.Get("Content-Type")
		if contentType != "text/javascript" {
			t.Errorf("GET /script.js: expected content-type text/javascript, got %v", contentType)
		}

		// Test CSS
		resp, err = http.Get(server.URL + "/style.css")
		if err != nil {
			t.Fatalf("HTTP GET /style.css failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /style.css: expected status OK, got %v", resp.StatusCode)
		}
		contentType = resp.Header.Get("Content-Type")
		if contentType != "text/css" {
			t.Errorf("GET /style.css: expected content-type text/css, got %v", contentType)
		}
	})

	t.Run("serves assets with prefix", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		setup.ac.AssetsURLPrefix = "/static/"
		am := assetmin.NewAssetMin(setup.ac)
		mux := http.NewServeMux()
		am.RegisterRoutes(mux)
		server := httptest.NewServer(mux)
		defer server.Close()

		if err := am.NewFileEvent("test.js", ".js", setup.createTempFile("test.js", "var b=2;"), "create"); err != nil {
			t.Fatalf("Error processing JS creation: %v", err)
		}

		// Test index (should still be at root)
		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("HTTP GET / failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /: expected status OK, got %v", resp.StatusCode)
		}

		// Test JS (with prefix)
		resp, err = http.Get(server.URL + "/static/script.js")
		if err != nil {
			t.Fatalf("HTTP GET /static/script.js failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /static/script.js: expected status OK, got %v", resp.StatusCode)
		}

		// Test JS (without prefix - should be handled by "/" and return HTML)
		resp, err = http.Get(server.URL + "/script.js")
		if err != nil {
			t.Fatalf("HTTP GET /script.js failed: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /script.js: expected status OK, got %v", resp.StatusCode)
		}
		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/html" {
			t.Errorf("GET /script.js: expected content-type text/html, got %v", contentType)
		}
	})

	t.Run("sprite is NOT exposed as route", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)
		mux := http.NewServeMux()
		am.RegisterRoutes(mux)
		server := httptest.NewServer(mux)
		defer server.Close()

		// Requesting icons.svg should return 404 or fall back to "/" (index.html)
		resp, err := http.Get(server.URL + "/icons.svg")
		if err != nil {
			t.Fatalf("HTTP GET /icons.svg failed: %v", err)
		}

		// Since "/" is registered as a catch-all in http.ServeMux (if we are not careful)
		// Or if assetmin registers "/" as indexHtmlHandler.
		// In http.go: mux.HandleFunc(c.indexHtmlHandler.GetURLPath(), c.serveAsset(c.indexHtmlHandler))
		// and c.indexHtmlHandler.urlPath = "/"
		// So http.ServeMux will treat "/" as a prefix if it's not a more specific match.
		// Thus /icons.svg will return index.html.

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status OK (fallback to index), got %v", resp.StatusCode)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType != "text/html" {
			t.Errorf("expected content-type text/html (index fallback), got %v", contentType)
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

	mux := http.NewServeMux()
	am.RegisterRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("HTTP GET / failed: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	resp.Body.Close()

	htmlContent := string(body)
	if !strings.Contains(htmlContent, "test-icon") {
		t.Error("index.html should contain injected icon ID")
	}
	if !strings.Contains(htmlContent, "<svg") {
		t.Error("index.html should contain <svg> tag for sprite")
	}
}

func TestWorks(t *testing.T) {
	t.Run("false does not write to disk", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)
		am.SetBuildOnDisk(false)

		err := am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{color:red}"), "create")
		if err != nil {
			t.Fatalf("Error processing CSS creation: %v", err)
		}

		// Check file does NOT exist
		_, err = os.Stat(filepath.Join(setup.outputDir, "style.css"))
		if !os.IsNotExist(err) {
			t.Errorf("File style.css should NOT exist when BuildOnDisk is false")
		}
	})

	t.Run("true writes to disk", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := assetmin.NewAssetMin(setup.ac)
		am.SetBuildOnDisk(true)

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
		mux := http.NewServeMux()
		am.RegisterRoutes(mux)
		server := httptest.NewServer(mux)
		defer server.Close()

		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("HTTP GET / failed: %v", err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		resp.Body.Close()

		htmlContent := string(body)
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

		mux := http.NewServeMux()
		am.RegisterRoutes(mux)
		server := httptest.NewServer(mux)
		defer server.Close()

		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("HTTP GET / failed: %v", err)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}
		resp.Body.Close()

		htmlContent := string(body)
		if !strings.Contains(htmlContent, "<div id='custom'>Injected</div>") {
			t.Error("index.html should contain injected content")
		}
	})
}
