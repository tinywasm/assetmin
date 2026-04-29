# PLAN: Cobertura de tests — IconSvg con métodos receiver

## Contexto

Al desarrollar el componente `selectsearch` con `tinywasm/app`, los iconos SVG definidos
en `ssr.go` usando métodos con receiver (`func (c *SelectSearch) IconSvg()`) no aparecían
en el HTML generado.

## Investigación

### El flujo de inyección de iconos es correcto

```
ExtractSSRAssets(moduleDir)           // parsea ssr.go vía AST → extrae Icons map
    ↓
updateSSRModuleInSlot(...)            // itera icons, llama addIcon por cada uno
    ↓
addIcon(id, svgContent)               // construye <symbol>, llama spriteSvgHandler.AddContentMiddle
    ↓
indexHtmlHandler.AddDynamicContent    // en cada request HTML: spriteSvgHandler.GetMinifiedContent()
                                      // inyecta sprite inline en el HTML
```

### El bug era en los tests, no en el código

`ExtractSSRAssets` usa `ast.Inspect` para encontrar `FuncDecl` con `fn.Name.Name == "IconSvg"`.
Este mecanismo funciona **idénticamente** para funciones libres y métodos con receiver — el
receiver está en `fn.Recv`, pero el nombre sigue siendo accesible vía `fn.Name.Name`.

Los tests existentes solo cubrían funciones libres (`func IconSvg()`). Los componentes reales
(como `selectsearch`) usan métodos con receiver (`func (c *SelectSearch) IconSvg()`).
La brecha de cobertura ocultó que el código ya funcionaba correctamente.

### Causa raíz del síntoma reportado

`tinywasm/app` llama `ReloadSSRModule(h.RootDir)` + `LoadSSRModules()` + `WaitForSSRLoad(5s)`
antes de que el servidor acepte requests (el servidor arranca después en `StartBackgroundServices`).
Por lo tanto el sprite ya tiene los iconos cuando el browser hace el primer request. El problema
observable era que no había test que verificara el caso de uso completo con receiver methods.

## Tests añadidos

### `tests/ssr_extract_test.go`

| Test | Qué verifica |
|------|-------------|
| `ExtractIconSvg_ReceiverMethod` | `ExtractSSRAssets` extrae icons de `func (c *T) IconSvg()` |
| `ExtractCSS_ReceiverMethod` | `ExtractSSRAssets` extrae CSS de `func (c *T) RenderCSS()` |

### `tests/ssr_loader_test.go`

| Test | Qué verifica |
|------|-------------|
| `LoadIconsFromReceiverMethod_InHTML` | Flujo completo: receiver method → `LoadSSRModules` → `HasIcon` → sprite en HTML inline |

## Invariantes verificados por los tests

1. `ExtractSSRAssets` extrae icons de métodos con receiver igual que de funciones libres.
2. `LoadSSRModules` + `WaitForSSRLoad` registran los icons en el sprite antes de retornar.
3. `RegenerateHTMLCache` + `GetCachedHTML` incluyen el sprite con los symbols inline.

## Nota sobre strings en tests

Al escribir contenido Go con backticks dentro de raw strings en Go, usar **concatenación de
strings con escaping explícito** en lugar de raw string literals para evitar contenido malformado:

```go
// ✗ Propenso a errores (backtick dentro de raw string)
content := `func IconSvg() { return map[...]{ "k": ` + "`val`" + `, } }`

// ✓ Correcto
content := "func IconSvg() { return map[...]{\n\t\"k\": \"val\",\n} }\n"
```
