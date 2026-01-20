# AssetMin

**AssetMin** is a thread-safe, concurrent asset pipeline and bundler for Go web applications. It manages CSS, JavaScript, SVG sprites, and HTML templates, providing minification and caching.

## Features

- **Bundling & Minification**: CSS, JS, HTML, and SVG.
- **Component Registration**: Automatically extract assets from Go structs.
- **SSR Support**: Safe injection of public component HTML.
- **Memory & Disk Modes**: Serve directly from RAM or build to disk.
- **Hot Reload**: Thread-safe cache regeneration.

## Documentation

1.  [Architecture](docs/ARCHITECTURE.md)
2.  [Assets](docs/ASSETS.md)
3.  [Component Registration](docs/COMPONENT_REGISTRATION.md)
4.  [HTTP Handlers](docs/HTTP_HANDLERS.md)
5.  [SSR](docs/SSR.md)

## Installation

```bash
go get github.com/tinywasm/assetmin
```
