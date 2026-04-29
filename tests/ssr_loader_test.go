package assetmin_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tinywasm/assetmin"
)

func TestSSRLoader(t *testing.T) {
	t.Run("LoadSSRModulesOrder", func(t *testing.T) {
		env := setupTestEnv("loader_order", t)
		am := env.AssetsHandler

		// Mock module directories
		rootModule := env.BaseDir
		domModule := filepath.Join(env.BaseDir, "vendor", "tinywasm", "dom")
		extModule := filepath.Join(env.BaseDir, "vendor", "other", "module")

		os.MkdirAll(domModule, 0755)
		os.MkdirAll(extModule, 0755)

		// Write ssr.go files
		os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte("package root\nfunc RenderCSS() string { return \".root{color:blue;}\" }"), 0644)
		os.WriteFile(filepath.Join(domModule, "ssr.go"), []byte("package dom\nfunc RenderCSS() string { return \".dom{color:red;}\" }"), 0644)
		os.WriteFile(filepath.Join(extModule, "ssr.go"), []byte("package ext\nfunc RenderCSS() string { return \".ext{color:green;}\" }"), 0644)

		am.RootDir = rootModule
		am.SetListModulesFn(func(root string) ([]string, error) {
			return []string{domModule, extModule, rootModule}, nil
		})

		// Since we now scan for imports, we need a .go file importing the external module
		mainGo := filepath.Join(rootModule, "main.go")
		os.WriteFile(mainGo, []byte("package main\nimport _ \"other/module\""), 0644)

		am.LoadSSRModules()
		am.WaitForSSRLoad(2 * time.Second)

		// Verify presence
		if !am.ContainsCSS(".dom") || !am.ContainsCSS(".ext") || !am.ContainsCSS(".root") {
			t.Error("Some CSS missing")
		}

		// Verify order via minified output
		css, _ := am.GetMinifiedCSS()
		cssStr := string(css)

		idxDom := strings.Index(cssStr, ".dom")
		idxExt := strings.Index(cssStr, ".ext")
		idxRoot := strings.Index(cssStr, ".root")

		if idxDom == -1 || idxExt == -1 || idxRoot == -1 {
			t.Fatalf("Missing CSS parts in bundle: %s", cssStr)
		}

		if !(idxDom < idxExt && idxExt < idxRoot) {
			t.Errorf("Wrong CSS order: dom=%d, ext=%d, root=%d", idxDom, idxExt, idxRoot)
		}
	})

	t.Run("ReloadSSRModuleHotReload", func(t *testing.T) {
		env := setupTestEnv("hot_reload", t)
		am := env.AssetsHandler

		moduleDir := filepath.Join(env.BaseDir, "mymodule")
		os.MkdirAll(moduleDir, 0755)

		ssrPath := filepath.Join(moduleDir, "ssr.go")
		os.WriteFile(ssrPath, []byte("package mypkg\nfunc RenderCSS() string { return \".old{}\" }"), 0644)

		if err := am.ReloadSSRModule(moduleDir); err != nil {
			t.Fatal(err)
		}
		if !am.ContainsCSS(".old{}") {
			t.Error("Initial CSS not found")
		}

		// Change file
		os.WriteFile(ssrPath, []byte("package mypkg\nfunc RenderCSS() string { return \".new{}\" }"), 0644)
		if err := am.ReloadSSRModule(moduleDir); err != nil {
			t.Fatal(err)
		}

		if am.ContainsCSS(".old{}") {
			t.Error("Old CSS still present after reload")
		}
		if !am.ContainsCSS(".new{}") {
			t.Error("New CSS not found after reload")
		}
	})

	t.Run("LoadIconsFromLocalRoot", func(t *testing.T) {
		env := setupTestEnv("local_icons", t)
		am := env.AssetsHandler

		rootModule := env.BaseDir
		os.WriteFile(filepath.Join(rootModule, "ssr.go"), []byte(`
package root
func IconSvg() map[string]string {
    return map[string]string{
        "local-icon": "<path d='M0 0l1 1'/>",
    }
}
`), 0644)

		am.RootDir = rootModule
		am.SetListModulesFn(func(root string) ([]string, error) {
			return []string{rootModule}, nil
		})

		am.LoadSSRModules()
		am.WaitForSSRLoad(2 * time.Second)

		if !am.HasIcon("local-icon") {
			t.Error("Icon from local root ssr.go not loaded")
		}
	})

	// TestWaitForSSRLoadNoRace verifica que ScheduleSSRLoad+WaitForSSRLoad no producen
	// data race cuando el goroutine interno aún no empezó al momento de llamar Wait.
	t.Run("WaitForSSRLoadNoRace", func(t *testing.T) {
		root := t.TempDir()
		// módulo mínimo sin imports para que LoadSSRModules termine rápido
		os.WriteFile(filepath.Join(root, "go.mod"), []byte("module testrace\ngo 1.21\n"), 0644)

		ac := &assetmin.Config{
			RootDir: root,
		}
		c := assetmin.NewAssetMin(ac)

		// Ejecutar múltiples veces para maximizar probabilidad de race en -race
		for i := 0; i < 20; i++ {
			c.ScheduleSSRLoad()
			c.WaitForSSRLoad(2 * time.Second)
		}
	})
}
