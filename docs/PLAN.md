# PLAN: assetmin — SSR Module CSS/JS/HTML/Icon Extraction

## Objetivo

Permitir que `assetmin` descubra y extraiga automáticamente los assets declarados en archivos
`ssr.go` (build tag `!wasm`) de todos los módulos Go del proyecto (tanto locales via `replace`
como en el proxy cache), los inyecte en memoria respetando el orden de carga, y los recargue
cuando un archivo `ssr.go` cambia (hot reload solo para módulos locales).

## Contratos esperados en `ssr.go` de módulos externos

```go
//go:build !wasm

package mypkg

// CSS — string literal o via //go:embed
func RenderCSS() string { return `.my-class { color: red; }` }

// JS — string literal o via //go:embed
func RenderJS() string { return `console.log("hello")` }

// SVG icons — map literal inline
func IconSvg() map[string]string {
    return map[string]string{"icon-id": `<svg>...</svg>`}
}

// HTML SSR — string literal
func RenderHTML() string { return `<div class="my-widget"></div>` }
```

Todos los métodos son opcionales. El extractor usa type assertions / interfaces.

## Orden de inyección en assets

| Slot | Contenido | Quién |
|---|---|---|
| `contentOpen` | Theme base: CSS vars de `tinywasm/dom` | Primero siempre |
| `contentMiddle` | CSS/JS de módulos externos (alphabético por módulo) | Paquetes dependencia |
| `contentClose` | CSS/JS del proyecto raíz (donde se ejecuta) | Último, puede sobrescribir |

## Cambios en `assetmin.Config`

Añadir `RootDir string` — directorio raíz del proyecto donde existe `go.mod`.
Necesario para ejecutar `go list -m -json all` y descubrir módulos en proxy.

```go
type Config struct {
    OutputDir          string
    RootDir            string                 // NUEVO: para descubrimiento de módulos
    GetSSRClientInitJS func() (string, error)
    AppName            string
    AssetsURLPrefix    string
    DevMode            bool
}
```

## Nuevos archivos

### `ssr_extract.go` — Extracción AST de `ssr.go` (build tag `!wasm`)

**Responsabilidad:** dado un path absoluto a un directorio de módulo, encuentra `ssr.go`,
parsea el AST de Go y extrae el valor de retorno de `RenderCSS()`, `RenderJS()`,
`RenderHTML()` e `IconSvg()`. Soporta strings literales, concatenaciones simples, y
archivos embebidos via `//go:embed`.

```go
//go:build !wasm

package assetmin

type SSRAssets struct {
    ModuleName string
    CSS        string
    JS         string
    HTML       string
    Icons      map[string]string
}

// ExtractSSRAssets parsea el archivo ssr.go en moduleDir y retorna los assets.
// Soporta: string literals, raw strings, concatenación, //go:embed.
func ExtractSSRAssets(moduleDir string) (*SSRAssets, error)
```

Casos que el parser AST debe manejar:
- `return "literal"` → extraer string directo
- `return `raw string`` → extraer raw string
- `//go:embed theme.css\nvar ThemeCSS string` → leer el archivo `.css` en moduleDir
- `return map[string]string{"id": "<svg>..."}` → extraer CompositeLit
- Si la función no existe en el archivo → campo vacío (no es error)

### `ssr_loader.go` — Descubrimiento y carga de módulos (build tag `!wasm`)

**Responsabilidad:** descubrir todos los módulos Go del proyecto, buscar `ssr.go` en cada
uno, extraer assets via AST y llamar `UpdateSSRModule`. `RegisterComponents` es un camino
separado para cuando el dev tiene instancias live de structs — `LoadSSRModules` nunca
lo usa.

```go
//go:build !wasm

package assetmin

// LoadSSRModules descubre todos los módulos via `go list -m -json all` ejecutado
// en Config.RootDir, busca ssr.go en cada uno, extrae assets e inyecta en memoria.
// Retorna error si falla completamente; degradación parcial se loguea como warning.
func (c *AssetMin) LoadSSRModules() error

// WaitForSSRLoad espera a que LoadSSRModules termine, hasta el timeout dado.
// Solo para tests — en producción LoadSSRModules corre en goroutine.
func (c *AssetMin) WaitForSSRLoad(timeout time.Duration)

// ReloadSSRModule re-extrae e inyecta los assets de un único módulo por su directorio.
// Llamado via GoModHandler.OnSSRFileChange cuando ssr.go cambia en un módulo local.
// Usa UpdateSSRModule internamente → dedup automático, no acumula.
func (c *AssetMin) ReloadSSRModule(moduleDir string) error
```

Lógica de descubrimiento:
1. Ejecutar `go list -m -json all` en `Config.RootDir`; si falla → warning + usar solo replace-locals
2. Para cada módulo: obtener `Dir` del JSON (path absoluto en disco)
3. El módulo raíz (primer resultado = módulo actual) → sus assets van a `contentClose`
4. `tinywasm/dom` → sus assets van a `contentOpen` (theme base, siempre primero)
5. Resto de módulos → `contentMiddle` en orden alfabético por module path
6. Buscar `ssr.go` en cada `Dir`; si no existe → saltar sin error
7. Si existe: `ExtractSSRAssets(dir)` → `UpdateSSRModule(modulePath, ...)`
8. Módulos proxy: solo al arrancar. Módulos replace locales: arrancar + hot reload via callback

### `ssr_register.go` — Registro e inyección con deduplicación

**Responsabilidad:** implementar `RegisterComponents` (del doc) y `UpdateSSRModule`
con semántica de reemplazo (no acumulación) para soportar hot reload sin duplicados.

```go
//go:build !wasm

package assetmin

// RegisterComponents registra structs que implementan las interfaces SSR.
// Usa type assertions. Llamado por el dev cuando tiene instancias de sus módulos.
// Usa UpdateSSRModule internamente → idempotente.
func (c *AssetMin) RegisterComponents(providers ...any) error

// UpdateSSRModule inyecta o reemplaza los assets de un módulo por nombre.
// Internamente usa UpdateContent con event="write" para dedup por path.
// name es el nombre del módulo, eg: "github.com/cdvelop/clinical_encounter"
func (c *AssetMin) UpdateSSRModule(name string, css, js, html string, icons map[string]string)
```

Interfaces internas (package-private si no se necesitan fuera):
```go
type cssProvider  interface { RenderCSS() string }
type jsProvider   interface { RenderJS() string }
type htmlProvider interface { RenderHTML() string }
type iconProvider interface { IconSvg() map[string]string }
```

## Hot reload — `GoModHandler` como relay (NO cambiar `SupportedExtensions`)

`devwatch` entrega eventos `.go` usando `depFinder.ThisFileIsMine(handler.MainInputFileRelativePath(), path)`.
Como `assetmin.MainInputFileRelativePath()` retorna `""`, assetmin nunca recibiría
eventos `.go` aunque los declare en `SupportedExtensions`. No se cambia esta lógica.

**Solución:** `GoModHandler` (en `tinywasm/devflow`) añade un campo callback:

```go
// En GoModHandler — tinywasm/devflow/go_mod.go
OnSSRFileChange func(moduleDir string)

// En NewFileEvent de GoModHandler, antes de retornar:
if g.OnSSRFileChange != nil && fileName == "ssr.go" {
    g.OnSSRFileChange(filepath.Dir(filePath))
}
```

`tinywasm/app` conecta el callback al arrancar:
```go
h.GoModHandler.OnSSRFileChange = func(moduleDir string) {
    if err := h.AssetsHandler.ReloadSSRModule(moduleDir); err != nil {
        h.AssetsHandler.Logger("SSR reload error:", err)
    }
}
```

`assetmin` no necesita cambios en `SupportedExtensions` ni en `NewFileEvent`.
`tinywasm/devflow` requiere solo el campo adicional en `GoModHandler`.

## Tests requeridos

### `ssr_extract_test.go`

| Test | Entrada | Esperado | Prioridad |
|---|---|---|---|
| `TestExtractLiteralCSS` | `ssr.go` con `return ".cls{}"` | CSS correcto | Alta |
| `TestExtractRawStringCSS` | raw string multilínea | CSS completo | Alta |
| `TestExtractEmbedCSS` | `//go:embed theme.css` + archivo real en TempDir | contenido del archivo | Alta |
| `TestExtractMultipleEmbedFiles` | embed CSS + embed JS en mismo `ssr.go` | cada embed va al campo correcto, sin mezcla | Alta |
| `TestExtractIconSvg` | map literal con 2 iconos | mapa correcto | Alta |
| `TestExtractMissingMethod` | `ssr.go` sin `RenderCSS()` | campo vacío, no error | Alta |
| `TestExtractEmptyMethods` | `ssr.go` con `return ""` en todos los métodos | todos los campos vacíos, no error | Alta |
| `TestExtractNonExistentFile` | directorio sin `ssr.go` | error descriptivo | Alta |
| `TestExtractInvalidGoFile` | `ssr.go` con sintaxis inválida | error, no panic | Alta |

### `ssr_register_test.go`

| Test | Qué verifica | Prioridad |
|---|---|---|
| `TestUpdateSSRModuleNoDuplicate` | segunda llamada con mismo name reemplaza, no acumula | Alta |
| `TestUpdateSSRModuleContentSlot` | CSS → CSS handler, JS → JS handler, nunca cruzado | Alta |
| `TestUpdateSSRModuleEmptyFieldsSkipped` | `css=""` con `js="alert()"` → solo JS inyectado | Media |
| `TestRegisterComponentsCSS` | struct con `RenderCSS()` → CSS en bundle | Alta |
| `TestRegisterComponentsAllInterfaces` | struct con todos los métodos → todos los assets | Alta |
| `TestRegisterComponentsPartial` | struct con solo `RenderCSS()` → solo CSS, sin error | Alta |
| `TestRegisterComponentsIdempotent` | `RegisterComponents` llamado 2 veces mismo struct → no duplica | Media |

### `ssr_loader_test.go`

| Test | Qué verifica | Prioridad |
|---|---|---|
| `TestLoadSSRModulesOrder` | theme en contentOpen, módulos en contentMiddle, proyecto en contentClose | Alta |
| `TestLoadSSRModulesOutputOrder` | string CSS final: vars dom → módulos externos → proyecto raíz | Alta |
| `TestLoadSSRModulesGoListFails` | `listModulesFn` retorna error → degrada a replace-locals + warning, sin panic | Media |
| `TestLoadSSRModulesIdempotent` | `LoadSSRModules` llamado 2 veces → assets no se duplican | Media |
| `TestLoadSSRModulesRootDirEmpty` | `Config.RootDir=""` → error claro, no ejecuta go list | Baja |
| `TestWaitForSSRLoadTimeout` | `WaitForSSRLoad(1ms)` con carga lenta → retorna sin bloquear | Baja |
| `TestReloadSSRModuleHotReload` | cambio en `ssr.go` → CSS nuevo en memoria, sin duplicado | Alta |

### `concurrency_test.go` — extensión del test existente

| Test | Qué verifica | Prioridad |
|---|---|---|
| `TestSSRReloadConcurrency` | 10 goroutines llamando `ReloadSSRModule` simultáneamente → sin race, bundle consistente | Alta |

### `tinywasm/devflow/test/gomod_handler_test.go` — callback

| Test | Qué verifica | Prioridad |
|---|---|---|
| `TestOnSSRFileChangeTriggered` | callback se dispara solo para `ssr.go`, no para `model.go` ni `client.go` | Alta |
| `TestOnSSRFileChangeNilSafe` | `OnSSRFileChange == nil` → no panic al recibir evento `ssr.go` | Alta |

## Stage 0 — Migración de tests al directorio `tests/`

**Prerrequisito de todos los demás stages.** Todos los tests (existentes y nuevos) viven
en `tests/` como paquete externo `package assetmin_test`. Esto evita que los tests accedan
a campos privados y fuerza un API pública clara — cualquier campo que un test necesite
ver ya debe ser observable desde fuera.

### Campos privados actualmente accedidos en tests

| Campo privado | Archivo | API pública a añadir |
|---|---|---|
| `am.mainStyleCssHandler.contentMiddle` | `injection_test.go` | `am.ContainsCSS(substr string) bool` |
| `am.mainJsHandler.contentMiddle` | `injection_test.go`, `concurrency_test.go` | `am.ContainsJS(substr string) bool` |
| `am.spriteSvgHandler.contentMiddle` | `injection_test.go` | `am.ContainsSVG(substr string) bool` |
| `am.indexHtmlHandler.contentMiddle` | `injection_test.go` | `am.ContainsHTML(substr string) bool` |
| `am.registeredIconIDs["id"]` | `injection_test.go` | `am.HasIcon(id string) bool` |
| `am.mainJsHandler.GetMinifiedContent(am.min)` | `concurrency_test.go` | `am.GetMinifiedJS() ([]byte, error)` |

### Métodos públicos a añadir en `assetmin.go` o nuevo `inspect.go`

```go
// Para verificación en tests — también útiles para debugging en producción
func (c *AssetMin) ContainsCSS(substr string) bool
func (c *AssetMin) ContainsJS(substr string) bool
func (c *AssetMin) ContainsSVG(substr string) bool
func (c *AssetMin) ContainsHTML(substr string) bool
func (c *AssetMin) HasIcon(id string) bool
func (c *AssetMin) GetMinifiedJS() ([]byte, error)
func (c *AssetMin) GetMinifiedCSS() ([]byte, error)
```

### Archivos a mover a `tests/`

| Origen | Destino | Acción |
|---|---|---|
| `*_test.go` (todos) | `tests/*_test.go` | Mover + cambiar `package assetmin` → `package assetmin_test` |
| `test_setup.go` | `tests/setup_test.go` | Mover + adaptar a API pública |
| `test_helper.go` | `tests/helper_test.go` | Mover + adaptar a API pública |
| `test/` (directorio fixtures) | `tests/testdata/` | Renombrar para convención Go estándar |

### Patrón del paquete externo

```go
// tests/injection_test.go
package assetmin_test

import (
    "testing"
    "github.com/tinywasm/assetmin"
)

func TestAssetMin_AddAssets(t *testing.T) {
    am := assetmin.NewAssetMin(&assetmin.Config{...})
    am.InjectCSS("mock", "body { color: blue; }")
    if !am.ContainsCSS("body { color: blue; }") {  // API pública
        t.Error("CSS not found")
    }
}
```

### Tests nuevos SSR también van en `tests/`

Todos los archivos nuevos del plan (`ssr_extract_test.go`, `ssr_register_test.go`,
`ssr_loader_test.go`) se crean directamente en `tests/` como `package assetmin_test`.

### `tests/setup_test.go` — migración del setup existente

`test_setup.go` y `test_helper.go` se mueven a `tests/setup_test.go` y
`tests/helper_test.go` cambiando solo lo mínimo necesario:

1. `package assetmin` → `package assetmin_test`
2. Accesos a campos privados (`assetsHandler.mainJsHandler.outputPath`, etc.) se
   reemplazan por los nuevos métodos públicos de `inspect.go`
3. `setupTestEnv` pasa a usar `t.TempDir()` en lugar del directorio `test/` fijo —
   evita estado compartido entre tests paralelos

Además se añaden los helpers SSR que los nuevos tests necesitan y que no existen hoy:

```go
// Añadir a tests/setup_test.go (no reescribir lo existente)

// withModules inyecta directorios de módulos sin ejecutar go list
func (env *TestEnvironment) withModules(dirs ...string) *TestEnvironment {
    env.AssetsHandler.SetListModulesFn(func(rootDir string) ([]string, error) {
        return dirs, nil
    })
    return env
}

// writeModuleSSR crea moduleDir/ssr.go con RenderCSS y/o RenderJS mínimos
func (env *TestEnvironment) writeModuleSSR(name, css, js string) string {
    moduleDir := filepath.Join(env.BaseDir, "modules", name)
    os.MkdirAll(moduleDir, 0755)
    // ... construye contenido ssr.go y lo escribe
    return moduleDir
}

// assertContainsCSS / assertNotContainsCSS — usan ContainsCSS() público
func (env *TestEnvironment) assertContainsCSS(substr string) { ... }
func (env *TestEnvironment) assertNotContainsCSS(substr string) { ... }
func (env *TestEnvironment) assertContainsJS(substr string) { ... }
func (env *TestEnvironment) assertHasIcon(id string) { ... }

// waitLoad espera WaitForSSRLoad con timeout de 2s
func (env *TestEnvironment) waitLoad() { ... }
```

## Orden de implementación

1. **Stage 0** — Añadir métodos públicos de inspección (`inspect.go`) + migrar todos los
   tests existentes a `tests/` + renombrar `test/` → `tests/testdata/` + verificar que
   todos los tests pasan sin cambios de comportamiento
2. Añadir `RootDir` a `Config` + `listModulesFn` + `SetListModulesFn` + `WaitForSSRLoad`
3. `ssr_extract.go` → `tests/ssr_extract_test.go`
4. `ssr_register.go` → `tests/ssr_register_test.go`
5. `ssr_loader.go` → `tests/ssr_loader_test.go` + `tests/concurrency_ssr_test.go`

**Nota:** `events.go` y `SupportedExtensions` NO se modifican. El hot reload de `ssr.go`
lo entrega `GoModHandler.OnSSRFileChange` (ya implementado en `tinywasm/devflow v0.4.16`),
no el sistema de eventos de assetmin.

## Decisiones de diseño incorporadas

| Decisión | Opción elegida | Justificación |
|---|---|---|
| Hot reload entrega de eventos | `GoModHandler` relay callback | devwatch bloquea `.go` en assetmin via `depFinder` |
| Arranque bloqueante | Goroutine + `WaitForSSRLoad` solo en tests | Dev no percibe gap de ~1s; no bloquear servidor |
| Fallo de `go list` | Degradar a replace-locals + warning | No bloquear dev sin red; módulos proxy no cambian |
| Inyección en tests | Campo `listModulesFn` inyectable | Tests rápidos sin red, reproducibles |

## Campo inyectable para tests

```go
type AssetMin struct {
    // ...campos existentes...
    listModulesFn func(rootDir string) ([]string, error) // nil = usa go list real
}

// SetListModulesFn reemplaza la función de descubrimiento de módulos.
// Solo para tests — permite inyectar directorios ficticios sin red.
func (c *AssetMin) SetListModulesFn(fn func(rootDir string) ([]string, error))
```

En tests: `am.SetListModulesFn(func(root string) ([]string, error) { return []string{moduleDir}, nil })`

`UpdateSSRModule` con campos vacíos (css="", js="", etc.) los salta silenciosamente,
igual que hace hoy `InjectCSS`. Un módulo sin ningún asset no genera error.
