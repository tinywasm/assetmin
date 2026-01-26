package assetmin

import (
	"os"
	"path/filepath"
	"testing"
)

type testSetup struct {
	t         *testing.T
	outputDir string
	config    *Config
}

func newTestSetup(t *testing.T) *testSetup {
	outputDir, err := os.MkdirTemp("", "assetmin_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config := &Config{
		OutputDir: outputDir,
		GetSSRClientInitJS: func() (string, error) {
			return "console.log('init');", nil
		},
	}

	return &testSetup{
		t:         t,
		outputDir: outputDir,
		config:    config,
	}
}

func (s *testSetup) cleanup() {
	os.RemoveAll(s.outputDir)
}

func (s *testSetup) createTempFile(name, content string) string {
	path := filepath.Join(s.outputDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		s.t.Fatalf("Failed to write temp file %s: %v", path, err)
	}
	return path
}
