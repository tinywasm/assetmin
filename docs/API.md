# AssetMin API Documentation

AssetMin is a lightweight web asset packager and minifier for Go applications. It bundles and minifies JavaScript, CSS, SVG, and HTML files with support for both memory-based and disk-based serving.

## Table of Contents

- [Core Concepts](#core-concepts)
- [Configuration](#configuration)
- [Work Modes](#work-modes)
- [Public API](#public-api)
- [Asset Types](#asset-types)
- [HTTP Serving](#http-serving)
- [File Events](#file-events)
- [Examples](#examples)

## Core Concepts

AssetMin processes and minifies web assets through a simple event-driven workflow:

1. **File Events**: Notify AssetMin when files are created, modified, or deleted.
2. **Content Processing**: Files are read, processed, and stored in memory.
3. **Minification**: Content is minified and cached for fast serving.
4. **Output**: Assets are served via HTTP or written to disk based on work mode.

### Asset Organization (Slots)

Assets are organized into three content sections to ensure correct loading order:

- **contentOpen**: Files processed first (e.g., base themes, CSS variables).
- **contentMiddle**: Main content files (e.g., external module files).
- **contentClose**: Files processed last (e.g., application-specific overrides).

## Configuration

```go
type Config struct {
    OutputDir          string                 // Directory for DiskMode output
    RootDir            string                 // Project root (used for module discovery)
    GetSSRClientInitJS func() (string, error) // Returns JS code to initialize the client
    AppName            string                 // Application name used in templates
    AssetsURLPrefix    string                 // URL prefix for assets (e.g., "/static/")
    Logger             func(msg ...any)       // Optional logging function
}
```

## Work Modes

AssetMin supports two work modes:

### MemoryMode (Default)
- Assets are served from memory cache only.
- No disk writes occur.
- Changes are immediately available via HTTP.

### DiskMirroredMode
- Activated by calling `FlushToDisk()`.
- All in-memory assets are written to disk (overwrites stale files).
- After a successful flush, every subsequent `NewFileEvent` also writes to disk automatically.
- HTTP requests continue to be served from the in-memory cache.

## Public API

### Creating an Instance
```go
func NewAssetMin(config *Config) *AssetMin
```

### File Event Processing
```go
func (c *AssetMin) NewFileEvent(fileName, extension, filePath, event string) error
```
Processes a file system event and updates the corresponding asset bundle.
- `fileName`: Name of the file (e.g., "button.css").
- `extension`: File extension (e.g., ".css", ".js", ".svg", ".html").
- `filePath`: Full path to the source file.
- `event`: Event type - "create", "write", "remove".

### Disk Flush & SSR Mode

#### EnableSSRMode()
Activates the SSR event-handling branch unconditionally. Call once at startup so that `.css` file events route through the SSR slot logic. Does NOT register a compiler — safe to call with no `SetSSRCompiler`.

#### SetSSRCompiler(fn func() error)
Registers a Go compiler callback that is invoked on `.go` file changes. Pure setter — does NOT call `fn` at registration time. Pass `nil` to unregister.

#### FlushToDisk() error
Snapshots all registered assets and writes them to disk (overwrites existing files). Sets the internal `diskMirrored` flag only on full success; subsequent `NewFileEvent` calls will also write to disk. Returns the first write error encountered.

```go
// Typical usage before starting an external server:
if err := am.FlushToDisk(); err != nil {
    return fmt.Errorf("assetmin flush: %w", err)
}
```

### SSR & Module Loading

#### LoadSSRModules()
Starts the discovery of all Go modules in the project tree, scans for `ssr.go` files, and extracts assets (CSS, JS, HTML, Icons) asynchronously via compile-and-invoke.

#### ReloadSSRModule(moduleDir string) error
Re-extracts and updates assets for a single module directory. Used for hot reload.

#### RegisterComponents(providers ...any) error
Registers live component instances that implement SSR interfaces.
- `RootCSS() *css.Stylesheet`: Routed to `open` slot.
- `RenderCSS() *css.Stylesheet`: Routed to `middle` or `close` slot.
- `RenderJS() string`
- `RenderHTML() string`
- `IconSvg() map[string]string`

#### RefreshWasmAssets()
Invalidates and regenerates JS and HTML assets. Use this when the WASM binary or initialization logic changes.

### HTTP Serving
```go
func (c *AssetMin) RegisterRoutes(mux *http.ServeMux)
```
Registers handlers:
- `GET /` -> `index.html`
- `GET /<prefix>style.css`
- `GET /<prefix>script.js`
- `GET /<prefix>favicon.svg`

## Asset Types

### JavaScript
- Output: `script.js`
- Features: Automatic `'use strict'` management, runtime initializer prepended.

### CSS
- Output: `style.css`
- Features: Merged bundles from all source files and SSR modules. Supports typed CSS via `github.com/tinywasm/css`.

### SVG
- **Sprite**: Delivered exclusively **inline** within `index.html`. No separate HTTP route.
- **Favicon**: Served as `favicon.svg`.

### HTML
- Output: `index.html`
- Features: Template-based, automatic injection of CSS/JS/Sprite.

## Thread Safety
AssetMin is fully thread-safe, utilizing `sync.RWMutex` for asset caches and protecting the global state with a primary mutex.

## Performance
SSR extraction uses a **compile-and-invoke** mechanism:
- Results are cached globally (`ssrGlobalCache`) using the **MD5 hash** of module Go files.
- Warm extractions are near-instant (~1ms).
- Cold extraction wall-time (edit -> extract) is ~300-500ms, dominated by `go run`.
