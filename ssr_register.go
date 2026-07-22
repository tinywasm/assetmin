package assetmin

import (
	"fmt"
	"strings"

	"github.com/tinywasm/css"
	"github.com/tinywasm/js"
	"github.com/tinywasm/svg/sprite"
	"slices"
)

type rootCssProvider interface{ RootCSS() *css.Stylesheet }
type cssProvider interface{ RenderCSS() *css.Stylesheet }
type jsProvider interface{ RenderJS() []*js.Script }
type htmlProvider interface{ RenderHTML() string }
type svgProvider interface{ IconSvg() *sprite.Sprite }

// RegisterComponents registra structs que implementan las interfaces SSR.
func (c *AssetMin) RegisterComponents(providers ...any) error {
	for _, p := range providers {
		var css, html string
		var scripts []*js.Script
		var icons *sprite.Sprite

		if rp, ok := p.(rootCssProvider); ok {
			rootCSS := rp.RootCSS().String()
			if rootCSS != "" {
				c.mu.Lock()
				c.fromRoot = &rootCandidate{name: fmt.Sprintf("%T", p), css: rootCSS}
				c.mu.Unlock()
				c.resolveAndApplyRootCSS()
			}
		}

		if cp, ok := p.(cssProvider); ok {
			css = cp.RenderCSS().String()
		}
		if jp, ok := p.(jsProvider); ok {
			scripts = jp.RenderJS()
		}
		if hp, ok := p.(htmlProvider); ok {
			html = hp.RenderHTML()
		}
		if sp, ok := p.(svgProvider); ok {
			icons = sp.IconSvg()
		}

		name := fmt.Sprintf("%T", p)
		if err := c.UpdateSSRModule(name, css, scripts, html, icons); err != nil {
			return err
		}
	}
	return nil
}

// UpdateSSRModule inyecta o reemplaza los assets de un módulo por nombre en el slot por defecto (middle).
func (c *AssetMin) UpdateSSRModule(name string, css string, scripts []*js.Script, html string, icons *sprite.Sprite) error {
	return c.UpdateSSRModuleInSlot(name, css, scripts, html, icons, "middle")
}

// UpdateSSRModuleInSlot inyecta o reemplaza los assets de un módulo en el slot especificado.
func (c *AssetMin) UpdateSSRModuleInSlot(name string, css string, scripts []*js.Script, html string, icons *sprite.Sprite, slot string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.updateSSRModuleInSlot(name, css, scripts, html, icons, slot)
}

func validateStandaloneName(name string) error {
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid standalone JS name %q: must not contain '/' or '..'", name)
	}
	return nil
}

func (c *AssetMin) updateSSRModuleInSlot(name string, css string, scripts []*js.Script, html string, icons *sprite.Sprite, slot string) error {
	if css != "" {
		c.mainStyleCssHandler.UpdateContentInSlot(name, "write", &ContentFile{Path: name, Content: []byte(css)}, slot)
	}

	// Bundled JS
	var bundledJS string
	var currentStandalone []string

	for _, s := range scripts {
		if s.Name == "" {
			bundledJS += s.Content
		} else {
			if err := validateStandaloneName(s.Name); err != nil {
				return err
			}
			// Standalone JS
			if _, exists := c.standaloneJS[s.Name]; !exists {
				c.standaloneJS[s.Name] = newAssetFile(s.Name, "text/javascript", c.Config, nil)
				c.standaloneJS[s.Name].urlPath = "/" + s.Name
				c.allAssets[c.standaloneJS[s.Name].outputPath] = c.standaloneJS[s.Name]
			}
			standaloneKey := name + ":" + s.Name
			currentStandalone = append(currentStandalone, s.Name)
			c.standaloneJS[s.Name].UpdateContentInSlot(standaloneKey, "write", &ContentFile{Path: standaloneKey, Content: []byte(s.Content)}, slot)
		}
	}

	if bundledJS != "" {
		c.mainJsHandler.UpdateContentInSlot(name, "write", &ContentFile{Path: name, Content: []byte(bundledJS)}, slot)
	}

	// Orphan cleanup for standalone JS
	previousStandalone := c.standaloneOwners[name]
	for _, oldName := range previousStandalone {
		if !slices.Contains(currentStandalone, oldName) {
			if h, ok := c.standaloneJS[oldName]; ok {
				standaloneKey := name + ":" + oldName
				h.UpdateContentInSlot(standaloneKey, "remove", nil, slot)
				// If no more modules are providing content for this standalone file, we might want to remove it from allAssets
				// but since they are slot-based, we'd need to check all slots. For simplicity, we keep the handler but with empty content.
			}
		}
	}
	c.standaloneOwners[name] = currentStandalone

	if html != "" {
		c.indexHtmlHandler.UpdateContentInSlot(name, "write", &ContentFile{Path: name, Content: []byte(html)}, slot)
	}
	c.setModuleSprite(name, icons)
	return nil
}
