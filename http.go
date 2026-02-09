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
		if c.DevMode || strings.Contains(asset.mediatype, "text/html") || strings.Contains(asset.mediatype, "application/javascript") || strings.Contains(asset.mediatype, "text/javascript") {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		} else {
			// Production: Strong cache
			// Since content includes hash in filename usually, or we want aggressive caching
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			// We can also add ETag if we wanted to be safer, but max-age is better for performance if filenames change
			// For now, let's use ETag as a fallback if filenames don't change
			// ethag := fmt.Sprintf(`"%x"`, md5.Sum(content))
			// w.Header().Set("ETag", ethag)
		}

		_, _ = w.Write(content)
	}
}
