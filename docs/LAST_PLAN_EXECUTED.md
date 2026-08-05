---
PLAN: "fix: viewport-fit=cover en el index.html generado"
TAG: v0.5.1
---

## Antes de escribir código: lee [CONSTRUCTION_HARNESS.md](CONSTRUCTION_HARNESS.md)

**Es vinculante, no orientativo.**

| # | Principio | Cómo se aplica aquí |
|---|---|---|
| 8 | Closed by default | El HTML correcto es el que sale **sin configurar nada**. No se añade una opción para activar el viewport |
| 9 | Lego pieces, never forks | El shell del documento es de `tinywasm/html`. Este plan corrige la copia que este repo mantiene y **deja anotada la deuda**, no la amplía |
| 4 | One way to do each thing | Un solo valor de viewport en el ecosistema. Si aquí dice una cosa y en `html` otra, hay dos |

---

## 1. El hueco

`assetmin` genera el `<head>` del `index.html` en `html.go:44`:

```go
// html.go:41-48
Content: []byte(`<!doctype html>
<html>
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	...
```

La etiqueta está, y `width=device-width, initial-scale=1` es correcto hasta
donde llega. **Le falta `viewport-fit=cover`.**

Lo mismo en `templates/index_basic.html:5`.

### 1.1 Qué provoca exactamente

Sin `viewport-fit=cover`, el documento se dibuja dentro del rectángulo "seguro"
de la pantalla y **`env(safe-area-inset-top/right/bottom/left)` vale `0px` en
todos los dispositivos**, incluido un iPhone con notch.

La consecuencia es la peor clase de fallo: el CSS que usa esas variables
*compila, se sirve y no hace nada*. Un header con
`padding-top: env(safe-area-inset-top)` recibe 0 y queda tapado por la Dynamic
Island; un botón inferior con `padding-bottom: env(safe-area-inset-bottom)` queda
bajo la barra de gestos. Nadie ve un error: se ve una app mal alineada, y la
sospecha cae sobre el CSS, que está bien.

Es decir: esta línea es la **precondición** de todo el trabajo de safe areas que
`tinywasm/css` va a declarar como tokens. Sin ella, aquel trabajo no es
verificable.

### 1.2 La deuda de principio 9, anotada y no ampliada

Este repo **no debería** estar escribiendo HTML a mano. `tinywasm/html` ya expone
el shell, y su propio comentario nombra a este consumidor:

```go
// html/document.go:48-49
// DocumentString renders the full HTML document including <!DOCTYPE html>.
// This is what assetmin writes to disk as index.html.
```

Hoy no es cierto: `html.go` mantiene su propia copia. `tinywasm/html` ya está en
`go.mod:10`, así que la dependencia ni siquiera habría que añadirla.

Reconciliar las dos implementaciones es la corrección de fondo, pero es un cambio
mayor —afecta a `contentOpen`/`contentClose`, a `parseExistingHtmlContent` y a los
tests de bundle— y no debe colarse dentro de un arreglo de una línea. **Queda
fuera de alcance y anotado en `docs/ARCHITECTURE.md`** para que sea deuda visible
y no un descubrimiento futuro.

Mientras tanto rige la regla mínima: **si el valor cambia en `tinywasm/html`,
cambia aquí en el mismo release.** Dos copias sin esa regla son dos verdades.

---

## 2. Cambio

Dos literales, mismo valor:

```
width=device-width, initial-scale=1, viewport-fit=cover
```

| Archivo | Línea | Nota |
|---|---|---|
| `html.go` | 44 | el shell del bundle |
| `templates/index_basic.html` | 5 | normalizar también `initial-scale=1.0` → `initial-scale=1`, para que ambos literales sean idénticos y un `grep` los encuentre juntos |

**No** se añade `user-scalable=no` ni `maximum-scale=1`: bloquean el zoom del
usuario (fallo de accesibilidad, WCAG 1.4.4) y Safari los ignora desde iOS 10.
Un valor que el navegador ignora es ruido que aparenta control.

**No** se añade opción de configuración. Por principio 8, el HTML correcto es el
que sale escribiendo nada; una opción convierte "la app se ve bien en el móvil"
en algo que hay que acordarse de activar, con fallo silencioso si se olvida.

---

## 3. Verificación

1. Test que afirme que el `index.html` generado por `NewHtmlHandler` contiene
   `viewport-fit=cover`. Es la guarda que impide que una futura reescritura del
   shell lo pierda sin que nadie lo note.
2. Test que afirme que el literal de `html.go` y el de
   `templates/index_basic.html` son **la misma cadena**. Es la regla de §1.2
   convertida en test: mientras haya dos copias, que no puedan divergir en
   silencio.
3. Los tests de bundle existentes deben pasar sin modificarse: el cambio es
   aditivo dentro de un atributo, no toca la estructura del documento.

```bash
go install github.com/tinywasm/devflow/cmd/gotest@latest
gotest
```

---

## 4. Alcance

### Dentro

- `html.go:44`, `templates/index_basic.html:5`.
- Los tests de §3.
- Una nota de deuda en `docs/ARCHITECTURE.md` (§1.2).

### Fuera

- Migrar el shell a `html.DocumentString` (§1.2). Deuda anotada, no ejecutada.
- El shell de `tinywasm/html`, que hoy **no emite ninguna** etiqueta viewport:
  es de aquel repo y tiene su propio plan.
- Los tokens de safe area: son de `tinywasm/css`. Este plan sólo crea la
  condición para que funcionen.

### Orden respecto a los planes hermanos

Este plan y el de `tinywasm/html` son **prerrequisitos** del de `tinywasm/css`.
Se pueden ejecutar en paralelo entre sí, pero el de `css` no se puede *verificar
en un dispositivo* antes que estos dos.
