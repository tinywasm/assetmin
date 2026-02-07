# Component Registration

`assetmin` allows Go structs to automatically register their assets. This is particularly useful for modular architectures like `tinywasm/site`.

## Usage

```go
am := assetmin.NewAssetMin(config)
err := am.RegisterComponents(myComponent1, myComponent2)
```

## Supported Interfaces

### CSS
```go
type CSSProvider interface {
    RenderCSS() string
}
```

### JavaScript
```go
type JSProvider interface {
    RenderJS() string
}
```

### SVG Icons
```go
type IconSvgProvider interface {
    IconSvg() map[string]string // {"id": "<svg>...</svg>"}
}
```

### HTML (SSR)
```go
type HTMLProvider interface {
    RenderHTML() string
}
```
*Note: HTML is only injected if the component is publicly readable (see SSR documentation).*
