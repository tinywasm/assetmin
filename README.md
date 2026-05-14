# AssetMin
<img src="docs/img/badges.svg">

**AssetMin** is a thread-safe, concurrent asset pipeline and bundler for Go web applications. It manages CSS, JavaScript, SVG sprites, and HTML templates, providing minification and caching.

## Features

- **Bundling & Minification**: CSS, JS, HTML, and SVG.
- **SSR Support**: Automatic discovery and extraction of assets from Go modules.
- **Component Registration**: Extract assets from live Go structs.
- **Typed CSS**: Native support for `github.com/tinywasm/css` stylesheets.
- **Memory & Disk Modes**: Serve directly from RAM or build to disk.
- **Hot Reload**: Instant thread-safe cache regeneration.

## Documentation

1.  [Architecture](docs/ARCHITECTURE.md)
2.  [Assets](docs/ASSETS.md)
3.  [Component Registration](docs/COMPONENT_REGISTRATION.md)
4.  [SSR & Module Extraction](docs/SSR.md)
5.  [API Reference](docs/API.md)
6.  [HTTP Handlers](docs/HTTP_HANDLERS.md)

### Diagrams
- [Core Architecture Flow](docs/diagrams/architecture.md)
- [Event & SSR Hot-Reload Sequence](docs/diagrams/event_flow_sequence.md)

## Performance

The compile-and-invoke SSR extraction mechanism provides:
- **Fast cold extraction**: ~450ms for typical projects.
- **Hash-based caching**: Instant warm extractions (~1ms).
- **Full typed CSS support** without performance penalty.

See [Benchmark Suite](benchmark/README.md) for detailed performance measurements.

## Installation

```bash
go get github.com/tinywasm/assetmin
```

## Get Started

```go
package main

import (
	"log"
	"github.com/tinywasm/assetmin"
)

func main() {
	config := &assetmin.Config{
		OutputDir:       "./public",
		RootDir:         ".",
		AppName:         "MyApp",
		AssetsURLPrefix: "/static/",
	}

	am := assetmin.NewAssetMin(config)

	// Load assets from imported Go modules asynchronously
	am.LoadSSRModules()
	
	log.Println("Asset bundler is ready!")
}
```

For more advanced use cases, such as registration of components or setting up file watchers, please refer to the documentation.
