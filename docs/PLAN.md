# PLAN — Eliminar `SSRInstance()` del contrato de módulos SSR

## Objetivo

Reducir la barrera de adopción de `tinywasm/app`: un módulo SSR sólo necesita
declarar `RenderCSS()`/`RenderHTML()`/`RenderJS()`/`IconSvg()` como métodos sobre
su tipo. El extractor debe construir el receiver por sí mismo, sin requerir
`func SSRInstance() *T { return &T{} }` en cada módulo (hoy ~10 ocurrencias
idénticas en el ecosistema).

## Justificación

- `SSRInstance` es boilerplate: cuerpo siempre `return &T{}`, sin lógica.
- El contrato real lo lleva la firma del método (`func (c *T) RenderCSS() *Stylesheet`).
- El extractor ya parsea `ssr.go` con regex; capturar el tipo receiver es trivial.
- Cumple `core-principles` y `simplify`: menos símbolos públicos, menos código.

## Cambios en assetmin

### 1. Detección del tipo receiver

**Archivo:** [ssr_invoke.go:143-148](../ssr_invoke.go#L143-L148)

Reemplazar regex actuales por versiones que capturen el nombre del tipo:

```go
reRenderCSS  = regexp.MustCompile(`(?m)^func \(\w+ \*?(\w+)\) RenderCSS\(\)`)
reRenderHTML = regexp.MustCompile(`(?m)^func \(\w+ \*?(\w+)\) RenderHTML\(\)`)
reRenderJS   = regexp.MustCompile(`(?m)^func \(\w+ \*?(\w+)\) RenderJS\(\)`)
reIconSvg    = regexp.MustCompile(`(?m)^func \(\w+ \*?(\w+)\) IconSvg\(\)`)
reRootCSS    = regexp.MustCompile(`(?m)^func \(\w+ \*?(\w+)\) RootCSS\(\)`)
```

Eliminar `reSSRInstance`.

### 2. `ModuleAlias`

**Archivo:** [ssr_invoke.go:22-37](../ssr_invoke.go#L22-L37)

- Eliminar campo `HasInstance`.
- Añadir campo `ReceiverType string` (el tipo capturado, consistente entre los
  distintos métodos del módulo — verificar en tests).
- `HasAnyFeature()` deja de depender de `HasInstance`.

### 3. Template del extractor

**Archivo:** [ssr_invoke.go:78-126](../ssr_invoke.go#L78-L126)

Eliminar la rama `{{if .HasInstance}}...{{else}}...{{end}}`. Reemplazar por
una sola forma:

```go
{{if .ReceiverType}}
inst := &{{.Alias}}.{{.ReceiverType}}{}
{{if .HasRoot}}s.Root = inst.RootCSS().String(){{end}}
{{if .HasRender}}s.Render = inst.RenderCSS().String(){{end}}
{{if .HasHTML}}s.HTML = inst.RenderHTML(){{end}}
{{if .HasJS}}s.JS = inst.RenderJS(){{end}}
{{if .HasIcons}}s.Icons = inst.IconSvg(){{end}}
{{else}}
// Fallback: funciones de paquete (sin receiver)
{{if .HasRender}}s.Render = {{.Alias}}.RenderCSS().String(){{end}}
...
{{end}}
```

### 4. `ModulesToAliases`

**Archivo:** [ssr_invoke.go:151-184](../ssr_invoke.go#L151-L184)

Cuando una de las regex casa, almacenar el grupo capturado en
`ma.ReceiverType`. Validar que todos los métodos del módulo coincidan en el
mismo tipo; si difieren, devolver error (caso ilegal hoy, queda explícito).

## Tests

| Test | Acción |
|---|---|
| `ssr_extract_subpackage_test.go` | Quitar `SSRInstance` del fixture, verificar extracción sigue funcionando |
| `tests/ssr_integration_test.go` | Igual: fixtures sin `SSRInstance` |
| `tests/css_ssr_hotreload_test.go` | Re-ejecutar; no debería tocarse |
| `testdata/integration_workspace/button/ssr.go` | Borrar `SSRInstance` |
| `tests/ssr_extract_root_test.go` | Verificar caso "ssr.go en root" sigue válido |

Añadir test nuevo: `TestExtract_NoSSRInstanceFunction` — confirma que un
módulo sin `SSRInstance` se extrae correctamente sólo con métodos receiver.

## Documentación a actualizar

- [`docs/SSR.md`](SSR.md) — eliminar mención de `SSRInstance`, documentar la
  detección automática del receiver.
- [`docs/ARCHITECTURE.md`](ARCHITECTURE.md) — actualizar diagrama del extractor.
- [`docs/COMPONENT_REGISTRATION.md`](COMPONENT_REGISTRATION.md) — el contrato
  mínimo pasa a ser sólo los métodos `Render*`.
- [`docs/QUICK_REFERENCE.md`](QUICK_REFERENCE.md) — quitar `SSRInstance` del
  snippet de "módulo mínimo".

## Stages

| # | Tarea | Done |
|---|---|---|
| 1 | Refactor regex + captura de `ReceiverType` | [ ] |
| 2 | Refactor template + eliminar rama `HasInstance` | [ ] |
| 3 | Actualizar tests SSR (fixtures sin `SSRInstance`) | [ ] |
| 4 | Test nuevo `TestExtract_NoSSRInstanceFunction` | [ ] |
| 5 | `go test ./...` en assetmin verde | [ ] |
| 6 | Actualizar 4 docs en `assetmin/docs/` | [ ] |
| 7 | Coordinar con PLAN.md de `components`, `layout`, `goflare-demo` | [ ] |

## Coordinación

Este cambio es **breaking** para módulos que dependen de assetmin. La eliminación
en consumidores se ejecuta en paralelo en los PLAN.md de:

- `tinywasm/components/docs/PLAN.md`
- `tinywasm/layout/docs/PLAN.md`
- `tinywasm/goflare-demo/docs/PLAN.md`

Orden de merge sugerido:
1. assetmin acepta AMBAS formas (con y sin `SSRInstance`) → merge.
2. Consumidores eliminan `SSRInstance` → merge.
3. assetmin elimina la rama de compatibilidad → merge final.
