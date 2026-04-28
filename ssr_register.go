package assetmin

import "fmt"

type cssProvider interface{ RenderCSS() string }
type jsProvider interface{ RenderJS() string }
type htmlProvider interface{ RenderHTML() string }
type iconProvider interface{ IconSvg() map[string]string }

// RegisterComponents registra structs que implementan las interfaces SSR.
func (c *AssetMin) RegisterComponents(providers ...any) error {
	for _, p := range providers {
		var css, js, html string
		var icons map[string]string

		if cp, ok := p.(cssProvider); ok {
			css = cp.RenderCSS()
		}
		if jp, ok := p.(jsProvider); ok {
			js = jp.RenderJS()
		}
		if hp, ok := p.(htmlProvider); ok {
			html = hp.RenderHTML()
		}
		if ip, ok := p.(iconProvider); ok {
			icons = ip.IconSvg()
		}

		name := fmt.Sprintf("%T", p)
		c.UpdateSSRModule(name, css, js, html, icons)
	}
	return nil
}

// UpdateSSRModule inyecta o reemplaza los assets de un módulo por nombre en el slot por defecto (middle).
func (c *AssetMin) UpdateSSRModule(name string, css, js, html string, icons map[string]string) {
	c.UpdateSSRModuleInSlot(name, css, js, html, icons, "middle")
}

// UpdateSSRModuleInSlot inyecta o reemplaza los assets de un módulo en el slot especificado.
func (c *AssetMin) UpdateSSRModuleInSlot(name string, css, js, html string, icons map[string]string, slot string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.updateSSRModuleInSlot(name, css, js, html, icons, slot)
}

func (c *AssetMin) updateSSRModuleInSlot(name string, css, js, html string, icons map[string]string, slot string) {
	if css != "" {
		c.mainStyleCssHandler.UpdateContentInSlot(name, "write", &ContentFile{Path: name, Content: []byte(css)}, slot)
	}
	if js != "" {
		c.mainJsHandler.UpdateContentInSlot(name, "write", &ContentFile{Path: name, Content: []byte(js)}, slot)
	}
	if html != "" {
		c.indexHtmlHandler.UpdateContentInSlot(name, "write", &ContentFile{Path: name, Content: []byte(html)}, slot)
	}
	for id, svg := range icons {
		c.addIcon(id, svg)
	}
}
