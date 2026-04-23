package assetmin_test

import (
	"github.com/tinywasm/assetmin"
	"os"
	"path/filepath"
	"testing"
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
		GetSSRClientInitJS: func() (string, error) {
			return "", nil
		},
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

func (s *testSetup) createTempFile(name, content string) string {
	path := filepath.Join(s.outputDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		s.t.Fatalf("Failed to create temp file %s: %v", name, err)
	}
	return path
}
