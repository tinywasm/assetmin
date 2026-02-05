package assetmin

// Methods for content injection

// AddCSS appends CSS content from providers to the bundle
// InjectCSS appends CSS content to the bundle.
// name is used for the virtual filename (e.g., "mycomponent.css").
// AddCSS appends CSS content from providers to the bundle
// InjectCSS appends CSS content to the bundle.
// name is used for the virtual filename (e.g., "mycomponent.css").
func (c *AssetMin) InjectCSS(name string, content string) {
	if content == "" {
		return
	}
	// AssetMin lock not strictly needed for accessing pre-initialized handlers,
	// but kept for consistency if handlers were dynamic (they aren't currently).
	// However, to avoid lock contention and potential deadlocks if handlers call back,
	// we rely on the asset's own lock.
	// c.mu.Lock() -> DELETED to rely on asset.mu
	// defer c.mu.Unlock()

	c.mainStyleCssHandler.AddContentMiddle(name+".css", []byte(content))
}

// AddJS appends JS content from providers to the bundle
// InjectJS appends JS content to the bundle.
// name is used for the virtual filename (e.g., "mycomponent.js").
func (c *AssetMin) InjectJS(name string, content string) {
	if content == "" {
		return
	}
	c.mainJsHandler.AddContentMiddle(name+".js", []byte(content))
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
	c.indexHtmlHandler.AddContentMiddle("injected.html", []byte(html))
}
