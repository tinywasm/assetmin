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
- `GET /assets/icons.svg` → icons.svg
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
// Refresh JavaScript bundle
am.RefreshAsset(".js")

// Refresh CSS bundle
am.RefreshAsset(".css")
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
    GetRuntimeInitializerJS: func() (string, error) {
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
}

am := assetmin.NewAssetMin(config)

// When WASM binary changes
am.RefreshAsset(".js")
```

## File Watcher Integration

```go
// Example with fsnotify or similar
watcher.Watch("src", func(event FileEvent) {
    am.NewFileEvent(
        event.Name,
        filepath.Ext(event.Name),
        event.Path,
        event.Type, // "create", "write", "remove"
    )
})
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
        am.RefreshAsset(".js")
        am.RefreshAsset(".css")
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
- Sprite: `icons.svg` (icon collection)
- Favicon: `favicon.svg` (single icon)

### HTML
- Output: `index.html`
- Fragments merged
- Complete documents ignored

## Common Patterns

### Hot Reload Setup

```go
// In your file watcher
func onFileChange(path string) {
    ext := filepath.Ext(path)
    name := filepath.Base(path)
    
    if err := am.NewFileEvent(name, ext, path, "write"); err != nil {
        log.Printf("Error: %v", err)
    }
}
```

### Build Script

```go
func build() {
    am.SetWorkMode(assetmin.DiskMode)
    am.EnsureOutputDirectoryExists()
    
    // Process all source files
    filepath.Walk("src", func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return err
        }
        
        ext := filepath.Ext(path)
        if slices.Contains(am.SupportedExtensions(), ext) {
            am.NewFileEvent(info.Name(), ext, path, "create")
        }
        return nil
    })
}
```

## Full Documentation

- [API Documentation](API.md) - Complete reference
- [Roadmap](ROADMAP.md) - Planned features
- [Contributing](CONTRIBUTING.md) - Contribution guide
