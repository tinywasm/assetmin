# PLAN: Fix CSS Hot-Reload — Encapsulate refreshAsset + Private API

## Problems

### 1. `RefreshAsset` pública viola SRP
Callers externos conocen las extensiones internas de assetmin (`.css`, `.js`, `.html`).
Si se añade un nuevo tipo de asset, todos los callers deben actualizarse.

### 2. SSR path gatea refresh en éxito de `ReloadSSRModule` (events.go:65-69)
Si `ReloadSSRModule` falla, el cache no se regenera pero `NewFileEvent` retorna `nil`
→ devwatch recarga el browser con assets stale sin ningún error visible.

### 3. SSR path sin sleep de timing (events.go)
El non-SSR path tiene `time.Sleep(20ms)` para que el OS termine de escribir el archivo
antes de leerlo. El SSR path carece de esta guarda.

---

## Design

`ReloadSSRModule` ya conoce qué assets actualizó: `updateSSRModuleInSlot` solo escribe
los slots donde el contenido es no-vacío. Por tanto puede llamar `refreshAsset`
internamente para cada tipo que realmente cambió.

**`RefreshAsset` pasa a privada (`refreshAsset`).** Único consumidor externo es
`tinywasm/app` — no hay otros consumidores. Breaking change controlado.

| Caller actual | Después |
|---|---|
| `app/section-build.go:208-210` (tras `ReloadSSRModule`) | eliminado — `ReloadSSRModule` encapsula el refresh |
| `app/section-build.go:243-244` (`OnWasmExecChange`) | usa nuevo método `RefreshJSAssets()` |
| `assetmin/events.go:66` | eliminado — `ReloadSSRModule` encapsula el refresh |

### Nuevo método público: `RefreshJSAssets()`
Cuando el binario WASM cambia (compilación Go estándar ↔ TinyGo), solo JS y HTML
dependen de él. CSS no cambia. Este método encapsula ese conocimiento dentro de
assetmin sin exponer extensiones al caller.

### `loadSSRModulesLocked` — lazy-regeneration se mantiene
`loadSSRModulesLocked` carga N módulos en background. Refreshear tras cada módulo
regeneraría el cache N veces. El double-checked locking en `GetMinifiedContent`
regenera una sola vez al primer request HTTP, cuando ya están todos cargados.
`ReloadSSRModule` refreshea explícitamente porque hay un browser esperando.
Son contextos distintos con responsabilidades distintas — no es inconsistencia.

---

## Mutex — fix de deadlock

`ReloadSSRModule` usa `defer c.mu.Unlock()`. Llamar `refreshAsset` dentro (que también
adquiere `c.mu`) provocaría deadlock. Fix: eliminar `defer`, hacer unlock manual antes
de las llamadas a `refreshAsset`.

```go
func (c *AssetMin) ReloadSSRModule(moduleDir string) error {
    c.mu.Lock()

    assets, err := ExtractSSRAssets(moduleDir)
    if err != nil {
        c.mu.Unlock()
        return err
    }

    slot := "middle"
    if strings.Contains(moduleDir, "tinywasm/dom") {
        slot = "open"
    } else if isRootDir(moduleDir, c.RootDir) {
        slot = "close"
    }

    c.updateSSRModuleInSlot(assets.ModuleName, assets.CSS, assets.JS, assets.HTML, assets.Icons, slot)
    c.mu.Unlock() // liberar antes de refreshAsset para evitar deadlock

    if assets.CSS != ""         { c.refreshAsset(".css") }
    if assets.JS != ""          { c.refreshAsset(".js") }
    if assets.HTML != ""        { c.refreshAsset(".html") }
    if len(assets.Icons) > 0    { c.refreshAsset(".svg") }
    return nil
}
```

---

## Changes

### `assetmin.go`
- Renombrar `RefreshAsset` → `refreshAsset` (privada)
- Añadir `RefreshJSAssets()` público que llama `refreshAsset` para `.js` y `.html`

### `ssr_loader.go` — `ReloadSSRModule`
- Eliminar `defer c.mu.Unlock()`; unlock manual antes de los refreshes
- Llamar `refreshAsset` solo para los assets que `ExtractSSRAssets` devolvió no-vacíos

### `events.go:60-70` — SSR path
```go
case ".css", ".js", ".svg", ".html":
    dir := filepath.Dir(filePath)
    c.mu.Unlock()
    time.Sleep(20 * time.Millisecond) // match non-SSR timing guard
    _ = c.ReloadSSRModule(dir)        // encapsula refresh internamente; si falla, no hay nada que refrescar
    return nil
```
Si `ReloadSSRModule` falla (no hay `ssr.go`), no se refreshea — el archivo no pertenece
a ningún módulo SSR registrado. Comportamiento correcto documentado en
`TestSSRMode_LooseAssetIgnored`.

---

## Tests

### Existente — falla antes del fix, pasa después
`TestCSSHotReload_NonSSRMode_KeyMismatchDuplicatesCSS`
Replica el key-mismatch: SSR mode inactivo → entrada duplicada en cache.
(El fix en app activa SSR mode al init — ver `app/docs/PLAN.md`.)

### Nuevo — `TestReloadSSRModule_OnlyRefreshesChangedAssets`
Registra un módulo con solo CSS (sin JS ni HTML). Dispara `ReloadSSRModule`.
Verifica que el cache CSS se actualizó y que el cache JS/HTML no fue tocado.
**Por qué:** si se rompe la condición `if assets.CSS != ""`, todos los assets
empiezan a refreshear siempre o ninguno lo hace — comportamiento central del fix.

### Nuevo — `TestReloadSSRModule_ConcurrentCallsNoDeadlock`
Llama `ReloadSSRModule` desde múltiples goroutines simultáneamente con `-race`.
**Por qué:** el unlock manual reemplaza `defer` — un refactor futuro que restaure
el `defer` causaría deadlock silencioso en producción. El race detector lo atrapa.

### Nuevo — `TestRefreshWasmAssets_RefreshesJSAndHTMLOnly`
Registra CSS, JS y HTML iniciales. Modifica el contenido JS dinámico. Llama
`RefreshJSAssets()`. Verifica que JS y HTML se actualizaron, CSS no cambió.
**Por qué:** es API pública nueva. Un error de implementación (añadir `.css` o
omitir `.html`) rompería silenciosamente el hot-reload del binario WASM.

## Files Affected

| File | Change |
|------|--------|
| `assetmin.go` | `RefreshAsset` → `refreshAsset` privada; añadir `RefreshJSAssets()` |
| `ssr_loader.go` | Unlock manual + `refreshAsset` por asset cambiado |
| `events.go` | Eliminar `refreshAsset` explícita; añadir `time.Sleep(20ms)` |
| `tests/css_ssr_hotreload_test.go` | 3 tests nuevos + test existente que falla |
