package assetmin

import "github.com/tdewolff/minify/v2"

// Label returns the TUI button label reflecting current minification state.
func (c *AssetMin) Label() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.minifyEnabled {
		return "Minify: ON"
	}
	return "Minify: OFF"
}

// Execute toggles minification and regenerates all assets.
func (c *AssetMin) Execute() {
	c.mu.Lock()
	c.minifyEnabled = !c.minifyEnabled
	c.mu.Unlock()

	c.regenerateAll()
}

func (c *AssetMin) regenerateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	handlers := []*asset{
		c.mainStyleCssHandler,
		c.mainJsHandler,
		c.spriteSvgHandler,
		c.faviconSvgHandler,
		c.indexHtmlHandler,
	}
	for _, h := range handlers {
		_ = c.processAsset(h)
	}
}

// activeMinifier returns the minifier to use based on current state.
// It assumes the caller holds c.mu.
func (c *AssetMin) activeMinifier() *minify.M {
	if c.minifyEnabled {
		return c.min
	}
	return nil
}
