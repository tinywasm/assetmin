# Asset Management

`assetmin` manages four primary types of assets:

## CSS (`style.css`)
- **Handler**: `mainStyleCssHandler`
- **Source**: `.css` files in modules or registered components.
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
