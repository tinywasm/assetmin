# AssetMin Quick Reference

Quick reference guide for common AssetMin operations.

## Installation

```bash
go get github.com/tinywasm/assetmin
```

## Basic Setup

```go
import "github.com/tinywasm/assetmin"

config := &assetmin.Config{
    OutputDir:       "web/public",
    Logger:          func(msg ...any) { log.Println(msg...) },
    AppName:         "MyApp",
    AssetsURLPrefix: "/assets/",
}

am := assetmin.NewAssetMin(config)
```

## SSR & Component Registration

### Manual Component Registration

```go
// Any struct implementing SSR interfaces
am.RegisterComponents(button, card)
```

### SSR Conventions
- **Components:** Receiver type for `Render*` methods is automatically detected and instantiated. No `SSRInstance()` required.
- **Core Modules:** Like `tinywasm/css`, may expose package-level functions instead.
- **Typed CSS:** `RenderCSS()` and `RootCSS()` return `*css.Stylesheet` (from `github.com/tinywasm/css`).

## File Events

```go
// File created
am.NewFileEvent("button.css", ".css", "/src/button.css", "create")

// File modified
am.NewFileEvent("app.js", ".js", "/src/app.js", "write")

// File deleted
am.NewFileEvent("old.css", ".css", "/src/old.css", "remove")
```

## HTTP Serving

```go
mux := http.NewServeMux()
am.RegisterRoutes(mux)
http.ListenAndServe(":8080", mux)
```

Routes registered:
- `GET /` → index.html
- `GET /assets/style.css` → style.css
- `GET /assets/script.js` → script.js
- `GET /assets/favicon.svg` → favicon.svg

## Work Modes

```go
// Development (default) - memory only
am.SetWorkMode(assetmin.MemoryMode)

// Production - write to disk
am.SetWorkMode(assetmin.DiskMode)
am.EnsureOutputDirectoryExists()

// Check current mode
mode := am.GetWorkMode()
```

## Manual Refresh

```go
// Refresh WASM-related assets (.js, .html)
am.RefreshWasmAssets()
```

## Utility Methods

```go
// Get supported extensions
exts := am.SupportedExtensions() // [".js", ".css", ".svg", ".html"]

// Get unobserved files (to avoid watch loops)
files := am.UnobservedFiles()
```

## WASM Integration

```go
config := &assetmin.Config{
    OutputDir: "web/public",
    GetSSRClientInitJS: func() (string, error) {
        return `console.log("WASM init");`, nil
    },
}

am := assetmin.NewAssetMin(config)

// When WASM binary changes
am.RefreshWasmAssets()
```

## Development vs Production

```go
func main() {
    config := &assetmin.Config{
        OutputDir: "dist",
        AppName:   "MyApp",
    }
    
    am := assetmin.NewAssetMin(config)
    
    if os.Getenv("ENV") == "production" {
        // Production: write to disk
        am.SetWorkMode(assetmin.DiskMode)
        am.EnsureOutputDirectoryExists()
        
        // Build all assets
        am.RefreshWasmAssets()
    } else {
        // Development: serve from memory
        mux := http.NewServeMux()
        am.RegisterRoutes(mux)
        log.Println("Dev server on :8080")
        http.ListenAndServe(":8080", mux)
    }
}
```

## Asset Types

### JavaScript
- Output: `script.js`
- Automatic `'use strict'` directive
- Runtime initializer prepended
- Minified via tdewolff/minify

### CSS
- Output: `style.css`
- All files merged
- Minified

### SVG
- Sprite: Inline icons collection (injected into `index.html`)
- Favicon: `favicon.svg` (single icon)

### HTML
- Output: `index.html`
- Fragments merged
- Complete documents ignored

## Full Documentation

- [API Documentation](API.md) - Complete reference
- [Architecture](ARCHITECTURE.md) - Internal design
- [SSR Documentation](SSR.md) - Module discovery & extraction
- [Contributing](CONTRIBUTING.md) - Contribution guide
