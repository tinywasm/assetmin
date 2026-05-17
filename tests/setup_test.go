package assetmin_test

import (
	"github.com/tinywasm/assetmin"
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
	AssetsHandler *assetmin.AssetMin
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
// objects param can contain *assetmin.ContentFile instances which will be written to disk
// before the AssetMin handler is created
func setupTestEnv(testCase string, t *testing.T, objects ...any) *TestEnvironment {
	// Create real directory instead of a temporary one
	baseDir := t.TempDir()
	themeDir := filepath.Join(baseDir, "web", "theme")
	publicDir := filepath.Join(baseDir, "web", "public")
	modulesDir := filepath.Join(baseDir, "modules")

	// Create asset configuration with logging using t.Log
	ac := &assetmin.Config{
		OutputDir: publicDir,
		GetSSRClientInitJS: func() (string, error) {
			return "console.log('init');", nil
		},
	}

	// Check if any of the objects is a ContentFile and write it to disk
	// Also allow passing a func() (string, error) to override GetRuntimeInitializerJS
	for _, obj := range objects {
		if file, ok := obj.(*assetmin.ContentFile); ok {
			if err := file.WriteToDisk(); err != nil {
				t.Logf("Error writing ContentFile to disk: %v", err)
			}
		}

		// add WebAssembly initialization code when a function is provided
		if funcInitJs, ok := obj.(func() (string, error)); ok {
			ac.GetSSRClientInitJS = funcInitJs
		}
	}

	// Create asset handler.
	assetsHandler := assetmin.NewAssetMin(ac)

	return &TestEnvironment{
		BaseDir:       baseDir,
		ThemeDir:      themeDir,
		PublicDir:     publicDir,
		ModulesDir:    modulesDir,
		MainJsPath:    assetsHandler.GetMainJsPath(),
		MainCssPath:   assetsHandler.GetMainCssPath(),
		MainSvgPath:   assetsHandler.GetMainSvgPath(),
		MainHtmlPath:  assetsHandler.GetMainHtmlPath(),
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

// withModules inyecta directorios de módulos sin ejecutar go list
func (env *TestEnvironment) withModules(dirs ...string) *TestEnvironment {
	env.AssetsHandler.SetListModulesFn(func(rootDir string) ([]string, error) {
		return dirs, nil
	})
	return env
}

// writeModuleSSR crea moduleDir/ssr.go con RenderCSS y/o RenderJS mínimos
func (env *TestEnvironment) writeModuleSSR(name, css, js string) string {
	moduleDir := filepath.Join(env.BaseDir, "modules", name)
	os.MkdirAll(moduleDir, 0755)

	content := "package " + name + "\n\n"
	if css != "" {
		content += "func RenderCSS() string { return `" + css + "` }\n"
	}
	if js != "" {
		content += "func RenderJS() string { return `" + js + "` }\n"
	}

	os.WriteFile(filepath.Join(moduleDir, "ssr.go"), []byte(content), 0644)
	return moduleDir
}

// assertContainsCSS / assertNotContainsCSS — usan ContainsCSS() público
func (env *TestEnvironment) AssertContainsCSS(substr string) {
	if !env.AssetsHandler.ContainsCSS(substr) {
		env.t.Errorf("CSS missing substring: %s", substr)
	}
}

func (env *TestEnvironment) AssertNotContainsCSS(substr string) {
	if env.AssetsHandler.ContainsCSS(substr) {
		env.t.Errorf("CSS should NOT contain substring: %s", substr)
	}
}

func (env *TestEnvironment) AssertContainsJS(substr string) {
	if !env.AssetsHandler.ContainsJS(substr) {
		env.t.Errorf("JS missing substring: %s", substr)
	}
}

func (env *TestEnvironment) AssertHasIcon(id string) {
	if !env.AssetsHandler.HasIcon(id) {
		env.t.Errorf("Icon missing: %s", id)
	}
}
