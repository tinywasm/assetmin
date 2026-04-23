# SSR Module Asset Extraction & Loading

`assetmin` supports automatic discovery and extraction of assets declared in `ssr.go` files (with build tag `!wasm`) from Go modules. This allows modules to ship their own CSS, JS, HTML, and SVG icons that are automatically bundled into the application.

## Asset Declaration (Contract)

Modules can expose their assets by including an `ssr.go` file in their root directory:

```go
//go:build !wasm

package mypkg

// CSS - string literal or via //go:embed
func RenderCSS() string { return `.my-class { color: red; }` }

// JS - string literal or via //go:embed
func RenderJS() string { return `console.log("hello from module")` }

// SVG icons - map literal inline
func IconSvg() map[string]string {
    return map[string]string{"icon-id": `<svg>...</svg>`}
}

// HTML SSR - string literal
func RenderHTML() string { return `<div class="my-widget"></div>` }
```

### Supported Extraction Patterns
The AST extractor supports:
- **String literals** and **Raw strings**.
- **String concatenation** (simple `+` operations).
- **Embedded files** via `//go:embed` (the extractor reads the referenced file).

## Automatic Discovery

When `Config.RootDir` is set to the project root (where `go.mod` exists), `assetmin` can automatically discover all modules used by the project using `go list -m -json all`.

### Loading Process
1. **Initial Load**: `NewAssetMin` triggers `LoadSSRModules()` in a background goroutine.
2. **Module Order**:
   - `tinywasm/dom`: Injected into the `open` slot (theme variables, always first).
   - External Modules: Injected into the `middle` slot (alphabetical order).
   - Root Project: Injected into the `close` slot (can override everything).

## Hot Reload

For local modules (via `replace` in `go.mod`), `assetmin` supports hot reloading of `ssr.go` changes.

When `ssr.go` is modified, the orchestrator (e.g., `tinywasm/app`) calls:
```go
am.ReloadSSRModule(moduleDir)
```
This re-extracts the assets and replaces them in the in-memory bundle without duplication.

## Manual Registration

If you have live instances of components that implement the SSR interfaces, you can register them manually:

```go
am.RegisterComponents(myComponent1, myComponent2)
```

## API Summary

- `LoadSSRModules() error`: Scans all project modules for `ssr.go` and loads assets.
- `ReloadSSRModule(dir string) error`: Reloads assets for a specific directory.
- `WaitForSSRLoad(timeout duration)`: Blocks until the background loading is complete (primarily for tests).
- `RegisterComponents(providers ...any)`: Registers component instances as asset providers.
