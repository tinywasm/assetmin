---
PLAN: "feat: entregar al navegador las fuentes declaradas en config/fonts.go"
TAG: v0.5.0
---
## Antes de escribir código: lee [CONSTRUCTION_HARNESS.md](CONSTRUCTION_HARNESS.md)

**Es vinculante, no orientativo.**

| # | Principio | Cómo se aplica aquí |
|---|---|---|
| 2 | Explicit over implicit | La declaración llega tipada dentro de `SSRAssets`. No se parsea Go ni se adivinan rutas. |
| 5 | Minimal surface | Ni interfaz nueva ni módulo nuevo: copiar cuatro archivos no justifica un contrato. |
| 9 | Lego pieces, never forks | El texto CSS lo emite `tinywasm/css`; el nombre de archivo lo deriva `tinywasm/font`; el valor lo extrae `tinywasm/ssr`. `assetmin` no reimplementa ninguno de los tres. |

**Prerrequisitos, en orden:** `font` v0.1.0 (nombre de la cara regular) → `css` v0.5.0
(`FontFaces`) y `ssr` (productor `Fonts`) → este plan. Cada uno tiene su `docs/PLAN.md`.

---

## 1. El flujo, de punta a punta

Hoy no existe ninguna ruta por la que un `.ttf` llegue al navegador. Este es el flujo
completo una vez cerrado; **lo que falta es la rama del medio** — y de ella, este plan
cubre el tramo `assetmin`.

```
config/fonts.go            (proyecto, SIN build tag)
   func Fonts() font.Declaration { return font.Declare("Roboto", "config/fonts") }
        │
        │ es Go normal: quien importe el paquete config lo llama y ya.
        │
        ├──────────────► config/css.go  (!wasm)          [YA FUNCIONA]
        │                  --font-sans: css.FontStack(Fonts().Family())
        │                  el valor viaja dentro del RootCSS() que ssr ya extrae
        │
        ├──────────────► ssr                              [ssr/docs/PLAN.md]
        │                  compila y EJECUTA un programa que llama a Fonts()
        │                  y devuelve la Declaration dentro de SSRAssets
        │                        │
        │                        ▼
        │                  assetmin                       [ESTE PLAN]
        │                    ├─ copia las 4 caras  RootDir/<Dir()>/*.ttf → OutputDir
        │                    └─ inyecta css.FontFaces(d, AssetsURLPrefix) en style.css
        │                       → /assets/Roboto-Bold.ttf …
        │
        └──────────────► WASM: pdf.LoadDeclared(...)      [YA FUNCIONA]
                           misma Family(), mismo .ttf que la página ya cacheó
```

Tres consumidores, **una sola declaración**. Es el requisito de producto: el documento
PDF y la página web tienen que verse con la misma tipografía, y eso sólo se garantiza si
la familia se escribe una vez.

### Por qué la declaración la extrae `ssr` y no la pasa nadie por `Config`

`assetmin` **no parsea Go, nunca**. Lo que necesita de código ajeno se lo inyecta el
composition root: `SetSSRExtractor` (lo implementa `tinywasm/ssr`, que compila y ejecuta
un programa generado), `SetImageProcessor` (lo implementa `tinywasm/image/min`, que sí usa
`go/ast`). No hay un solo `import "go/parser"` en este repo y este plan no lo introduce.

Y no hay atajo posible: **`assetmin.Config` lo construye `tinywasm/app`**
(`app/section-build.go:74`), que **no importa el paquete `config` del proyecto**. `app` es
el CLI que corre *dentro* del proyecto; el proyecto es un directorio (`RootDir`), no
código que `app` compile contra sí mismo. Ningún proyecto tiene un `main.go` que llame a
`app`. Así que una línea como `assetmin.Config{Fonts: config.Fonts()}` **nadie puede
escribirla**.

Leer un valor del código del proyecto sólo tiene un camino en este ecosistema, y ya
existe: el programa que `ssr` genera e invoca. Por eso `Fonts()` es un productor más,
junto a `RootCSS()` y `RenderCSS()`.

> **Consecuencia:** editar `config/fonts.go` recompila el servidor y el WASM como
> cualquier otro `.go` (`server.SupportedExtensions() → [".go"]`), **y además** entra por
> el enrutado de `ssr_watcher.go` para reextraer el módulo. Ver §4.4.
>
> Lo que **no** se recarga en caliente son los `.ttf`: añadir un archivo de fuente no
> dispara nada hasta el siguiente arranque. Es deliberado — ver §4.5.

---

## 2. Por qué el trabajo cae aquí y no en `css` ni en `font`

Un `@font-face` necesita tres datos que viven en tres sitios distintos:

| Dato | Quién es el dueño | Por qué nadie más puede |
|---|---|---|
| familia y nombre de cada cara | `tinywasm/font` | `Family.Face(Style)` es la única derivación; escribir `"Roboto-Bold"` a mano es duplicarla |
| sintaxis CSS del bloque | `tinywasm/css` | `css/AGENTS.md §5`: la superficie para emitir CSS es esa y sólo esa |
| la URL final del archivo | `assetmin` | `AssetsURLPrefix` + `OutputDir` son suyos; `css` no los conoce y adivinarlos lo acoplaría a una convención que no controla |

Sólo `assetmin` ve los tres a la vez, y sólo `assetmin` mueve bytes a disco. Por eso el
*orquestador* es este módulo — pero **no escribe ni una llave de CSS**: llama a
`css.FontFaces(...)` (prerrequisito §3.1) y le pasa la URL.

Esa es la respuesta a la objeción legítima de «¿por qué assetmin genera CSS?»: no lo
genera. Aporta el único dato que le falta a `css` y `css` emite el texto.

## 3. Prerrequisitos: dos defectos aguas arriba

**No empieces por `assetmin`.** Los dos siguientes son de otros repos y sin ellos este
trabajo termina copiando código o inventando fallbacks.

### 3.1 `tinywasm/css` no sabe emitir un `@font-face` — hay que añadirlo allí

Firma exacta, en `css` (que ya importa `font`, ver `css.go:5`):

```go
// FontFaces devuelve el bloque @font-face de las cuatro caras de la familia
// declarada, servidas desde urlPrefix. El prefijo es un dato del llamador:
// css no conoce ni inventa rutas de servidor.
func FontFaces(d font.Declaration, urlPrefix string) *Stylesheet
```

Una regla por cara, derivando peso y estilo de `font.Style` — nunca inventados:

| `font.Style` | `font-weight` | `font-style` |
|---|---|---|
| `Regular` | 400 | normal |
| `Bold` | 700 | normal |
| `Italic` | 400 | italic |
| `BoldItalic` | 700 | italic |

**El formato es `truetype`, no `woff2`.** El ecosistema sirve un solo TTF por cara para
web y PDF: el documento se genera en el frontend, así que pide el mismo archivo que la
página ya bajó — acierto de caché en vez de una segunda descarga. Escribir
`format("woff2")` haría que el navegador rechace el archivo, y el fallo se ve como «la
fuente no carga», no como un error. Números y decisión en
`app-releases/docs/TYPOGRAPHY_MASTER_PLAN.md` §5.2.

`font-display: swap` en las cuatro.

### 3.2 Hay dos nombres válidos para la cara regular, y tiene que haber uno

El dev elige la familia y el directorio: eso es lo que declara. **No elige cómo se
llaman los cuatro archivos** — los deriva `Family.Face(Style)`. Fue deliberado:
`LoadDeclared(d)` sustituyó a `LoadTypeface(cuatro rutas)` para que nadie escribiera
cuatro nombres a mano ni olvidara uno. Si el nombre fuera libre, `pdf` y `assetmin`
necesitarían otra vez la lista de rutas, es decir, el API que se eliminó.

Pero `pdf` no confía en esa derivación y acepta dos nombres:

```go
// pdf/document.go:38-43
regPath := dir + f.Face(font.Regular) + ".ttf"      // "Roboto.ttf"
if err != nil {
    regPathAlt := dir + string(f) + "-Regular.ttf"  // "Roboto-Regular.ttf" ← el segundo
```

Con los dos funcionando, el dev no puede saber cuál es el correcto — hasta que `assetmin`
copie usando uno solo y la web quede sin fuente mientras el PDF sí la tiene. Y si
`assetmin` replica el fallback «por si acaso», el defecto queda en tres sitios: *a
wrapper that patches a defect is a fork with a friendlier name*.

**Elegir uno y escribirlo en el README de `font`** como el contrato que el dev cumple al
nombrar sus archivos. Recomendación: `-Regular`, coherente con las otras tres caras y con
cómo Google Fonts nombra sus estáticas. Es convención, no verdad: lo que no es negociable
es que sea **uno solo**.

Y **borrar las dos ramas de fallback** de `pdf/document.go`. La de la línea 59 no es un
problema de nombres y es la peor: si falta la itálica carga la **regular** en su lugar,
en silencio, y el documento sale sin cursivas sin un solo aviso. El comentario la
justifica «para fuentes como DroidSans» — descontinuada desde Android 4.0 y ya excluida
del ecosistema por eso mismo. Una cara ausente debe fallar nombrando el archivo.

---

## 4. Cambios en este módulo

### 4.1 `SSRAssets` trae la declaración

```go
// ssr_extractor.go, dentro de SSRAssets
Fonts font.Declaration // familia declarada por el módulo; cero-valor = ninguna
```

`assetmin` pasa a importar `github.com/tinywasm/font`. **Es correcto y no es una
dependencia nueva de facto**: este mismo DTO ya transporta tipos de otros módulos del
ecosistema (`[]*js.Script`, `*sprite.Sprite`, `ssr_extractor.go:8-17`), y `font` ya está
en el grafo de módulos porque `css` lo requiere (`css/go.mod`). Se importa la *identidad*,
que no trae bytes ni dependencias: `font/go.mod` no requiere nada.

Quien rellena el campo es `ssr` (`ssr/docs/PLAN.md`). Aquí sólo se declara y se consume.

**Sólo el módulo raíz declara fuentes.** La tipografía es una por producto. Si un módulo
no-raíz devuelve una `Declaration`, se ignora con un aviso en el log — es la misma regla
de único-override que ya rige `RootCSS()`. El cero-valor desactiva la función entera: un
proyecto sin `config/fonts.go` no cambia en nada y no ve ni un error.

### 4.2 Copiar las cuatro caras

El único sitio donde se consume un `*SSRAssets` es `routeAssets(a, isRoot, isFramework)`
(`ssr_loader.go:74` y `:153`). La copia cuelga de ahí, para el módulo raíz:

- origen: `filepath.Join(c.RootDir, d.Dir(), d.Family().Face(s) + ".ttf")` para las
  cuatro `font.Style`.
- destino: `filepath.Join(c.OutputDir, <mismo nombre>)`.
- si falta una cara → **error explícito nombrando el archivo**, no un aviso. Una
  tipografía a la que le falta la negrita no es un asset degradado: es un producto roto,
  y el navegador lo disimulará sintetizándola.
- copiar sólo si el destino no existe o es más viejo (`os.Stat` + `ModTime`), como hace
  `image/min` con `IsUpToDate`.

`Dir()` es relativo a la raíz del proyecto (`RootDir`). Es el mismo campo que `pdf` usa
como prefijo de `fetch` en WASM: **`Family()` es el origen único; `Dir()` es propio de
cada medio**, porque el disco del build y la URL del navegador son sitios distintos.

### 4.3 Inyectar el `@font-face` en el CSS principal

La declaración se guarda en un campo **no exportado** del `AssetMin` al enrutar el módulo
raíz (`c.fonts`), y el gancho ya existe: `asset.AddDynamicContent(fn func() []byte)`
(`asset.go:38`, consumido en `:175`). Se registra una vez, sobre `c.mainStyleCssHandler`:

```go
prefix := path.Join("/", c.AssetsURLPrefix)
c.mainStyleCssHandler.AddDynamicContent(func() []byte {
    if c.fonts.Family() == "" {
        return nil
    }
    return []byte(css.FontFaces(c.fonts, prefix).String())
})
```

Se lee dentro del closure, no al registrarlo: así una reextracción posterior
(`ReloadSSRModule`) actualiza el CSS sin volver a registrar nada. Todo el texto CSS sale
de `css`.

### 4.4 Enrutar `fonts.go` y excluir los `.ttf` copiados

**Enrutado:** añadir `"fonts.go"` a `ssrTextAssetFiles` (`ssr_watcher.go:9`). Su
comentario —«archivos Go cuyo contenido se EXTRAE»— describe exactamente este caso: el
valor lo produce `ssr` ejecutando la función, igual que con `css.go`. El evento dispara
`ReloadSSRModule`, que reextrae el módulo y trae la `Declaration` nueva.

**Exclusión:** en `UnobservedFiles()` (`events.go:144`) añadir las cuatro rutas de
destino. Sin esto, el watcher ve aparecer los archivos que acaba de copiar y se entra en
bucle — es la razón por la que `imageProcessor.UnobservedFiles()` existe.

### 4.5 Lo que NO se debe hacer

- **No añadir `.ttf` a `SupportedExtensions()`.** Esa lista enruta contenido de texto que
  se fusiona; un binario ahí entra al concatenador de `asset` (`contentOpen` +
  `contentMiddle` + `contentClose`, `asset.go:22-24`) y dos TTF concatenados no son una
  fuente, son basura. Las imágenes tampoco están, y por lo mismo.
- **No crear `newAssetFile("font.ttf", …)`**, por simétrico que parezca.
- **No crear una interfaz `FontProcessor`.** `ImageProcessor` existe porque convertir a
  WebP arrastra un codificador pesado que este módulo no debe cargar; copiar cuatro
  archivos con `io.Copy` no arrastra nada. Una interfaz con una sola implementación que
  no tiene motivo para vivir fuera es superficie inventada.
- **No parsear `config/fonts.go`.** Ver §1: lo ejecuta `ssr`, no lo lee nadie con
  `go/ast`.
- **No recargar los `.ttf` en caliente.** Añadir un archivo de fuente no dispara nada
  hasta el siguiente arranque, y está bien así: las cuatro caras se copian una vez, al
  montar el proyecto — no es un bucle de edición como el CSS. Y si falta una, el arranque
  falla nombrándola (§4.2), así que no hay servidor corriendo al que recargarle nada. El
  coste de cerrarlo sería meter un binario en un modelo de asset que concatena texto, o
  un watcher dentro de `tinywasm/font` que rompería su invariante de no tocar `os.` ni
  `[]byte`. A cambio de ahorrar **un reinicio, una vez**.

---

## 5. Verificación

1. Un proyecto con `config/fonts.go` obtiene sus cuatro `.ttf` en `OutputDir` y servidos
   por HTTP, sin que nadie escriba una ruta a mano.
2. El CSS emitido contiene el `@font-face` y su URL **resuelve** contra el servidor de
   desarrollo — comprobado con una petición real, no leyendo la cadena.
3. La página carga la fuente **sin ninguna petición a un host externo**. Es requisito de
   producto: los despliegues objetivo no tienen internet.
4. El bloque declara `format("truetype")` en las cuatro caras.
5. Faltando `Roboto-Bold.ttf` en el origen, el arranque falla nombrando ese archivo.
6. Segundo arranque sin cambios: no vuelve a copiar (`ModTime`), y el watcher no se
   dispara con los `.ttf` de salida.
7. Un proyecto **sin** `config/fonts.go`: `assetmin` se comporta exactamente como hoy, sin
   un solo aviso. Y un módulo no-raíz que declare fuentes se ignora con log, no las aplica.
8. Editar `config/fonts.go` reextrae el módulo y el `@font-face` emitido cambia — sin
   reiniciar el proceso de `assetmin`.
8. El mismo `.ttf` que sirve la página es el que `tinywasm/pdf` pide por `fetch`: una
   sola descarga, comprobada en el panel de red.
9. `gotest`.

`docs/ASSETS.md` y `docs/API.md` se actualizan en el mismo commit.
