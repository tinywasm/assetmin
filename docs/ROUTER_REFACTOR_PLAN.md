# Plan — Refactor de `assetmin` al contrato de enrutado `tinywasm/router`

> `assetmin` sirve los assets minificados (index, css, js, favicon…). Hoy registra
> sus rutas con `*http.ServeMux` y handlers `http.HandlerFunc`. El refactor lo pasa al
> contrato isomórfico `github.com/tinywasm/router`. Autocontenido, en español.

---

## Reglas de Desarrollo

Las reglas del arnés viven en el **`AGENTS.md` de la raíz de esta librería** — léelo
antes de cualquier cambio. Este PLAN no las repite; describe solo el *cómo*.

Alcance (responsabilidad única): construir/servir el bundle de assets. El transporte
concreto no debe aparecer en su superficie.

---

## El contrato que consume (reexpresado para ser autocontenido)

```go
// package router (github.com/tinywasm/router)
type Context interface { Method() string; Path() string; Body() []byte
	GetHeader(k string) string; SetHeader(k, v string); WriteStatus(code int); Write([]byte) (int, error) }
type HandlerFunc func(Context)
type Router interface { Get(path string, h HandlerFunc); Handle(method, path string, h HandlerFunc); /* … */ }
```

---

## Estado de partida

```go
func (c *AssetMin) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(c.indexHtmlHandler.GetURLPath(), c.serveAsset(c.indexHtmlHandler))
	mux.HandleFunc(c.mainStyleCssHandler.GetURLPath(), c.serveAsset(c.mainStyleCssHandler))
	// … js, favicon, y assets adicionales
}
func (c *AssetMin) serveAsset(asset *asset) http.HandlerFunc { … }
```

---

## Cambios (antes → después)

| Antes (`net/http`) | Después (`router`) |
|---|---|
| `RegisterRoutes(mux *http.ServeMux)` | `RegisterRoutes(r router.Router)` |
| `mux.HandleFunc(path, c.serveAsset(h))` | `r.Get(path, c.serveAsset(h))` |
| `serveAsset(asset) http.HandlerFunc` | `serveAsset(asset) router.HandlerFunc` (escribe con `ctx.SetHeader`/`ctx.Write`) |

`assetmin` deja de nombrar `*http.ServeMux`/`http.HandlerFunc`/`http.ResponseWriter`
en su API pública.

---

## Pasos de implementación

1. Añadir dependencia `github.com/tinywasm/router` en `go.mod`.
2. Migrar `RegisterRoutes` a `router.Router`.
3. Migrar `serveAsset` a `router.HandlerFunc`, escribiendo cabeceras (`Content-Type`,
   `Content-Encoding`) y cuerpo por `router.Context`.

---

## Estrategia de pruebas y criterios de aceptación

- **Sin `net/http` en la superficie pública:** ninguna firma exportada nombra tipos
  de `net/http`. Verificable por búsqueda.
- **Cada asset se sirve por contrato:** un `router.Router` de mentira captura los
  paths registrados; un `router.Context` de mentira recibe el cuerpo/cabeceras del
  asset. `var _ router.HandlerFunc = c.serveAsset(h)` fija el contrato.
