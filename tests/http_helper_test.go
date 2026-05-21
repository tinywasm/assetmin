package assetmin_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
)

type testSetup struct {
	t         *testing.T
	outputDir string
	ac        *assetmin.Config
}

func newTestSetup(t *testing.T) *testSetup {
	outputDir := t.TempDir()

	ac := &assetmin.Config{
		OutputDir: outputDir,
	}

	return &testSetup{
		t:         t,
		outputDir: outputDir,
		ac:        ac,
	}
}

func (s *testSetup) cleanup() {
	// t.TempDir handles cleanup
}

func newTestMux(am *assetmin.AssetMin) *http.ServeMux {
	mux := http.NewServeMux()
	am.RegisterRoutes(mux)
	return mux
}

func newTestServer(mux *http.ServeMux) *httptest.Server {
	return httptest.NewServer(mux)
}

func doGet(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(b)
}

func (s *testSetup) createTempFile(name, content string) string {
	path := filepath.Join(s.outputDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		s.t.Fatalf("Failed to create temp file %s: %v", name, err)
	}
	return path
}
