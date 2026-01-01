package assetmin

import (
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

		am := NewAssetMin(setup.config)
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
		if contentType := resp.Header.Get("Content-Type"); contentType != "text/html" {
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
		if contentType := resp.Header.Get("Content-Type"); contentType != "text/javascript" {
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
		if contentType := resp.Header.Get("Content-Type"); contentType != "text/css" {
			t.Errorf("GET /style.css: expected content-type text/css, got %v", contentType)
		}
	})

	t.Run("serves assets with prefix", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		setup.config.AssetsURLPrefix = "/static/"
		am := NewAssetMin(setup.config)
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
		if contentType := resp.Header.Get("Content-Type"); contentType != "text/html" {
			t.Errorf("GET /script.js: expected content-type text/html, got %v", contentType)
		}
	})
}

func TestWorks(t *testing.T) {
	t.Run("false does not write to disk", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := NewAssetMin(setup.config)
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

		am := NewAssetMin(setup.config)
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

		setup.config.AssetsURLPrefix = "/assets"
		am := NewAssetMin(setup.config)
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
}
