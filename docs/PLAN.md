---
message: "feat: serve assets with router.PublicAsset — public by construction, not a forgettable marker"
---

> Este plan se despacha vía el flujo CodeJob. Ver skill: agents-workflow.
> Orquestado por `tinywasm/docs/PUBLIC_ASSETS_MASTER_PLAN.md` — **Fase C1**.
> Autocontenido: el agente que lo ejecuta no tiene contexto previo.
>
> **COMPUERTA:** requiere `github.com/tinywasm/router` **v0.1.7+** (ya publicado), que es
> quien declara `PublicAsset`. Sube `go.mod` primero. Si `router.Router` no tiene el método
> `PublicAsset`, PARA y reporta.
> No dependes de `server`: este repo solo consume la **interfaz**. Corre en paralelo con él.

# PLAN — `assetmin`: los assets se registran con `PublicAsset`

**En una frase:** `RegisterRoutes` deja de usar `r.Get(...)` (+ el marcador `.Public()`) y
pasa al método tipado `r.PublicAsset(...)`.

Es un cambio pequeño y mecánico. Lo que importa es **por qué**, para que no se deshaga.

---

## El problema (contexto, ya diagnosticado — no lo reabras)

`tinywasm/router` es **privado por defecto**: una ruta que no declara `Public()` ni
`Requires()` deniega a quien no tiene identidad. Correcto, y no se toca.

`assetmin/http.go` registraba sus assets con `r.Get(...)` a secas. Y un navegador que pide
`index.html`, la hoja de estilos, el bundle o el favicon **siempre es anónimo** — todavía no
hay sesión. Resultado: **403 en todo**. El build terminaba en verde y la página que veía el
usuario decía `Forbidden`. Ni un proyecto renderizaba.

**Por qué nadie lo detectó:** el mock del router copiaba el `RouteInfo` **por valor** al
registrar, así que un `.Public()` encadenado después mutaba otra copia. Para el mock **toda
ruta era privada, siempre**, y un test que afirmara lo contrario habría fallado incluso con
el código bien. Corregido en `router` v0.1.6; por eso ahora sí se puede afirmar.

## La decisión (no la reabras)

Se descartó **añadir `.Public()` a cada `r.Get`**. Es el *"no olvides llamar a X"* que el
arnés de construcción clasifica como hueco: el próximo autor que añada una ruta de assets lo
volverá a olvidar, y el olvido **no falla en compilación ni hace ruido**.

`router` v0.1.7 lo cierra con tipos:

```go
// UN archivo, UNA ruta. Público por construcción. NO devuelve Route: no hay permiso
// que colgarle → no se puede olvidar abrirlo, ni cerrarlo por error.
PublicAsset(path string, h HandlerFunc)
```

> ⚠️ **Puede que encuentres `.Public()` ya añadido** en `http.go` (parche provisional).
> **Quítalo**: se sustituye por `PublicAsset`, no se acumula con él.

---

## Paso 1 — `go.mod`

Sube `github.com/tinywasm/router` a **v0.1.7+**. Verifica que `PublicAsset` existe antes de seguir.

## Paso 2 — `http.go`: `RegisterRoutes`

Las cinco registraciones pasan a `PublicAsset`. Todas sirven archivos, todas son públicas
por construcción:

```go
func (c *AssetMin) RegisterRoutes(r router.Router) {
	r.PublicAsset(c.indexHtmlHandler.GetURLPath(), c.serveAsset(c.indexHtmlHandler))
	r.PublicAsset(c.mainStyleCssHandler.GetURLPath(), c.serveAsset(c.mainStyleCssHandler))
	r.PublicAsset(c.mainJsHandler.GetURLPath(), c.serveAsset(c.mainJsHandler))
	r.PublicAsset(c.faviconSvgHandler.GetURLPath(), c.serveAsset(c.faviconSvgHandler))

	// Standalone JS assets
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, h := range c.standaloneJS {
		r.PublicAsset(h.GetURLPath(), c.serveAsset(h))
	}
}
```

`serveAsset` **no cambia**. `PublicAsset` no devuelve `Route`, así que no hay nada que
encadenar detrás.

## Paso 3 — el guard

Ya existe `tests/http_public_test.go` con `TestAssetRoutesArePublic`, que recorre
`Routes()` y falla si alguna ruta de asset no es pública. **Debe seguir en verde.** Si no
existe en tu árbol, escríbelo:

```go
for _, route := range newTestRouter(am).Routes() {
    if !route.Public {
        t.Errorf("ruta de asset %q privada → el navegador recibe 403 y la página sale en blanco", route.Path)
    }
}
```

Comprueba que **de verdad caza el bug**: quita temporalmente un `PublicAsset` (déjalo como
`r.Get`), confirma que el test se pone rojo señalando esa ruta, y restáuralo.

---

## ⚠️ Anti-footguns (NO hagas esto)

- **NO añadas `.Public()`.** Si lo estás buscando, quieres `PublicAsset`.
- **NO toques la verja del router** ni intentes "arreglar" el 403 desde el servidor.
- **NO cambies `serveAsset`**, ni las cabeceras de cache, ni el minificado.
- **NO toques `client` ni `server`**: migran con sus propios planes.
- Nunca ejecutes `gopush` ni `codejob`.

## Criterios de aceptación

```bash
grep -rn '\.Public()' .        # → vacío
grep -rn 'r\.Get(' http.go     # → vacío
gotest                          # verde, con TestAssetRoutesArePublic incluido
```

## Al cerrar

Anota en `AGENTS.md`: *"los assets se registran con `router.PublicAsset` — son públicos por
construcción; nunca `Get(...).Public()`"*. Luego **borra este `docs/PLAN.md`**.
