# HTTP Handlers

`assetmin` provides a built-in HTTP handler to serve the bundled assets.

## Registration

```go
am := assetmin.NewAssetMin(config)
// Registers routes on the provided ServeMux
am.RegisterRoutes(myMux)
```

## Routes

- `/`: Serves `index.html`.
- `/{AssetsURLPrefix}/style.css`: Serves bundled CSS.
- `/{AssetsURLPrefix}/script.js`: Serves bundled JS.
- `/{AssetsURLPrefix}/sprite.svg`: Serves SVG sprite.
- `/{AssetsURLPrefix}/favicon.svg`: Serves favicon.

## Caching
Assets are served with `ETag` and `Cache-Control` headers. If an asset changes, the ETag updates automatically.
