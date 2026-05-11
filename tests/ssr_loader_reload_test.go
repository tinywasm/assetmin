package assetmin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReload_AppGainsRootCSS(t *testing.T) {
	env := setupTestEnv("app_gains", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	cssModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "css")
	os.MkdirAll(cssModule, 0755)

	os.WriteFile(filepath.Join(cssModule, "ssr.go"), []byte(`package css
func RootCSS() string { return ":root{--css:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{cssModule, rootModule}, nil
	})

	am.LoadSSRModules()
	am.WaitForSSRLoad(1e9)

	if !am.ContainsCSS("--css:1") {
		t.Fatal("Initial framework css tokens not found")
	}

	// App gains RootCSS override
	os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte(`package root
func RootCSS() string { return ":root{--app:1;}" }
`), 0644)

	if err := am.ReloadSSRModule(rootModule); err != nil {
		t.Fatal(err)
	}

	output, _ := am.GetMinifiedCSS()
	// Both must be present: framework tokens + app override (cascade resolves conflicts)
	if !strings.Contains(string(output), "--css:1") {
		t.Error("Framework css tokens should remain after app override")
	}
	if !strings.Contains(string(output), "--app:1") {
		t.Error("App root css should be present after reload")
	}
}

func TestReload_AppLosesRootCSS(t *testing.T) {
	env := setupTestEnv("app_loses", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	cssModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "css")
	os.MkdirAll(cssModule, 0755)

	os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte(`package root
func RootCSS() string { return ":root{--app:1;}" }
`), 0644)
	os.WriteFile(filepath.Join(cssModule, "ssr.go"), []byte(`package css
func RootCSS() string { return ":root{--css:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{cssModule, rootModule}, nil
	})

	am.LoadSSRModules()
	am.WaitForSSRLoad(1e9)

	if !am.ContainsCSS("--app:1") {
		t.Fatal("Initial app root css not found")
	}

	// App loses RootCSS
	os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte(`package root
func RenderCSS() string { return ".other{}" }
`), 0644)

	if err := am.ReloadSSRModule(rootModule); err != nil {
		t.Fatal(err)
	}

	output, _ := am.GetMinifiedCSS()
	if strings.Contains(string(output), "--app:1") {
		t.Error("App root css should be gone")
	}
	if !strings.Contains(string(output), "--css:1") {
		t.Error("Framework css tokens should remain when app has no RootCSS")
	}
}

func TestReload_ThirdPartyAddsRootCSS(t *testing.T) {
	env := setupTestEnv("third_adds", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	cssModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "css")
	thirdModule := filepath.Join(env.BaseDir, "vendor", "other", "module")
	os.MkdirAll(cssModule, 0755)
	os.MkdirAll(thirdModule, 0755)

	os.WriteFile(filepath.Join(cssModule, "ssr.go"), []byte(`package css
func RootCSS() string { return ":root{--css:1;}" }
`), 0644)
	os.WriteFile(filepath.Join(thirdModule, "ssr.go"), []byte(`package third
func RenderCSS() string { return ".third{}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{cssModule, thirdModule, rootModule}, nil
	})

	am.LoadSSRModules()
	am.WaitForSSRLoad(1e9)

	// Third party adds RootCSS
	os.WriteFile(filepath.Join(thirdModule, "ssr.go"), []byte(`package third
func RootCSS() string { return ":root{--third:1;}" }
`), 0644)

	if err := am.ReloadSSRModule(thirdModule); err != nil {
		t.Fatal(err)
	}

	output, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(output), "--css:1") {
		t.Error("Framework css tokens should still be present")
	}
	if strings.Contains(string(output), "--third:1") {
		t.Error("Third party root css should be ignored even on reload")
	}
}
