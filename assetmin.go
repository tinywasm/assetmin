package assetmin

import (
	"os"
	"path"
	"regexp"
	"sync"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	"github.com/tdewolff/minify/v2/svg"
)

type AssetMin struct {
	mu sync.Mutex // Added mutex for synchronization
	*Config
	mainStyleCssHandler *asset
	mainJsHandler       *asset
	spriteSvgHandler    *asset
	faviconSvgHandler   *asset
	indexHtmlHandler    *asset
	min                 *minify.M
	buildOnDisk         bool // Build assets to disk if true
	log                 func(message ...any)
}

type Config struct {
	OutputDir               string                 // eg: web/static, web/public, web/assets
	GetRuntimeInitializerJS func() (string, error) // javascript code to initialize the wasm or other handlers
	AppName                 string                 // Application name for templates (default: "MyApp")
	AssetsURLPrefix         string                 // New: for HTTP routes
}

func NewAssetMin(ac *Config) *AssetMin {
	c := &AssetMin{
		Config: ac,
		min:    minify.New(),
	}

	if c.AppName == "" {
		c.AppName = "MyApp"
	}

	jsMainFileName := "script.js"
	cssMainFileName := "style.css"
	svgMainFileName := "sprite.svg"
	svgFaviconFileName := "favicon.svg"
	htmlMainFileName := "index.html"

	c.mainStyleCssHandler = newAssetFile(cssMainFileName, "text/css", ac, nil)
	c.mainJsHandler = newAssetFile(jsMainFileName, "text/javascript", ac, ac.GetRuntimeInitializerJS)
	c.spriteSvgHandler = NewSvgHandler(ac, svgMainFileName)
	c.faviconSvgHandler = NewFaviconSvgHandler(ac, svgFaviconFileName)

	// Set URL paths before creating the index handler that depends on them
	c.mainStyleCssHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, cssMainFileName)
	c.mainJsHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, jsMainFileName)
	c.spriteSvgHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, svgMainFileName)
	c.faviconSvgHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, svgFaviconFileName)

	c.indexHtmlHandler = NewHtmlHandler(ac, htmlMainFileName, c.mainStyleCssHandler.URLPath(), c.mainJsHandler.URLPath())
	c.indexHtmlHandler.urlPath = "/" // Index is always at root
	c.min.Add("text/html", &html.Minifier{
		KeepDocumentTags: true,
		KeepEndTags:      true,
		KeepWhitespace:   true,
		KeepQuotes:       true,
	})

	c.min.AddFunc("text/css", css.Minify)
	c.min.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	c.min.AddFunc("image/svg+xml", svg.Minify)

	c.mainJsHandler.initCode = c.startCodeJS

	return c
}

func (c *AssetMin) Name() string {
	return "ASSETS"
}

func (c *AssetMin) SetLog(f func(message ...any)) {
	c.log = f
}

func (c *AssetMin) Logger(messages ...any) {
	if c.log != nil {
		c.log(messages...)
	}
}

func (c *AssetMin) SupportedExtensions() []string {
	return []string{".js", ".css", ".svg", ".html"}
}

func (c *AssetMin) writeMessage(messages ...any) {
	c.Logger(messages...)
}

func fileExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func (c *AssetMin) EnsureOutputDirectoryExists() {
	outputDir := c.OutputDir
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		c.writeMessage("dont create output dir", err)
	}
}

func (c *AssetMin) RefreshAsset(extension string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var fh *asset
	switch extension {
	case ".js":
		fh = c.mainJsHandler
	case ".css":
		fh = c.mainStyleCssHandler
	case ".svg":
	}

	if fh != nil {
		if err := c.processAsset(fh); err != nil {
			c.writeMessage("Error refreshing asset "+extension, err)
		}
	}
}

// SetBuildOnDisk sets the work mode for AssetMin.
func (c *AssetMin) SetBuildOnDisk(onDisk bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buildOnDisk = onDisk

	c.Logger("SetBuildOnDisk:", onDisk)

	if onDisk {
		// Ensure all assets are updated on disk immediately
		c.processAsset(c.mainStyleCssHandler)
		c.processAsset(c.mainJsHandler)
		c.processAsset(c.spriteSvgHandler)
		c.processAsset(c.faviconSvgHandler)
		c.processAsset(c.indexHtmlHandler)
	}
}

// BuildOnDisk returns true if assets are written to disk.
func (c *AssetMin) BuildOnDisk() bool {

	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buildOnDisk
}
