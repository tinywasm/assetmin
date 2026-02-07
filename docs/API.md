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

1. **File Events**: Notify AssetMin when files are created, modified, or deleted
2. **Content Processing**: Files are read, processed, and stored in memory
3. **Minification**: Content is minified and cached for fast serving
4. **Output**: Assets are served via HTTP or written to disk based on work mode

### Asset Organization

Assets are organized into three content sections:

- **contentOpen**: Files processed first (e.g., initialization code)
- **contentMiddle**: Main content files (e.g., module files)
- **contentClose**: Files processed last (e.g., cleanup code)

## Configuration

See [`assetmin.go`](../assetmin.go#L35-L41) for the Config struct definition.

```go
type Config struct {
    // OutputDir is the directory where assets will be written in DiskMode
    // Example: "web/public", "dist", "static"
    OutputDir string

    // Logger is called with messages for debugging and monitoring
    // Example: func(msg ...any) { fmt.Println(msg...) }
    Logger func(message ...any)

    // GetRuntimeInitializerJS returns JavaScript code to initialize the application
    // This code is prepended to the main JavaScript bundle
    // Example: WASM initialization code, analytics setup, etc.
    GetRuntimeInitializerJS func() (string, error)

    // AppName is used in generated HTML templates
    // Default: "MyApp"
    AppName string

    // AssetsURLPrefix is the URL prefix for serving static assets
    // Examples: "/assets/", "/static/", "" (root)
    // Note: index.html is always served at "/" regardless of this prefix
    AssetsURLPrefix string
}
```

### Configuration Example

```go
config := &assetmin.Config{
    OutputDir: "web/public",
    Logger: func(msg ...any) {
        log.Println(msg...)
    },
    GetRuntimeInitializerJS: func() (string, error) {
        return "console.log('App initialized!');", nil
    },
    AppName: "MyWebApp",
    AssetsURLPrefix: "/assets/",
}
```

## Work Modes

See [`assetmin.go`](../assetmin.go#L16-L21) for WorkMode constants.

AssetMin supports two work modes:

### MemoryMode (Default)

- Assets are served from memory cache only
- No disk writes occur
- Fastest performance for development
- Changes are immediately available via HTTP

### DiskMode

- Assets are written to disk AND cached in memory
- HTTP requests still served from cache for performance
- Useful for deployment or when disk files are needed

### Switching Modes

```go
am := assetmin.NewAssetMin(config)

// Switch to disk mode
am.SetWorkMode(assetmin.DiskMode)

// Switch back to memory mode
am.SetWorkMode(assetmin.MemoryMode)

// Check current mode
mode := am.GetWorkMode()
```

When switching from MemoryMode to DiskMode, all cached assets are immediately written to disk.

## Public API

### Creating an AssetMin Instance

See [`assetmin.go`](../assetmin.go#L43-L86) for NewAssetMin implementation.

```go
func NewAssetMin(config *Config) *AssetMin
```

Creates and initializes a new AssetMin instance with the following assets:

- `script.js` - Main JavaScript bundle
- `style.css` - Main CSS bundle
- `icons.svg` - SVG sprite for icons
- `favicon.svg` - Favicon handler
- `index.html` - Main HTML page

### File Event Processing

See [`events.go`](../events.go#L49-L93) for NewFileEvent implementation.

```go
func (c *AssetMin) NewFileEvent(fileName, extension, filePath, event string) error
```

Processes a file system event and updates the corresponding asset.

**Parameters:**
- `fileName`: Name of the file (e.g., "button.css")
- `extension`: File extension (e.g., ".css", ".js", ".svg", ".html")
- `filePath`: Full path to the source file
- `event`: Event type - "create", "write", "modify", "remove", or "delete"

**Example:**
```go
// File created
am.NewFileEvent("button.css", ".css", "/src/components/button.css", "create")

// File modified
am.NewFileEvent("header.js", ".js", "/src/components/header.js", "write")

// File deleted
am.NewFileEvent("old.css", ".css", "/src/old.css", "remove")
```

### HTTP Route Registration

See [`http.go`](../http.go#L8-L14) for RegisterRoutes implementation.

```go
func (c *AssetMin) RegisterRoutes(mux *http.ServeMux)
```

Registers HTTP handlers for all assets with the provided ServeMux.

**Example:**
```go
mux := http.NewServeMux()
am.RegisterRoutes(mux)

// Routes registered:
// GET /              -> index.html
// GET /assets/style.css   -> style.css (if AssetsURLPrefix="/assets/")
// GET /assets/script.js   -> script.js
// GET /assets/icons.svg  -> icons.svg
// GET /assets/favicon.svg -> favicon.svg
```

### Asset Refresh

See [`assetmin.go`](../assetmin.go#L113-L131) for RefreshAsset implementation.

```go
func (c *AssetMin) RefreshAsset(extension string)
```

Manually triggers a rebuild of an asset by extension. Useful when external content changes (e.g., WASM initialization code).

**Example:**
```go
// Refresh JavaScript bundle
am.RefreshAsset(".js")

// Refresh CSS bundle
am.RefreshAsset(".css")
```

### Utility Methods

```go
// Get list of supported file extensions
func (c *AssetMin) SupportedExtensions() []string

// Get list of output files that should not be watched for changes
// (to avoid infinite loops when writing to OutputDir)
func (c *AssetMin) UnobservedFiles() []string

// Ensure output directory exists (called automatically in DiskMode)
func (c *AssetMin) EnsureOutputDirectoryExists()
```

## Asset Types

### JavaScript Assets

- **Output**: `script.js`
- **Features**:
  - Automatic `'use strict'` directive prepended
  - Duplicate `'use strict'` directives removed from source files
  - Runtime initializer code prepended (from `GetRuntimeInitializerJS`)
  - Minification via tdewolff/minify

### CSS Assets

- **Output**: `style.css`
- **Features**:
  - All CSS files merged into single bundle
  - Minification preserves functionality
  - Source order preserved

### SVG Assets

#### Sprite SVG
- **Output**: `icons.svg`
- **Purpose**: Icon sprite sheet
- **Features**:
  - Multiple SVG files combined into single sprite
  - Each icon accessible via `<use>` element
  - Automatic ID management

#### Favicon SVG
- **Output**: `favicon.svg`
- **Purpose**: Site favicon
- **Features**:
  - Single SVG file
  - Optimized for browser favicon display

### HTML Assets

- **Output**: `index.html`
- **Features**:
  - Template-based generation
  - Automatic CSS and JS references
  - Complete HTML documents are ignored (not merged)
  - HTML fragments are merged

## HTTP Serving

See [`http.go`](../http.go) for HTTP serving implementation.

### Content Delivery

All assets are served from an in-memory cache for optimal performance:

1. Request arrives for an asset
2. Cache validity is checked
3. If invalid, content is regenerated and minified
4. Cached content is served with appropriate headers

### Headers

All assets are served with:
- `Content-Type`: Appropriate MIME type for the asset
- `Cache-Control`: `no-cache, no-store, must-revalidate` (development-friendly)

### URL Paths

Asset URLs are determined by the `AssetsURLPrefix` configuration:

| Asset | Default URL | With Prefix "/assets/" |
|-------|-------------|------------------------|
| index.html | `/` | `/` |
| style.css | `/style.css` | `/assets/style.css` |
| script.js | `/script.js` | `/assets/script.js` |
| icons.svg | `/icons.svg` | `/assets/icons.svg` |
| favicon.svg | `/favicon.svg` | `/assets/favicon.svg` |

**Note**: `index.html` is always served at the root path `/`.

## File Events

### Event Types

- **create**: New file created
- **write/modify**: Existing file modified
- **remove/delete**: File deleted
- **rename**: File renamed (handled as delete + create)

### Event Processing

See [`events.go`](../events.go#L12-L46) for UpdateFileContentInMemory implementation.

1. File content is read from disk (except for delete events)
2. Content is processed based on file type
3. Asset cache is invalidated
4. Cache is regenerated
5. If in DiskMode, output is written to disk

### Infinite Loop Prevention

AssetMin automatically ignores events for its own output files to prevent infinite loops:

```go
// These files are ignored when detected as event sources:
// - web/public/script.js
// - web/public/style.css
// - web/public/icons.svg
// - web/public/favicon.svg
// - web/public/index.html
```

See [`events.go`](../events.go#L147-L179) for isOutputPath implementation.

## Examples

### Basic Setup

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/tinywasm/assetmin"
)

func main() {
    // Create configuration
    config := &assetmin.Config{
        OutputDir: "web/public",
        Logger: func(msg ...any) {
            log.Println(msg...)
        },
        AppName: "MyApp",
    }
    
    // Initialize AssetMin
    am := assetmin.NewAssetMin(config)
    
    // Register HTTP routes
    mux := http.NewServeMux()
    am.RegisterRoutes(mux)
    
    // Start server
    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", mux)
}
```

### With File Watcher Integration

```go
// Assuming you have a file watcher that calls this function
func onFileChange(fileName, extension, filePath, event string) {
    if err := am.NewFileEvent(fileName, extension, filePath, event); err != nil {
        log.Printf("Error processing file event: %v", err)
    }
}

// Example file watcher setup (pseudo-code)
watcher.Watch("src/components", func(event FileEvent) {
    onFileChange(
        event.Name,
        filepath.Ext(event.Name),
        event.Path,
        event.Type,
    )
})
```

### Development vs Production

```go
func main() {
    config := &assetmin.Config{
        OutputDir: "dist",
        AppName: "MyApp",
    }
    
    am := assetmin.NewAssetMin(config)
    
    if os.Getenv("ENV") == "production" {
        // Write assets to disk for deployment
        am.SetWorkMode(assetmin.DiskMode)
        am.EnsureOutputDirectoryExists()
        
        // Trigger initial build
        am.RefreshAsset(".js")
        am.RefreshAsset(".css")
    } else {
        // Development: serve from memory
        mux := http.NewServeMux()
        am.RegisterRoutes(mux)
        http.ListenAndServe(":8080", mux)
    }
}
```

### WASM Integration

```go
config := &assetmin.Config{
    OutputDir: "web/public",
    GetRuntimeInitializerJS: func() (string, error) {
        // Return WASM initialization code
        return `
            const go = new Go();
            WebAssembly.instantiateStreaming(
                fetch("main.wasm"),
                go.importObject
            ).then((result) => {
                go.run(result.instance);
            });
        `, nil
    },
    AppName: "WASMApp",
}

am := assetmin.NewAssetMin(config)

// When WASM binary is rebuilt, refresh JavaScript
am.RefreshAsset(".js")
```

## Thread Safety

AssetMin is designed to be thread-safe:

- All public methods use mutex locks where necessary
- Asset cache uses RWMutex for concurrent read access
- File event processing is serialized to prevent race conditions

See [`asset.go`](../asset.go#L26-L28) for cache mutex implementation.

## Performance Considerations

### Memory Usage

- Each asset maintains a minified cache in memory
- Typical memory overhead: ~100KB for a small application
- Cache is regenerated only when content changes

### Minification

- Minification occurs once per content change
- HTTP requests serve pre-minified cached content
- No minification overhead on request path

### Disk I/O

- MemoryMode: Zero disk writes after initial file read
- DiskMode: Single write per asset change
- File reads are buffered and cached

## Related Documentation

- [Contributing Guide](CONTRIBUTING.md)
- [Future Features](issues/README.md)
