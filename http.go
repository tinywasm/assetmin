package assetmin

import (
	"net/http"
	"strings"
)

// RegisterRoutes registers the HTTP handlers for all assets.
func (c *AssetMin) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(c.indexHtmlHandler.URLPath(), c.serveAsset(c.indexHtmlHandler))
	mux.HandleFunc(c.mainStyleCssHandler.URLPath(), c.serveAsset(c.mainStyleCssHandler))
	mux.HandleFunc(c.mainJsHandler.URLPath(), c.serveAsset(c.mainJsHandler))
	mux.HandleFunc(c.spriteSvgHandler.URLPath(), c.serveAsset(c.spriteSvgHandler))
	mux.HandleFunc(c.faviconSvgHandler.URLPath(), c.serveAsset(c.faviconSvgHandler))
}

func (c *AssetMin) serveAsset(asset *asset) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := asset.GetMinifiedContent(c.min)
		if err != nil {
			http.Error(w, "Error getting minified content", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", asset.mediatype)

		// Robust check for HTML/JS regardless of charset
		isDevMutableText := c.DevMode && strings.Contains(asset.mediatype, "text/")
		if isDevMutableText ||
			strings.Contains(asset.mediatype, "text/html") ||
			strings.Contains(asset.mediatype, "application/javascript") ||
			strings.Contains(asset.mediatype, "text/javascript") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			// Production or non-text assets (images, fonts, etc.): Strong cache
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}

		_, _ = w.Write(content)
	}
}
