package assetmin

import (
	"github.com/tdewolff/minify/v2"
	"github.com/tinywasm/tui"
)

// Minify toggle group label, option keys and captions — exported constants so
// callers (including tests) never hardcode the literal strings; the source of
// truth for "on"/"off" and their captions lives here, once.
const (
	MinifyLabel      = "Minify Assets"
	MinifyOptionOn   = "on"
	MinifyOptionOff  = "off"
	MinifyCaptionOn  = "ON"
	MinifyCaptionOff = "OFF"
)

// Label returns the group label shown before the ON/OFF buttons
// (devtui.HandlerSelection — see Options/Value/Change below).
func (c *AssetMin) Label() string {
	return MinifyLabel
}

// Value returns the key of the currently active minify option.
func (c *AssetMin) Value() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.minifyEnabled {
		return MinifyOptionOn
	}
	return MinifyOptionOff
}

// Options returns the minify choices as ordered {value: label} pairs so
// DevTUI renders them as a radio / segmented control instead of a plain
// button — more intuitive to switch than the previous single toggle button.
func (c *AssetMin) Options() []map[string]string {
	return []map[string]string{
		{MinifyOptionOn: MinifyCaptionOn},
		{MinifyOptionOff: MinifyCaptionOff},
	}
}

// Change updates the minify state and regenerates all assets. Implements
// HandlerSelection.Change: called with the selected option's key ("on"/"off")
// when the user confirms a choice.
//
// Regeneration can take a moment on larger projects, so it's wrapped in
// LogOpen/LogClose to drive the TUI's animated "..." indicator instead of the
// footer looking stuck while it runs.
func (c *AssetMin) Change(newValue string) {
	c.mu.Lock()
	c.minifyEnabled = newValue == MinifyOptionOn
	c.mu.Unlock()

	c.Logger(tui.LogOpen, "Applying minify: "+newValue)
	c.regenerateAll()
	c.Logger(tui.LogClose, "Minify: "+newValue)
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
