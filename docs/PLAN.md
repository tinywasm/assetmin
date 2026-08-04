---
PLAN: "feat: Entregar las fuentes declaradas en config/fonts.go"
TAG: v0.5.0
STATUS: review
SESSION: 79707023854572119
PR: https://github.com/tinywasm/assetmin/pull/37
---
## Antes de escribir código: lee [CONSTRUCTION_HARNESS.md](CONSTRUCTION_HARNESS.md)

**Es vinculante, no orientativo.** Los principios que gobiernan este trabajo:

| # | Principio | Cómo se aplica aquí |
|---|---|---|
| 4 | One way to do each thing | Un solo camino para entregar un asset binario. Existe (`ImageProcessor`); las fuentes lo siguen, no inventan otro. |
| 5 | Minimal surface | `assetmin` declara el contrato; no implementa subsetting ni conversión. |
| 9 | Lego pieces, never forks | `assetmin` posee la entrega de assets. Que hoy no sepa de fuentes es **su** hueco: por eso se cierra aquí y no en `css`. |

Depende de `tinywasm/font`, ya publicado (v0.0.3).

---

## 1. El hueco

`assetmin` no sabe entregar fuentes. No hay ruta por la que un `.ttf` llegue al navegador.

La consecuencia no es sólo la falta: es que el hueco **aflora en la hoja**. La primera
versión de `css/docs/PLAN.md` proponía que `tinywasm/css` embebiera la fuente en base64
dentro del `@font-face` para esquivarlo — exactamente lo que el harness llama *a fork with
a friendlier name*. Ese plan quedó bloqueado esperando a este.

> **Why this matters more than it looks.** An API gap always surfaces at the leaf, where the
> agent has no authority to publish upstream — so it patches locally. Technical debt is then
> not an accident: the workflow guarantees it.

---

## 2. Por qué una fuente no cabe en el modelo `asset`

`asset` es un **concatenador de texto**: `contentOpen` + `contentMiddle` + `contentClose`,
unidos y minificados (`asset.go:22-24`). Sirve para CSS, JS, HTML y el sprite SVG porque
todos son texto que se fusiona.

Una fuente es binaria y **se copia, no se fusiona**. Concatenar dos `.ttf` no produce una
fuente; produce basura. Así que un `newAssetFile("font.ttf", …)` es el camino equivocado
por más que parezca el simétrico.

El repo ya resolvió este caso para imágenes, y el comentario del watcher lo dice con
precisión:

```go
// ssrTextAssetFiles: archivos Go cuyo contenido se EXTRAE (string) y se fusiona/inyecta.
var ssrTextAssetFiles = []string{"css.go", "js.go", "svg.go", "html.go"}

// imageAssetFile: archivo Go que DECLARA imágenes a procesar (no se extrae string).
const imageAssetFile = "image.go"
```

Dos categorías, no una. **Las fuentes son de la segunda.**

---

## 3. Cambios

### 3.1 El contrato: `FontProcessor`, calcado de `ImageProcessor`

```go
// FontProcessor procesa las fuentes declaradas en los config/fonts.go de los proyectos.
// Implementado fuera de este paquete; inyectado por el composition root (app).
type FontProcessor interface {
    LoadFonts() error                    // escaneo completo inicial (startup)
    ReloadModule(moduleDir string) error // reproceso (config/fonts.go cambió)
    UnobservedFiles() []string           // outputs a excluir del watcher
}

// SetFontProcessor inyecta el pipeline de fuentes. Pasar nil lo desactiva.
func (c *AssetMin) SetFontProcessor(p FontProcessor)
```

Misma forma que `ImageProcessor` (`image_processor.go`), misma inyección, misma
desactivación por `nil`. Un lector que ya conoce el pipeline de imágenes no tiene nada nuevo
que aprender — que es el punto del principio 4.

**`assetmin` no implementa el procesador.** No recorta fuentes ni interpreta el formato:
copia archivos y emite una regla CSS. Declara el contrato y llama; la implementación vive
fuera, como `tinywasm/image/min`.

#### Decisión pendiente: dónde vive la implementación

`ImageProcessor` lo implementa `tinywasm/image/min`, que **convierte** a WebP: trabajo
real que justifica su propio paquete. El de fuentes sólo tiene que leer la declaración y
copiar cuatro archivos — no convierte nada, porque web y PDF comparten el mismo TTF.

Con tan poco cuerpo, las opciones son:

| | Dónde | A favor | En contra |
|---|---|---|---|
| **A** | `tinywasm/font/assets` | junto al concern; `font` ya sabe derivar los nombres | la raíz de `font` es identidad pura sin build tag — el subpaquete leería archivos |
| **B** | aquí mismo, sin contrato | es copiar archivos, no un pipeline | rompe la simetría con `ImageProcessor` y mete E/S de fuentes en `assetmin` |

**A no viola la regla de `font`**: igual que `cmd/gofont`, un subpaquete que nadie importa
desde el frontend no contamina lo que la raíz exporta. Es la recomendada, pero **decidir
antes de escribir el contrato** — si al final es B, el `FontProcessor` sobra entero.

### 3.2 El `@font-face` lo emite `assetmin`, y sólo puede emitirlo él

La regla `@font-face` necesita la URL final del archivo. Esa URL la construye `assetmin`:

```go
c.mainStyleCssHandler.urlPath = path.Join("/", ac.AssetsURLPrefix, cssMainFileName)
```

`tinywasm/css` **no puede** escribirla: no conoce `AssetsURLPrefix` ni el `OutputDir`, y
adivinarla sería acoplarlo a una convención que no controla. Por eso `css` aporta la
*familia* (`--font-sans`) y `assetmin` aporta la *entrega*.

El gancho ya existe: `asset.dynamicContent []func() []byte` (`asset.go:26`, consumido en
`:175`). El `@font-face` se inyecta ahí, en el asset CSS principal, con las URLs que
`assetmin` acaba de decidir. Sin API nueva.

Una regla por cara, y **el formato es `truetype`**, no `woff2`:

```css
@font-face {
  font-family: "Roboto";
  src: url("/assets/fonts/Roboto-Bold.ttf") format("truetype");
  font-weight: 700;
  font-style: normal;
  font-display: swap;
}
```

El ecosistema sirve **un solo TTF por cara para web y PDF**: el documento se genera en el
frontend, así que pide el mismo archivo que la página ya bajó — acierto de caché en vez de
una segunda descarga. Escribir `format("woff2")` haría que el navegador rechace el
archivo. La decisión y sus números están en
`app-releases/docs/TYPOGRAPHY_MASTER_PLAN.md` §5.2.

Los cuatro `font-weight`/`font-style` salen de `font.Style`: `Regular` es 400/normal,
`Bold` 700/normal, `Italic` 400/italic, `BoldItalic` 700/italic. No se inventan aquí.

### 3.3 El watcher enruta `config/fonts.go`

Una constante junto a `imageAssetFile` en `ssr_watcher.go`, y su rama en el filtrado por
basename. Fonts entra por la puerta de "declara assets a procesar", no por
`ssrTextAssetFiles`.

### 3.4 Lo que NO se debe hacer

**No añadir `.ttf` a `SupportedExtensions()`.** Esa lista enruta *fuentes de
contenido de texto* que se fusionan; un binario ahí entraría en el concatenador. Las
imágenes tampoco están, y por la misma razón. Los archivos generados se excluyen del
watcher vía `UnobservedFiles()`, igual que los `.webp`.

*(Corrige lo que decía una versión previa de `css/docs/PLAN.md`.)*

---

## 4. La identidad no pasa por aquí

Las fuentes se parecen a `image.go` en una cosa y se diferencian en otra, y conviene
tenerlo claro antes de implementar.

**Se parecen** en que declaran un asset binario a procesar, no un texto a fusionar. Por
eso el contrato se calca de `ImageProcessor` y `config/fonts.go` entra por la puerta de
`imageAssetFile`, no por `ssrTextAssetFiles`.

**Se diferencian** en que la identidad de la tipografía —qué familia— también la
necesitan `tinywasm/css` en build-time y `tinywasm/pdf` en runtime. Pero eso **no es
trabajo de este módulo ni de `ssr`**: el proyecto declara en `config/fonts.go`, sin build
tag, y `config/css.go` —mismo paquete Go— llama a esa función directamente. El valor
viaja dentro del `RootCSS()` que `ssr` ya extrae.

Aquí sólo llegan los bytes: copiar las caras a la carpeta pública y emitir el
`@font-face` con la URL que sólo este módulo conoce.

---

## 5. Verificación

1. Un proyecto con `config/fonts.go` obtiene sus `.ttf` en `OutputDir` y servidos por HTTP.
2. El CSS emitido contiene un `@font-face` cuya URL **resuelve** contra el servidor de
   desarrollo — comprobado con una petición real, no por inspección de la cadena.
3. La página carga la fuente **sin ninguna petición a un host externo**. Es el requisito de
   producto: los despliegues objetivo no tienen internet.
4. Editar `config/fonts.go` dispara el reproceso; el `.ttf` **copiado** al directorio de
   salida no vuelve a disparar el watcher (`UnobservedFiles()`), o se entra en bucle.
5. El `@font-face` declara `format("truetype")`. Con `format("woff2")` el navegador
   rechaza el archivo, y el fallo se ve como «la fuente no carga», no como un error.
6. El mismo `.ttf` que sirve la página es el que `tinywasm/pdf` pide por `fetch` — una
   sola descarga, comprobada en el panel de red del navegador.
7. Sin `FontProcessor` inyectado, `assetmin` funciona exactamente como hoy. La capacidad
   es opcional; su ausencia no es un error.
8. Un proyecto sin `config/fonts.go` no cambia en nada.
9. `gotest`.

`docs/ASSETS.md` y `docs/API.md` se actualizan en el mismo commit.
