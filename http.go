package assetmin

import (
	"strings"

	"github.com/tinywasm/router"
)

// RegisterRoutes registers the asset handlers on the router.
//
// Every asset route is Public: the router is private by default, and a browser
// fetching the page, its stylesheet, its bundle or its favicon has no identity —
// a non-public asset route answers 403 Forbidden and nothing renders.
func (c *AssetMin) RegisterRoutes(r router.Router) {
	r.PublicAsset(c.indexHtmlHandler.GetURLPath(), c.serveAsset(c.indexHtmlHandler))
	r.PublicAsset(c.mainStyleCssHandler.GetURLPath(), c.serveAsset(c.mainStyleCssHandler))
	r.PublicAsset(c.mainJsHandler.GetURLPath(), c.serveAsset(c.mainJsHandler))
	r.PublicAsset(c.faviconSvgHandler.GetURLPath(), c.serveAsset(c.faviconSvgHandler))

	// Standalone JS assets
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, h := range c.standaloneJS {
		r.PublicAsset(h.GetURLPath(), c.serveAsset(h))
	}
}

func (c *AssetMin) serveAsset(asset *asset) router.HandlerFunc {
	return func(ctx router.Context) {
		content, err := asset.GetMinifiedContent(c.min)
		if err != nil {
			ctx.WriteStatus(500)
			ctx.Write([]byte("Error getting minified content"))
			return
		}

		ctx.SetHeader("Content-Type", asset.mediatype)

		// Robust check for HTML/JS regardless of charset
		isDevMutableText := c.DevMode && strings.Contains(asset.mediatype, "text/")
		if isDevMutableText ||
			strings.Contains(asset.mediatype, "text/html") ||
			strings.Contains(asset.mediatype, "application/javascript") ||
			strings.Contains(asset.mediatype, "text/javascript") {
			ctx.SetHeader("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			// Production or non-text assets (images, fonts, etc.): Strong cache
			ctx.SetHeader("Cache-Control", "public, max-age=31536000, immutable")
		}

		ctx.Write(content)
	}
}
