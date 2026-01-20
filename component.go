package assetmin

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

// HTMLProvider indicates a component can render HTML (for SSR).
type HTMLProvider interface {
	RenderHTML() string
}

// AccessLevel is used to check permission for SSR injection.
type AccessLevel interface {
	AllowedRoles(action byte) []byte
}

// RegisterComponents iterates over the provided items and extracts assets.
// It leverages existing handlers: mainStyleCssHandler, mainJsHandler, spriteSvgHandler, indexHtmlHandler.
func (c *AssetMin) RegisterComponents(components ...any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, comp := range components {
		// CSS Extraction
		if provider, ok := comp.(CSSProvider); ok {
			css := provider.RenderCSS()
			if css != "" {
				c.mainStyleCssHandler.contentMiddle = append(
					c.mainStyleCssHandler.contentMiddle,
					&contentFile{path: "component.css", content: []byte(css)},
				)
				c.mainStyleCssHandler.cacheValid = false
			}
		}

		// JS Extraction
		if provider, ok := comp.(JSProvider); ok {
			js := provider.RenderJS()
			if js != "" {
				c.mainJsHandler.contentMiddle = append(
					c.mainJsHandler.contentMiddle,
					&contentFile{path: "component.js", content: []byte(js)},
				)
				c.mainJsHandler.cacheValid = false
			}
		}

		// Icon SVG Extraction (with collision detection)
		if provider, ok := comp.(IconProvider); ok {
			icons := provider.IconSvg()
			for _, icon := range icons {
				id := icon["id"]
				svg := icon["svg"]
				if err := c.addIcon(id, svg); err != nil {
					return err // Fail-fast on collision or invalid icon
				}
			}
		}

		// HTML Extraction (SSR for public components)
		if provider, ok := comp.(HTMLProvider); ok {
			if isPublicReadable(comp) {
				html := provider.RenderHTML()
				if html != "" {
					c.indexHtmlHandler.contentMiddle = append(
						c.indexHtmlHandler.contentMiddle,
						&contentFile{path: "component.html", content: []byte(html)},
					)
					c.indexHtmlHandler.cacheValid = false
				}
			}
		}
	}
	return nil
}

// isPublicReadable checks if the component allows public read access.
// It looks for AllowedRoles('r') containing '*'.
func isPublicReadable(comp any) bool {
	if al, ok := comp.(AccessLevel); ok {
		roles := al.AllowedRoles('r')
		for _, r := range roles {
			if r == '*' {
				return true
			}
		}
	}
	return false
}
