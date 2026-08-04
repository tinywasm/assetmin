# Asset Management

`assetmin` manages five primary types of assets:

## CSS (`style.css`)
- **Handler**: `mainStyleCssHandler`
- **Source**: `.css` files in modules or registered components; plus dynamic `@font-face` when the root module declares fonts.
- **Processing**: Minified using `tdewolff/minify/css`.

## JavaScript (`script.js`)
- **Handler**: `mainJsHandler`
- **Source**: `.js` files in modules or registered components.
- **Processing**: Minified using `tdewolff/minify/js`. Supports "use strict" removal and runtime wrapper injection.

## SVG Sprites (Inline)
- **Handler**: `spriteSvgHandler`
- **Source**: Individual `.svg` icons.
- **Processing**: Wrapped in `<symbol>` tags and combined into a single sprite sheet.
- **Delivery**: Injected directly into the `<body>` of the main HTML file.

## HTML (`index.html`)
- **Handler**: `indexHtmlHandler`
- **Source**: `index.html` template and SSR content from components.
- **Processing**: Minified using `tdewolff/minify/html`.

## Fonts (`.ttf` faces)
- **Source**: `Fonts() font.Declaration` from the **root** module only (extracted by `tinywasm/ssr` from `fonts.go`).
- **Processing**: The four faces (`Family.Face(Style) + ".ttf"`) are copied from `RootDir/<Dir()>` into `OutputDir` when missing or stale. Missing face → hard error naming the file.
- **CSS**: `css.FontFaces(d, AssetsURLPrefix)` is injected into `style.css` as dynamic content (`format("truetype")`, `font-display: swap`).
- **Not** registered as a concatenating asset handler and **not** in `SupportedExtensions()` — binaries must not enter the text merger. Hot-reload of `.ttf` bytes is deliberately unsupported; edit `fonts.go` to re-extract the declaration.
- Non-root modules that declare `Fonts()` are ignored with a log warning (same single-override rule as `RootCSS()`).
