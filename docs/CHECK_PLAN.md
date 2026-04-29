# PLAN: Sprite SVG exclusivamente inline — eliminar ruta HTTP externa

## Estado actual (problema)

El sprite SVG existe en **dos lugares simultáneamente**:

1. **Inline en HTML** — `assetmin.go` usa `AddDynamicContent` para inyectar el sprite dentro del `<body>` de `index.html`.
2. **Archivo externo** — `RegisterRoutes` registra `/assets/icons.svg` como ruta HTTP adicional.

Esto genera ambigüedad: ¿cuál es la fuente de verdad? ¿Los componentes deben usar `<use href="#id">` (inline) o `url(icons.svg#id)` (externo)? Respuesta: el inline es el único que funciona (`<symbol>` + CSS mask no es compatible), pero la ruta externa existe y puede confundir.

Adicionalmente, el entorno dev de componentes individuales (`tinywasm/components/*/web/`) no registra los iconos del propio componente al arrancar, por lo que el sprite se genera vacío y los `<use>` no resuelven.

## Comportamiento objetivo

**Un solo comportamiento:** el sprite siempre va inline en el HTML. No existe ruta HTTP `/assets/icons.svg`.

```
IconSvg() → sprite SVG generado en memoria → inyectado en <body> del HTML
<svg><use href="#id">  →  resuelve contra el sprite inline
```

## Cambios en assetmin

### 1. `http.go` — eliminar ruta del sprite

```go
// ANTES
func (c *AssetMin) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc(c.indexHtmlHandler.GetURLPath(), ...)
    mux.HandleFunc(c.mainStyleCssHandler.GetURLPath(), ...)
    mux.HandleFunc(c.mainJsHandler.GetURLPath(), ...)
    mux.HandleFunc(c.spriteSvgHandler.GetURLPath(), ...)  // ← ELIMINAR
    mux.HandleFunc(c.faviconSvgHandler.GetURLPath(), ...)
}

// DESPUÉS
func (c *AssetMin) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc(c.indexHtmlHandler.GetURLPath(), ...)
    mux.HandleFunc(c.mainStyleCssHandler.GetURLPath(), ...)
    mux.HandleFunc(c.mainJsHandler.GetURLPath(), ...)
    mux.HandleFunc(c.faviconSvgHandler.GetURLPath(), ...)
}
```

### 2. `assetmin.go` — eliminar URL path del sprite

```go
// ELIMINAR esta línea — el sprite no tiene URL pública
c.spriteSvgHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, svgMainFileName)
```

El campo `spriteSvgHandler` y su inicialización se mantienen — sigue siendo necesario para generar el contenido del sprite que se inyecta inline. Solo se elimina su exposición HTTP.

### 3. `assetmin.go` — `svgMainFileName` pasa a ser constante interna

```go
// Antes: nombre de archivo usado también para URL
svgMainFileName := "icons.svg"

// Después: sigue siendo el nombre lógico del asset, sin URL asociada
// (sin cambio de código, solo de semántica)
```

### 4. `RefreshAsset` — sin cambios

El switch en `RefreshAsset` que refresca el sprite en `".svg"` se mantiene igual.

## Tests afectados

### Tests que NO cambian
- `TestSvgSpriteGeneration` — prueba `InjectSpriteIcon`, `ContainsSVG`, `HasIcon`. No depende de la ruta HTTP.
- `TestIconSpriteStructure` — prueba el contenido `<symbol>` generado. No depende de ruta HTTP.
- `TestSSRLoader` — prueba carga de CSS. No toca iconos.
- `TestRegisterRoutes` — prueba `/`, `/script.js`, `/style.css`. **No prueba `/assets/icons.svg`** — no cambia.
- `TestWorks` — prueba CSS a disco y prefijo URL. No toca iconos.

### Tests a verificar manualmente
- `favicon_test.go` — usa `RegisterRoutes` solo para favicon, no para sprite. Sin cambio.

### Tests nuevos (opcionales)
- `TestSpriteNotExposedAsRoute` — verifica que `GET /assets/icons.svg` devuelve 404 tras el refactor.
- `TestSpriteInjectedInHTML` — verifica que el HTML de `index.html` contiene el bloque `<svg class="sprite-icons">`.

## Bug: sprite vacío en entorno dev de componentes

### Causa
En `tinywasm/components/*/web/`, el servidor de desarrollo arranca el WASM pero no ejecuta `IconSvg()` del `ssr.go` del componente. El `AssetMin` se inicializa con sprite vacío y el `AddDynamicContent` inyecta un `<svg><defs></defs></svg>` vacío.

### Fix (en `tinywasm/app` o en el generador de entorno dev)
Al iniciar el proyecto de un componente, el setup debe:
1. Llamar a `LoadSSRModules()` apuntando al directorio del componente.
2. Asegurarse de que `IconSvg()` sea invocado durante la carga SSR y los iconos registrados via `InjectSpriteIcon` antes de servir el primer request HTML.

Esto ya es el comportamiento de `LoadSSRModules` + `ssr_extract.go` — el fix es garantizar que el entorno dev del componente lo invoque correctamente apuntando al paquete local.

## Orden de implementación

1. Eliminar `mux.HandleFunc(c.spriteSvgHandler.GetURLPath(), ...)` de `http.go`
2. Eliminar `c.spriteSvgHandler.urlPath = ...` de `assetmin.go`
3. Correr todos los tests existentes — ninguno debe fallar
4. Añadir `TestSpriteNotExposedAsRoute` y `TestSpriteInjectedInHTML`
5. Investigar y corregir el setup del entorno dev de componentes para que `LoadSSRModules` cubra el paquete local

## Impacto en componentes

Sin cambios en la API de componentes. `IconSvg()` se define igual en `ssr.go`. Los componentes usan `<svg><use href="#id">` en `Render()`. El CSS controla `fill` y `transform`.

No existe modo alternativo — hay una sola forma de usar iconos en el framework.
