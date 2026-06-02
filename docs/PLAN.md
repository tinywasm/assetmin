# PLAN: assetmin — test de integración real para el hot reload SSR

## Repositorio
`github.com/tinywasm/assetmin` — path local: `tinywasm/assetmin/`

## Contexto
El hot reload de assets SSR está roto: al editar `css.go` (u otro asset-source
`.go`) en un componente, el CSS servido NO se actualiza hasta reiniciar el daemon.
La **causa raíz está en devwatch** (gate de `depfind` que bloquea al
`SSRFileWatcher`); el fix principal va allí — ver `devwatch/docs/PLAN.md`.

Este plan cubre la parte de assetmin: **cerrar la brecha de tests** que dejó pasar
el bug, y dejar el `SSRFileWatcher` robusto frente a la regresión.

---

## Problema de cobertura

`SSRFileWatcher` ([ssr_watcher.go](../ssr_watcher.go)) implementa
`devwatch.FilesEventHandlers`:

```go
func (w *SSRFileWatcher) MainInputFileRelativePath() string { return "go.mod" }
func (w *SSRFileWatcher) SupportedExtensions() []string     { return []string{".go"} }
func (w *SSRFileWatcher) NewFileEvent(fileName, extension, filePath, event string) error { ... }
```

`NewFileEvent` se auto-filtra por basename (`css.go`/`js.go`/`svg.go`/`html.go` →
`ReloadSSRModule`; `image.go` → `imageProcessor.ReloadModule`).

Los tests actuales llaman `NewFileEvent` o `ReloadSSRModule` **directamente**, por
lo que prueban la lógica interna del watcher pero **saltan** el ruteo de devwatch
(`depfind.ThisFileIsMine` sobre `MainInputFileRelativePath()=="go.mod"`), que es
exactamente donde se rompe. Un unit test que llama `NewFileEvent` directo nunca
puede detectar que el evento jamás le llega.

---

## Cambio 1: Test de integración end-to-end (gate real)

Crear `tests/ssr_hotreload_devwatch_test.go` que ejercite el camino COMPLETO:
escribir un `.go` en disco → evento FS de `devwatch.DevWatch` real →
`depfind.ThisFileIsMine` → `SSRFileWatcher.NewFileEvent` → `ReloadSSRModule` →
contenido servido actualizado.

Esqueleto:

```go
func TestSSRHotReload_ThroughDevWatch(t *testing.T) {
    root := t.TempDir()
    // 1. Crear un módulo real: go.mod + css.go con RenderCSS() que emite un token
    //    distinguible (p.ej. ".probe{color:#111}").
    writeGoMod(root, "probemod")
    writeCSSGo(root, "#111111")

    // 2. Levantar AssetMin en modo SSR + inyectar un SSRExtractor real (tinywasm/ssr)
    //    con ListModulesFn apuntando al módulo temporal.
    am := NewAssetMin(&Config{OutputDir: ..., RootDir: root, DevMode: true})
    am.EnableSSRMode()
    am.SetSSRExtractor(ssrExtractorFor(root))

    // 3. Carga inicial → contiene #111111.
    am.ReloadSSRModule(root)
    requireContains(t, am.GetMainStyleCSS(), "#111111")

    // 4. Registrar el SSRFileWatcher en un devwatch.DevWatch REAL y arrancarlo.
    var reloaded int32
    w := am.NewSSRFileWatcher(func() error { atomic.AddInt32(&reloaded, 1); return nil })
    dw := devwatch.New(&devwatch.WatchConfig{
        FilesEventHandlers: []devwatch.FilesEventHandlers{w},
        ExitChan:           make(chan bool),
    })
    dw.AddDirectoriesToWatch(root)
    go dw.FileWatcherStart() // o el entrypoint que corresponda

    // 5. Editar css.go en disco → #222222 y esperar el debounce.
    writeCSSGo(root, "#222222")

    // 6. ASSERT: el browser reload se disparó Y el CSS servido ahora contiene
    //    #222222 (no #111111). HOY ESTO FALLA (gate de depfind).
    eventually(t, func() bool {
        return atomic.LoadInt32(&reloaded) > 0 &&
            strings.Contains(am.GetMainStyleCSS(), "#222222")
    })
}
```

Notas de implementación:
- Reutilizar helpers existentes en `tests/` para montar AssetMin y el extractor.
- El módulo temporal necesita un `go.mod` real (depfind y ssr resuelven el root vía
  `go.mod`).
- Si correr `go run` (extracción SSR real) es muy lento/frágil en CI, ver Cambio 3.
- Este test **debe fallar antes** del fix de devwatch y **pasar después** — es el
  guard del bug.

---

## Cambio 2: Helper de inspección del contenido servido (si falta)

El assert necesita leer el CSS SSR en memoria. Si no existe un getter público
para tests, exponer uno mínimo (p.ej. `GetMainStyleCSS() string` que devuelva el
contenido cacheado del `mainStyleCssHandler`) o reusar el handler HTTP
(`RegisterRoutes` + `httptest`) para hacer `GET /style.css`. Preferir la vía HTTP
si ya está disponible — es la que ve el browser de verdad.

---

## Cambio 3 (opcional): SSRExtractor fake para tests rápidos

`ExtractModule`/`ExtractAll` reales hacen `go run` de un main generado — caro y
sensible al entorno. Para tests deterministas, inyectar un `SSRExtractor` fake
que lea el `css.go` del módulo y devuelva su contenido como `SSRAssets.CSS`
(o un mapeo fijo basado en el contenido en disco). Así el test valida el RUTEO
(devwatch→watcher→ReloadSSRModule→served) sin depender del toolchain Go.

> El test del Cambio 1 con extractor REAL puede vivir detrás de un build tag o
> `testing.Short()` skip; el del Cambio 3 (fake) corre siempre y es el guard
> primario de la regresión de ruteo.

---

## Cambio 4: Documentar el contrato del watcher

En [ssr_watcher.go](../ssr_watcher.go), documentar explícitamente que el watcher
DEPENDE de que devwatch NO le aplique el gate de propiedad de depfind (porque su
`MainInputFileRelativePath` no es un `.go`), y que se auto-filtra por basename.
Esto evita que un futuro refactor de devwatch reintroduzca el gate sin un test
que lo proteja.

---

## Dependencia de orden

Este trabajo depende del fix en **devwatch** (`devwatch/docs/PLAN.md`). Secuencia:
1. Aplicar fix devwatch (gate solo para handlers con main input `.go`).
2. Publicar devwatch.
3. Agregar tests de este plan en assetmin (con devwatch ya corregido, el test
   del Cambio 1/3 pasa; sin el fix, falla → confirma que el test es válido).

---

## Verificación

```bash
cd tinywasm/assetmin
go build ./...
gotest
```

End-to-end con el daemon de app (tras publicar devwatch + assetmin):
1. `curl http://localhost:6060/style.css | grep pd-color-secondary` → valor actual.
2. Editar `css.go` de un componente → guardar.
3. `curl ...` de nuevo → refleja el cambio **sin reiniciar** el daemon.
