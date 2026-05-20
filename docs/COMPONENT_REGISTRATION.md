# Component Registration

`assetmin` allows Go structs to register their assets at runtime. Useful for modular architectures and for any app that needs to inject computed content.

## Usage

```go
am := assetmin.NewAssetMin(config)
err := am.RegisterComponents(myComponent1, myComponent2)
```

Each component is inspected for the optional interfaces below. Implemented interfaces contribute their content to the corresponding slot; unimplemented ones are skipped.

## Supported interfaces

### Root CSS (theme)
```go
import "github.com/tinywasm/css"

type rootCssProvider interface {
    RootCSS() *css.Stylesheet
}
```
Routed to the `open` slot. Subject to the [single-override rule](SSR.md#single-override-rule-for-rootcss): runtime registration is treated as authoritative (the app is registering it explicitly), so it wins over the framework's fallback theme.

### Component CSS
```go
import "github.com/tinywasm/css"

type cssProvider interface {
    RenderCSS() *css.Stylesheet
}
```
Routed to the `middle` slot. Use this for component-scoped styles, NOT for `:root` tokens.

### JavaScript
```go
type jsProvider interface {
    RenderJS() string
}
```

### SVG icons
```go
type iconProvider interface {
    IconSvg() map[string]string // {"id": "<svg>…</svg>"}
}
```

### HTML (SSR)
```go
type htmlProvider interface {
    RenderHTML() string
}
```

## Conventions

### Automatic Discovery
For automatic module discovery via `ssr.go`, `assetmin` automatically detects the receiver type of your methods and instantiates the component. You no longer need to export an `SSRInstance()` function.

### Typed CSS
Both `RootCSS()` and `RenderCSS()` use the concrete type `*css.Stylesheet` from the `github.com/tinywasm/css` library. This ensures type safety and allows the use of Go-based CSS DSLs. The extractor and `RegisterComponents` call `.String()` on these objects to get the raw CSS.

## When to use registration vs `ssr.go`

| Need | Mechanism |
|---|---|
| Assets shipped with a Go module | `ssr.go` declarations (extracted via compile-and-invoke) |
| Dynamic content built from struct fields or runtime config | `RegisterComponents` |
| Custom theme generated at startup (e.g., from a config file) | `RegisterComponents` with `rootCssProvider` |
