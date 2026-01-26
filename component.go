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

// AccessLevel is used to check permission for SSR injection.
type AccessLevel interface {
	AllowedRoles(action byte) []byte
}

// AddCSS appends CSS content to the bundle
func (c *AssetMin) AddCSS(name, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mainStyleCssHandler.contentMiddle = append(
		c.mainStyleCssHandler.contentMiddle,
		&contentFile{path: name + ".css", content: []byte(content)},
	)
	c.mainStyleCssHandler.cacheValid = false
}

// AddJS appends JS content (kept for future use)
func (c *AssetMin) AddJS(name, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mainJsHandler.contentMiddle = append(
		c.mainJsHandler.contentMiddle,
		&contentFile{path: name + ".js", content: []byte(content)},
	)
	c.mainJsHandler.cacheValid = false
}

// AddIcon adds an icon (public wrapper for addIcon)
func (c *AssetMin) AddIcon(id, svg string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.addIcon(id, svg)
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
