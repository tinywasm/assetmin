package assetmin

import (
	"strings"
)

// ContainsCSS checks if the CSS bundle contains the given substring.
func (c *AssetMin) ContainsCSS(substr string) bool {
	return c.mainStyleCssHandler.containsContent(substr)
}

// ContainsJS checks if the JS bundle contains the given substring.
func (c *AssetMin) ContainsJS(substr string) bool {
	return c.mainJsHandler.containsContent(substr)
}

// ContainsSVG checks if the SVG sprite contains the given substring.
func (c *AssetMin) ContainsSVG(substr string) bool {
	return c.spriteSvgHandler.containsContent(substr)
}

// ContainsHTML checks if the HTML bundle contains the given substring.
func (c *AssetMin) ContainsHTML(substr string) bool {
	return c.indexHtmlHandler.containsContent(substr)
}

// HasIcon checks if an icon with the given ID is registered.
func (c *AssetMin) HasIcon(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.registeredIconIDs != nil && c.registeredIconIDs[id]
}

// GetMinifiedJS returns the minified content of the JS bundle.
func (c *AssetMin) GetMinifiedJS() ([]byte, error) {
	return c.mainJsHandler.GetMinifiedContent(c.min)
}

// GetMinifiedCSS returns the minified content of the CSS bundle.
func (c *AssetMin) GetMinifiedCSS() ([]byte, error) {
	return c.mainStyleCssHandler.GetMinifiedContent(c.min)
}

// GetMainJsPath returns the output path of the main JS file.
func (c *AssetMin) GetMainJsPath() string {
	return c.mainJsHandler.outputPath
}

// GetMainCssPath returns the output path of the main CSS file.
func (c *AssetMin) GetMainCssPath() string {
	return c.mainStyleCssHandler.outputPath
}

// GetMainSvgPath returns the output path of the main SVG file.
func (c *AssetMin) GetMainSvgPath() string {
	return c.spriteSvgHandler.outputPath
}

// GetMainHtmlPath returns the output path of the main HTML file.
func (c *AssetMin) GetMainHtmlPath() string {
	return c.indexHtmlHandler.outputPath
}

// RegenerateHTMLCache forces regeneration of the HTML cache.
func (c *AssetMin) RegenerateHTMLCache() error {
	return c.indexHtmlHandler.RegenerateCache(c.min)
}

// GetCachedHTML returns the cached minified HTML content.
func (c *AssetMin) GetCachedHTML() []byte {
	return c.indexHtmlHandler.GetCachedMinified()
}

// IsSSRMode returns true if the package is being used as a dependency (SSR mode).
func (c *AssetMin) IsSSRMode() bool {
	return c.isSSRMode()
}

// ParseExistingHtmlContent is a public wrapper for tests.
func ParseExistingHtmlContent(content string) (openContent, closeContent string) {
	return parseExistingHtmlContent(content)
}

// RewriteAssetUrls is a public wrapper for tests.
func RewriteAssetUrls(html string, newRoot string) string {
	return rewriteAssetUrls(html, newRoot)
}

// StripLeadingUseStrict is a public wrapper for tests.
func StripLeadingUseStrict(b []byte) []byte {
	return stripLeadingUseStrict(b)
}

// GetInitCodeJS returns the init code for the JS bundle.
func (c *AssetMin) GetInitCodeJS() (string, error) {
	return c.startCodeJS()
}

// GetJSURLPath returns the URL path for the main JS file.
func (c *AssetMin) GetJSURLPath() string {
	return c.mainJsHandler.GetURLPath()
}

// GetCSSURLPath returns the URL path for the main CSS file.
func (c *AssetMin) GetCSSURLPath() string {
	return c.mainStyleCssHandler.GetURLPath()
}

// GetSVGURLPath returns the URL path for the SVG sprite file.
func (c *AssetMin) GetSVGURLPath() string {
	return c.spriteSvgHandler.GetURLPath()
}

// GetFaviconURLPath returns the URL path for the favicon file.
func (c *AssetMin) GetFaviconURLPath() string {
	return c.faviconSvgHandler.GetURLPath()
}

// containsContent is a helper on the asset struct to check all its content sections.
func (h *asset) containsContent(substr string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Check contentOpen
	for _, f := range h.contentOpen {
		if strings.Contains(string(f.Content), substr) {
			return true
		}
	}

	// Check contentMiddle
	for _, f := range h.contentMiddle {
		if strings.Contains(string(f.Content), substr) {
			return true
		}
	}

	// Check contentClose
	for _, f := range h.contentClose {
		if strings.Contains(string(f.Content), substr) {
			return true
		}
	}

	return false
}
