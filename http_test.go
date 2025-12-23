package assetmin

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.NoError(t, am.NewFileEvent("test.js", ".js", setup.createTempFile("test.js", "var a=1;"), "create"))
		assert.NoError(t, am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{}"), "create"))

		// Test index
		resp, err := http.Get(server.URL + "/")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html", resp.Header.Get("Content-Type"))

		// Test JS
		resp, err = http.Get(server.URL + "/script.js")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/javascript", resp.Header.Get("Content-Type"))

		// Test CSS
		resp, err = http.Get(server.URL + "/style.css")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/css", resp.Header.Get("Content-Type"))
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

		assert.NoError(t, am.NewFileEvent("test.js", ".js", setup.createTempFile("test.js", "var b=2;"), "create"))

		// Test index (should still be at root)
		resp, err := http.Get(server.URL + "/")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test JS (with prefix)
		resp, err = http.Get(server.URL + "/static/script.js")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Test JS (without prefix - should be handled by "/" and return HTML)
		resp, err = http.Get(server.URL + "/script.js")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "text/html", resp.Header.Get("Content-Type"))
	})
}

func TestWorks(t *testing.T) {
	t.Run("false does not write to disk", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := NewAssetMin(setup.config)
		am.SetBuildOnDisk(false)

		err := am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{color:red}"), "create")
		assert.NoError(t, err)

		// Check file does NOT exist
		_, err = os.Stat(filepath.Join(setup.outputDir, "style.css"))
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("true writes to disk", func(t *testing.T) {
		setup := newTestSetup(t)
		defer setup.cleanup()

		am := NewAssetMin(setup.config)
		am.SetBuildOnDisk(true)

		err := am.NewFileEvent("test.css", ".css", setup.createTempFile("test.css", "body{color:red}"), "create")
		assert.NoError(t, err)

		// Check file EXISTS
		content, err := os.ReadFile(filepath.Join(setup.outputDir, "style.css"))
		assert.NoError(t, err)
		assert.Equal(t, "body{color:red}", string(content))
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
		assert.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		resp.Body.Close()

		htmlContent := string(body)
		assert.True(t, strings.Contains(htmlContent, `href="/assets/style.css"`))
		assert.True(t, strings.Contains(htmlContent, `src="/assets/script.js"`))
	})
}
