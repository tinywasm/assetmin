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
	"github.com/tdewolff/minify/v2/svg"
	"github.com/tinywasm/fmt"
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
	registeredIconIDs   map[string]bool
	listModulesFn       func(rootDir string) ([]string, error)
	ssrLoading          sync.WaitGroup
	scanner             *importScanner
	minifyEnabled       bool
	fromRoot            *rootCandidate
	fromCss             *rootCandidate
	standaloneJS        map[string]*asset
	standaloneOwners    map[string][]string // module name -> list of standalone asset names (outputs)
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
		registeredIconIDs: make(map[string]bool),
		scanner:           newImportScanner(),
		minifyEnabled:     true,
		standaloneJS:      make(map[string]*asset),
		standaloneOwners:  make(map[string][]string),
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
	c.min.AddFunc("image/svg+xml", svg.Minify)

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

		// Attempt to get the latest minified sprite content
		content, err := c.spriteSvgHandler.GetMinifiedContent(c.min)
		if err != nil {
			c.Logger("Error getting sprite content for auto-injection:", err)
			return nil
		}
		return content
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

// SetListModulesFn replaces the module discovery function.
// Only for tests — allows injecting dummy directories without network.
func (c *AssetMin) SetListModulesFn(fn func(rootDir string) ([]string, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.listModulesFn = fn
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

// ExtractSSRAssetsWithContext uses the AssetMin's listModulesFn (if set) when discovering modules.
// This allows tests to mock module discovery without running actual go list commands.
func (c *AssetMin) ExtractSSRAssetsWithContext(moduleDir string) (*SSRAssets, error) {
	rootDir, err := findProjectRoot(moduleDir)
	if err != nil {
		return nil, fmt.Err("failed to find project root from", moduleDir, err)
	}

	// Check for any of the ssrSourceFiles
	foundSSR := false
	for _, f := range ssrSourceFiles {
		if _, err := os.Stat(filepath.Join(moduleDir, f)); err == nil {
			foundSSR = true
			break
		}
	}
	if !foundSSR {
		return nil, fmt.Err("no SSR source files found in", moduleDir)
	}

	var modules []Module
	if c.listModulesFn != nil {
		dirs, err := c.listModulesFn(rootDir)
		if err == nil {
			for _, d := range dirs {
				modules = append(modules, Module{
					Path: filepath.Base(d),
					Dir:  d,
				})
			}
		}
	}

	if len(modules) == 0 {
		var err error
		modules, err = discoverModules(rootDir)
		if err != nil {
			modules = []Module{{Path: filepath.Base(moduleDir), Dir: moduleDir}}
		}
	}

	var targetModule Module
	found := false
	for _, m := range modules {
		if m.Dir == moduleDir {
			targetModule = m
			found = true
			break
		}
	}

	if !found {
		// Construct a proper import path by reading go.mod root and appending relative path
		relPath, _ := filepath.Rel(rootDir, moduleDir)
		importPath, _ := readGoModulePath(rootDir)
		if importPath != "" && relPath != "." {
			targetModule.Path = importPath + "/" + relPath
		} else {
			targetModule.Path = filepath.Base(moduleDir)
		}
		targetModule.Dir = moduleDir
	}

	return extractSSRAssetsForModule(targetModule, rootDir, modules, "")
}
