# PLAN — Orquestador de `tinywasm/assetmin`

> `docs/PLAN.md` es el **orquestador** de esta librería: coordina los planes activos,
> no contiene sus pasos. Cada plan es autocontenido en su propio archivo.

---

## Reglas de Desarrollo

Las reglas del arnés viven en el **`AGENTS.md` de la raíz de esta librería** — léelo
antes de cualquier cambio.

---

## Planes coordinados

1. **Test de integración del hot reload SSR** → `docs/SSR_HOTRELOAD_TEST_PLAN.md`
   Cierra la brecha de tests que dejó pasar el bug de hot reload (ruteo
   devwatch→watcher→ReloadSSRModule). Depende del fix en `devwatch`. *(Pendiente: no
   existe `tests/ssr_hotreload_devwatch_test.go` ni `GetMainStyleCSS`.)*

2. **Refactor a `tinywasm/router`** → `docs/ROUTER_REFACTOR_PLAN.md`
   Migra `RegisterRoutes`/`serveAsset` de `*http.ServeMux`/`http.HandlerFunc` al
   contrato isomórfico `router.Router`/`router.HandlerFunc`.

---

## Orden de ejecución y por qué

Son independientes; el orden sugerido minimiza retrabajo:

1. Primero **(2) router**: cambia la firma de `RegisterRoutes` y `serveAsset`. Es un
   cambio de superficie acotado.
2. Luego **(1) test hot reload**: al escribir el test de integración, su verificación
   por HTTP (`GET /style.css`) ya usa el registro por `router.Router` migrado, no un
   `*http.ServeMux` que habría que reescribir después.

(Si el fix de `devwatch` tarda, (2) puede avanzar sin bloquearse en él.)

---

## Criterio de cierre

Cuando un plan quede **ejecutado**, se elimina su archivo y se retira de esta lista.
Este orquestador desaparece cuando ambos planes estén cerrados.
