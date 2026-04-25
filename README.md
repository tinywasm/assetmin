# AssetMin
<img src="docs/img/badges.svg">

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

### Diagrams
- [Core Architecture Flow](docs/diagrams/architecture.md)
- [Event & SSR Hot-Reload Sequence](docs/diagrams/event_flow_sequence.md)

## Installation

```bash
go get github.com/tinywasm/assetmin
```

## Get Started

Here is a quick example of how to initialize and use AssetMin in your project:

```go
package main

import (
	"log"

	"github.com/tinywasm/assetmin"
)

func main() {
	// 1. Configure AssetMin
	config := &assetmin.Config{
		OutputDir:       "./public",
		RootDir:         ".",
		AppName:         "MyApp",
		AssetsURLPrefix: "static",
		DevMode:         true, // Disable caching during development
	}

	// 2. Initialize the bundler
	bundler := assetmin.NewAssetMin(config)

	// 3. (Optional) Load SSR modules if using Server-Side Rendering
	// bundler.LoadSSRModules()

	// 4. You can now serve your minified assets via HTTP handlers!
	// See docs/HTTP_HANDLERS.md for more details.
	
	log.Println("Asset bundler is ready!")
}
```

For more advanced use cases, such as extracting assets from Go structs or setting up file watchers, please refer to the documentation links above.
