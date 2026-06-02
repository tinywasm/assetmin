package assetmin

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/js"
	minifySvg "github.com/tdewolff/minify/v2/svg"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/svg"
)

type AssetMin struct {
	mu sync.Mutex // Mutex for synchronization
	*Config
	mainStyleCssHandler *asset
	mainJsHandler       *asset
	spriteSvgHandler    *asset
	faviconSvgHandler   *asset
	indexHtmlHandler    *asset
	min                 *minify.M
	ssrEnabled          bool              // SSR branch activation flag
	diskMirrored        bool              // If true, assets are being mirrored to disk
	allAssets           map[string]*asset // Keyed by outputPath - dedup
	log                 func(message ...any)
	onSSRCompile        func() error
	ssrLoading          sync.WaitGroup
	minifyEnabled       bool
	fromRoot            *rootCandidate
	fromCss             *rootCandidate
	standaloneJS        map[string]*asset
	standaloneOwners    map[string][]string // module name -> list of standalone asset names (outputs)
	imageProcessor      ImageProcessor
	ssrExtractor        SSRExtractor
	masterSprite        *svg.Sprite
	spriteMu            sync.RWMutex
}

type rootCandidate struct {
	name string
	css  string
}

type Config struct {
	OutputDir       string // eg: web/static, web/public, web/assets
	RootDir         string // Root directory of the project where go.mod exists
	AppName         string // Application name for templates (default: "MyApp")
	AssetsURLPrefix    string                 // New: for HTTP routes
	DevMode            bool                   // If true, disables caching (default: false)
}

func NewAssetMin(ac *Config) *AssetMin {
	c := &AssetMin{
		Config:            ac,
		min:               minify.New(),
		minifyEnabled:     true,
		standaloneJS:      make(map[string]*asset),
		standaloneOwners:  make(map[string][]string),
		masterSprite:      &svg.Sprite{},
	}

	if c.AppName == "" {
		c.AppName = "MyApp"
	}

	c.allAssets = make(map[string]*asset)

	jsMainFileName := "script.js"
	cssMainFileName := "style.css"
	svgMainFileName := "icons.svg"
	svgFaviconFileName := "favicon.svg"
	htmlMainFileName := "index.html"

	c.mainStyleCssHandler = newAssetFile(cssMainFileName, "text/css", ac, nil)
	c.mainJsHandler = newAssetFile(jsMainFileName, "text/javascript", ac, nil)
	c.spriteSvgHandler = NewSvgHandler(ac, svgMainFileName)
	c.faviconSvgHandler = NewFaviconSvgHandler(ac, svgFaviconFileName)

	// Set URL paths before creating the index handler that depends on them
	c.mainStyleCssHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, cssMainFileName)
	c.mainJsHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, jsMainFileName)
	c.faviconSvgHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, svgFaviconFileName)

	c.indexHtmlHandler = NewHtmlHandler(ac, htmlMainFileName, c.mainStyleCssHandler.GetURLPath(), c.mainJsHandler.GetURLPath(), c.faviconSvgHandler.GetURLPath())
	c.indexHtmlHandler.urlPath = "/" // Index is always at root
	c.min.Add("text/html", &html.Minifier{
		KeepDocumentTags: true,
		KeepEndTags:      true,
		KeepWhitespace:   true,
		KeepQuotes:       true,
	})

	c.min.AddFunc("text/css", css.Minify)
	c.min.AddFuncRegexp(regexp.MustCompile("^(application|text)/(x-)?(java|ecma)script$"), js.Minify)
	c.min.AddFunc("image/svg+xml", minifySvg.Minify)

	c.mainJsHandler.initCode = c.startCodeJS

	// Register main assets
	for _, a := range []*asset{
		c.mainStyleCssHandler, c.mainJsHandler,
		c.spriteSvgHandler, c.faviconSvgHandler, c.indexHtmlHandler,
	} {
		c.allAssets[a.outputPath] = a
	}

	// Automatic Sprite Injection:
	// Link the Sprite Handler to the HTML Handler so the sprite is injected dynamically
	// into the HTML body. This avoids manual injection in build scripts.
	c.indexHtmlHandler.AddDynamicContent(func() []byte {
		c.spriteMu.RLock()
		defer c.spriteMu.RUnlock()
		return []byte(c.masterSprite.String())
	})

	c.spriteSvgHandler.AddDynamicContent(func() []byte {
		c.spriteMu.RLock()
		defer c.spriteMu.RUnlock()
		return []byte(c.masterSprite.String())
	})

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

func (c *AssetMin) EnsureOutputDirectoryExists() {
	outputDir := c.OutputDir
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		c.writeMessage("dont create output dir", err)
	}
}

func (c *AssetMin) refreshAsset(extension string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var handlers []*asset
	switch extension {
	case ".js":
		handlers = append(handlers, c.mainJsHandler)
		for _, h := range c.standaloneJS {
			handlers = append(handlers, h)
		}
	case ".css":
		handlers = append(handlers, c.mainStyleCssHandler)
	case ".html":
		handlers = append(handlers, c.indexHtmlHandler)
	case ".svg":
		handlers = append(handlers, c.spriteSvgHandler)
	}

	for _, fh := range handlers {
		if err := c.processAsset(fh); err != nil {
			c.writeMessage("Error refreshing asset "+extension, err)
		}
	}
}

// RefreshJSAssets triggers a refresh of JS assets.
// Call this when the WASM binary changes to ensure they are up to date.
func (c *AssetMin) RefreshJSAssets() {
	c.refreshAsset(".js")
}

// readGoModulePath extracts the module path from go.mod (e.g., "example.com/demo")
func readGoModulePath(rootDir string) (string, error) {
	gomodPath := filepath.Join(rootDir, "go.mod")
	content, err := os.ReadFile(gomodPath)
	if err != nil {
		return "", err
	}

	lines := string(content)
	newlineIdx := findIndex(lines, "\n")
	if newlineIdx < 0 {
		newlineIdx = len(lines)
	}
	firstLine := lines[:newlineIdx]

	if len(firstLine) > 7 && firstLine[:7] == "module " {
		return firstLine[7:], nil
	}
	return "", fmt.Err("no module line in go.mod")
}

func findIndex(s string, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
