# PLAN — Correcciones post-migración compile-and-invoke

> Este plan reemplaza al plan anterior ("Typed CSS migration"). La migración se ejecutó
> parcialmente. Las fallas actuales son consecuencia de cuatro inconsistencias entre la
> implementación y los tests heredados. Este plan describe cada problema, su causa raíz
> y la corrección exacta a aplicar.

---

## Estado actual (diagnóstico)

`gotest` reporta 8 tests fallidos en el mismo paquete. Todos comparten una causa raíz
común con dos variantes:

| Test | Error observado | Causa |
|---|---|---|
| `TestCSSHotReload_SSRMode_UpdatesCorrectly` | stale CSS remains | cache no se invalida en ReloadSSRModule |
| `TestSSRMode_EmbeddedAssetHotReload` | CSS not updated | ExtractSSRAssets exige go.mod que no existe |
| `TestReload_AppGainsRootCSS` | Initial framework css not found | ídem |
| `TestReload_AppLosesRootCSS` | Initial app root css not found | ídem |
| `TestReload_ThirdPartyAddsRootCSS` | no go.mod found | ídem, explícito |
| `TestLoader_AppFullyReplacesCss` | framework css persiste | cache contaminada entre sub-tests |
| `TestSSRLoader/LoadSSRModulesOrder` | Some CSS missing | ssr.go sin go.mod ni SSRInstance |
| `TestSSRLoader/ReloadSSRModuleHotReload` | no go.mod found | ídem |
| `TestSSRLoader/LoadIconsFromLocalRoot` | Icon not loaded | ídem |
| `TestSSRLoader/LoadIconsFromReceiverMethod_InHTML` | Icon not registered | ídem |
| `TestReloadSSRModule_OnlyRefreshesChangedAssets` | no go.mod found | ídem |

---

## Problema 1 — `ExtractSSRAssets` exige go.mod pero los tests no lo proveen

### Causa raíz

`ssr_extract.go:26-31` rechaza cualquier directorio sin `go.mod` y sin `ssr.go`
exactamente en esa forma:

```go
if _, err := os.Stat(filepath.Join(moduleDir, "go.mod")); err != nil {
    return nil, fmt.Errorf("no go.mod found: %w", err)
}
```

Los tests en `ssr_loader_test.go`, `ssr_event_filter_test.go`,
`ssr_loader_reload_test.go`, `ssr_refresh_test.go` y `css_ssr_hotreload_test.go`
escriben archivos `ssr.go` con el **contrato antiguo** (retorno de `string` plano, sin
`SSRInstance()`). Compile-and-invoke no puede usarlos.

La función `ExtractSSRAssets` es llamada tanto desde `loadSSRModulesLocked`
(carga inicial) como desde `ReloadSSRModule` (hot-reload). Ambos paths heredan el
problema.

### Corrección

**Separar en dos capas de extracción:**

**Capa A — Regex/AST rápida** (`ssr_read.go`, función `readSSRAssets`):  
Lee `ssr.go` directamente por expresiones regulares para los tres patrones simples:

```
func RenderCSS() string { return "literal" }
func RootCSS()   string { return "literal" }
func IconSvg()   map[string]string { return map[string]string{"k": "v"} }
```

No requiere `go.mod`. Siempre disponible. Es la ruta para tests y para módulos que
no adoptan el DSL tipado.

**Capa B — Compile-and-invoke** (`ssr_invoke.go`, función `invokeSSRExtractorOnce`):  
Solo se activa cuando `ssr.go` contiene `func SSRInstance()` — detectado por grep
simple antes de intentar la compilación. Requiere `go.mod` porque ejecuta `go run`.

**Selector en `ExtractSSRAssets`:**

```go
func ExtractSSRAssets(moduleDir string) (*SSRAssets, error) {
    ssrPath := filepath.Join(moduleDir, "ssr.go")
    if _, err := os.Stat(ssrPath); err != nil {
        return emptyAssets(moduleDir), nil  // sin ssr.go → vacío, no error
    }
    if hasSSRInstance(ssrPath) {
        return extractViaInvoke(moduleDir)  // Capa B
    }
    return extractViaRead(moduleDir)        // Capa A
}
```

**Justificación:** La Capa A cubre el 100% de los tests actuales y es la ruta de
steady-state durante hot-reload (el archivo ya compiló, solo el CSS cambia). La Capa B
es el camino de producción para DSL tipado. No son mutuamente excluyentes: un módulo
migra a Capa B simplemente añadiendo `SSRInstance()` y `go.mod`.

---

## Problema 2 — El caché nunca se invalida en `ReloadSSRModule`

### Causa raíz

`ssr_extract.go:61-78` cachea por `rootDir`:

```go
ssrCacheMu.RLock()
cachedResults, hasCached := ssrExtractCache[rootDir]
ssrCacheMu.RUnlock()

if !hasCached {
    results, err := invokeSSRExtractorOnce(rootDir, modules)
    ssrExtractCache[rootDir] = results
}
```

Cuando `ReloadSSRModule` llama `ExtractSSRAssets` tras un cambio de archivo, la
entrada `ssrExtractCache[rootDir]` todavía existe → retorna datos viejos → el test
comprueba que el CSS viejo sigue presente y falla.

El archivo `ssr_cache.go` implementa correctamente un caché basado en hash de
contenido (`computeModuleHashSet`, `newSSRCache`) pero **nunca se usa** —
`ssr_extract.go` mantiene su propio `ssrExtractCache` independiente.

### Corrección

**Reemplazar `ssrExtractCache` con la instancia de `ssrCache` definida en
`ssr_cache.go`.**

El flujo correcto en `extractViaInvoke`:

```
1. computeModuleHashSet(modules)  →  hashKey
2. ssrGlobalCache.get(hashKey)    →  hit → retornar
3. miss → invokeSSRExtractorOnce  →  results
4. ssrGlobalCache.set(hashKey, results)
```

En `ReloadSSRModule`, antes de llamar `ExtractSSRAssets`, invalidar la entrada:

```go
// ssr_loader.go ReloadSSRModule
hashKey, _ := computeModuleHashSet([]Module{{Dir: moduleDir}})
ssrGlobalCache.invalidate(hashKey)
assets, err := ExtractSSRAssets(moduleDir)
```

O — alternativa más simple — hacer que `extractViaInvoke` siempre recompute el hash
del directorio modificado y compare con el hash almacenado. Si difieren, recompila.

**Justificación:** El hash MD5 de todos los `.go` del módulo cambia en cuanto se
escribe un archivo. El caché basado en hash es correcto-por-construcción: no requiere
invalidación manual explícita. `ssr_cache.go` ya lo implementa; falta conectarlo.

---

## Problema 3 — El caché global contamina tests consecutivos

### Causa raíz

`ssrExtractCache` es una variable global de paquete:

```go
var (
    ssrExtractCache = make(map[string]map[string]ssrCollectorOutput)
    ssrCacheMu      sync.RWMutex
)
```

Dos tests que usan el mismo `rootDir` (probable en `t.TempDir()` cuando el sistema
reutiliza paths) comparten entradas de caché. `TestLoader_AppFullyReplacesCss` falla
porque lee resultados de un test anterior: ve `--css:1` aunque el test configuró
`--app:1` para el módulo raíz.

### Corrección

Al resolver el Problema 2 (usar `ssrCache` con hash de contenido), este problema
desaparece automáticamente: el hash del nuevo directorio de test no coincide con ningún
hash previo → siempre miss → datos frescos.

Como medida de seguridad adicional, en tests que usan `extractViaInvoke`, el helper
`writeStubModule` (ver Problema 4) debe incluir un token único por test en el
contenido del `ssr.go` para garantizar hashes distintos.

---

## Problema 4 — Tests que ejercen el pipeline completo no tienen go.mod ni SSRInstance

### Causa raíz

Los tests `TestLoader_AppFullyReplacesCss` y `TestLoader_CssDefaultWins_NoAppRoot`
**sí** escriben `go.mod` y `SSRInstance()` (correctos para Capa B). Pero sus módulos
usan `replace` directives y no tienen `go.sum`. `go list -m -json all` falla en
entornos sin red. El fallback `discoverModules → error → basename` produce paths de
módulo incorrectos que el generated `main.go` no puede importar.

Los tests de hot-reload (`ssr_loader_reload_test.go`, `ssr_refresh_test.go`) NO
escriben `go.mod` — son Capa A, deben ser migrados a usar `readSSRAssets` (Problema 1).

### Corrección

**Para tests Capa A** (hot-reload, CSS simple): eliminar la dependencia en
`ExtractSSRAssets` para el contrato `string`. Estos tests deben funcionar sin `go.mod`.
Implementar `readSSRAssets` (Problema 1) y redirigir `ExtractSSRAssets` al detector.

**Para tests Capa B** (pipeline completo con go.mod): simplificar el fixture para evitar
dependencias entre módulos. Cada stub module debe ser auto-contenido (sin `require`
externo) y usar `replace` solo cuando sea inevitable. Alternativa: escribir el generated
`main.go` directamente en el tempdir del módulo sin necesidad de `go list` cuando el
módulo es el único.

**Helper `writeStubModule`** (ya mencionado en el PLAN anterior, aún no implementado):

```go
// tests/stub_module_test.go
func writeStubModule(t *testing.T, dir, modulePath, pkgName, body string) {
    t.Helper()
    gomod := fmt.Sprintf("module %s\ngo 1.21\n", modulePath)
    os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)
    os.WriteFile(filepath.Join(dir, "ssr.go"), []byte(
        fmt.Sprintf("package %s\n%s", pkgName, body)), 0644)
}
```

Tests `ReloadSSRModule` que ejercen la Capa B deben usar este helper con el patrón
completo:

```go
writeStubModule(t, moduleDir, "example.com/mymodule", "mymodule", `
type C struct{}
func (c *C) RenderCSS() interface{ String() string } { return str(".new{}") }
func (c *C) RenderHTML() string { return "" }
func (c *C) RenderJS() string { return "" }
func (c *C) IconSvg() map[string]string { return nil }
type str string
func (s str) String() string { return string(s) }
func SSRInstance() *C { return &C{} }
`)
```

---

## Problema 5 — `loadSSRModulesLocked` usa `listModulesFn` pero `ExtractSSRAssets` ignora ese override

### Causa raíz

`loadSSRModulesLocked` tiene un override testeable:

```go
if c.listModulesFn != nil {
    dirs, err := c.listModulesFn(c.RootDir)
    // construye []Module con Path = filepath.Base(dir)
}
```

Pero `ExtractSSRAssets(m.Dir)` internamente llama `discoverModules(rootDir)` que
siempre ejecuta `go list -m -json all`. Los dos sistemas de descubrimiento de módulos
son independientes y producen resultados distintos. El override de test es ignorado
durante la extracción real.

### Corrección

`ExtractSSRAssets` no debe hacer descubrimiento propio cuando se llama desde el
loader. Refactorizar para aceptar el `Module` pre-resuelto:

```go
// Función interna usada por el loader
func extractSSRAssetsForModule(m Module) (*SSRAssets, error)

// API pública conserva la firma pero hace su propio descubrimiento
func ExtractSSRAssets(moduleDir string) (*SSRAssets, error) {
    m := Module{Dir: moduleDir, Path: filepath.Base(moduleDir)}
    // detect go.mod para Path correcto
    return extractSSRAssetsForModule(m)
}
```

El loader pasa el `Module` directamente (con `Path` ya conocido), eliminando el
doble `go list`.

**Justificación:** Reduce `go list` de N veces a 1 vez por ciclo de carga, y elimina la
discrepancia entre el discovery del loader y el de `ExtractSSRAssets`.

---

## Problema 6 — `strings` stdlib en `ssr_loader.go` y `ssr_extract.go`

`ssr_loader.go:9` y `ssr_extract.go:9` importan `"strings"`. Por convención del
proyecto, todo uso de `strings` debe reemplazarse con `github.com/tinywasm/fmt`.

### Corrección

En `ssr_loader.go`: `strings.Contains` → `fmtpkg.Contains` (o equivalente de
`tinywasm/fmt`). En `ssr_extract.go`: `strings.HasSuffix`, `strings.NewReader` →
equivalentes de `tinywasm/fmt` donde existan; si no existe equivalente para
`strings.NewReader`, usar `bytes.NewReader` sobre `[]byte`.

---

## Secuencia de implementación

Las correcciones tienen dependencias entre sí. Aplicar en este orden:

```
1. Implementar readSSRAssets (Capa A)            → desbloquea tests Capa A
2. Conectar ssrCache con hash (Problema 2)        → desbloquea invalidación
3. Selector en ExtractSSRAssets (Problema 1)      → une Capas A y B
4. Refactorizar Module param (Problema 5)         → elimina doble discovery
5. writeStubModule helper + migrar tests Capa B   → desbloquea tests complejos
6. Reemplazar strings stdlib (Problema 6)         → conformidad de proyecto
7. Ejecutar gotest — todos los tests deben pasar
```

---

## Criterios de aceptación

- `gotest` retorna 0 tests fallidos en `github.com/tinywasm/assetmin`.
- Hot-reload (`TestCSSHotReload_SSRMode_UpdatesCorrectly`, `TestSSRMode_EmbeddedAssetHotReload`) pasa sin `go.mod` en el dir del módulo.
- `TestLoader_AppFullyReplacesCss` pasa: `--css:1` ausente, `--app:1` presente.
- `TestSSRLoader/ReloadSSRModuleHotReload` pasa: CSS viejo eliminado, CSS nuevo presente.
- Benchmark warm path ≤ 10 ms (caché hash efectivo, sin re-compilar cuando archivos no cambiaron).
- Ningún archivo de producción importa `"strings"` de stdlib.
