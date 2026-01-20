# Feature: SSR Implementation Details

## Overview
Server-Side Rendering (SSR) support for `tinywasm/dom` components injection into `index.html`.

## Related Documents
- [FEATURE_HTTP_ROUTES_WORK_MODES.md](FEATURE_HTTP_ROUTES_WORK_MODES.md) - Work modes (SSR uses MemoryMode)
- [FEATURE_ASSET_CACHING.md](FEATURE_ASSET_CACHING.md) - Cache system for SSR content
- [FEATURE_TEMPLATE_REFACTOR.md](FEATURE_TEMPLATE_REFACTOR.md) - Dynamic template paths

## Architecture

### HTML Content Structure
- `contentOpen`: Header, opening body (from theme or default)
- `contentMiddle`: SSR modules + dynamic content
- `contentClose`: Scripts, closing body

### Content Insertion Points (Priority Order)
1. `<!-- MODULES_PLACEHOLDER -->` or `{{.Modules}}`
2. Before `</main>` tag
3. Before first `<script>` tag
4. Before `</body>`

## Interface Definition

```go
// In assetmin/ssr.go

// SSRModule represents a component that can be pre-rendered to HTML.
// Compatible with tinywasm/dom Component interface.
type SSRModule interface {
    ModuleName() string   // Identifier for the module
    RenderHTML() string   // Returns pre-rendered HTML
}
```

## Configuration

```go
type Config struct {
    // ... existing fields ...
    
    // SSRModules for pre-rendering in index.html
    SSRModules []SSRModule
}
```

## Injection Logic

When processing HTML asset:
1. Check if `SSRModules` is not empty
2. For each module, call `RenderHTML()`
3. Call `htmlHandler.AddMiddleContent()` for each module

```go
// In html.go - public method for SSR injection
func (h *asset) AddMiddleContent(virtualPath string, content []byte) {
    h.contentMiddle = append(h.contentMiddle, &contentFile{
        path:    virtualPath,
        content: content,
    })
    h.cacheValid = false  // Invalidate cache
}

// In assetmin.go - SSR injection on initialization
func (c *AssetMin) injectSSRModules() {
    for _, module := range c.config.SSRModules {
        html := module.RenderHTML()
        c.indexHtmlHandler.AddMiddleContent(
            "ssr-"+module.ModuleName()+".html",
            []byte(html),
        )
    }
}
```

## Duplicate Prevention

The `parseExistingHtmlContent` function filters lines with:
- `class="module-"`
- `class="theme-"`
- `class="ssr-"` (new)

This prevents duplicate SSR content on rebuild.

## Integration with MemoryMode

SSR content is:
1. Rendered on module registration
2. Stored in `contentMiddle`
3. Included in minified cache
4. Served via HTTP in MemoryMode
5. Written to disk only in DiskMode

## Files to Create/Modify

| File | Action | Description |
|------|--------|-------------|
| `ssr.go` | Create | SSRModule interface |
| `assetmin.go` | Modify | Add SSRModules to config |
| `html.go` | Modify | Inject SSR content in htmlHandler |
