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
	domModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "dom")
	os.MkdirAll(domModule, 0755)

	os.WriteFile(filepath.Join(domModule, "ssr.go"), []byte(`package dom
func RootCSS() string { return ":root{--dom:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{domModule, rootModule}, nil
	})

	am.LoadSSRModules()
	am.WaitForSSRLoad(1e9)

	if !am.ContainsCSS("--dom:1") {
		t.Fatal("Initial dom root css not found")
	}

	// App gains RootCSS
	os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte(`package root
func RootCSS() string { return ":root{--app:1;}" }
`), 0644)

	if err := am.ReloadSSRModule(rootModule); err != nil {
		t.Fatal(err)
	}

	css, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(css), "--app:1") {
		t.Error("App root css should be present after reload")
	}
	if strings.Contains(string(css), "--dom:1") {
		t.Error("Dom root css should have been overridden after reload")
	}
}

func TestReload_AppLosesRootCSS(t *testing.T) {
	env := setupTestEnv("app_loses", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	domModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "dom")
	os.MkdirAll(domModule, 0755)

	os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte(`package root
func RootCSS() string { return ":root{--app:1;}" }
`), 0644)
	os.WriteFile(filepath.Join(domModule, "ssr.go"), []byte(`package dom
func RootCSS() string { return ":root{--dom:1;}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{domModule, rootModule}, nil
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

	css, _ := am.GetMinifiedCSS()
	if strings.Contains(string(css), "--app:1") {
		t.Error("App root css should be gone")
	}
	if !strings.Contains(string(css), "--dom:1") {
		t.Error("Should have fallen back to dom root css")
	}
}

func TestReload_ThirdPartyAddsRootCSS(t *testing.T) {
	env := setupTestEnv("third_adds", t)
	am := env.AssetsHandler

	rootModule := env.BaseDir
	domModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "dom")
	thirdModule := filepath.Join(env.BaseDir, "vendor", "other", "module")
	os.MkdirAll(domModule, 0755)
	os.MkdirAll(thirdModule, 0755)

	os.WriteFile(filepath.Join(domModule, "ssr.go"), []byte(`package dom
func RootCSS() string { return ":root{--dom:1;}" }
`), 0644)
	os.WriteFile(filepath.Join(thirdModule, "ssr.go"), []byte(`package third
func RenderCSS() string { return ".third{}" }
`), 0644)

	am.RootDir = rootModule
	am.SetListModulesFn(func(root string) ([]string, error) {
		return []string{domModule, thirdModule, rootModule}, nil
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

	css, _ := am.GetMinifiedCSS()
	if !strings.Contains(string(css), "--dom:1") {
		t.Error("Dom root css should still be present")
	}
	if strings.Contains(string(css), "--third:1") {
		t.Error("Third party root css should be ignored even on reload")
	}
}
