//go:build !wasm

package assetmin

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Module representa la salida de `go list -m -json all`
type Module struct {
	Path string
	Dir  string
	Main bool
}

// LoadSSRModules descubre todos los módulos e inyecta sus assets.
func (c *AssetMin) LoadSSRModules() error {
	c.ssrLoading.Add(1)
	defer c.ssrLoading.Done()

	var listFn = c.listModulesFn
	if listFn == nil {
		listFn = func(rootDir string) ([]string, error) {
			cmd := exec.Command("go", "list", "-m", "-json", "all")
			cmd.Dir = rootDir
			out, err := cmd.Output()
			if err != nil {
				return nil, err
			}

			var modules []Module
			dec := json.NewDecoder(strings.NewReader(string(out)))
			for dec.More() {
				var m Module
				if err := dec.Decode(&m); err != nil {
					return nil, err
				}
				modules = append(modules, m)
			}

			// Sort modules for deterministic order
			sort.Slice(modules, func(i, j int) bool {
				return modules[i].Path < modules[j].Path
			})

			var dirs []string
			for _, m := range modules {
				if m.Dir != "" {
					dirs = append(dirs, m.Dir)
				}
			}
			return dirs, nil
		}
	}

	dirs, err := listFn(c.RootDir)
	if err != nil {
		c.Logger("LoadSSRModules error:", err)
		return err
	}

	for _, dir := range dirs {
		assets, err := ExtractSSRAssets(dir)
		if err != nil {
			// No ssr.go found is fine
			continue
		}

		// Determine slot based on module path or special naming
		slot := "middle"
		moduleName := assets.ModuleName
		if strings.Contains(dir, "tinywasm/dom") {
			slot = "open"
		} else if isRootDir(dir, c.RootDir) {
			slot = "close"
		}

		c.UpdateSSRModuleInSlot(moduleName, assets.CSS, assets.JS, assets.HTML, assets.Icons, slot)
	}

	return nil
}

func isRootDir(dir, rootDir string) bool {
	if rootDir == "" {
		return false
	}
	absDir, _ := filepath.Abs(dir)
	absRoot, _ := filepath.Abs(rootDir)
	return absDir == absRoot
}

// ReloadSSRModule re-extrae e inyecta los assets de un único módulo por su directorio.
func (c *AssetMin) ReloadSSRModule(moduleDir string) error {
	assets, err := ExtractSSRAssets(moduleDir)
	if err != nil {
		return err
	}

	slot := "middle"
	if strings.Contains(moduleDir, "tinywasm/dom") {
		slot = "open"
	} else if isRootDir(moduleDir, c.RootDir) {
		slot = "close"
	}

	c.UpdateSSRModuleInSlot(assets.ModuleName, assets.CSS, assets.JS, assets.HTML, assets.Icons, slot)
	return nil
}

// WaitForSSRLoad espera a que LoadSSRModules termine, hasta el timeout dado.
func (c *AssetMin) WaitForSSRLoad(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		c.ssrLoading.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(timeout):
	}
}
