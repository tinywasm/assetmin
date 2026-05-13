# PLAN — Correcciones post-migración compile-and-invoke

> Este plan reemplaza al plan anterior ("Typed CSS migration"). La migración se ejecutó
> parcialmente. Las fallas actuales son consecuencia de seis inconsistencias entre la
> implementación y los contratos reales del proyecto (Problemas 1–6). El Problema 7
> (binario persistente) es una optimización diferida, no un bug.

---

## Contrato real de los módulos (fuente de verdad)

Los componentes en `tinywasm/components` y `tinywasm/css` usan exclusivamente
**retorno tipado** `*css.Stylesheet`. No existe ni existirá un retorno `string`:

```go
// tinywasm/css/ssr.go — módulo-level functions, SIN SSRInstance, SIN receptor
func RootCSS() *Stylesheet { return New(Root(...)) }
func RenderCSS() *Stylesheet { return New(Rule(...)) }

// tinywasm/components/selectsearch/ssr.go — métodos con receptor
func SSRInstance() *SelectSearch { return &SelectSearch{} }
func (c *SelectSearch) RenderCSS() *Stylesheet { return New(...) }
func (c *SelectSearch) IconSvg() map[string]string { ... }
```

`*Stylesheet` implementa `String() string` (`css/dsl.go:14`). No existe adaptador
`interface{ String() string }` en el contrato — se llama directamente `m.RootCSS().String()`.

---

## Estado actual (diagnóstico)

`gotest` reporta 11 tests fallidos. Las causas raíz son cuatro, distintas e
independientes (mapeadas a Problemas 1–4):

| Test | Error observado | Causa raíz |
|---|---|---|
| `TestCSSHotReload_SSRMode_UpdatesCorrectly` | stale CSS remains | caché no se invalida |
| `TestSSRMode_EmbeddedAssetHotReload` | CSS not updated | ExtractSSRAssets exige go.mod |
| `TestReload_AppGainsRootCSS` | framework css not found | ExtractSSRAssets exige go.mod |
| `TestReload_AppLosesRootCSS` | app root css not found | ExtractSSRAssets exige go.mod |
| `TestReload_ThirdPartyAddsRootCSS` | no go.mod found | ExtractSSRAssets exige go.mod |
| `TestLoader_AppFullyReplacesCss` | framework css persiste | caché contaminada + ssr_invoke tipo incorrecto |
| `TestSSRLoader/LoadSSRModulesOrder` | Some CSS missing | ssr.go sin go.mod ni SSRInstance |
| `TestSSRLoader/ReloadSSRModuleHotReload` | no go.mod found | ExtractSSRAssets exige go.mod |
| `TestSSRLoader/LoadIconsFromLocalRoot` | Icon not loaded | ExtractSSRAssets exige go.mod |
| `TestSSRLoader/LoadIconsFromReceiverMethod_InHTML` | Icon not registered | ídem |
| `TestReloadSSRModule_OnlyRefreshesChangedAssets` | no go.mod found | ídem |

---

## Problema 1 — `ssr_invoke.go` genera código con tipo de interfaz incorrecto

### Causa raíz

`ssr_invoke.go:86-105` genera un `collect()` que exige:

```go
func collect(inst interface {
    RenderCSS() interface{ String() string }   // ← incorrecto
    RootCSS()   interface{ String() string }   // ← incorrecto
    ...
})
```

Los componentes retornan `*css.Stylesheet`. En Go, `*Stylesheet` no satisface
`interface{ RenderCSS() interface{ String() string } }` aunque `*Stylesheet` tenga
`String()` — los tipos de retorno deben ser idénticos para satisfacción de interfaz
estructural. El `collect()` nunca puede compilar contra código real.

Además, `tinywasm/css/ssr.go` no tiene `SSRInstance()` — el generated main.go
intenta llamar `css.SSRInstance()` que no existe → error de compilación.

### Corrección

**Eliminar `collect()`. El generador emite llamadas directas sobre tipos concretos.**

Ningún módulo cambia. `generateExtractorMain` inspecciona cada `ssr.go` con regex
**anclado a firma de función** (`^func SSRInstance\(`, `^func.*RootCSS\(\) \*Stylesheet`,
etc.) — no substring suelto, para evitar falsos positivos por variables/comentarios
que mencionen los nombres. Las firmas a detectar son exactamente seis:

Notación: `<pkg>` = nombre corto del paquete (declarado en `package X` del `ssr.go`,
no el import path completo). `<x>` = `inst` si el módulo tiene `SSRInstance`, o
`<pkg>` si son funciones de paquete.

| Patrón regex | Significado | Emite |
|---|---|---|
| `^func SSRInstance\(` | componente con receptor → instancia | `inst := <pkg>.SSRInstance()` |
| `^func.*RootCSS\(\) \*Stylesheet` | exporta CSS root | `Root: <x>.RootCSS().String()` |
| `^func.*RenderCSS\(\) \*Stylesheet` | exporta CSS de instancia | `Render: <x>.RenderCSS().String()` |
| `^func.*RenderHTML\(\) string` | exporta HTML | `HTML: <x>.RenderHTML()` |
| `^func.*RenderJS\(\) string` | exporta JS | `JS: <x>.RenderJS()` |
| `^func.*IconSvg\(\) map\[string\]string` | iconos | `Icons: <x>.IconSvg()` |

`assetmin` ya tiene `cssModulePath` como constante especial — esa distinción existente
se aplica en la generación:

```go
// Componentes — tienen SSRInstance() + métodos con receptor
inst := selectsearch.SSRInstance()
all["selectsearch"] = ssr{
    Render: inst.RenderCSS().String(),  // *Stylesheet.String() → compila
    HTML:   inst.RenderHTML(),
    JS:     inst.RenderJS(),
    Icons:  inst.IconSvg(),
    // Root: inst.RootCSS().String()  ← solo si ssr.go contiene "RootCSS"
}

// tinywasm/css — funciones de paquete, sin SSRInstance
all["tinywasm/css"] = ssr{
    Root:   css.RootCSS().String(),
    Render: css.RenderCSS().String(),
}
```

Go verifica los tipos en compilación dentro del generated `main.go`. `assetmin` no
importa `tinywasm/css`, no conoce `*Stylesheet` — solo genera texto de código fuente
que el compilador valida. Sin reflexión, sin interfaces en runtime.

---

## Problema 2 — `ExtractSSRAssets` exige `go.mod` en todos los casos

### Causa raíz

`ssr_extract.go:26-31` rechaza cualquier directorio sin `go.mod`:

```go
if _, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil {
    return nil, fmt.Errorf("no go.mod found: %w", err)
}
```

Los tests de hot-reload (`ssr_event_filter_test.go`, `css_ssr_hotreload_test.go`,
`ssr_loader_reload_test.go`, `ssr_refresh_test.go`) y los tests del loader
(`ssr_loader_test.go`) escriben `ssr.go` **sin** `go.mod`. Esto era válido con el
extractor AST anterior. La restricción de `go.mod` es necesaria para `go run`, pero
no todos los tests ejercen el path de compilación.

El **hot-reload** no necesita re-compilar: cuando un archivo `.css` cambia en un módulo,
el CSS ya fue compilado en la carga inicial. El único dato que cambia es el contenido
de la cadena que retornarían los métodos. Re-ejecutar `go run` para eso es incorrecto
arquitecturalmente — debería haber un mecanismo de recarga más liviano.

### Corrección

**Realidad del proyecto:** todos los módulos actuales (`tinywasm/css`, componentes)
usan DSL tipado. **No existe** todavía un módulo "embed-only sin SSRInstance" en el
codebase. La rama "regex liviano" para módulos sin DSL queda **fuera de scope**
hasta que aparezca un caso real — sería YAGNI implementarla preventivamente.

**Para el hot-reload** (`ReloadSSRModule`): mantener `ExtractSSRAssets` como
mecanismo único. El costo (`go run` ~500 ms) se mitiga via Problema 3 (caché por
hash) y, si la medición lo justifica, via Problema 7 (binario persistente).

**Para los tests de hot-reload**: migrarlos al contrato correcto.
- Tests que prueban el slot del cache (`UpdateSSRModule` + `RegenerateCache`)
  **no necesitan** `ExtractSSRAssets` — registran CSS directamente con
  `am.UpdateSSRModule(name, css, ...)` y verifican el slot.
- Tests que prueban `ReloadSSRModule` extremo a extremo deben proveer módulo
  completo con `go.mod` + `SSRInstance` (vía `writeStubModule`, Problema 5).

---

## Problema 3 — Caché global nunca se invalida

### Causa raíz

`ssr_extract.go:61-78` usa `rootDir` como clave fija del caché global:

```go
ssrCacheMu.RLock()
cachedResults, hasCached := ssrExtractCache[rootDir]
ssrCacheMu.RUnlock()
```

Cuando `ReloadSSRModule` llama `ExtractSSRAssets` tras un cambio de archivo, la
entrada sigue presente → retorna datos viejos → CSS stale.

`ssr_cache.go` implementa un caché basado en hash de contenido (`computeModuleHashSet`,
`ssrCache.get/set/invalidate`) pero **nunca se usa**: `ssr_extract.go` mantiene su
propio mapa independiente.

### Corrección

Reemplazar `ssrExtractCache` con la instancia de `ssrCache` definida en `ssr_cache.go`.
El flujo correcto:

```
1. computeModuleHashSet(modules)   →  hashKey
2. ssrGlobalCache.get(hashKey)     →  hit → retornar inmediatamente
3. miss → invokeSSRExtractorOnce   →  results
4. ssrGlobalCache.set(hashKey, results)
```

El hash cambia automáticamente cuando cualquier `.go` del módulo cambia, sin
necesidad de invalidación explícita. `ReloadSSRModule` no necesita invalidar
manualmente — basta con que `ExtractSSRAssets` use el hash actualizado.

**Sobre `ssrCache.invalidate`:** tras este cambio el método queda sin uso en
producción. **Eliminarlo** junto con `ssrExtractCache` (no mantenerlo "por
simetría"). Si en el futuro un test requiere forzar invalidación, podrá
reintroducirse con un caso de uso real.

---

## Problema 4 — Doble descubrimiento de módulos incoherente

### Causa raíz

`loadSSRModulesLocked` tiene un override testeable (`c.listModulesFn`) que en tests
devuelve la lista de directorios. Pero `ExtractSSRAssets` llama internamente
`discoverModules(rootDir)` que ejecuta `go list -m -json all` sin respetar ese
override. Los dos paths de descubrimiento producen `Module.Path` distintos.

Esto causa que `TestLoader_AppFullyReplacesCss` falle: el loader construye módulos
con `Path = filepath.Base(dir)` (e.g. `"css"`), pero el extractor descubre módulos
con el path real del go.mod (e.g. `"example.com/test/css"`). Las claves no coinciden
en `cachedResults[targetModulePath]` → el módulo raíz no encuentra su entrada.

### Corrección

`ExtractSSRAssets` no debe hacer descubrimiento propio cuando el módulo ya es conocido.
Refactorizar para aceptar el `Module` como parámetro en la función interna:

```go
// API pública — descubrimiento propio (uso desde fuera del loader)
func ExtractSSRAssets(moduleDir string) (*SSRAssets, error)

// Interna — módulo ya resuelto (usada desde loadSSRModulesLocked)
// El parámetro binCachePath queda reservado para Problema 7 (binario persistente).
// Por ahora se pasa "" y la implementación sigue usando `go run`.
func extractSSRAssetsForModule(m Module, rootDir, binCachePath string) (*SSRAssets, error)
```

`loadSSRModulesLocked` pasa el `Module` directamente (con `Path` ya conocido).
Elimina el doble `go list` y garantiza que las claves del caché sean consistentes.
Firmar `binCachePath` ahora evita re-tocar la API cuando se implemente Problema 7.

---

## Problema 5 — Tests de pipeline completo con go.mod y replace fallan en CI

### Causa raíz

`TestLoader_AppFullyReplacesCss` y `TestLoader_CssDefaultWins_NoAppRoot` crean
fixtures con `replace` directives entre módulos. `go list -m -json all` en estos
directorios falla si no hay `go.sum`. El fallback en `discoverModules` produce paths
incorrectos (ver Problema 4).

### Corrección

Añadir helper `writeStubModule` (mencionado en el plan original, nunca implementado):

```go
// tests/stub_module_test.go
func writeStubModule(t *testing.T, dir, modulePath, pkgName, body string) {
    t.Helper()
    gomod := fmt.Sprintf("module %s\ngo 1.22\n", modulePath)
    os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)
    os.WriteFile(filepath.Join(dir, "ssr.go"),
        []byte("//go:build !wasm\n\npackage "+pkgName+"\n\n"+body), 0644)
}
```

Los fixtures que prueban el pipeline completo end-to-end (compile-and-invoke real)
deben ser **auto-contenidos**: sin `require` entre módulos del test. Cada módulo
define sus propios tipos auxiliares (`type strVal string; func (s strVal) String() string { ... }`).
No usar `replace` a menos que sea imprescindible — si se necesita simular la relación
root → css, modelarlo con dos módulos independientes que el loader recibe vía
`SetListModulesFn`.

---

## Problema 6 — `"strings"` stdlib en archivos de producción

`ssr_loader.go`, `ssr_extract.go` e `ssr_invoke.go` importan `"strings"`. Por
convención del proyecto todo manejo de strings debe usar `github.com/tinywasm/fmt`.

### Corrección

Reemplazar `strings.Contains`, `strings.HasSuffix`, `strings.Split`,
`strings.ReplaceAll` con equivalentes de `tinywasm/fmt`. Para `strings.NewReader`
(decodificación JSON de salida de `go list`) usar `bytes.NewReader(out)` donde `out`
ya es `[]byte`.

---

## Problema 7 — `go run` recompila en cada invocación (diferido)

### Causa raíz

El cuello de botella del dev loop no es la extracción ni el caché de resultados,
sino el costo fijo de `go run`: ~250–350 ms de compilación + link por invocación,
incluso cuando las fuentes no cambiaron. El benchmark `PERFORMANCE_COMPARISON.md`
mide warm path como "cache hit del resultado JSON" (~10 ms), pero el flujo real de
edición desde `tinywasm/app` siempre invalida ese caché (estás editando) → el
usuario percibe ~500 ms por cada cambio.

### Por qué diferir, no implementar ahora

- El plan actual repara correctness. Mezclar optimización rompe el bisect cuando
  un test falle: ¿es la migración a `*Stylesheet` o el binario persistente?
- No hay baseline medido todavía. Si el 95% de los cambios reales en `tinywasm/app`
  son a archivos `.css` embebidos (path liviano del Problema 2), el dev loop ya es
  de ~50 ms y esta optimización es innecesaria.
- La firma `extractSSRAssetsForModule(m, rootDir, binCachePath)` (Problema 4) ya
  deja el hook listo. No hay deuda arquitectónica por esperar.

### Corrección futura (cuando los benchmarks lo justifiquen)

Reemplazar `go run` por `go build -o <cache>/ssr_extractor_<hash>` + ejecución
directa del binario:

```
1. Computar hash de las fuentes del extractor + módulos.
2. Si existe binario para ese hash → ejecutar directo (~50 ms total).
3. Si no → go build (incremental gracias a $GOCACHE: ~150–200 ms) + ejecutar.
```

Aprovecha el build cache nativo de Go para link incremental. Sin `plugin`, sin
warm subprocess, sin dependencias nuevas. Esperado: dev loop de ~500 ms → ~150 ms
en cambios incrementales, ~50 ms cuando nada cambia.

### Por qué NO usar `tinywasm/gobuild` ni `CompileToMemory`

- `gobuild.CompileToMemory` ahorra ~5 ms (el disk write), no los 350 ms del compile.
- El binario debe ejecutarse para evaluar `RenderCSS().String()` — un `[]byte` en
  RAM sigue requiriendo `fork+exec` (memfd o disco, da igual).
- Extender `gobuild` con compile-and-invoke + cache por hash rompe su SRP (solo
  compila). Si se necesita warm subprocess más adelante, será un paquete nuevo.

### Descartado: warm subprocess + `plugin`

`plugin` no funciona en Windows, rompe entre versiones de Go, y exige mismo
toolchain exacto. Mala relación complejidad/beneficio. No considerar.

---

## Secuencia de implementación

```
1. Corregir generateExtractorMain (Problema 1):
   - Detectar patrón por módulo (SSRInstance vs funciones de paquete)
   - Generar código con tipo concreto *Stylesheet, no interface{}
   - Sin collect() genérico

2. Conectar ssr_cache.go con hash (Problema 3):
   - Eliminar ssrExtractCache
   - Usar ssrGlobalCache = newSSRCache() como variable global
   - computeModuleHashSet → get/set

3. Refactorizar Module param (Problema 4):
   - Añadir extractSSRAssetsForModule(m Module, rootDir, binCachePath string)
     (binCachePath = "" por ahora; hook para Problema 7)
   - loadSSRModulesLocked lo usa directamente
   - ExtractSSRAssets pública como wrapper

4. Migrar tests hot-reload al contrato correcto (Problema 2):
   - Tests que prueban slot/cache: usar UpdateSSRModule directamente
   - Tests que prueban ReloadSSRModule con DSL: añadir writeStubModule + go.mod

5. Escribir writeStubModule + fixtures Capa B auto-contenidos (Problema 5)

6. Eliminar imports strings stdlib (Problema 6)

7. gotest — todos los tests deben pasar

8. Añadir BenchmarkIncrementalChange como baseline medido
   (gate de decisión para Problema 7 — NO implementar binario persistente aquí)

9. Actualizar documentación afectada (puntual, no rewrite):
   - API.md / QUICK_REFERENCE.md: nueva firma extractSSRAssetsForModule
     (interna) + ExtractSSRAssets (pública wrapper, sin cambios)
   - SSR.md / COMPONENT_REGISTRATION.md: convención SSRInstance() para
     componentes + excepción documentada de tinywasm/css (funciones de paquete)
   - ARCHITECTURE.md: caché unificado por hash (ssrCache); eliminar mención
     a ssrExtractCache; eliminar mención a collect()/interface{ String() string }
   - README.md: revisar snippets que muestren el flujo de extracción
```

---

## Criterios de aceptación

- `gotest` retorna 0 tests fallidos.
- Hot-reload (`TestCSSHotReload_SSRMode_UpdatesCorrectly`) pasa: CSS stale eliminado tras cambio.
- `TestLoader_AppFullyReplacesCss` pasa: `--css:1` ausente, `--app:1` presente.
- El generated main.go compila correctamente contra `*css.Stylesheet` real.
- `tinywasm/css/ssr.go` (sin `SSRInstance`, funciones de paquete) es manejado sin error.
- Benchmark warm path ≤ 10 ms (hash efectivo).
- Coverage de `assetmin` ≥ 80% tras los cambios.
- Ningún archivo de producción importa `"strings"` de stdlib.
- `ssrCache.invalidate` y `ssrExtractCache` eliminados del código de producción.
- Documentación (`API.md`, `SSR.md`, `COMPONENT_REGISTRATION.md`, `ARCHITECTURE.md`,
  `QUICK_REFERENCE.md`, `README.md`) sin referencias a `collect()`,
  `interface{ String() string }`, ni `ssrExtractCache`. La convención
  `SSRInstance()` y la excepción de `tinywasm/css` aparecen documentadas.
- `BenchmarkIncrementalChange` registrado como baseline (edita un `.go` entre
  iteraciones, mide wall-time real del dev loop). Sin umbral en esta fase — el
  dato gobierna la decisión de implementar Problema 7.
- `extractSSRAssetsForModule` acepta el parámetro `binCachePath` (puede ser `""`
  por ahora), dejando el hook listo para el binario persistente.
