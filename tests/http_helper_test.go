//go:build !wasm

package assetmin_test

import (
	"testing"

	"github.com/tinywasm/assetmin"
	"github.com/tinywasm/router/mock"
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

func newTestRouter(am *assetmin.AssetMin) *mock.Router {
	r := &mock.Router{}
	am.RegisterRoutes(r)
	return r
}

func (s *testSetup) createTempFile(name, content string) string {
	path := filepath.Join(s.outputDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		s.t.Fatalf("Failed to create temp file %s: %v", name, err)
	}
	return path
}
