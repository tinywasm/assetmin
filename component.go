package assetmin

import "fmt"

// Interfaces for component asset extraction

// CSSProvider indicates a component can render CSS.
type CSSProvider interface {
	RenderCSS() string
}

// JSProvider indicates a component can render JS.
type JSProvider interface {
	RenderJS() string
}

// IconProvider indicates a component can provide SVG icons.
type IconProvider interface {
	IconSvg() []map[string]string // Each map: {"id": "...", "svg": "<svg>...</svg>"}
}

// AccessLevel is used to check permission for SSR injection.
type AccessLevel interface {
	AllowedRoles(action byte) []byte
}

// AddCSS appends CSS content from providers to the bundle
func (c *AssetMin) AddCSS(providers ...CSSProvider) {
	if len(providers) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range providers {
		name := fmt.Sprintf("%T", p)
		content := p.RenderCSS()
		if content == "" {
			continue
		}

		c.mainStyleCssHandler.contentMiddle = append(
			c.mainStyleCssHandler.contentMiddle,
			&contentFile{path: name + ".css", content: []byte(content)},
		)
	}
	c.mainStyleCssHandler.cacheValid = false
}

// AddJS appends JS content from providers to the bundle
func (c *AssetMin) AddJS(providers ...JSProvider) {
	if len(providers) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range providers {
		name := fmt.Sprintf("%T", p)
		content := p.RenderJS()
		if content == "" {
			continue
		}

		c.mainJsHandler.contentMiddle = append(
			c.mainJsHandler.contentMiddle,
			&contentFile{path: name + ".js", content: []byte(content)},
		)
	}
	c.mainJsHandler.cacheValid = false
}

// AddIcon adds icons from providers to the bundle
func (c *AssetMin) AddIcon(providers ...IconProvider) error {
	if len(providers) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, p := range providers {
		for _, icon := range p.IconSvg() {
			if err := c.addIcon(icon["id"], icon["svg"]); err != nil {
				return err
			}
		}
	}
	return nil
}

// InjectBodyContent appends HTML to the body
func (c *AssetMin) InjectBodyContent(html string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.indexHtmlHandler.contentMiddle = append(
		c.indexHtmlHandler.contentMiddle,
		&contentFile{path: "injected.html", content: []byte(html)},
	)
	c.indexHtmlHandler.cacheValid = false
}
