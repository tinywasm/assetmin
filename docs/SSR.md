# SSR Module Asset Extraction & Loading

`assetmin` automatically discovers Go modules in the project tree and extracts their assets — CSS, JS, HTML, SVG icons — routing them into the rendered `<head>`. Modules ship their own assets without ever importing `assetmin`; the contract is purely the function names and conventions in `ssr.go`.

## Asset Extraction Mechanism

Assets are extracted via **compile-and-invoke**: `assetmin` generates a single combined `main.go` that imports all discovered components, instantiates each via `SSRInstance()`, and invokes their asset methods (`RenderCSS()`, `RenderHTML()`, etc.), collecting the results into JSON. This replaces earlier AST-based parsing, which could only handle string literals and simple concatenation.

The extraction happens once per unique set of component file hashes (cached), then the aggregated output is parsed into per-component `SSRAssets`.

## Asset Declaration (Contract)

A module exposes its assets by adding an `ssr.go` file in its package root:

```go
//go:build !wasm

package mypkg

import _ "embed"

//go:embed theme.css
var rootCSS string

// Default `:root { … }` theme tokens. Routed to the `open` slot
// (rendered first in <head>). At most one module wins this slot —
// see "Single-override rule" below.
func RootCSS() string { return rootCSS }

// Component-level CSS. Routed to the `middle` slot for dependencies
// or to the `close` slot when this is the root project.
func RenderCSS() string { return `.my-widget { … }` }

// Component-level JS. Same slot routing as RenderCSS.
func RenderJS() string { return `console.log("ready")` }

// HTML fragment for SSR.
func RenderHTML() string { return `<div class="my-widget"></div>` }

// SVG icons collected into the global sprite sheet.
func IconSvg() map[string]string {
    return map[string]string{"icon-id": `<svg>…</svg>`}
}
```

### Function-to-slot map

| Function | `SSRAssets` field | Destination slot | Notes |
|---|---|---|---|
| `RootCSS()` | `RootCSS` | `open` | Single-override (see below) |
| `RenderCSS()` | `CSS` | `middle` (deps) / `close` (root project) | |
| `RenderJS()` | `JS` | same as `RenderCSS` | |
| `RenderHTML()` | `HTML` | same as `RenderCSS` | Only if publicly readable |
| `IconSvg()` | `Icons` | sprite registry (no slot) | Keys are icon IDs |

### The `SSRInstance()` convention

To enable compile-and-invoke extraction, each module's `ssr.go` must expose a function:

```go
// SSRInstance returns a zero-value instance implementing the SSR interfaces.
// This is called by the generated asset extractor to collect asset values
// without requiring reflection or complex setup.
func SSRInstance() *MyComponent {
    return &MyComponent{}
}
```

Replace `MyComponent` with your actual struct (or interface type implementing `RenderCSS()`, `RenderHTML()`, etc.). The instance does not need to be initialized with application state — it only needs to be capable of calling the asset methods.

**Example:**

```go
//go:build !wasm

package button

type Button struct{}

func (b *Button) RenderCSS() interface{ String() string } {
    return css.New(
        css.Rule(".button", css.Decl("padding", "1rem")),
    )
}

func (b *Button) RenderHTML() string { return `<button></button>` }
func (b *Button) RenderJS() string   { return "" }
func (b *Button) IconSvg() map[string]string { return nil }

func SSRInstance() *Button {
    return &Button{}
}
```

### Supported asset method returns

Asset methods may now return dynamic values — function calls, conditionals, Go DSL helpers, etc. — because they are evaluated by actual Go code execution, not static AST parsing. For example:

- `RenderCSS()` can return typed CSS objects with `.String()` methods (from `tinywasm/css` or similar)
- `RenderHTML()` and `RenderJS()` remain strings but can be computed
- `IconSvg()` returns a computed map

The compile-and-invoke mechanism removes the limitation of static evaluation. If you were previously returning empty strings due to function calls or dynamic logic, you can now express those values directly in the asset methods.

## Single-override rule for `RootCSS()`

`:root { … }` is a global namespace. To prevent silent theme corruption from transitive dependencies, only one `RootCSS()` reaches the bundle:

1. If the **root project** declares `RootCSS()` → it wins, fully replacing any framework tokens.
2. Otherwise, if **`tinywasm/css`** declares `RootCSS()` → it wins (the default fallback theme).
3. If a **third-party module** (neither root nor css) declares `RootCSS()` → ignored, with a warning logged via `Config.Logger`.

The fallback module path is the unexported constant `cssModulePath = "tinywasm/css"` in `ssr_loader.go`.

`RenderCSS()`, `RenderJS()`, `RenderHTML()`, and `IconSvg()` from third-party modules are NOT subject to single-override — they accumulate normally in the `middle` slot.

## Slot ordering in `<head>`

```
<head>
  …
  [open]    — RootCSS() single winner (app root or framework fallback)
  [middle]  — RenderCSS() / RenderJS() from imported dependencies
  [close]   — RenderCSS() / RenderJS() from the root project
  …
</head>
```

CSS cascade order: dependencies cannot override the root project; the root project cannot override `:root` if it didn't declare its own `RootCSS()` (it already won the `open` slot if it did).

## Automatic discovery

When `Config.RootDir` points at the project root (where `go.mod` lives), `assetmin` runs `go list -m -json all` to enumerate every module the project transitively imports, then parses each candidate `ssr.go`.

```go
am := assetmin.NewAssetMin(&assetmin.Config{
    RootDir: ".",
    // …
})
am.LoadSSRModules() // async; returns immediately
am.WaitForSSRLoad(2 * time.Second) // optional; mostly for tests
```

`LoadSSRModules()` is non-blocking; it dispatches a goroutine. `ScheduleSSRLoad()` is the lower-level entry point if you want to call it from a custom lifecycle.

## Hot reload

For local modules (e.g., via `replace` in `go.mod`), the orchestrator (`tinywasm/app`) calls:

```go
am.ReloadSSRModule(moduleDir)
```

The loader re-extracts the assets, re-evaluates the `RootCSS()` single-override (so an app that just gained or lost its own `RootCSS()` flips back and forth between its theme and framework's), and replaces in-memory bundle entries without duplication.

## Manual registration

If you have live struct instances implementing the SSR interfaces, register them directly:

```go
am.RegisterComponents(myComponent1, myComponent2)
```

Components implementing `RootCSS() string` route to the `open` slot under the same single-override rule (runtime registration is treated as coming from the app, so it replaces the framework theme). See [Component Registration](COMPONENT_REGISTRATION.md) for the full interface list.

## API summary

| Method | Purpose |
|---|---|
| `LoadSSRModules()` | Scan all modules and load assets asynchronously |
| `ScheduleSSRLoad()` | Lower-level async dispatch |
| `ReloadSSRModule(dir string) error` | Re-extract one module (for hot reload) |
| `WaitForSSRLoad(timeout)` | Block until loading finishes (test helper) |
| `RegisterComponents(providers ...any)` | Register live struct instances as asset providers |
| `UpdateSSRModule(name, css, js, html, icons)` | Manually inject content into the `middle` slot |
| `UpdateSSRModuleInSlot(name, css, js, html, icons, slot)` | Manually inject into a specific slot (`open`/`middle`/`close`) |
