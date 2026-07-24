# PLAN_ASSETMIN: Endurecimiento del ensamblado de style.css en tinywasm/assetmin

Fecha: 2026-07-24 (revisado)
Estado: propuesto — repo `tinywasm/assetmin` no accesible desde esta sesión
(ni clonado ni con permisos de push); este documento es la especificación
completa para implementarlo en ese repo sin research adicional.

**Prioridad: OPCIONAL, no bloqueante.** La causa raíz del síntoma reportado
("el CSS no queda en orden hasta reiniciar la app") ya tiene fix definido en
`docs/PLAN.md` de este mismo repo (`tinywasm/ssr`, Paso 1: `ExtractModule`
resuelve al módulo propietario). Ese fix por sí solo cierra el síntoma porque
hace que `ExtractAll` y `ExtractModule` siempre devuelvan la misma
`ModuleName`/`IsRoot` para un mismo directorio — con eso, assetmin ya recibe
clave y slot consistentes y los 4 puntos de este documento quedan **dormidos**
(el código con el defecto sigue ahí, pero nada lo dispara).

Este documento es defensa en profundidad: endurece assetmin para que NO
vuelva a romperse si en el futuro aparece otro `SSRExtractor` (no
necesariamente `tinywasm/ssr`) que sea inconsistente entre sus dos métodos.
Impleméntalo solo si el trabajo asignado es específicamente este
endurecimiento; no es un prerrequisito de ningún otro plan.

**Regla para quien implemente esto**: cada sección de abajo tiene EXACTAMENTE
una solución. No hay que evaluar alternativas ni diseñar nada — el diseño ya
está decidido. Si algo no encaja con el código real de assetmin en el momento
de implementar (la versión pudo avanzar), detente y reconcilia contra el
código actual antes de improvisar una vía distinta a la aquí descrita.

## Modelo mental necesario (para no tener que releer todo assetmin)

- `AssetMin` mantiene 5 "handlers" de salida (`asset` en `asset.go`):
  `mainStyleCssHandler` (style.css), `mainJsHandler`, `spriteSvgHandler`,
  `faviconSvgHandler`, `indexHtmlHandler`.
- Cada `asset` guarda su contenido en TRES slices ordenados, no en un mapa:
  `contentOpen`, `contentMiddle`, `contentClose` (campo `[]*ContentFile`,
  cada uno con `Path` y `Content`). `WriteContent` los concatena en ese orden
  fijo: open → dynamic → middle → close. Por eso el slot importa: dos
  entradas para el "mismo" módulo en slots distintos NO se pisan, coexisten y
  se concatenan las dos.
- `UpdateContentInSlot(filePath, event, f, slot)` (`asset.go`) es la única
  vía de escritura. Con `event="write"` busca `filePath` dentro del slice del
  slot pedido (`findFileIndex`, compara por `f.Path == filePath`); si lo
  encuentra, REEMPLAZA en el mismo índice; si no, hace `append` al final. Con
  `event="remove"` busca y borra por el mismo criterio. Esto es clave para
  todos los fixes de abajo: ya existe una primitiva de reemplazo-en-sitio,
  solo hay que dirigirla al slot correcto.
- `routeAssets(a *SSRAssets, isRoot, isFramework bool)` (`ssr_loader.go`)
  decide el slot: `close` si `isRoot`, si no `middle`. La clave siempre es
  `a.ModuleName`.
- `ScheduleSSRLoad` (arranque, `ExtractAll`, asíncrono en goroutine) y
  `ReloadSSRModule` (hot-reload, `ExtractModule`, disparado por el watcher)
  son las DOS vías que llaman a `routeAssets`/`updateSSRModuleInSlot`. El bug
  de fondo en todo este documento es que ambas vías pueden, en teoría,
  desacordar en qué clave/slot le corresponde al mismo módulo.

## 1. `updateSSRModuleInSlot` no reconcilia contra otros slots — ACCIÓN REQUERIDA (si se implementa este plan)

Archivo: `ssr_register.go`, función `updateSSRModuleInSlot` (la interna, con
receptor `*AssetMin`, la que recibe `slot string` y hace el trabajo real).

**Síntoma si esto no se corrige**: si dos llamadas para la MISMA `name`
llegan con `slot` distinto (p. ej. la primera con `isRoot=true` → `close`, la
segunda con `isRoot=false` → `middle`), la segunda no encuentra la entrada de
la primera (slots distintos) y hace `append` en vez de reemplazar. Quedan DOS
entradas para el mismo módulo, en slots distintos, y ambas se concatenan en
`style.css` — la vieja y la nueva. Como `middle` se escribe antes que
`close`, si la entrada vieja terminó en `close` y la nueva en `middle`, la
vieja queda DESPUÉS en el CSS final y gana la cascada.

**Fix (único, exacto)**: al inicio de `updateSSRModuleInSlot`, antes de
escribir en el slot pedido, retirar cualquier entrada previa de `name` de los
OTROS DOS slots, en los tres handlers de texto (css, js, html). Añadir esta
función nueva en `ssr_register.go` y llamarla desde el principio de
`updateSSRModuleInSlot`:

```go
// enforceSingleSlot retira cualquier entrada previa de `name` de los slots
// DISTINTOS a `slot`, en los tres handlers de texto. Un módulo vive en
// exactamente un slot a la vez — sin esto, dos llamadas para el mismo name
// con slot distinto (p. ej. ExtractAll con IsRoot=true y luego un reload con
// IsRoot=false para el mismo módulo) apilan un duplicado en vez de
// reemplazar, y el duplicado viejo puede ganar la cascada CSS.
func (c *AssetMin) enforceSingleSlot(name, slot string) {
	for _, other := range [3]string{"open", "middle", "close"} {
		if other == slot {
			continue
		}
		c.mainStyleCssHandler.UpdateContentInSlot(name, "remove", nil, other)
		c.mainJsHandler.UpdateContentInSlot(name, "remove", nil, other)
		c.indexHtmlHandler.UpdateContentInSlot(name, "remove", nil, other)
	}
}
```

Y en `updateSSRModuleInSlot`, como primera línea del cuerpo (antes de `if css
!= ""`):

```go
func (c *AssetMin) updateSSRModuleInSlot(name string, css string, scripts []*js.Script, html string, icons *sprite.Sprite, slot string) error {
	c.enforceSingleSlot(name, slot) // NUEVO — primera línea

	if css != "" {
		c.mainStyleCssHandler.UpdateContentInSlot(name, "write", &ContentFile{Path: name, Content: []byte(css)}, slot)
	}
	// ... resto sin cambios
```

**No hacer**: no crear un índice/mapa global `map[string]slot` paralelo a los
tres slices, y no fusionar `contentOpen/Middle/Close` en una sola estructura.
`enforceSingleSlot` reutiliza la primitiva `UpdateContentInSlot(..., "remove",
...)` que ya existe y ya es la fuente de verdad — cualquier estructura nueva
sería una segunda fuente de verdad que puede desincronizarse de los slices
reales.

Test de aceptación (portar tal cual desde el repo ssr, adaptando el import):
`tests/consumer_hot_reload_test.go` → `TestConsumerHotReload_SubpackageEdit`
del repo `tinywasm/ssr` es la reproducción end-to-end. Para un test unitario
propio de assetmin, sin depender de ssr, añadir en assetmin:

```go
// ssr_register_slot_test.go
package assetmin

import "testing"

// Reproduce un SSRExtractor inconsistente: la misma clave de módulo llega
// primero en slot "close" (como isRoot=true) y luego en slot "middle" (como
// isRoot=false para el mismo módulo). Tras la segunda llamada debe quedar
// UNA sola entrada, con el contenido nuevo, y ninguna en el slot viejo.
func TestUpdateSSRModuleInSlot_ReplacesAcrossSlots(t *testing.T) {
	c := NewAssetMin(&Config{OutputDir: t.TempDir()})

	if err := c.updateSSRModuleInSlot("mod", ".mod{color:blue}", nil, "", nil, "close"); err != nil {
		t.Fatal(err)
	}
	if err := c.updateSSRModuleInSlot("mod", ".mod{color:red}", nil, "", nil, "middle"); err != nil {
		t.Fatal(err)
	}

	css, err := c.GetMinifiedCSS()
	if err != nil {
		t.Fatal(err)
	}
	got := string(css)

	if want := ".mod{color:red}"; !contains(got, want) {
		t.Errorf("missing new rule %q in %q", want, got)
	}
	if bad := ".mod{color:blue}"; contains(got, bad) {
		t.Errorf("stale rule %q still present in %q — old slot was not cleared", bad, got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
```

(Si assetmin ya importa `strings` en algún test del paquete, usa
`strings.Contains` en vez del `contains` casero de arriba y borra el helper —
está escrito standalone solo para no asumir imports del archivo real.)

## 2. Orden dentro de un slot depende del orden de llegada — ACCIÓN REQUERIDA (si se implementa este plan)

Archivo: `asset.go`, función `UpdateContentInSlot`, rama `case "create",
"write", "modify":`, subrama `if !replaced { *filesToUpdate = append(...) }`.

**Síntoma si esto no se corrige**: cuando una entrada es NUEVA (no hay
`Path` previo en el slot), hoy se hace `append` al final — su posición final
en `style.css` es su orden de LLEGADA, no su `Path`. `ScheduleSSRLoad` corre
en goroutine (arranque asíncrono) en paralelo con el watcher de archivos; si
un evento de reload de un módulo nuevo llega mientras el escaneo inicial
sigue en curso, ese módulo puede terminar insertado antes o después según el
timing exacto de esa ejecución — dos arranques de la misma app pueden
producir `style.css` con orden distinto entre módulos NUEVOS que aparecen
durante esa ventana de carrera. (Los módulos ya existentes no se ven
afectados por esto: para ellos `findFileIndex` encuentra el `Path` y
reemplaza en su posición actual, sin moverlos — ver la sección 1, que cubre
justamente el caso de reemplazo. Esta sección 2 es solo sobre altas nuevas.)

**Fix (único, exacto)**: cuando no hay reemplazo, insertar en la posición
ordenada por `Path` en vez de `append` al final. Reemplazar el bloque:

```go
if !replaced {
    // No match found: append as new file
    *filesToUpdate = append(*filesToUpdate, f)
}
```

por:

```go
if !replaced {
    // No match found: insert at the Path-sorted position so a slot's order
    // depends only on the set of modules present, never on the order in
    // which their registration events arrived (ScheduleSSRLoad's background
    // scan and a watcher-driven reload can race on process boot).
    idx := sort.Search(len(*filesToUpdate), func(i int) bool {
        return (*filesToUpdate)[i].Path >= filePath
    })
    *filesToUpdate = append(*filesToUpdate, nil)
    copy((*filesToUpdate)[idx+1:], (*filesToUpdate)[idx:])
    (*filesToUpdate)[idx] = f
}
```

Y añadir `"sort"` al bloque de imports de `asset.go` (hoy tiene `bytes`,
`os`, `path/filepath`, `slices`, `sync`).

**No hacer**: no reordenar el slice entero en cada mutación (`sort.Slice`
sobre todo `*filesToUpdate` tras cada `write`) — es innecesario (el slice ya
está ordenado antes de la operación; solo la nueva entrada necesita
posicionarse) y es más caro en el caso común de reemplazo, que no debe tocar
el orden en absoluto.

Test de aceptación: portar `TestConsumerStartup_ReloadRacingInitialLoad` del
repo ssr, o el equivalente unitario en assetmin:

```go
func TestUpdateContentInSlot_NewEntriesInsertSortedByPath(t *testing.T) {
	h := newAssetFile("style.css", "text/css", &Config{OutputDir: t.TempDir()}, nil)

	// Arrival order deliberately NOT sorted: zeta, then alpha, then beta.
	h.UpdateContentInSlot("zeta", "write", &ContentFile{Path: "zeta", Content: []byte(".z{}")}, "middle")
	h.UpdateContentInSlot("alpha", "write", &ContentFile{Path: "alpha", Content: []byte(".a{}")}, "middle")
	h.UpdateContentInSlot("beta", "write", &ContentFile{Path: "beta", Content: []byte(".b{}")}, "middle")

	var got []string
	for _, f := range h.contentMiddle {
		got = append(got, f.Path)
	}
	want := []string{"alpha", "beta", "zeta"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d: got %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}
```

## 3. Fallo permanente de `ExtractAll` deja la app sin estilos en silencio — ACCIÓN REQUERIDA (si se implementa este plan)

Archivos: `assetmin.go` (struct `AssetMin`) y `ssr_loader.go`
(`ScheduleSSRLoad`, `ReloadSSRModule`).

**Síntoma si esto no se corrige**: `ScheduleSSRLoad` reintenta `ExtractAll`
5 veces con backoff (200ms → 400ms → 800ms → 1.6s → 3s). Si las 5 fallan
(típico: el código del usuario no compila justo en el instante del arranque,
por ejemplo mid-edit en el propio editor), se loguea `"FATAL: SSR ExtractAll
failed permanently..."` y la goroutine termina — nadie vuelve a llamar
`ExtractAll` nunca más para esa instancia de `AssetMin`. La app queda sin CSS
de forma permanente aunque el código se arregle 2 segundos después; la única
recuperación es reiniciar el proceso. Es otro camino hacia el mismo síntoma
raíz reportado ("tengo que detener y reiniciar la app").

**Fix (único, exacto)**:

1. Añadir un campo a `AssetMin` (`assetmin.go`, junto a los demás campos
   `bool`/mapas del struct):

   ```go
   initialLoadFailed bool // true tras agotar los reintentos de ExtractAll; el próximo evento SSR debe reintentar el escaneo completo
   ```

2. En `ScheduleSSRLoad` (`ssr_loader.go`), donde hoy solo loguea el fallo
   final, marcar el flag bajo `c.mu`:

   ```go
   if err != nil {
       c.Logger("FATAL: SSR ExtractAll failed permanently after", attempts, "attempts. Error:", err)
       c.mu.Lock()
       c.initialLoadFailed = true
       c.mu.Unlock()
   }
   ```

   (Esta línea va exactamente donde ya está el `c.Logger("FATAL: ...")`
   existente — solo se añaden las 3 líneas del lock/set/unlock a continuación.)

3. En `ReloadSSRModule` (`ssr_loader.go`), como primeras líneas del cuerpo
   (antes de `if c.ssrExtractor == nil`), consumir el flag y disparar un
   reintento completo:

   ```go
   func (c *AssetMin) ReloadSSRModule(moduleDir string) error {
       c.mu.Lock()
       retryFullScan := c.initialLoadFailed
       if retryFullScan {
           c.initialLoadFailed = false
       }
       c.mu.Unlock()
       if retryFullScan {
           c.ScheduleSSRLoad() // el arranque falló permanentemente; el código cambió (por eso llegó este evento) — reintentar todo
       }

       if c.ssrExtractor == nil {
           return nil
       }
       // ... resto de la función sin cambios
   ```

**No hacer**: no reintentar indefinidamente dentro de `ScheduleSSRLoad`
mismo (un `for { }` sin salida) — eso bloquearía la goroutine para siempre
en un proyecto roto y ocultaría el error en vez de propagarlo al próximo
evento real del watcher. El fix de arriba es correcto porque ata el
reintento a una señal real de "algo cambió" (un evento de archivo, que es
justo lo que dispara `ReloadSSRModule`).

Test de aceptación:

```go
func TestReloadSSRModule_RetriesFullScanAfterPermanentExtractAllFailure(t *testing.T) {
	c := NewAssetMin(&Config{OutputDir: t.TempDir(), RootDir: t.TempDir()})

	extractAllCalls := 0
	c.SetSSRExtractor(&fakeExtractor{
		extractAll: func() ([]*SSRAssets, error) {
			extractAllCalls++
			return nil, fmt.Errorf("boom")
		},
		extractModule: func(dir string) (*SSRAssets, error) { return nil, nil },
	})

	c.initialLoadFailed = true // simula que ScheduleSSRLoad ya agotó sus 5 reintentos

	_ = c.ReloadSSRModule(t.TempDir())

	if extractAllCalls == 0 {
		t.Error("ReloadSSRModule after a permanent ExtractAll failure must retry the full scan, not just the single module")
	}
	if c.initialLoadFailed {
		t.Error("initialLoadFailed must be cleared once a retry has been scheduled")
	}
}
```

(`fakeExtractor` debe implementar la interfaz `SSRExtractor` de
`ssr_extractor.go`; si assetmin no tiene ya un fake de este tipo en sus
tests, créalo minimal, solo con los dos métodos de la interfaz.)

## 4. Menor: `FlushToDisk` itera `allAssets` (mapa) — ACCIÓN REQUERIDA (trivial, si se implementa este plan)

Archivo: `ssr.go`, función `FlushToDisk`.

Esto NO afecta el contenido de ningún archivo, solo el ORDEN en que se
escriben archivos DISTINTOS a disco (style.css vs script.js vs index.html,
etc.) — inocuo para el bug de CSS investigado. Se incluye aquí solo por
completitud/higiene: logs y trazas de escritura reproducibles.

**Fix (único, exacto)**: ordenar por `outputPath` antes de escribir. En
`FlushToDisk`, tras construir `snapshots` desde el `range c.allAssets`,
añadir un `sort.Slice` antes del segundo bucle (`for _, s := range
snapshots`):

```go
sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].path < snapshots[j].path })
```

Añadir `"sort"` a los imports de `ssr.go` (hoy tiene `bytes`, `fmt`).

## Explícitamente fuera de este plan

`RegisterComponents` (clave `fmt.Sprintf("%T", provider)`, distinta del
import-path que usa el extractor) tiene el mismo tipo de riesgo de colisión
de claves que la sección 1, PERO: `tinywasm/core`, el único consumidor real
inspeccionado durante esta investigación, no llama a `RegisterComponents` en
ningún punto de su código (verificado por búsqueda literal en el repo) — solo
usa `SetSSRExtractor` + `ReloadSSRModule`/`LoadSSRModules`. No hay evidencia
de que este camino esté en uso. No lo toques como parte de este plan; si en
el futuro aparece un consumidor real de `RegisterComponents` que además use
el extractor SSR para el mismo componente, ese es el momento de revisar esto
con ese caso real delante, no antes.

## Orden de implementación y verificación final

Implementar 1 → 2 → 3 → 4 en ese orden (cada uno es independiente de los
demás, pero este orden va de mayor a menor impacto). Tras cada sección,
correr `go test ./...` en `assetmin` completo antes de pasar a la siguiente.

Al terminar las 4, correr la suite completa de assetmin y confirmar que
ningún test existente se rompió (los cuatro fixes son aditivos: nueva
función `enforceSingleSlot`, una inserción ordenada en vez de un `append`
que ya no tenía garantía de orden, un flag nuevo que empieza en `false`, y un
`sort.Slice` sobre una operación que no garantizaba orden antes). Si algún
test existente asume el orden de llegada como orden final de `style.css`,
ese test codificaba el bug de la sección 2 como comportamiento esperado —
actualízalo para afirmar orden por `Path`, no orden de llegada.
