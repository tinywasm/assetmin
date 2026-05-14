# PLAN — Correcciones regresión post-agente

> El agente anterior migró las interfaces de `ssr_register.go` a `*css.Stylesheet`
> correctamente, pero introdujo dos regresiones en los tests. `gotest` reporta
> fallos en 5 archivos de tests.

---

## Diagnóstico

### Problema A — `css.New` no existe en `tinywasm/css v0.1.0`

La API de `tinywasm/css` cambió: `css.New(...)` → `css.NewStylesheet(...)`.
El agente usó la firma antigua en los mocks de runtime.

**Archivos afectados (compile error `undefined: css.New`):**

| Archivo | Líneas |
|---|---|
| `tests/ssr_register_test.go` | 13 |
| `tests/ssr_register_root_test.go` | 12, 15, 16, 33, 36 |
| `tests/ssr_loader_root_override_test.go` | 15 |

**Corrección:** reemplazar `css.New(` → `css.NewStylesheet(` en todos los casos.
El resto de la lógica es correcta: estos tests usan `*css.Stylesheet` para
`RegisterComponents` (runtime), lo cual es el contrato correcto.

---

### Problema B — Fixtures del extractor importan `tinywasm/css` en directorios temporales

El agente modificó `createSSRTestModule` (y los cuerpos de los fixtures en
`ssr_extract_root_test.go`) para que el `ssr.go` generado en temp dirs importe
`github.com/tinywasm/css` y retorne `*css.Stylesheet`. Eso rompe el `go run`
del extractor porque:

1. Los temp dirs no tienen `go.sum`.
2. La versión en el `require` generado es `v0.0.4`, no `v0.1.0`.
3. El extractor compila el temp module en un entorno aislado sin acceso a la
   caché de módulos correcta.

**Causa raíz conceptual:** los tests del extractor SSR no necesitan retornar
`*css.Stylesheet`. El extractor genera `inst.RenderCSS().String()` — cualquier
tipo con método `String() string` funciona. La migración a `*css.Stylesheet` es
correcta solo para `RegisterComponents` (runtime), no para las fixtures de
extracción (compile-time aislado).

**Archivos afectados:**

| Archivo | Problema |
|---|---|
| `tests/ssr_extract_test.go` | `createSSRTestModule` agrega `require github.com/tinywasm/css v0.0.4` al go.mod del fixture y usa `css.New` en el body |
| `tests/ssr_extract_root_test.go` | Bodies de fixture usan `css.New(css.Raw(...))` y retornan `*css.Stylesheet` |

---

## Correcciones

### Corrección A — `css.New` → `css.NewStylesheet`

En los 3 archivos de runtime tests, reemplazar globalmente:

```go
// antes
css.New(css.Raw(":root{--a:1;}"))

// después
css.NewStylesheet(css.Raw(":root{--a:1;}"))
```

Archivos: `tests/ssr_register_test.go`, `tests/ssr_register_root_test.go`,
`tests/ssr_loader_root_override_test.go`.

---

### Corrección B — Revertir fixtures del extractor a tipos auto-contenidos

#### `tests/ssr_extract_test.go` — función `createSSRTestModule`

El `go.mod` generado NO debe incluir `require github.com/tinywasm/css`. El
`ssr.go` generado NO debe importar `github.com/tinywasm/css`. Usar un tipo local
con `String()`:

```go
func createSSRTestModule(t *testing.T, parentDir, modulePath, pkgName, body string) string {
    // ... crear dirs igual que antes ...

    // go.mod sin dependencias externas
    gomod := fmt.Sprintf("module %s\n\ngo 1.22\n", modulePath)
    // ... escribir go.mod igual que antes ...

    // ssr.go auto-contenido: sin import de tinywasm/css
    structName := ...
    ssrGo := fmt.Sprintf(`//go:build !wasm

package %s

// stylesheet es un tipo local con String() para satisfacer el extractor.
type stylesheet string
func (s stylesheet) String() string { return string(s) }

%s

func SSRInstance() *%s {
    return &%s{}
}
`, pkgName, body, structName, structName)
    // ...
}
```

Los bodies de los fixtures en `TestExtractSSRAssets` deben usar `stylesheet`
en vez de `*css.Stylesheet`:

```go
// antes (agente)
func (c *Css) RenderCSS() *css.Stylesheet {
    return css.New(css.Raw(".cls{color:red;}"))
}

// después (corregido)
func (c *Css) RenderCSS() stylesheet {
    return stylesheet(".cls{color:red;}")
}
```

#### `tests/ssr_extract_root_test.go`

Mismo patrón. Los bodies de los fixtures (Theme, Noroot, Combined, Embed) deben
retornar `stylesheet` local en vez de `*css.Stylesheet`. Eliminar `import "github.com/tinywasm/css"` de todos los bodies de fixtures.

Ejemplos concretos a corregir:

```go
// TestExtract_RootCSS_FromLiteral — body del fixture
// antes:
func (t *Theme) RootCSS() *css.Stylesheet {
    return css.New(css.Raw(":root{--x:1;}"))
}
// después:
func (t *Theme) RootCSS() stylesheet {
    return stylesheet(":root{--x:1;}")
}

// TestExtract_RootCSS_Missing — Noroot
// antes:
func (n *Noroot) RenderCSS() *css.Stylesheet { return css.New(css.Raw(".component { color: blue; }")) }
// después:
func (n *Noroot) RenderCSS() stylesheet { return stylesheet(".component { color: blue; }") }

// TestExtract_BothRootAndRender — Combined
// TestExtract_RootCSS_FromEmbed — Embed
// mismo patrón
```

El tipo `stylesheet` se define UNA vez dentro de `createSSRTestModule` como
parte del `ssrGo` generado (junto con `SSRInstance`), así está disponible en
todos los fixtures que usen `createSSRTestModule`. Para `ssr_extract_root_test.go`
que NO usa `createSSRTestModule` sino que escribe directamente: el body debe
incluir la definición de `stylesheet` o usar `string` directamente como tipo de
retorno local.

**Opción alternativa más simple para ssr_extract_root_test.go:** retornar
`string` directamente, ya que el extractor genera `.String()` sobre el retorno.
Pero Go no tiene `String()` en `string` built-in. Usar el tipo local `stylesheet`
o simplemente retornar un tipo que implemente la interfaz detectada por el
generador — verificar qué regex usa `generateExtractorMain` para detectar `RootCSS`.

> **Nota**: verificar en `ssr_invoke.go:GenerateExtractorMain` qué regex usa
> para detectar `RootCSS()` y `RenderCSS()`. Si solo busca la firma de función
> por nombre (no por tipo de retorno), cualquier tipo de retorno con `.String()`
> funciona. Si filtra por `\*Stylesheet`, hay que ajustar el regex también.

---

## Prerrequisito — instalar gotest

`gotest` es el runner preferido del proyecto: muestra colores, cobertura y
warnings de tests lentos. Instalarlo antes de ejecutar cualquier paso:

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
```

Usar siempre `gotest` (sin argumentos) en lugar de `go test ./...`. Maneja
automáticamente `-vet`, `-race`, `-cover` y badges.

---

## Secuencia de ejecución

```
1. Corrección A: reemplazar css.New → css.NewStylesheet en 3 archivos de runtime
   (ssr_register_test.go, ssr_register_root_test.go, ssr_loader_root_override_test.go)

2. Verificar regex de GenerateExtractorMain para saber si filtra por tipo de retorno
   o solo por nombre de función

3. Corrección B: revertir createSSRTestModule en ssr_extract_test.go a tipo local

4. Corrección B: revertir bodies de fixtures en ssr_extract_root_test.go a tipo local

5. gotest — verificar 0 fallos (requiere gotest instalado, ver Prerrequisito)

6. Si quedan fallos, diagnosticar con stderr visible:
   en ssr_invoke.go:56 temporalmente cambiar cmd.Output() por cmd.CombinedOutput()
   y loggear el output completo en el error
```

---

## Criterios de aceptación

- `gotest` retorna 0 fallos.
- `tests/ssr_register_test.go`, `tests/ssr_register_root_test.go`,
  `tests/ssr_loader_root_override_test.go` usan `css.NewStylesheet` y compilan.
- Los fixtures de `ssr_extract_test.go` y `ssr_extract_root_test.go` son
  auto-contenidos: sin `require github.com/tinywasm/css` en sus go.mod temporales.
- `ssr_register.go` mantiene las interfaces con `*css.Stylesheet` (correcto,
  no revertir).
