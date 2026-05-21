# AssetMin Architecture

`assetmin` is a server-side asset pipeline for Go web applications. It focuses on bundling and minifying CSS, JS, and SVG files, as well as providing SSR support for HTML components via automatic module discovery.

## Core Concepts

- **AssetMin**: The main struct that orchestrates the asset pipeline.
- **Handlers**: Specific handlers for different asset types (`mainStyleCssHandler`, `mainJsHandler`, `spriteSvgHandler`, `indexHtmlHandler`). It also supports dynamic handlers for standalone JS files (e.g., service workers).
- **Memory Mode vs. Disk Mode**: Assets can be served directly from memory or written to disk for static serving.
- **SSR Extraction**: Automatic extraction of assets from Go modules via compile-and-invoke mechanism (replaced AST parsing).
- **Slot System**: Content is organized into three slots to ensure correct loading order:
  - `open`: Base themes and CSS variables (e.g., from `tinywasm/css`).
  - `middle`: External module assets.
  - `close`: Application-specific overrides.

## Data Flow

1.  **Discovery**: `assetmin` uses `go list` to find all modules in the project.
2.  **Compilation**: A temporary `main.go` is generated that imports all modules and automatically detects their receiver type (or uses package-level functions).
3.  **Extraction**: `go run main.go` executes once, instantiating components via their detected types and calling asset methods (`RenderCSS()`, `RenderHTML()`, etc.), collecting results as JSON.
4.  **Caching**: Results are cached globally (`ssrGlobalCache`) using the hash of module Go files (not `rootDir`) as key, via `computeModuleHashSet`.
5.  **Injection**: Extracted assets are injected into the appropriate handlers and slots.
6.  **Processing**: Concatenation and Minification (using `tdewolff/minify`).
7.  **Output**: HTTP serving or File system writing.

## Compile-and-Invoke Mechanism

The extraction mechanism uses actual Go code compilation and execution:

- **Generated Main**: `assetmin` creates a temporary `main.go` that imports all discovered component modules.
- **Single Invocation**: All components are instantiated and their asset methods called in one `go run` execution.
- **Hash-based Cache**: Results are keyed by the MD5 hash of all module Go files; cached results are reused if hashes match.
- **Type Safety**: Supports typed CSS builders (e.g., `github.com/tinywasm/css`) and any dynamic Go expression in asset methods. The generator calls `.String()` on the concrete types.
- **Better DX**: Compiler errors from invalid components are surfaced verbatim to developers.

## Internal API and Future Optimizations

The internal API uses `extractSSRAssetsForModule(m, rootDir, modules, binCachePath)`. The `binCachePath` parameter is a reserved hook for future optimization (persistent binary) to further reduce extraction time below the cost of `go run`.

## Performance Baseline

The wall-time of the dev loop (edit -> extract) is approximately 300-500ms depending on the architecture, dominated by `go run` overhead. This is measured via `BenchmarkIncrementalChange` in `benchmark/benchmark_test.go`.

## Hot Reload

`assetmin` integrates with the dev server to support hot reloading. When a local asset file (`css.go`, `js.go`, `svg.go`, `html.go`, `ssr.go`) changes, the changes are re-extracted and the in-memory cache is invalidated, providing instant updates without restarting the server.
