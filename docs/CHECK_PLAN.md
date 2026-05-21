# PLAN — `RenderJS()` retorna `[]*js.Script` (bundle + archivos standalone)

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

## Objetivo

Adaptar el extractor y el escritor de assets para que `RenderJS()` retorne
`[]*js.Script` en vez de `string`. Los `Script` con `Name == ""` se acoplan al
bundle global `script.js` (comportamiento actual); los `Script` con
`Name != ""` se escriben como archivos independientes en la raíz pública
(p.ej. `sw.js` para service workers en PWA).

## Justificación

`RenderJS() string` colapsa "fragmento de bundle" y "archivo standalone" en
una sola cadena, lo que hace imposible publicar archivos JS que **deben**
estar separados (service workers requieren root scope; web workers se cargan
por URL). Hoy el usuario debe escribir esos archivos manualmente — contradice
la promesa de `tinywasm/app`.

Este es un **breaking change** consciente en la API de assetmin.

## Cambios

### 1. Interfaces y registro

**Archivo:** [ssr_register.go](../ssr_register.go)

```go
import "github.com/tinywasm/js"

type jsProvider interface{ RenderJS() []*js.Script }
```

Reescribir `RegisterComponents` / `UpdateSSRModule(InSlot)` para aceptar
`[]*js.Script` en vez de `js string`. Para cada Script:

- `Name == ""` → acumular `Content` en el slot del bundle (`mainJsHandler`).
- `Name != ""` → validar nombre simple, registrar como archivo standalone.

Firma nueva (sugerencia):

```go
func (c *AssetMin) UpdateSSRModule(name string, css string, scripts []*js.Script, html string, icons map[string]string)
```

### 1.1. Eliminar el campo `GetSSRClientInitJS` de `Config`

El bundle de la página (`wasm_exec` + bootstrap WASM) deja de entrar por un
callback opaco. Ahora llega como un `*js.Script{Name:""}` más — producido por
`js.PageBootstrap()` y registrado por `tinywasm/app`.

Eliminar:
- `assetmin.go:49` — campo `GetSSRClientInitJS func() (string, error)` en `Config`.
- `assetmin.go:77` — `c.mainJsHandler = newAssetFile(..., ac.GetSSRClientInitJS)`.
  El `mainJsHandler` pasa a alimentarse exclusivamente de los Scripts con
  `Name==""` acumulados vía `RegisterComponents`/`UpdateSSRModule`.
- `events.go:148-152` — la rama que invoca `c.GetSSRClientInitJS()`.

Breaking change en la API de `assetmin.Config`: `app` deja de pasar ese
callback (ver su propio PLAN). Cualquier otro consumidor del campo debe
migrar a registrar un `*js.Script` con `Name==""`.

### 2. Estructura interna para standalone

Añadir un mapa `c.standaloneJS map[string]*ContentFile` (clave = `Name`) y un
nuevo handler análogo a `mainJsHandler` pero que produce N archivos en lugar
de uno. Reglas:

- Colisión de Name entre módulos → error explícito (no merge silencioso).
- Cada standalone se minifica como cualquier otro JS (a menos que la
  extensión sugiera lo contrario — fase 2, no bloquear este plan).

### 3. Extractor de JSON

**Archivo:** [ssr_invoke.go:18-25](../ssr_invoke.go#L18-L25)

```go
type ssrCollectorOutput struct {
    Root   string            `json:"root"`
    Render string            `json:"render"`
    HTML   string            `json:"html"`
    Scripts []ScriptOutput   `json:"scripts"` // antes: JS string
    Icons  map[string]string `json:"icons"`
}

type ScriptOutput struct {
    Name    string `json:"name"`
    Content string `json:"content"`
}
```

Template generado (`GenerateExtractorMain`) debe convertir `[]*js.Script` a
`[]ScriptOutput` antes de encodear JSON.

### 4. Flujo de escritura a disco

**Archivos:** `filewrite.go`, `http.go`, `assetmin.go`

- `FlushToDisk` ahora itera `c.standaloneJS` y escribe cada uno en
  `<publicRoot>/<Name>` además del bundle. **`Content` se escribe literal**
  — assetmin nunca compone ni transforma JS (espejo de `tinywasm/css`).
- El handler HTTP debe servir esos archivos por URL `/Name`.
- Hot reload: cuando cambia el `ssr.go` de un módulo, regenerar bundle Y
  standalones (los módulos vuelven a llamar `js.ServiceWorker(...)` y el
  string final se renueva).

assetmin **sólo conoce `tinywasm/js`**. No conoce ni a `client` ni a ningún
otro proveedor de assets. Si un Script (SW, Worker, legacy) necesita
contenido derivado de otros paquetes, ese contenido ya viene resuelto dentro
de `Content` antes de llegar a `RegisterComponents`.

### 5. Limpieza de archivos huérfanos

Si un módulo deja de emitir un Script con `Name="sw.js"`, el archivo previo
debe borrarse. Llevar registro del último set publicado por módulo.

## Tests

| Test | Verifica |
|---|---|
| `TestRegister_BundledScript` | `Script{Name:"",Content:"x"}` aparece en `script.js` |
| `TestRegister_StandaloneScript` | `Script{Name:"sw.js",Content:"x"}` produce archivo separado |
| `TestRegister_MixedScripts` | Un módulo aporta ambos: bundle conserva uno, root tiene el otro |
| `TestRegister_NameCollision` | Dos módulos con mismo Name → error |
| `TestRegister_InvalidName` | `"a/b.js"` y `"../x"` rechazados |
| `TestFlushToDisk_WritesStandalone` | `/public/sw.js` existe tras `FlushToDisk` |
| `TestHTTP_ServesStandalone` | `GET /sw.js` retorna el contenido |
| `TestHotReload_RemovesOrphanStandalone` | Eliminar el Script borra el archivo |

Adaptar `tests/ssr_integration_test.go` y `testdata/integration_workspace/`
para incluir un módulo con `Script{Name:"sw.js"}`.

## Documentación

- [`docs/SSR.md`](SSR.md) — documentar el nuevo contrato `[]*js.Script`,
  semántica de `Name`, ejemplo de service worker.
- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md) — incluir el handler standalone en
  el diagrama del pipeline.
- [`docs/COMPONENT_REGISTRATION.md`](COMPONENT_REGISTRATION.md) — actualizar
  firma de `RegisterComponents` y ejemplos.
- [`docs/QUICK_REFERENCE.md`](QUICK_REFERENCE.md) — snippet "módulo mínimo"
  con ejemplo de Script bundleable y standalone.

## Precondiciones

- `tinywasm/js` publicado con `Script` (campos `Name`, `Content` + método
  `String()`). v0.2.0 añade además los constructores `ServiceWorker` /
  `WebWorker` que devuelven `*Script` con `Content` final — assetmin los
  trata como cualquier otro Script (no requiere lógica especial).
- assetmin valida internamente el `Name` (sin `/` ni `..`) en
  `RegisterComponents` / `UpdateSSRModule` — la regla pertenece al boundary
  que escribe al filesystem, no al tipo de datos.
- `tinywasm/dom` con `JSProvider.RenderJS()` retornando `[]*js.Script` (la
  interfaz pública del lado del consumidor debe ser coherente).

Verificación:

```bash
go list -m github.com/tinywasm/js github.com/tinywasm/dom
```

## Stages

| # | Tarea | Done |
|---|---|---|
| 1 | Añadir dependencia `github.com/tinywasm/js` al `go.mod` | [ ] |
| 1.1 | Eliminar campo `GetSSRClientInitJS` de `Config` y sus usos (`assetmin.go:49,77`, `events.go:148-152`) | [ ] |
| 2 | Cambiar tipo de `jsProvider` en `ssr_register.go` | [ ] |
| 3 | Refactor `RegisterComponents` + `UpdateSSRModule(InSlot)` con la firma nueva | [ ] |
| 4 | Añadir `standaloneJS` y handler análogo a `mainJsHandler` | [ ] |
| 5 | Cambiar `ssrCollectorOutput.JS` → `Scripts []ScriptOutput` y adaptar template | [ ] |
| 6 | Adaptar `FlushToDisk` + servidor HTTP para servir standalones | [ ] |
| 7 | Implementar limpieza de standalones huérfanos en hot reload | [ ] |
| 8 | Test suite completa listada arriba — `go test ./...` verde | [ ] |
| 9 | Actualizar 4 docs en `assetmin/docs/` | [ ] |
| 10 | Verificación E2E con service worker real vía MCP browser | [ ] |

---

## Migración adicional: `ssr.go` → split por extensión (breaking change)

### Objetivo

Eliminar el nombre reservado `ssr.go`. El motor de extracción pasa a descubrir
los assets SSR de un paquete leyendo un **conjunto fijo de archivos por
extensión**, todos con `//go:build !wasm`:

| Archivo | Métodos que contiene |
|---|---|
| `css.go` | `RootCSS()` y/o `RenderCSS()` |
| `js.go` | `RenderJS()` |
| `html.go` | `RenderHTML()` |
| `svg.go` | `IconSvg()` |

`ssr.go` deja de reconocerse. Un paquete declara solo los archivos que necesita
(p.ej. un componente con CSS + iconos tiene `css.go` + `svg.go`).

### Justificación

`ssr.go` es un nombre mágico que el autor debe aprender; no comunica qué
contiene. Los nombres por extensión son autoexplicativos, alinean con SRP
(`core-principles`) y co-localizan cada `//go:embed` con su concern. El precio
es proliferación de archivos (un componente con iconos pasa a 2 archivos), que
se acepta como trade-off de descubribilidad.

### Cambios en el motor

1. **Whitelist única.** Definir en `assetmin`:
   ```go
   var ssrSourceFiles = []string{"css.go", "js.go", "svg.go", "html.go"}
   ```
2. **`ssr_extract.go:32-35`** — el chequeo `os.Stat(.../ssr.go)` se reemplaza
   por "existe al menos uno de `ssrSourceFiles` en `moduleDir`". Si ninguno
   existe, el módulo no aporta assets (no es error).
3. **`ssr_invoke.go` `ModulesToAliases` (≈L176-196)** — en vez de leer un único
   `ssr.go`, leer cada archivo de `ssrSourceFiles` que exista, **concatenar su
   contenido** y correr los regex de features (`reRootCSS`, `reRenderCSS`,
   `reRenderHTML`, `reRenderJS`, `reIconSvg`) y `detectReceiverType` sobre el
   contenido combinado. El tipo receptor debe ser consistente entre archivos
   (ya lo es: todos los métodos de un componente comparten receiver).
4. Eliminar toda referencia literal a `"ssr.go"` del paquete.

### Tests

| Test | Verifica |
|---|---|
| `TestExtract_CssOnly` | Paquete con solo `css.go` extrae CSS |
| `TestExtract_CssPlusSvg` | `css.go` + `svg.go` → CSS + iconos, mismo receiver |
| `TestExtract_AllFour` | `css.go/js.go/html.go/svg.go` juntos |
| `TestExtract_NoSSRFiles` | Paquete sin ninguno → sin error, assets vacíos |
| `TestExtract_PackageLevelFuncs` | Fallback sin receiver sigue funcionando |

Renombrar `testdata/integration_workspace/button/ssr.go` → `button/css.go`
y adaptar `ssr_extract_subpackage_test.go`.

### Docs a actualizar

`SSR.md`, `ARCHITECTURE.md` (sección Hot Reload + Compile-and-Invoke),
`COMPONENT_REGISTRATION.md`, `QUICK_REFERENCE.md` — toda mención de `ssr.go`
pasa al modelo de archivos por extensión.

### Sequencing cross-repo (aditivo → cleanup)

`assetmin` se publica como dependencia de los consumidores, así que **no** puede
ser un cambio atómico de un solo PR: el motor debe publicarse primero y los
consumidores migrar después contra la versión publicada. Para no dejar nunca el
extractor incapaz de descubrir assets, el cambio se parte en dos fases:

- **Fase aditiva (no breaking):** la whitelist `ssrSourceFiles` acepta los 4
  nombres nuevos **y además** sigue aceptando `ssr.go`. Se publica. En este
  punto cualquier consumidor puede renombrar a su ritmo sin romperse.
- **Fase cleanup (breaking):** una vez `components`, `layout`, `goflare-demo`,
  `css` y `form` ya no contienen ningún `ssr.go`, se elimina `ssr.go` de la
  whitelist y se publica el bump final. Estado final: `ssr.go` no existe en el
  ecosistema.

Esta separación es la única forma de que un agente externo ejecute repo-por-repo
sin un estado intermedio roto. Ver el orden global en
[`docs/MASTER_PLAN.md`](../../docs/MASTER_PLAN.md) §"Track B".

### Stages (split por extensión)

| # | Tarea | Done |
|---|---|---|
| S1 | **(aditivo)** Añadir whitelist `ssrSourceFiles = {css,js,svg,html}.go` **+ `ssr.go`** y refactor del chequeo de existencia (`ssr_extract.go`) | [ ] |
| S2 | **(aditivo)** Refactor `ModulesToAliases` para leer/concatenar todos los archivos de la whitelist que existan | [ ] |
| S3 | **(aditivo)** Tests de extracción multi-archivo (`css.go`, `css.go`+`svg.go`, los 4 juntos, sin archivos, package-level) — `go test ./...` verde | [ ] |
| S4 | **(aditivo)** Publicar bump de `assetmin` — desbloquea a los 5 consumidores | [ ] |
| S5 | **(cleanup)** Tras consumidores migrados: quitar `"ssr.go"` de la whitelist y toda referencia literal | [ ] |
| S6 | **(cleanup)** Renombrar `testdata/.../button/ssr.go` → `css.go` y adaptar `ssr_extract_subpackage_test.go` | [ ] |
| S7 | **(cleanup)** Actualizar `SSR.md`, `ARCHITECTURE.md`, `COMPONENT_REGISTRATION.md`, `QUICK_REFERENCE.md`; publicar bump final | [ ] |
