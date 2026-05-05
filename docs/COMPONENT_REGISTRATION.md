# Component Registration

`assetmin` allows Go structs to register their assets at runtime. Useful for modular architectures (e.g., `tinywasm/site`) and for any app that needs to inject computed content the AST extractor cannot see.

## Usage

```go
am := assetmin.NewAssetMin(config)
err := am.RegisterComponents(myComponent1, myComponent2)
```

Each component is inspected for the optional interfaces below. Implemented interfaces contribute their content to the corresponding slot; unimplemented ones are skipped.

## Supported interfaces

### Root CSS (theme)
```go
type RootCSSProvider interface {
    RootCSS() string
}
```
Routed to the `open` slot. Subject to the [single-override rule](SSR.md#single-override-rule-for-rootcss): runtime registration is treated as authoritative (the app is registering it explicitly), so it wins over `tinywasm/dom`'s fallback theme.

### Component CSS
```go
type CSSProvider interface {
    RenderCSS() string
}
```
Routed to the `middle` slot. Use this for component-scoped styles, NOT for `:root` tokens.

### JavaScript
```go
type JSProvider interface {
    RenderJS() string
}
```

### SVG icons
```go
type IconSvgProvider interface {
    IconSvg() map[string]string // {"id": "<svg>…</svg>"}
}
```

### HTML (SSR)
```go
type HTMLProvider interface {
    RenderHTML() string
}
```
*Only injected if the component is publicly readable — see [SSR](SSR.md).*

## When to use registration vs `ssr.go`

| Need | Mechanism |
|---|---|
| Static, build-time-known assets shipped with a Go module | `ssr.go` declarations (extracted via AST) |
| Dynamic content built from struct fields or runtime config | `RegisterComponents` |
| Custom theme generated at startup (e.g., from a config file) | `RegisterComponents` with `RootCSSProvider` |
