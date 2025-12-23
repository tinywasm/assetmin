package assetmin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitialRegistration(t *testing.T) {
	env := setupTestEnv("initial_registration", t)
	defer env.CleanDirectory()

	// Create test files
	file1Path := filepath.Join(env.BaseDir, "script1.js")
	file2Path := filepath.Join(env.BaseDir, "script2.js")
	require.NoError(t, os.WriteFile(file1Path, []byte("console.log('File 1');"), 0644))
	require.NoError(t, os.WriteFile(file2Path, []byte("console.log('File 2');"), 0644))

	// Process in false
	env.AssetsHandler.SetBuildOnDisk(false)
	require.NoError(t, env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "create"))
	require.NoError(t, env.AssetsHandler.NewFileEvent("script2.js", ".js", file2Path, "create"))

	// Verify no file is written
	_, err := os.Stat(env.MainJsPath)
	require.True(t, os.IsNotExist(err), "File should not be written in false")

	// Switch to true and trigger a write
	env.AssetsHandler.SetBuildOnDisk(true)
	require.NoError(t, env.AssetsHandler.NewFileEvent("script1.js", ".js", file1Path, "write"))

	// Verify file is written with all content
	require.FileExists(t, env.MainJsPath, "File should be written in true")
	content, err := os.ReadFile(env.MainJsPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "File 1")
	require.Contains(t, string(content), "File 2")
}
