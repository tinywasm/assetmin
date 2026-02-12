package assetmin

import (
	"os"
	"path/filepath"
	"testing"
)

// TestEnvironment holds all the paths and components needed for asset tests
type TestEnvironment struct {
	BaseDir       string
	ThemeDir      string
	PublicDir     string
	ModulesDir    string
	MainJsPath    string
	MainCssPath   string
	MainSvgPath   string
	MainHtmlPath  string
	AssetsHandler *AssetMin
	t             *testing.T
}

// CleanDirectory removes all content from the test directory but keeps the directory itself
func (env *TestEnvironment) CleanDirectory() {
	if _, err := os.Stat(env.BaseDir); err == nil {
		// env.t.Log("Cleaning test directory content...")
		// Remove content but keep the directory
		entries, err := os.ReadDir(env.BaseDir)
		if err == nil {
			for _, entry := range entries {
				entryPath := filepath.Join(env.BaseDir, entry.Name())
				os.RemoveAll(entryPath)
			}
		} else {
			env.t.Fatalf("Error reading directory: %v", err)
		}
	}
}

// setupTestEnv configures a minimal environment for testing AssetMin
// default write to disk is true, but can be set to false for testing purposes
// objects param can contain *contentFile instances which will be written to disk
// before the AssetMin handler is created
func setupTestEnv(testCase string, t *testing.T, objects ...any) *TestEnvironment {
	// Create real directory instead of a temporary one
	baseDir := filepath.Join(".", "test", testCase)
	themeDir := filepath.Join(baseDir, "web", "theme")
	publicDir := filepath.Join(baseDir, "web", "public")
	modulesDir := filepath.Join(baseDir, "modules")

	// Create asset configuration with logging using t.Log
	config := &Config{
		OutputDir: publicDir,
		GetSSRClientInitJS: func() (string, error) {
			return "console.log('init');", nil
		},
	}

	// \n// WebAssembly initialization code\nconst wasmMemory = new WebAssembly.Memory({initial:10, maximum:100});\n

	// Check if any of the objects is a contentFile and write it to disk
	// Also allow passing a func() (string, error) to override GetRuntimeInitializerJS
	for _, obj := range objects {
		if file, ok := obj.(*contentFile); ok {
			if err := file.WriteToDisk(); err != nil {
				t.Logf("Error writing contentFile to disk: %v", err)
			}
		}

		// add WebAssembly initialization code when a function is provided
		if funcInitJs, ok := obj.(func() (string, error)); ok {
			config.GetSSRClientInitJS = funcInitJs
		}
	}

	// Create asset handler.
	assetsHandler := NewAssetMin(config)

	// Create only the base directory if it doesn't exist
	err := os.MkdirAll(baseDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create base directory: %v", err)
	}

	return &TestEnvironment{
		BaseDir:       baseDir,
		ThemeDir:      themeDir,
		PublicDir:     publicDir,
		ModulesDir:    modulesDir,
		MainJsPath:    assetsHandler.mainJsHandler.outputPath,
		MainCssPath:   assetsHandler.mainStyleCssHandler.outputPath,
		MainSvgPath:   assetsHandler.spriteSvgHandler.outputPath,
		MainHtmlPath:  assetsHandler.indexHtmlHandler.outputPath,
		AssetsHandler: assetsHandler,
		t:             t,
	}
}

// CreateThemeDir creates the theme directory if it doesn't exist
func (env *TestEnvironment) CreateThemeDir() *TestEnvironment {
	err := os.MkdirAll(env.ThemeDir, 0755)
	if err != nil {
		env.t.Fatalf("Failed to create theme directory: %v", err)
	}
	return env
}

// CreatePublicDir creates the public directory if it doesn't exist
func (env *TestEnvironment) CreatePublicDir() *TestEnvironment {
	err := os.MkdirAll(env.PublicDir, 0755)
	if err != nil {
		env.t.Fatalf("Failed to create public directory: %v", err)
	}
	return env
}

// CreateModulesDir creates the modules directory if it doesn't exist
func (env *TestEnvironment) CreateModulesDir() *TestEnvironment {
	err := os.MkdirAll(env.ModulesDir, 0755)
	if err != nil {
		env.t.Fatalf("Failed to create modules directory: %v", err)
	}
	return env
}
