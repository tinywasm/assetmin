package assetmin

// SetOnSSRCompile sets the external compiler trigger for SSR mode.
func (c *AssetMin) SetOnSSRCompile(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onSSRCompile = fn
}

// HandleModEvent processes go.mod changes and returns nil to satisfy NewFileEvent.
func (c *AssetMin) HandleModEvent(filePath string) error {
	c.goModHandler.NewFileEvent(filePath, c.writeMessage)
	return nil
}

// isSSRMode returns true if the package is being used as a dependency (SSR mode).
func (c *AssetMin) isSSRMode() bool {
	return c.goModHandler.IsUsed()
}
