# PLAN: tinywasm/assetmin — Fix Hotreload + Convención `X.go` + image/min inyectado

## Repositorio
`github.com/tinywasm/assetmin` — path local: `tinywasm/assetmin/`

## Dependencias de ejecución
```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

## Decisiones de diseño (confirmadas)

1. **Convención única `X.go`** — el nombre del archivo Go indica el tipo de asset.
   `css.go`, `js.go`, `svg.go`, `html.go` → **extraer** contenido (string) y fusionar/inyectar.
   `image.go` → **procesar** imágenes (no se extrae string). **`ssr.go` eliminado** (sin legacy).
2. **image unificado vía interfaz inyectada** — assetmin reconoce `image.go` en el dispatch,
   pero **no** importa `image/min` ni hace el trabajo WebP. Delega a una interfaz
   `ImageProcessor` que `tinywasm/app` inyecta con el `image/min` real (tests: mock).
3. **Detección de cambio de imagen por mtime** (en `image/min`, no en assetmin) — sin manifest
   ni hash: los `.webp` son el cache; requiere `OutputDir` gitignored.

---

## Problema 1: Hotreload Solo Dispara el Browser Sin Reextraer SSR

### Root Cause (verificado en código fuente)

**Archivo:** `tinywasm/devflow/go_mod.go`

```go
// LÍNEA ~257 — BUG:
func (g *GoModHandler) SupportedExtensions() []string {
    return []string{".mod"}  // solo recibe eventos de archivos .mod
}
```

`devwatch` filtra por extensión ANTES de llamar `NewFileEvent`. Como `GoModHandler` solo
declara `[".mod"]`, los eventos `.go` (incluyendo `css.go`, `svg.go`, `html.go`, `image.go`)
nunca llegan. gobuild SÍ recibe eventos `.go` → recompila WASM → recarga browser, pero los
assets SSR quedan obsoletos.

### Solución: `SSRFileWatcher` en assetmin

**Por qué NO modificar GoModHandler:** su única responsabilidad es `go.mod`. La interface
`devwatch.FilesEventHandlers` existe exactamente para esto. assetmin expone `SSRFileWatcher`
y el orquestador (`tinywasm/app`) lo registra en el watcher.

---

## Cambio 1: Crear `assetmin/ssr_watcher.go`

Clasifica los `.go` en dos grupos según la acción:

```go
package assetmin

import (
    "path/filepath"
    "slices"
)

// ssrTextAssetFiles: archivos Go cuyo contenido se EXTRAE (string) y se fusiona/inyecta.
// Adding a new text asset type requires only adding it here.
var ssrTextAssetFiles = []string{
    "css.go",
    "js.go",
    "svg.go",
    "html.go",
}

// imageAssetFile: archivo Go que DECLARA imágenes a procesar (no se extrae string).
// Se delega al ImageProcessor inyectado.
const imageAssetFile = "image.go"

// SSRFileWatcher implements devwatch.FilesEventHandlers.
// Watches .go events; routes only recognized asset-source files.
type SSRFileWatcher struct {
    am              *AssetMin
    onBrowserReload func() error
}

// NewSSRFileWatcher creates an SSRFileWatcher bound to this AssetMin instance.
// onBrowserReload is called after a successful reload (pass h.Browser.Reload in app).
func (am *AssetMin) NewSSRFileWatcher(onBrowserReload func() error) *SSRFileWatcher {
    return &SSRFileWatcher{am: am, onBrowserReload: onBrowserReload}
}

// MainInputFileRelativePath DEBE devolver "go.mod" (igual que GoModHandler), NO "css.go".
// devwatch, para eventos .go, llama depFinder.ThisFileIsMine(MainInputFileRelativePath(), ...)
// que hace os.Stat(rootDir + ese path) y FALLA si no existe → el handler se saltaría (continue)
// y el watcher NUNCA correría. "go.mod" existe en root y enruta todos los .go del proyecto al
// handler (checkPackageBasedOwnership), que luego filtramos por nombre en NewFileEvent.
func (w *SSRFileWatcher) MainInputFileRelativePath() string { return "go.mod" }
func (w *SSRFileWatcher) SupportedExtensions() []string     { return []string{".go"} }
func (w *SSRFileWatcher) UnobservedFiles() []string         { return nil }

// NewFileEvent routes a .go event to the correct action.
//   css/js/svg/html.go → ReloadSSRModule (text extraction)
//   image.go           → imageProcessor.ReloadModule (WebP pipeline, inyectado)
//   otro .go           → ignore
func (w *SSRFileWatcher) NewFileEvent(fileName, extension, filePath, event string) error {
    moduleDir := filepath.Dir(filePath)

    switch {
    case slices.Contains(ssrTextAssetFiles, fileName):
        if err := w.am.ReloadSSRModule(moduleDir); err != nil {
            w.am.Logger("SSR hot reload error:", moduleDir, err)
            return err
        }
    case fileName == imageAssetFile:
        if w.am.imageProcessor == nil {
            return nil // no processor injected — ignore
        }
        if err := w.am.imageProcessor.ReloadModule(moduleDir); err != nil {
            w.am.Logger("image hot reload error:", moduleDir, err)
            return err
        }
    default:
        return nil // not an asset source — ignore silently
    }
    // NOTA (caso B): el routing es por NOMBRE de archivo, no por contenido. Un image.go (o
    // css.go, etc.) recién creado y aún SIN su Render* dispara igual: ExtractImages/extracción
    // devuelve vacío (no-op inofensivo). Así, en cuanto el dev/LLM agrega RenderImages(), el
    // hot reload YA funciona — no hay que reiniciar para que el archivo "empiece a escucharse".

    if w.onBrowserReload != nil {
        if err := w.onBrowserReload(); err != nil {
            w.am.Logger("browser reload error:", err)
        }
    }
    return nil
}
```

---

## Cambio 2: `ImageProcessor` — interfaz inyectada + dispatch

assetmin **define** la interfaz (estrecha) y **no** importa `image/min`. La implementa
estructuralmente `github.com/tinywasm/image/min` (sin importar assetmin → sin ciclo).

### 2a: `assetmin/image_processor.go` (nuevo)

```go
package assetmin

// ImageProcessor procesa imágenes declaradas en los image.go de los módulos.
// Implementado por github.com/tinywasm/image/min; inyectado por el composition root (app).
// Los nombres coinciden con los métodos existentes de image/min: cero churn en el impl.
type ImageProcessor interface {
    LoadImages() error                    // escaneo completo inicial (startup)
    ReloadModule(moduleDir string) error  // reproceso de un módulo (image.go cambió)
    UnobservedFiles() []string            // outputs .webp a excluir del watcher
}
```

### 2b: campo + setter en `assetmin.go`

En el struct `AssetMin`, agregar:
```go
imageProcessor ImageProcessor // inyectado por app; nil en tests sin imágenes
```

Setter (archivo `image_processor.go`):
```go
// SetImageProcessor inyecta el pipeline de imágenes. Pasar nil lo desactiva.
func (c *AssetMin) SetImageProcessor(p ImageProcessor) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.imageProcessor = p
}
```

### 2c: escaneo inicial en el load SSR

`LoadImages()` hace su propio descubrimiento de módulos (`go list`) — es un **one-shot global**.
Se invoca **una vez** dentro de `LoadSSRModules()`, junto al `ssrExtractor.ExtractAll()`
(ver el cuerpo completo de `LoadSSRModules` en Cambio 7c — ahí están ambas llamadas).

> **Inyección set-once:** `SetImageProcessor` y `SetSSRExtractor` se llaman una sola vez al
> iniciar, **antes** de registrar el `SSRFileWatcher` y de `LoadSSRModules`. Como los campos no
> cambian tras el arranque, la lectura en el hot path no necesita lock (los setters sí lo toman
> por consistencia).

### 2d: UnobservedFiles incluye outputs de imagen

En `assetmin/events.go`, `func (c *AssetMin) UnobservedFiles()` agrega los del processor:
```go
out := []string{
    c.mainStyleCssHandler.outputPath,
    c.mainJsHandler.outputPath,
    c.spriteSvgHandler.outputPath,
}
if c.imageProcessor != nil {
    out = append(out, c.imageProcessor.UnobservedFiles()...)
}
return out
```

**Archivos afectados:** `image_processor.go` (nuevo), `assetmin.go`, `ssr_loader.go`, `events.go`

---

## Cambio 3: Eliminar soporte legacy `ssr.go`

`ssr.go` como archivo SSR multi-función queda **eliminado** (decisión: una sola convención).

- En `ssr_extract.go`, la lista de archivos fuente queda **solo** con los de extracción de texto:
  ```go
  // ANTES:
  var ssrSourceFiles = []string{"css.go", "js.go", "svg.go", "html.go", "ssr.go"}
  // DESPUÉS:
  var ssrSourceFiles = []string{"css.go", "js.go", "svg.go", "html.go"}
  ```
  > `image.go` **no** va aquí — no se extrae contenido; lo maneja el dispatch del Cambio 1
  > vía `imageProcessor`.

- Buscar y eliminar cualquier rama que mencione `"ssr.go"` en `ssr_extract.go`, `ssr_invoke.go`,
  `ssr_register.go`, `events.go`. No hay reemplazo legacy.

- **Test:** `assetmin/ssr_extract_subpackage_test.go` prueba la extracción por codegen → **se
  mueve a `tinywasm/ssr`** (ver Cambio 7 y ssr PLAN Paso 5), NO se reescribe en assetmin. Allí el
  fixture debe escribir `css.go` (no `ssr.go`).

---

## Cambio 4: Actualizar `tinywasm/app/section-build.go`

Ver detalle completo en `tinywasm/app/docs/PLAN.md`. Resumen del lado assetmin:

- El callback `onBrowserReload` del `SSRFileWatcher` **solo** recarga el browser
  (el reproceso de imagen ya ocurre dentro del dispatch del Cambio 1):
  ```go
  ssrWatcher := h.AssetsHandler.NewSSRFileWatcher(func() error {
      return h.Browser.Reload()
  })
  h.Watcher.AddFilesEventHandlers(ssrWatcher)
  ```
- app construye `image/min` y lo inyecta: `h.AssetsHandler.SetImageProcessor(imgProc)`.
- app construye `tinywasm/ssr` y lo inyecta: `h.AssetsHandler.SetSSRExtractor(ssr.New(...))`.
- **Eliminar** el registro de `image/min` como handler separado de devwatch/TUI.

---

## Cambio 5: Migrar sprite a *svg.Sprite + separar responsabilidad

**Principio:** `tinywasm/svg` construye el HTML del sprite. `assetmin` solo acumula e inyecta.

### 5a: `*svg.Sprite` master interno

En `assetmin.go` (struct `AssetMin`), agregar:
```go
masterSprite *svg.Sprite  // acumula todos los íconos de componentes
```
Inicializar en `New()`:
```go
c.masterSprite = svg.New()
```

### 5b: Reemplazar `addIcon` por `Merge`

**`assetmin/svg.go`:** eliminar `addIcon` (XML decoder, `<symbol>`, viewBox extractor):
```go
func (c *AssetMin) mergeSprite(s *svg.Sprite) {
    c.masterSprite.Merge(s)
}
```

> ⚠️ **Preservar el favicon.** En `events.go`, el branch `.svg` distingue favicon vs sprite:
> ```go
> if extension == ".svg" && filepath.Base(filePath) != c.faviconSvgHandler.fileOutputName {
>     return c.addIcon(fileName, string(content))  // ← ELIMINAR solo esta rama (sprite raw)
> }
> ```
> Eliminar **solo** la rama del sprite (`addIcon`). El `faviconSvgHandler` y su escucha de `.svg`
> **se mantienen**: `favicon.svg` es un archivo genuino (no Go-autorado) que se minifica y copia.
> Por eso `SupportedExtensions()` conserva `.svg` (para favicon), aunque los íconos del sprite
> ahora vengan de `IconSvg() *svg.Sprite` vía `svg.go`.

**`assetmin/ssr_register.go`:**
```go
// DESPUÉS:
type svgProvider interface{ IconSvg() *svg.Sprite }
if sp, ok := p.(svgProvider); ok {
    c.mergeSprite(sp.IconSvg())
}
```

### 5c: Inyectar `masterSprite.String()`

Donde hoy se inyecta el contenido del `spriteSvgHandler`, usar `c.masterSprite.String()`.
> `spriteSvgHandler` (archivo `.svg` físico) se mantiene para `icons.svg` estático y favicon
> (file serving, concern distinto).

### 5d: codegen del extractor (ahora en `tinywasm/ssr`)

⚠️ `ssr_invoke.go` **se mueve a `tinywasm/ssr`** (ver Cambio 7). Esta edición del template del
codegen aplica al archivo movido (`ssr/invoke.go`):
```go
{{if .HasIcons}}s.Icons = inst.IconSvg(){{end}}   // *svg.Sprite, serializado a JSON por el IPC
```
`reIconSvg` sigue buscando `func ... IconSvg()`, solo cambia el tipo de retorno (`*svg.Sprite`).
El merge al `masterSprite` lo hace assetmin al **consumir** el `SSRAssets.Icons` (Cambio 7),
no el programa generado. Requiere que `*svg.Sprite` sea JSON round-trip (ver ssr PLAN Paso 3).

### 5e: Cambiar la firma del parámetro `icons` en toda la cadena
El tipo `icons map[string]string` pasa a `*svg.Sprite` en **toda** la cadena de funciones:
```go
RegisterComponents(...)            // ip.IconSvg() ya devuelve *svg.Sprite
UpdateSSRModule(name, css, scripts, html, icons *svg.Sprite) error
UpdateSSRModuleInSlot(..., icons *svg.Sprite, slot string) error
updateSSRModuleInSlot(..., icons *svg.Sprite, slot string) error
routeAssets(a *SSRAssets, ...)     // a.Icons es *svg.Sprite
```
Donde antes se iteraba el `map` para `addIcon`, ahora `c.masterSprite.Merge(icons)`.
El `iconProvider interface{ IconSvg() map[string]string }` → `svgProvider{ IconSvg() *svg.Sprite }`
(Cambio 5b) — afecta tanto `RegisterComponents` (in-process) como la detección codegen.

### 5f: go.mod
```
require github.com/tinywasm/svg v<nueva-version>   # v0.0.3 con JSON round-trip (ver svg PLAN)
```

**Archivos afectados:** `assetmin.go`, `svg.go`, `ssr_register.go`, `events.go`, `ssr_loader.go`,
`go.mod`. (El codegen `ssr_invoke.go` se edita ya movido en `tinywasm/ssr`.)

---

## Cambio 6: HTMLProvider — `RenderHTML() string`

`*html.HTML` fue eliminado. Contrato simplificado `RenderHTML() string`. La detección existe en
**dos lugares** (mantener ambos coherentes):

1. **In-process** (`ssr_register.go` `RegisterComponents`, se queda en assetmin): el
   `htmlProvider interface{ RenderHTML() string }` ya usa string — verificar que sea `string`,
   no `*html.HTML`.
2. **Codegen** (regex `reRenderHTML` + template, **se mueve a `tinywasm/ssr`**): el `main.go`
   generado hace `s.HTML = inst.RenderHTML()` (string directo).

```go
// forma in-process:
if prov, ok := comp.(interface{ RenderHTML() string }); ok {
    html = prov.RenderHTML()
}
```
> Sin legacy: `RenderHTML() *html.HTML` nunca se publicó. Break change intencional.

Ejemplo en `assetmin/docs/SSR.md`:
```go
// html.go
//go:build !wasm
package mycomponent
import . "github.com/tinywasm/html"

func (c *MyComponent) RenderHTML() string {
    return Div(clsRoot.AsAttr()).String()
}
```

---

## Cambio 7: Delegar la extracción SSR a `tinywasm/ssr` (inyectada)

**Tesis:** assetmin = **bundler/minifier** puro. La extracción (codegen + `go run` sobre el
proyecto) se va a `tinywasm/ssr`, simétrico con `image/min`. assetmin **define el contrato** y
**no importa** `tinywasm/ssr` (sigue sin `os/exec`/toolchain en su código).

### 7a: Definir el contrato en assetmin (`ssr_extractor.go`, nuevo)

```go
package assetmin

// SSRAssets es el DTO de assets crudos por módulo (lo produce tinywasm/ssr).
type SSRAssets struct {
    ModuleName  string
    RootCSS     string
    CSS         string
    JS          []*js.Script
    HTML        string
    Icons       *svg.Sprite   // ← antes map[string]string (Cambio 5)
    IsRoot      bool
    IsFramework bool
}

// SSRExtractor lo implementa github.com/tinywasm/ssr; lo inyecta app.
type SSRExtractor interface {
    ExtractModule(moduleDir string) (*SSRAssets, error)
    ExtractAll() ([]*SSRAssets, error)
}

func (c *AssetMin) SetSSRExtractor(e SSRExtractor) {
    c.mu.Lock(); defer c.mu.Unlock()
    c.ssrExtractor = e
}
```
> El tipo `SSRAssets` ya existe en `ssr_extract.go`; al moverse el archivo a `tinywasm/ssr`,
> su definición **se queda en assetmin** (es el contrato). Agregar campos `IsRoot`/`IsFramework`
> y cambiar `Icons` a `*svg.Sprite`.

### 7b: Mover los archivos de extracción a `tinywasm/ssr`

**Eliminar de assetmin** (van a `tinywasm/ssr`, ver ssr PLAN): `ssr_invoke.go`, `ssr_extract.go`
(salvo el `type SSRAssets`, que queda), `ssr_cache.go`, `import_scanner.go`.

**Código dependiente del scanner que también se elimina/mueve** (porque `importScanner` se va):
- `assetmin.go`: campo `scanner *importScanner` y su init `newImportScanner()` en `NewAssetMin`.
- `inspect.go`: `TestOnly_ScanImports` y `TestOnly_ModuleSubpackagesUsed` (usan `c.scanner`) →
  mover a `tinywasm/ssr` como helpers de test, o eliminar.
- `assetmin.go`: `SetListModulesFn` + campo `listModulesFn` quedan **obsoletos** — la discovery
  de módulos ahora vive en `ssr` (`Extractor.SetListModulesFn`). Eliminar de assetmin; app pasa
  el `listModulesFn` al extractor (ver app PLAN 3g), no a assetmin.

### 7c: `ReloadSSRModule` / `LoadSSRModules` delegan, `routeAssets` se queda

`ssr_loader.go` se **divide**: la parte de extracción (`loadSSRModulesLocked` con `go list` +
scanner) se va a `ssr`; la parte de **routing** se queda. assetmin conserva el routing y delega
solo la extracción:
```go
func (c *AssetMin) ReloadSSRModule(moduleDir string) error {
    if c.ssrExtractor == nil { return nil }
    a, err := c.ssrExtractor.ExtractModule(moduleDir)
    if err != nil || a == nil { return err }
    c.routeAssets(a, a.IsRoot, a.IsFramework)
    return nil
}

func (c *AssetMin) LoadSSRModules() {  // se mantiene la orquestación async (ScheduleSSRLoad/WaitForSSRLoad)
    // 1) assets de texto/svg vía el extractor SSR inyectado:
    if c.ssrExtractor != nil {
        if all, err := c.ssrExtractor.ExtractAll(); err == nil {
            for _, a := range all { c.routeAssets(a, a.IsRoot, a.IsFramework) }
        } else { c.Logger("SSR ExtractAll error:", err) }
    }
    // 2) imágenes vía el ImageProcessor inyectado (Cambio 2c):
    if c.imageProcessor != nil {
        if err := c.imageProcessor.LoadImages(); err != nil { c.Logger("image load error:", err) }
    }
}
```
**Se quedan** en assetmin: `routeAssets`, `resolveAndApplyRootCSS`, `ScheduleSSRLoad`,
`WaitForSSRLoad`, `UpdateSSRModule(InSlot)`, `RegisterComponents` (in-process, type assertions).

### 7d: Icons como `*svg.Sprite` en el routing
`routeAssets`/`updateSSRModuleInSlot` reciben `*svg.Sprite` y hacen `c.masterSprite.Merge(a.Icons)`
(Cambio 5). El path in-process (`RegisterComponents`) y el codegen (vía `SSRExtractor`) convergen
en el mismo merge.

**Archivos afectados:** `ssr_extractor.go` (nuevo), `assetmin.go` (campo `ssrExtractor`),
`ssr_loader.go` (delegación), eliminar los 4 archivos movidos.

---

## Cambio 8 (Tier 2): mover lógica de tipo-específico a su paquete

1. **Shell `index.html` → `tinywasm/html`.** `html.go`/`NewHtmlHandler` construye el documento a
   mano. Reemplazar por un builder `html.Document(opts)` en `tinywasm/html` (doctype, `<head>`
   con links css/js/favicon, `<body>` con punto de inyección). assetmin importa `html`, rellena
   URLs e inyecta sprite/HTML de componentes. Ver html PLAN.

2. **`use_strict.go` → `tinywasm/js`.** Mover `stripLeadingUseStrict` y el prefijo
   `'use strict';` a `tinywasm/js` (`js.StripLeadingUseStrict`, `js.UseStrictPrefix`). assetmin
   los llama en `events.go`/`startCodeJS`. Ver js PLAN.

3. **`urlRewrite.go` → `tinywasm/html`.** `rewriteAssetUrls` reescribe `href`/`src` de HTML →
   pertenece a `tinywasm/html` (`html.RewriteAssetURLs(html, newRoot)`). assetmin lo llama en
   `inspect.go`. Ver html PLAN.

> Estos tres son refinamientos de cohesión (lógica de un tipo viviendo en el bundler genérico).
> No cambian comportamiento, solo ubicación + un import.

---

## Tests

### `assetmin/ssr_watcher_test.go`

```go
package assetmin_test

import (
    "testing"
    "github.com/tinywasm/assetmin"
)

// mockImageProcessor implementa assetmin.ImageProcessor para tests.
type mockImageProcessor struct {
    reloadedDir string
    loadCalled  bool
}

func (m *mockImageProcessor) LoadImages() error { m.loadCalled = true; return nil }
func (m *mockImageProcessor) ReloadModule(dir string) error { m.reloadedDir = dir; return nil }
func (m *mockImageProcessor) UnobservedFiles() []string { return []string{"web/public/img"} }

func TestSSRFileWatcher_SupportedExtensions(t *testing.T) {
    am := assetmin.NewAssetMin(&assetmin.Config{RootDir: t.TempDir()})
    w := am.NewSSRFileWatcher(nil)
    if exts := w.SupportedExtensions(); len(exts) != 1 || exts[0] != ".go" {
        t.Fatalf("expected [.go], got %v", exts)
    }
}

func TestSSRFileWatcher_IgnoresNonAssetFiles(t *testing.T) {
    reloads := 0
    am := assetmin.NewAssetMin(&assetmin.Config{RootDir: t.TempDir()})
    w := am.NewSSRFileWatcher(func() error { reloads++; return nil })
    for _, name := range []string{"main.go", "handler.go", "model.go", "ssr.go"} {
        _ = w.NewFileEvent(name, ".go", "/proj/"+name, "write")
    }
    if reloads != 0 {
        t.Fatalf("expected 0 reloads for non-asset files (incl. deprecated ssr.go), got %d", reloads)
    }
}

func TestSSRFileWatcher_RoutesImageToProcessor(t *testing.T) {
    mp := &mockImageProcessor{}
    am := assetmin.NewAssetMin(&assetmin.Config{RootDir: t.TempDir()})
    am.SetImageProcessor(mp)
    w := am.NewSSRFileWatcher(func() error { return nil })

    _ = w.NewFileEvent("image.go", ".go", "/proj/comp/image.go", "write")
    if mp.reloadedDir != "/proj/comp" {
        t.Fatalf("expected image.go routed to processor, got %q", mp.reloadedDir)
    }
}

// mockSSRExtractor implementa assetmin.SSRExtractor para probar el branch de texto sin codegen.
type mockSSRExtractor struct{ extractedDir string }

func (m *mockSSRExtractor) ExtractModule(dir string) (*assetmin.SSRAssets, error) {
    m.extractedDir = dir
    return &assetmin.SSRAssets{ModuleName: dir}, nil
}
func (m *mockSSRExtractor) ExtractAll() ([]*assetmin.SSRAssets, error) { return nil, nil }

func TestSSRFileWatcher_RoutesTextToExtractor(t *testing.T) {
    me := &mockSSRExtractor{}
    am := assetmin.NewAssetMin(&assetmin.Config{RootDir: t.TempDir()})
    am.SetSSRExtractor(me)
    w := am.NewSSRFileWatcher(func() error { return nil })

    _ = w.NewFileEvent("css.go", ".go", "/proj/comp/css.go", "write")
    if me.extractedDir != "/proj/comp" {
        t.Fatalf("expected css.go routed to SSR extractor, got %q", me.extractedDir)
    }
}
```

> Estos unit tests **saltan** `depFinder.ThisFileIsMine` (llaman `NewFileEvent` directo). El gate
> real (`MainInputFileRelativePath() == "go.mod"`) se cubre con el test de integración en
> `tinywasm/app` (ver app PLAN Cambio 4).

---

## Verificación

```bash
cd tinywasm/assetmin
gotest

cd tinywasm/app
gotest
```

Verificación manual de hotreload:
1. Iniciar `tinywasm/layout/platformd/web` con el servidor de desarrollo
2. Modificar `css.go` (un color) → browser recarga con el nuevo CSS
3. Modificar `image.go` (agregar una imagen) → se genera el WebP y recarga
4. **Sin** reiniciar el servidor

---

## Documentación a Actualizar

### `assetmin/docs/SSR.md`
- **Asset Declaration (Contract)** — convención `X.go` uniforme (el contenido se autoría **en Go**
  vía los builders `tinywasm/css|js|svg`, no en archivos `.css`/`.js` sueltos):
  - `css.go`  → `RenderCSS() *css.Stylesheet`
  - `js.go`   → `RenderJS() []*js.Script`  (slice → varios scripts por módulo; varios módulos suman)
  - `svg.go`  → `IconSvg() *svg.Sprite`
  - `html.go` → `RenderHTML() string`
  - `image.go`→ `RenderImages() []image.Asset` (lo procesa `image/min` inyectado, no va por slots)
  - **Eliminar** toda mención a `ssr.go`.
  - Aclarar: una sola regla para todos los tipos → "el asset se declara en `<tipo>.go` con su
    `Render<Tipo>()`". El `SSRFileWatcher` escucha `.go`; la propiedad es el módulo del componente.
- **Hot reload** — describir `SSRFileWatcher` y el routing texto-vs-imagen.
- **API summary** — agregar `NewSSRFileWatcher(fn)` y `SetImageProcessor(p)`.

### `assetmin/docs/ARCHITECTURE.md`
- **Data Flow** — tipos de retorno actuales:
  ```
  - RenderCSS()    → *css.Stylesheet (.String())
  - RenderHTML()   → string
  - IconSvg()      → *svg.Sprite (mergeado al masterSprite)
  - RenderImages() → []image.Asset (procesado por ImageProcessor inyectado)
  ```
- Documentar que image NO es un asset de texto: se delega a `ImageProcessor`.

Ver `tinywasm/docs/MASTER_PLAN.md` para el orden global de ejecución.
