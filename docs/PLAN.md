# PLAN — Tareas pendientes post-migración compile-and-invoke

> Continuación del plan previo (CHECK_PLAN.md, eliminado). Los Problemas 1–6
> originales fueron implementados parcialmente: `gotest` pasa 100%, pero quedan
> 4 tareas sin cerrar que afectan el contrato real con `*css.Stylesheet` y la
> documentación. El Problema 7 (binario persistente) sigue diferido.

---

## Estado verificado (2026-05-13)

| Item original | Estado |
|---|---|
| Problema 1 — `collect()` eliminado, generador emite tipos concretos | ✅ Hecho |
| Problema 2 — Tests hot-reload migrados | ✅ Hecho |
| Problema 3 — `ssrGlobalCache` por hash, `ssrExtractCache` eliminado | ✅ Hecho |
| Problema 4 — `extractSSRAssetsForModule(m, rootDir, modules, binCachePath)` | ✅ Hecho |
| Problema 5 — `writeStubModule` helper | ❌ **Pendiente** |
| Problema 6 — `"strings"` stdlib en producción | ✅ Hecho (usa `tinywasm/fmt`) |
| `ssrCache.invalidate` eliminado | ✅ Hecho |
| `gotest` 0 fallos | ✅ `ok github.com/tinywasm/assetmin/tests 8.258s` |
| Documentación actualizada | ❌ **Pendiente** |
| `BenchmarkIncrementalChange` baseline | ❌ **Pendiente** |
| `ssr_register.go` sin `interface{ String() string }` | ❌ **Pendiente** |

---

## Contrato real (recordatorio)

Los componentes (`tinywasm/components/*`) y `tinywasm/css` usan **retorno tipado**
`*css.Stylesheet`. No existe ni existirá un retorno `string` ni un adaptador
`interface{ String() string }`. El generated `main.go` llama directamente
`m.RootCSS().String()` sobre el tipo concreto.

```go
// componentes
func SSRInstance() *SelectSearch { return &SelectSearch{} }
func (c *SelectSearch) RenderCSS() *Stylesheet { return New(...) }

// tinywasm/css — funciones de paquete, sin SSRInstance
func RootCSS() *Stylesheet { return New(Root(...)) }
```

---

## Tarea 1 — Eliminar `interface{ String() string }` de `ssr_register.go`

### Causa raíz

`ssr_register.go:5-9` define interfaces de runtime que exigen el tipo de retorno
`interface{ String() string }`:

```go
type rootCssProvider interface{ RootCSS() interface{ String() string } }
type cssProvider    interface{ RenderCSS() interface{ String() string } }
```

En Go, `*css.Stylesheet` **no** satisface `interface{ RootCSS() interface{ String() string } }`
aunque `*Stylesheet` tenga método `String()` — la satisfacción estructural de
interfaces requiere igualdad exacta de tipos de retorno. Los tests pasan hoy
únicamente porque los mocks (`StringValue`, `mockRootProvider`, etc.) retornan
literalmente `interface{ String() string }`. Si un componente real
(`tinywasm/components/*`) se registra vía `RegisterComponents`, **fallará la
aserción de tipo silenciosamente** y se ignorará su CSS/HTML/JS.

### Corrección

Cambiar las interfaces a tipos de retorno concretos `*css.Stylesheet`:

```go
import "github.com/tinywasm/css"

type rootCssProvider interface{ RootCSS()   *css.Stylesheet }
type cssProvider    interface{ RenderCSS() *css.Stylesheet }
type jsProvider     interface{ RenderJS()   string }
type htmlProvider   interface{ RenderHTML() string }
type iconProvider   interface{ IconSvg()    map[string]string }
```

Luego en `RegisterComponents`:

```go
if rp, ok := p.(rootCssProvider); ok {
    rootCSS := rp.RootCSS().String()   // *Stylesheet.String() directo, sin adaptador
    ...
}
```

**No hay riesgo de ciclo de imports**: `tinywasm/css` y `tinywasm/dom` son
capas base del proyecto y nunca importan `assetmin`. `assetmin` puede importar
`tinywasm/css` libremente.

**Contrato estructural**: cualquier componente (en `tinywasm/components/*` o en
repos de terceros) que exponga `RenderCSS() *css.Stylesheet` satisface
automáticamente la interfaz, sin necesidad de importar `assetmin`. Los terceros
solo importan `tinywasm/css` (para construir el `*Stylesheet`) y `tinywasm/dom`
(para la firma de HTML/eventos). Esa es justamente la razón de fijar el tipo
concreto: el bug previo (`interface{ String() string }`) rompía la coincidencia
exacta requerida por la satisfacción estructural de Go.

### Actualizar mocks de test

Tras el cambio, los mocks en `tests/ssr_register_test.go`,
`tests/ssr_register_root_test.go`, `tests/ssr_loader_root_override_test.go`,
`tests/ssr_extract_root_test.go`, `tests/ssr_extract_test.go` deben retornar
`*css.Stylesheet` real (construido con el DSL) en vez de `StringValue`.

---

## Tarea 2 — Helper `writeStubModule` para fixtures Capa B

### Causa raíz

El plan original requería un helper para tests que ejercen el pipeline completo
end-to-end (compile-and-invoke real con `go.mod` + `SSRInstance`). Actualmente
cada test que necesita un módulo stub escribe inline su `go.mod` + `ssr.go`,
duplicando código.

### Corrección

Crear `tests/stub_module_test.go`:

```go
package tests

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/tinywasm/fmt"
)

// writeStubModule escribe un módulo Go auto-contenido con go.mod + ssr.go.
// modulePath: ruta del módulo (ej. "example.com/test/css").
// pkgName: nombre del paquete declarado en ssr.go.
// body: contenido del archivo ssr.go DESPUÉS de la línea `package X`.
func writeStubModule(t *testing.T, dir, modulePath, pkgName, body string) {
    t.Helper()
    gomod := fmt.Sprintf("module %s\ngo 1.22\n", modulePath)
    if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
        t.Fatalf("write go.mod: %v", err)
    }
    src := "//go:build !wasm\n\npackage " + pkgName + "\n\n" + body
    if err := os.WriteFile(filepath.Join(dir, "ssr.go"), []byte(src), 0644); err != nil {
        t.Fatalf("write ssr.go: %v", err)
    }
}
```

**Migrar callers**: identificar tests que escriben `go.mod` + `ssr.go` inline
para fixtures Capa B y reemplazar por `writeStubModule(...)`. Candidatos:
`ssr_loader_test.go`, `ssr_loader_reload_test.go`, `ssr_refresh_test.go`.

**Regla**: fixtures auto-contenidos, **sin `replace` directives** entre módulos.
Cada módulo define sus propios tipos auxiliares.

---

## Tarea 3 — `BenchmarkIncrementalChange` como baseline medido

### Causa raíz

Los benchmarks actuales (`benchmark/benchmark_test.go`) miden:
- `BenchmarkExtractSSRAssets_SingleModule`
- `BenchmarkExtractSSRAssets_ThreeModules`
- `BenchmarkExtractSSRAssets_LargeCSS`

Pero **ninguno** mide el escenario real del dev loop: edición incremental de
`.go` entre iteraciones (que invalida hash → fuerza re-compile). Sin este dato
no hay base objetiva para decidir si el Problema 7 (binario persistente) vale
la pena.

### Corrección

Añadir en `benchmark/benchmark_test.go`:

```go
// BenchmarkIncrementalChange mide el wall-time real del dev loop:
// edita un .go entre iteraciones, forzando invalidación de hash y re-compile.
// NO hay umbral de pass/fail — el dato es input para decidir Problema 7.
func BenchmarkIncrementalChange(b *testing.B) {
    // setup: módulo stub con go.mod + ssr.go + un .css embebido
    // por iteración:
    //   1. modificar ssr.go (cambiar una constante para invalidar hash)
    //   2. medir ExtractSSRAssets end-to-end
    // reporta b.ReportMetric(elapsedMs, "ms/edit")
}
```

Documentar el resultado en `docs/ARCHITECTURE.md` sección "Performance baseline"
o en un nuevo `docs/PERFORMANCE.md` con la cifra medida (ej. "edit→extract:
~480 ms en M1, ~520 ms en linux/amd64"). Esa cifra es la que gobierna la
decisión futura del Problema 7.

---

## Tarea 4 — Actualizar documentación

### Archivos afectados y cambios concretos

#### `docs/SSR.md`

- **Línea 7**: la frase "instantiates each via `SSRInstance()`" debe matizarse:
  "instantiates each via `SSRInstance()` for components, or calls package-level
  functions for modules without instance (e.g. `tinywasm/css`)".
- **Línea 80**: el snippet de ejemplo usa `RenderCSS() interface{ String() string }`.
  Reemplazar por:
  ```go
  func (b *Button) RenderCSS() *css.Stylesheet {
      return css.New(css.Rule(".btn", css.Decl("color", "red")))
  }
  ```
- Añadir párrafo: "Components return typed `*css.Stylesheet`. The generated
  extractor calls `.String()` on the concrete type — no `interface{ String() string }`
  adapter exists."

#### `docs/ARCHITECTURE.md`

- **Línea 20**: descripción de "Extraction" — añadir que el caché global
  (`ssrGlobalCache`) usa hash de contenido del módulo (no `rootDir`) como clave,
  vía `computeModuleHashSet`.
- Eliminar cualquier mención a `collect()` genérico, `ssrExtractCache`, o
  `interface{ String() string }`.
- Añadir nota: la API interna es `extractSSRAssetsForModule(m, rootDir, modules, binCachePath)`.
  El parámetro `binCachePath` es un hook reservado para optimización futura
  (binario persistente); actualmente siempre `""`.

#### `docs/QUICK_REFERENCE.md`

- Verificar que no quedan referencias a `interface{ String() string }`,
  `collect()`, ni `ssrExtractCache` (búsqueda dio match en línea 168 pero es
  "icons collection" — texto no relacionado, dejar como está).
- Añadir entrada para la convención: "Components must export `SSRInstance() *T`;
  modules without instance (like `tinywasm/css`) expose package-level functions".

#### `docs/COMPONENT_REGISTRATION.md`

- Documentar la convención `SSRInstance()` para componentes con receptor.
- Documentar la excepción `tinywasm/css` (funciones de paquete sin
  `SSRInstance`).
- Mostrar el patrón de generación: el extractor detecta cada caso por regex
  anclado a firma de función.

#### `docs/API.md`

- Firma pública sin cambios: `ExtractSSRAssets(moduleDir string) (*SSRAssets, error)`.
- Documentar `extractSSRAssetsForModule` como interna (no exportada), con
  el parámetro `binCachePath` reservado.

#### `README.md`

- Revisar snippets de "quick start" / ejemplos de extracción y alinear con el
  contrato `*css.Stylesheet`.

---

## Tarea 5 — Verificación final

Tras Tareas 1–4:

```bash
cd /home/cesar/Dev/Project/tinywasm/assetmin
go test ./...                                    # 0 fallos
grep -rn "ssrExtractCache\|collect(" .           # 0 matches en .go
grep -rn "interface{ String() string }" *.go     # 0 matches en producción
grep -rn "\"strings\"" *.go                      # 0 matches en producción
grep -rn "interface{ String() string }\|ssrExtractCache\|collect()" docs/  # 0 matches
go test -bench=BenchmarkIncrementalChange ./benchmark/
```

---

## Criterios de aceptación

- `gotest` retorna 0 fallos tras todos los cambios.
- `ssr_register.go` usa tipos concretos `*css.Stylesheet` en sus interfaces.
- Componentes reales (`tinywasm/components/*` y componentes de terceros en otros
  repos que retornen `*css.Stylesheet`) son aceptados por `RegisterComponents`
  sin necesidad de adaptador, importando solo `tinywasm/css` y `tinywasm/dom`.
- `writeStubModule` existe en `tests/stub_module_test.go` y es usado por al
  menos los tests de pipeline completo.
- `BenchmarkIncrementalChange` corre y reporta `ms/edit`; resultado anotado en
  `docs/ARCHITECTURE.md` o `docs/PERFORMANCE.md`.
- Documentación (`SSR.md`, `ARCHITECTURE.md`, `QUICK_REFERENCE.md`,
  `COMPONENT_REGISTRATION.md`, `API.md`, `README.md`) sin referencias a
  `collect()`, `interface{ String() string }`, ni `ssrExtractCache`.
- Convención `SSRInstance()` documentada con la excepción de `tinywasm/css`.

---

## Problema 7 — Binario persistente (sigue diferido)

Sin cambios respecto al plan anterior. Implementar **solo cuando**
`BenchmarkIncrementalChange` (Tarea 3) demuestre que el costo de `go run`
(~250–350 ms) domina el dev loop real. La firma
`extractSSRAssetsForModule(m, rootDir, modules, binCachePath)` ya deja el hook
listo: cuando se implemente, `binCachePath != ""` activará `go build -o <cache>/ssr_extractor_<hash>`
+ ejecución directa, con cache hit ~50 ms.

Descartado permanentemente: `plugin` (no Windows, frágil), warm subprocess
(complejidad alta).
