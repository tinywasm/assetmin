package assetmin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type testSetup struct {
	t         *testing.T
	outputDir string
	config    *Config
}

func newTestSetup(t *testing.T) *testSetup {
	outputDir, err := os.MkdirTemp("", "assetmin_test_")
	require.NoError(t, err)

	config := &Config{
		OutputDir: outputDir,
		GetRuntimeInitializerJS: func() (string, error) {
			return "", nil
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
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(s.t, err)
	return path
}
