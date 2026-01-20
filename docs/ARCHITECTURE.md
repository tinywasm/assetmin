# AssetMin Architecture

`assetmin` is a server-side asset pipeline for Go web applications. It focuses on bundling and minifying CSS, JS, and SVG files, as well as providing SSR support for HTML components.

## Core Concepts

- **AssetMin**: The main struct that orchestrates the asset pipeline.
- **Handlers**: Specific handlers for different asset types (`mainStyleCssHandler`, `mainJsHandler`, `spriteSvgHandler`, `indexHtmlHandler`).
- **Memory Mode vs. Disk Mode**: Assets can be served directly from memory or written to disk for static serving.
- **Component Registration**: A system to automatically extract assets from Go components.

## Data Flow

1.  **Input**: Files (via `NewFileEvent` or `RefreshAsset`) or Components (via `RegisterComponents`).
2.  **Processing**: Concatenation, Minification (using `tdewolff/minify`).
3.  **Caching**: In-memory caching with thread-safe access.
4.  **Output**: HTTP serving or File system writing.
