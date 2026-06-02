package assetmin

import (
	"path/filepath"
	"time"

	"github.com/tinywasm/fmt"
)

// cssModulePath is the module path that provides the default `:root` theme
const cssModulePath = "tinywasm/css"

// LoadSSRModules descubre todos los módulos e inyecta sus assets (asíncrono).
func (c *AssetMin) LoadSSRModules() {
	c.ScheduleSSRLoad()
}

// ScheduleSSRLoad inicia la carga de módulos SSR en segundo plano de forma segura.
func (c *AssetMin) ScheduleSSRLoad() {
	c.ssrLoading.Add(1)
	go func() {
		defer c.ssrLoading.Done()
		c.mu.Lock()
		defer c.mu.Unlock()

		// 1) assets de texto/svg vía el extractor SSR inyectado:
		if c.ssrExtractor != nil {
			if all, err := c.ssrExtractor.ExtractAll(); err == nil {
				for _, a := range all {
					c.routeAssets(a, a.IsRoot, a.IsFramework)
				}
			} else {
				c.Logger("SSR ExtractAll error:", err)
			}
		}
		// 2) imágenes vía el ImageProcessor inyectado:
		if c.imageProcessor != nil {
			if err := c.imageProcessor.LoadImages(); err != nil {
				c.Logger("image load error:", err)
			}
		}

		c.resolveAndApplyRootCSS()
	}()
}

func (c *AssetMin) routeAssets(a *SSRAssets, isRoot, isFramework bool) {
	if isRoot {
		c.fromRoot = nil
	} else if isFramework {
		c.fromCss = nil
	}

	if a.RootCSS != "" {
		switch {
		case isRoot:
			c.fromRoot = &rootCandidate{name: a.ModuleName, css: a.RootCSS}
		case isFramework:
			c.fromCss = &rootCandidate{name: a.ModuleName, css: a.RootCSS}
		default:
			c.Logger("warning: module", a.ModuleName, "declares RootCSS() but only the root project or", cssModulePath, "may; ignoring")
		}
	}

	slot := "middle"
	if isRoot {
		slot = "close"
	}
	// RootCSS deliberately NOT passed here — it has its own slot resolution above.
	c.updateSSRModuleInSlot(a.ModuleName, a.CSS, a.JS, a.HTML, a.Icons, slot)
}

func (c *AssetMin) resolveAndApplyRootCSS() {
	var entries []*ContentFile
	if c.fromRoot != nil {
		entries = append(entries, &ContentFile{Path: c.fromRoot.name, Content: []byte(c.fromRoot.css)})
	} else if c.fromCss != nil {
		entries = append(entries, &ContentFile{Path: c.fromCss.name, Content: []byte(c.fromCss.css)})
	}

	c.mainStyleCssHandler.mu.Lock()
	c.mainStyleCssHandler.contentOpen = entries
	c.mainStyleCssHandler.cacheValid = false
	c.mainStyleCssHandler.mu.Unlock()
}

func (c *AssetMin) ReloadSSRModule(moduleDir string) error {
	if c.ssrExtractor == nil {
		return nil
	}

	a, err := c.ssrExtractor.ExtractModule(moduleDir)
	if err != nil || a == nil {
		return err
	}

	c.mu.Lock()
	isFramework := a.IsFramework || fmt.Contains(moduleDir, cssModulePath)
	isRoot := a.IsRoot || isRootDir(moduleDir, c.RootDir)

	c.routeAssets(a, isRoot, isFramework)

	if isFramework || isRoot || a.RootCSS != "" {
		c.resolveAndApplyRootCSS()
	}
	c.mu.Unlock()

	// Refresh assets only if they were actually changed/extracted
	if a.CSS != "" {
		c.refreshAsset(".css")
	}
	if len(a.JS) > 0 {
		c.refreshAsset(".js")
	}
	if a.HTML != "" {
		c.refreshAsset(".html")
	}
	if a.Icons != nil {
		c.refreshAsset(".svg")
	}

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

func isRootDir(dir, rootDir string) bool {
	if rootDir == "" {
		return false
	}
	absDir, _ := filepath.Abs(dir)
	absRoot, _ := filepath.Abs(rootDir)
	return absDir == absRoot
}
