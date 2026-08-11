# PLAN: el HTML de los módulos debe caer DENTRO de `#app`

## El problema

El `index.html` que servimos hoy termina así:

```html
<body>
<svg aria-hidden="true" style="display:none">…símbolos…</svg>
<div id="app"></div>
<script src="/script.js"></script>
</body>
```

`#app` sale **vacío**. El navegador pinta una página en blanco hasta que el WASM
descarga, compila y renderiza. Eso es el "carga dos veces" que se ve al arrancar.

Y si un módulo declara `RenderHTML()` hoy, su HTML cae en `contentMiddle`, que se
escribe **antes** de `<div id="app">` — o sea fuera. Cuando el WASM renderiza,
el contenido del módulo queda visible *además* del renderizado: duplicado en
pantalla, no reemplazado.

## El cambio

Mover la frontera de `contentMiddle` para que quede dentro de `#app`:

```
contentOpen     <!doctype …><body>            +  <div id="app">   ← se agrega aquí
dynamicContent  <svg>símbolos</svg>                                ← SIN CAMBIO, queda fuera
contentMiddle   RenderHTML() de los módulos                        ← ahora dentro de #app
contentClose    </div> <script> </body></html>                     ← se agrega </div>
```

Son dos literales en `NewHtmlHandler` (`html.go`). Nada más.

## Por qué esto alcanza (y por qué NO hace falta un motor de hidratación)

`dom.Render` hace `parent.Set("innerHTML", html)` — [dom_frontend.go:201]. Borra
y reemplaza; no hidrata, no compara, no preserva.

Eso, que suena a limitación, es justo lo que hace el arreglo trivial: si el
servidor deja dentro de `#app` **el mismo markup** que el WASM va a generar, el
reemplazo es visualmente un no-op. Primera pintura instantánea y con estilos; el
swap no se ve. No hay que escribir reconciliación de DOM ni marcar nodos.

La única regla que el consumidor debe cumplir: *el HTML del servidor tiene que
ser el mismo que produce `Render()`*. Se garantiza gratis si `RenderHTML()` se
implementa como `Render().String()` en vez de como una segunda plantilla.

## Por qué es seguro moverlo

- Hoy **ningún módulo** de mjosefa-cms declara `RenderHTML()`: `contentMiddle` de
  index.html está vacío. Verificado sirviendo `/`.
- Los símbolos SVG **no** vienen de `RenderHTML()`: vienen de
  `AddDynamicContent` ([assetmin.go:121]), que se escribe antes de
  `contentMiddle`. Quedan fuera de `#app`, que es donde tienen que estar — si
  entraran, el primer `Render()` los borraría y todos los íconos desaparecerían.

## Bug adyacente que se arregla de paso

`routeAssets` manda el módulo raíz al slot `"close"`. Para el handler de HTML,
`contentClose` ya arranca con `<div id="app"></div><script></body></html>`, así
que el HTML del módulo raíz se agrega **después de `</html>`**. Nunca se notó
porque ningún proyecto raíz declaró `RenderHTML()` todavía — y el proyecto raíz
es justo quien más lo necesita.

Arreglo: el slot `"close"` es una decisión sobre CSS (orden de cascada), no sobre
HTML. Rutear el HTML del raíz a `"middle"` como el de cualquier otro módulo.

## Invariante de seguridad (documentar + testear)

`index.html` se genera **una sola vez, en build**, y se sirve idéntico a todo el
mundo. No hay render por request. Entonces:

> Lo que entra por `RenderHTML()` es público y cacheable por definición. Nunca
> puede contener datos de un usuario, ni un token, ni nada que dependa de sesión.

Esto no es una limitación a documentar y olvidar: es una **propiedad de
seguridad**, y es la razón por la que este enfoque es seguro detrás de un CDN /
túnel de Cloudflare. Un CMS que sirviera el shell por request tendría que
razonar sobre caché envenenada; este no puede tener ese bug porque no hay
request involucrado.

## Pasos

1. `html.go` — `NewHtmlHandler`: cerrar `contentOpen` con `<div id="app">` y
   abrir `contentClose` con `</div>`.
2. `ssr_loader.go` — `routeAssets`: no mandar HTML al slot `close`; el `close`
   sigue aplicando a CSS/JS.
3. Test en `tests/`: registrar un módulo con `RenderHTML()` y afirmar que su
   markup queda **entre** `<div id="app">` y `</div>`, y que los símbolos SVG
   quedan **antes** de `<div id="app">`.
4. Test: el `index.html` generado no cambia cuando cambia el estado de sesión —
   no hay estado de sesión que pueda alcanzarlo (guarda del invariante de
   arriba).

## Verificación

`gotest` en assetmin. Después, en mjosefa-cms, `curl -s localhost:8080/` y
confirmar que `#app` ya no está vacío.
