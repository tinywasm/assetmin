package assetmin

// SetExternalSSRCompiler sets the external compiler trigger for SSR mode.
func (c *AssetMin) SetExternalSSRCompiler(fn func() error, buildOnDisk bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.onSSRCompile = fn
	c.buildOnDisk = buildOnDisk

	if buildOnDisk {
		if c.onSSRCompile != nil {
			if err := c.onSSRCompile(); err != nil {
				c.Logger("SetExternalSSRCompiler init error:", err)
			}
		}
		// Ensure all assets are updated on disk immediately but safely (don't overwrite)
		c.processAssetSafe(c.mainStyleCssHandler)
		c.processAssetSafe(c.mainJsHandler)
		c.processAssetSafe(c.spriteSvgHandler)
		c.processAssetSafe(c.faviconSvgHandler)
		c.processAssetSafe(c.indexHtmlHandler)
	}
}

// isSSRMode returns true if the package is being used as a dependency (SSR mode).
func (c *AssetMin) isSSRMode() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.onSSRCompile != nil
}
