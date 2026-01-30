package assetmin

// Methods for content injection

// AddCSS appends CSS content from providers to the bundle
// InjectCSS appends CSS content to the bundle.
// name is used for the virtual filename (e.g., "mycomponent.css").
func (c *AssetMin) InjectCSS(name string, content string) {
	if content == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.mainStyleCssHandler.contentMiddle = append(
		c.mainStyleCssHandler.contentMiddle,
		&contentFile{path: name + ".css", content: []byte(content)},
	)
	c.mainStyleCssHandler.cacheValid = false
}

// AddJS appends JS content from providers to the bundle
// InjectJS appends JS content to the bundle.
// name is used for the virtual filename (e.g., "mycomponent.js").
func (c *AssetMin) InjectJS(name string, content string) {
	if content == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.mainJsHandler.contentMiddle = append(
		c.mainJsHandler.contentMiddle,
		&contentFile{path: name + ".js", content: []byte(content)},
	)
	c.mainJsHandler.cacheValid = false
}

// AddIcon adds icons from providers to the bundle
// InjectSpriteIcon adds an icon to the sprite bundle.
// id: unique icon ID.
// svg: raw SVG content.
func (c *AssetMin) InjectSpriteIcon(id, svg string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.addIcon(id, svg)
}

// InjectHTML appends HTML to the body
func (c *AssetMin) InjectHTML(html string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.indexHtmlHandler.contentMiddle = append(
		c.indexHtmlHandler.contentMiddle,
		&contentFile{path: "injected.html", content: []byte(html)},
	)
	c.indexHtmlHandler.cacheValid = false
}
