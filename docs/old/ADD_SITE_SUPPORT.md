# Add Site Support

This document details the required features and concrete implementation guidance for `tinywasm/assetmin` to support the `tinywasm/site` rendering engine.

## 1. Component Registration via Interface Detection

`assetmin` must provide a variadic registration function that accepts `...any` and uses type assertions to detect supported interfaces.

### New File: `component.go`

```go
package assetmin

// Interfaces for component asset extraction
type CSSProvider interface {
    RenderCSS() string
}

type JSProvider interface {
    RenderJS() string
}

type IconSvgProvider interface {
    IconSvg() map[string]string // map[id]svg_content
}

// RegisterComponents iterates over the provided items and extracts assets.
// It leverages existing handlers: mainStyleCssHandler, mainJsHandler, spriteSvgHandler.
func (c *AssetMin) RegisterComponents(components ...any) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    for _, comp := range components {
        // CSS Extraction
        if provider, ok := comp.(CSSProvider); ok {
            css := provider.RenderCSS()
            if css != "" {
                c.mainStyleCssHandler.contentMiddle = append(
                    c.mainStyleCssHandler.contentMiddle,
                    &contentFile{path: "component.css", content: []byte(css)},
                )
                c.mainStyleCssHandler.cacheValid = false
            }
        }

        // JS Extraction
        if provider, ok := comp.(JSProvider); ok {
            js := provider.RenderJS()
            if js != "" {
                c.mainJsHandler.contentMiddle = append(
                    c.mainJsHandler.contentMiddle,
                    &contentFile{path: "component.js", content: []byte(js)},
                )
                c.mainJsHandler.cacheValid = false
            }
        }

        // Icon SVG Extraction (with collision detection)
        if provider, ok := comp.(IconSvgProvider); ok {
            icons := provider.IconSvg()
            for _, icon := range icons {
                id := icon["id"]
                svg := icon["svg"]
                if err := c.addIcon(id, svg); err != nil {
                    return err // Fail-fast on collision or invalid icon
                }
            }
        }
    }
    return nil
}
```

## 2. SVG Icon Registration with Collision Detection

### Modify: `svg.go`

Add a registry for icon IDs and an `addIcon` method that returns an error on collision.

```go
package assetmin

import (
    "errors"
    "sync"
)

// Add to AssetMin struct (assetmin.go):
// registeredIconIDs map[string]bool

// addIcon adds an icon to the sprite handler with collision detection.
// Returns an error if an icon with the same ID is already registered.
func (c *AssetMin) addIcon(id string, svgContent string) error {
    // Initialize map if nil (lazy initialization)
    if c.registeredIconIDs == nil {
        c.registeredIconIDs = make(map[string]bool)
    }

    // Collision check
    if c.registeredIconIDs[id] {
        return errors.New("icon already registered: " + id)
    }

    // Register the icon
    c.registeredIconIDs[id] = true

    // Wrap SVG content as a <symbol> for the sprite
    symbolContent := `<symbol id="` + id + `">` + svgContent + `</symbol>`

    c.spriteSvgHandler.contentMiddle = append(
        c.spriteSvgHandler.contentMiddle,
        &contentFile{path: id + ".svg", content: []byte(symbolContent)},
    )
    c.spriteSvgHandler.cacheValid = false

    return nil
}
```

### Required Tests: `svg_test.go`

- `TestAddIcon_Success`: Successfully adds an icon to the sprite.
- `TestAddIcon_Collision`: Returns error when adding an icon with a duplicate ID.

## 3. Reusing Existing Architecture

The implementation MUST leverage the existing `asset` struct and its methods:

| Existing Component | How to Reuse |
|--------------------|--------------|
| `mainStyleCssHandler` | Append CSS from `RenderCSS()` to `contentMiddle`. |
| `mainJsHandler` | Append JS from `RenderJS()` to `contentMiddle`. |
| `spriteSvgHandler` | Add `<symbol>` elements to `contentMiddle` via `addIcon()`. |
| `asset.RegenerateCache()` | Called automatically by `processAsset` for minification. |
| `asset.GetMinifiedContent()` | Serves the final bundled and minified content. |

**No new handlers are needed.** The existing architecture handles concatenation and minification.

## 4. Warm-Up Policy

To ensure early error detection, call `RegisterComponents` during application startup.

```go
// In application initialization (web/server.go)
am := assetmin.NewAssetMin(config)
if err := am.RegisterComponents(modules.Init()...); err != nil {
    log.Fatalf("Asset registration failed: %v", err)
}
```

---
**Status**: Implemented

---

## 5. HTML SSR Rendering (Public Components)

Public components' HTML should be **injected into the `indexHtmlHandler.contentMiddle`**, similar to how CSS/JS/SVG are bundled. This allows the `index.html` to be served as a complete SSR document.

### Detection Logic

A component should have its HTML pre-rendered for SSR if:
1. It implements `RenderHTML() string`.
2. It implements `AccessLevel` from `crudp`.
3. `AllowedRoles('r')` returns a slice containing `'*'` (public).

### New Interface in `component.go`

```go
// HTMLProvider indicates a component can render HTML
type HTMLProvider interface {
    RenderHTML() string
}
```

### Modified `RegisterComponents` (in `component.go`)

```go
func (c *AssetMin) RegisterComponents(components ...any) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    for _, comp := range components {
        // ... existing CSS, JS, Icon logic ...

        // HTML Extraction (SSR for public components)
        if provider, ok := comp.(HTMLProvider); ok {
            if isPublicReadable(comp) {
                html := provider.RenderHTML()
                if html != "" {
                    c.indexHtmlHandler.contentMiddle = append(
                        c.indexHtmlHandler.contentMiddle,
                        &contentFile{path: "component.html", content: []byte(html)},
                    )
                    c.indexHtmlHandler.cacheValid = false
                }
            }
        }
    }
    return nil
}

// isPublicReadable checks if the component allows public read access
func isPublicReadable(comp any) bool {
    type AccessLevel interface {
        AllowedRoles(action byte) []byte
    }
    if al, ok := comp.(AccessLevel); ok {
        roles := al.AllowedRoles('r')
        for _, r := range roles {
            if r == '*' {
                return true
            }
        }
    }
    return false
}
```

### How It Works

The existing `indexHtmlHandler` in `html.go` already has the structure:

```
contentOpen:   <!doctype html><html><head>...</head><body>
contentMiddle: [COMPONENT HTML GOES HERE]  <-- We inject here
contentClose:  <script>...</script></body></html>
```

When `RegisterComponents` detects a public component, it appends its `RenderHTML()` output to `contentMiddle`. The final `index.html` will contain all public component HTML pre-rendered.

### Build Tags Not Required

`assetmin` is inherently **server-only** because it depends on `net/http` (in `http.go`), which does not compile with TinyGo/WASM. Therefore:

- No `//go:build !wasm` tags are needed in `assetmin`.
- If someone accidentally imports `assetmin` in WASM code, the compiler will fail immediately with a clear error.

### Summary

| Asset | Handler | Where in Handler |
|-------|---------|------------------|
| CSS | `mainStyleCssHandler` | `contentMiddle` |
| JS | `mainJsHandler` | `contentMiddle` |
| Icons | `spriteSvgHandler` | `contentMiddle` |
| HTML (SSR) | `indexHtmlHandler` | `contentMiddle` |

---

## 6. Post-Implementation: Documentation Update

After completing the implementation, review and reorganize the entire `assetmin` documentation:

### Tasks

1. **Simplify `README.md`**: Keep it concise with a brief overview and installation instructions. Link to detailed docs in `docs/`.

2. **Update `docs/` structure**: Ensure each topic has its own document:
   - `docs/ARCHITECTURE.md` - High-level design
   - `docs/ASSETS.md` - How CSS, JS, SVG, HTML are processed
   - `docs/COMPONENT_REGISTRATION.md` - The new `RegisterComponents` API
   - `docs/HTTP_HANDLERS.md` - How HTTP routes are registered
   - `docs/SSR.md` - Server-side rendering integration

3. **Link from README**: The `README.md` should have a Documentation section like:
   ```markdown
   ## 📚 Documentation

   1. [Architecture](docs/ARCHITECTURE.md): High-level design and structure.
   2. [Assets](docs/ASSETS.md): CSS, JS, SVG, and HTML processing.
   3. [Component Registration](docs/COMPONENT_REGISTRATION.md): Using `RegisterComponents()`.
   4. [HTTP Handlers](docs/HTTP_HANDLERS.md): Serving assets via HTTP.
   5. [SSR](docs/SSR.md): Server-side rendering integration.
   ```

4. **Remove redundant content**: Avoid duplicating explanations across documents. Use cross-references instead.

5. **Mark this document as complete**: Change status to "Implemented" and archive or delete `ADD_SITE_SUPPORT.md` once all features are live.
