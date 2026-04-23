# AssetMin Architecture

`assetmin` is a server-side asset pipeline for Go web applications. It focuses on bundling and minifying CSS, JS, and SVG files, as well as providing SSR support for HTML components via automatic module discovery.

## Core Concepts

- **AssetMin**: The main struct that orchestrates the asset pipeline.
- **Handlers**: Specific handlers for different asset types (`mainStyleCssHandler`, `mainJsHandler`, `spriteSvgHandler`, `indexHtmlHandler`).
- **Memory Mode vs. Disk Mode**: Assets can be served directly from memory or written to disk for static serving.
- **SSR Extraction**: Automatic extraction of assets from Go modules via `ssr.go` file analysis (AST parsing).
- **Slot System**: Content is organized into three slots to ensure correct loading order:
  - `open`: Base themes and CSS variables (e.g., from `tinywasm/dom`).
  - `middle`: External module assets.
  - `close`: Application-specific overrides.

## Data Flow

1.  **Discovery**: `assetmin` uses `go list` to find all modules in the project.
2.  **Extraction**: For each module, it looks for `ssr.go` and extracts CSS, JS, HTML, and SVG icons using AST parsing.
3.  **Injection**: Extracted assets are injected into the appropriate handlers and slots.
4.  **Processing**: Concatenation and Minification (using `tdewolff/minify`).
5.  **Caching**: In-memory caching with thread-safe access.
6.  **Output**: HTTP serving or File system writing.

## Hot Reload

`assetmin` integrates with the dev server to support hot reloading. When a local `ssr.go` file changes, the changes are re-extracted and the in-memory cache is invalidated, providing instant updates without restarting the server.
